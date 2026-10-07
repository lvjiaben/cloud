// This file implements bounded, permission-checked, tenant-scoped audit search.

package audit

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Search returns one indexed keyset page only after fail-closed management authorization.
func (s *serviceImpl) Search(ctx context.Context, input SearchInput) (*Page, error) {
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.requireReadAuthority(ctx, actor); err != nil {
		return nil, err
	}
	input, err = s.normalizeSearchInput(input)
	if err != nil {
		return nil, err
	}
	columns := dao.BusinessAuditEvent.Columns()
	model := dao.BusinessAuditEvent.Ctx(ctx).Where(do.BusinessAuditEvent{
		TenantId: idccap.DefaultTenantID,
	})
	if input.ActorUserID > 0 {
		model = model.Where(columns.ActorUserId, input.ActorUserID)
	}
	if input.ActingUserID > 0 {
		model = model.Where(columns.ActingUserId, input.ActingUserID)
	}
	if input.CustomerNo != "" {
		auditTable := dao.BusinessAuditEvent.Table()
		linkTable := dao.BusinessAuditCustomerLink.Table()
		model = model.Where(
			"("+columns.CustomerNo+" = ?) OR EXISTS (SELECT 1 FROM "+linkTable+
				" AS audit_customer_link WHERE audit_customer_link.tenant_id = ?"+
				" AND audit_customer_link.audit_no = "+auditTable+".audit_no"+
				" AND audit_customer_link.customer_no = ?)",
			input.CustomerNo,
			idccap.DefaultTenantID,
			input.CustomerNo,
		)
	}
	if input.OperationCode != "" {
		model = model.Where(columns.OperationCode, input.OperationCode)
	}
	if input.TargetType != "" {
		model = model.Where(columns.TargetType, input.TargetType)
	}
	if input.TargetNo != "" {
		model = model.Where(columns.TargetNo, input.TargetNo)
	}
	if input.Result != "" {
		model = model.Where(columns.ResultCode, string(input.Result))
	}
	if input.Risk != "" {
		model = model.Where(columns.RiskCode, string(input.Risk))
	}
	if input.CorrelationNo != "" {
		model = model.Where(columns.CorrelationNo, input.CorrelationNo)
	}
	if input.OccurredFrom != nil {
		model = model.WhereGTE(columns.OccurredAt, *input.OccurredFrom)
	}
	if input.OccurredUntil != nil {
		model = model.WhereLT(columns.OccurredAt, *input.OccurredUntil)
	}
	if input.Cursor != nil {
		model = model.Where(
			"("+columns.OccurredAt+" < ?) OR ("+columns.OccurredAt+" = ? AND "+columns.Id+" < ?)",
			input.Cursor.OccurredAt,
			input.Cursor.OccurredAt,
			input.Cursor.InternalID,
		)
	}
	var rows []*entity.BusinessAuditEvent
	if err = model.
		Fields(
			columns.Id,
			columns.TenantId,
			columns.AuditNo,
			columns.OccurredAt,
			columns.ActorType,
			columns.ActorUserId,
			columns.ActingUserId,
			columns.ActorAudienceCode,
			columns.ActorCustomerNo,
			columns.CustomerNo,
			columns.OperationCode,
			columns.TargetType,
			columns.TargetNo,
			columns.ResultCode,
			columns.ReasonCode,
			columns.RiskCode,
			columns.BeforeSnapshot,
			columns.AfterSnapshot,
			columns.ContextSnapshot,
			columns.RequestId,
			columns.TraceId,
			columns.CorrelationNo,
			columns.IpDigest,
			columns.UserAgentDigest,
		).
		OrderDesc(columns.OccurredAt).
		OrderDesc(columns.Id).
		Limit(input.Limit + 1).
		Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "search audit timeline"), CodeUnavailable)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	reservations, err := loadAuditReservations(ctx, rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	customerLinks, err := loadAuditCustomerLinks(ctx, rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	page := &Page{Events: make([]Event, 0, len(rows))}
	budgetLimited := false
	eventBytes := 0
	for index, row := range rows {
		reservation := reservations[row.AuditNo]
		event, projectErr := projectEventWithCustomerLinks(
			row,
			reservation,
			s,
			customerLinks[row.AuditNo],
		)
		if projectErr != nil {
			return nil, bizerr.WrapCode(projectErr, CodeUnavailable)
		}
		encodedEvent, encodeErr := json.Marshal(event)
		if encodeErr != nil {
			return nil, bizerr.WrapCode(
				gerror.Wrap(encodeErr, "measure audit search event"),
				CodeUnavailable,
			)
		}
		candidateHasMore := hasMore || index+1 < len(rows)
		var candidateNext *Cursor
		if candidateHasMore {
			cursor, cursorErr := cursorFromAuditRow(row)
			if cursorErr != nil {
				return nil, bizerr.WrapCode(cursorErr, CodeUnavailable)
			}
			candidateNext = cursor
		}
		candidateBytes, encodeErr := serializedPageSize(
			eventBytes+len(encodedEvent),
			len(page.Events)+1,
			candidateNext,
		)
		if encodeErr != nil {
			return nil, bizerr.WrapCode(
				gerror.Wrap(encodeErr, "measure audit search response"),
				CodeUnavailable,
			)
		}
		if candidateBytes > s.config.MaximumResponseBytes {
			budgetLimited = true
			break
		}
		page.Events = append(page.Events, *event)
		page.Next = candidateNext
		eventBytes += len(encodedEvent)
	}
	if budgetLimited && len(page.Events) == 0 {
		return nil, bizerr.WrapCode(
			gerror.New("audit response budget cannot contain one persisted event"),
			CodeUnavailable,
		)
	}
	if budgetLimited {
		last := rows[len(page.Events)-1]
		page.Next, err = cursorFromAuditRow(last)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
	}
	encoded, err := json.Marshal(page)
	if err != nil || len(encoded) > s.config.MaximumResponseBytes {
		if err == nil {
			err = gerror.New("audit search response exceeds configured byte budget")
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return page, nil
}

// serializedPageSize measures a candidate without repeatedly serializing all
// previously accepted snapshots. An empty event array contributes the same
// brackets as a populated array, so only event bytes and commas are added.
func serializedPageSize(eventBytes int, eventCount int, next *Cursor) (int, error) {
	if eventBytes < 0 || eventCount < 0 || (eventCount == 0 && eventBytes != 0) {
		return 0, gerror.New("audit page size input is invalid")
	}
	skeleton, err := json.Marshal(&Page{Events: []Event{}, Next: next})
	if err != nil {
		return 0, err
	}
	commaBytes := max(eventCount-1, 0)
	return len(skeleton) + eventBytes + commaBytes, nil
}

// loadAuditReservations batch-loads the integrity owner projections for one
// page in a fixed second query, avoiding per-row lookups.
func loadAuditReservations(
	ctx context.Context,
	rows []*entity.BusinessAuditEvent,
) (map[string]*entity.BusinessNumber, error) {
	result := make(map[string]*entity.BusinessNumber, len(rows))
	if len(rows) == 0 {
		return result, nil
	}
	auditNumbers := make([]string, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.AuditNo == "" {
			return nil, gerror.New("audit search projection is missing a business number")
		}
		auditNumbers = append(auditNumbers, row.AuditNo)
	}
	columns := dao.BusinessNumber.Columns()
	var reservations []*entity.BusinessNumber
	if err := dao.BusinessNumber.Ctx(ctx).
		Fields(
			columns.TenantId,
			columns.NumberType,
			columns.BusinessNo,
			columns.ReservationScopeDigest,
			columns.IdempotencyKeyDigest,
			columns.OwnerType,
			columns.OwnerInternalId,
			columns.AssignedAt,
		).
		Where(do.BusinessNumber{
			TenantId:   idccap.DefaultTenantID,
			NumberType: businessNumberTypeAudit,
		}).
		WhereIn(columns.BusinessNo, auditNumbers).
		Scan(&reservations); err != nil {
		return nil, gerror.Wrap(err, "batch load audit number reservations")
	}
	for _, reservation := range reservations {
		if reservation == nil || reservation.BusinessNo == "" {
			return nil, gerror.New("audit number projection is invalid")
		}
		if _, duplicate := result[reservation.BusinessNo]; duplicate {
			return nil, gerror.New("duplicate audit number projection")
		}
		result[reservation.BusinessNo] = reservation
	}
	if len(result) != len(auditNumbers) {
		return nil, gerror.New("audit number projection is incomplete")
	}
	return result, nil
}

// cursorFromAuditRow returns the keyset boundary for one authenticated page row.
func cursorFromAuditRow(row *entity.BusinessAuditEvent) (*Cursor, error) {
	if row == nil || row.OccurredAt == nil || row.Id <= 0 {
		return nil, gerror.New("audit cursor row is missing persisted ordering fields")
	}
	return &Cursor{
		OccurredAt: databaseTime(*row.OccurredAt),
		InternalID: row.Id,
	}, nil
}
