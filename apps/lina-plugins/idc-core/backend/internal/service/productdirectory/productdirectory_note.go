// This file saves exact-customer service notes with an independent revision and
// transactional digest-only audit, without changing fulfillment or billing.
package productdirectory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/audit"
)

const (
	noteOperation = "customer_service.note"
	noteTarget    = "service_entitlement"
)

func (s *serviceImpl) note(ctx context.Context, customer, entitlement string, lock bool) (*CustomerNote, error) {
	model := s.database.Model(do.ServiceEntitlement{}).Ctx(ctx).Where(do.ServiceEntitlement{TenantId: 0, CustomerNo: customer, EntitlementNo: entitlement}).Fields("entitlement_no,support_association_reference,customer_remark AS remark,customer_remark_revision AS revision")
	if lock {
		model = model.LockUpdate()
	}
	var result *CustomerNote
	if err := model.Scan(&result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return result, nil
}

// GetNote validates the actor before reading an exact customer-owned entitlement.
func (s *serviceImpl) GetNote(ctx context.Context, entitlement string) (result *CustomerNote, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	if !referencePattern.MatchString(entitlement) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	customer, err := s.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	if customer == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.note(ctx, customer, entitlement, false)
}

// SaveNote checks the independent revision even when the submitted text matches.
func (s *serviceImpl) SaveNote(ctx context.Context, input CustomerNote) (result *CustomerNote, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	input.Remark = strings.TrimSpace(strings.ReplaceAll(input.Remark, "\r\n", "\n"))
	if !referencePattern.MatchString(input.EntitlementNo) || input.Revision < 1 || !utf8.ValidString(input.Remark) || utf8.RuneCountInString(input.Remark) > 1000 || strings.IndexFunc(input.Remark, func(value rune) bool { return unicode.IsControl(value) && value != '\n' }) >= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	customer, err := s.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	if customer == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	err = s.database.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		before, readErr := s.note(tx, customer, input.EntitlementNo, true)
		if readErr != nil {
			return readErr
		}
		if before.Revision != input.Revision {
			return bizerr.NewCode(CodeNoteConflict)
		}
		if before.Remark == input.Remark {
			result = before
			return nil
		}
		result = &CustomerNote{EntitlementNo: input.EntitlementNo, Remark: input.Remark, Revision: before.Revision + 1}
		write, writeErr := s.database.Model(do.ServiceEntitlement{}).Ctx(tx).Where(do.ServiceEntitlement{TenantId: 0, CustomerNo: customer, EntitlementNo: input.EntitlementNo, CustomerRemarkRevision: input.Revision}).Data(do.ServiceEntitlement{CustomerRemark: result.Remark, CustomerRemarkRevision: result.Revision}).Update()
		if writeErr != nil {
			return writeErr
		}
		affected, writeErr := write.RowsAffected()
		if writeErr != nil {
			return writeErr
		}
		if affected != 1 {
			return bizerr.NewCode(CodeNoteConflict)
		}
		return s.audit.AppendBusinessTransition(tx, idccap.BusinessAuditTransition{IdempotencyScope: noteOperation, IdempotencyKey: input.EntitlementNo + ":" + strconv.FormatInt(result.Revision, 10), OperationCode: noteOperation, TargetType: noteTarget, TargetNo: input.EntitlementNo, CustomerNo: customer, ResultCode: string(audit.ResultSucceeded), RiskCode: string(audit.RiskLow), BeforeSnapshot: noteAudit(before), AfterSnapshot: noteAudit(result)})
	})
	return result, err
}

func noteAudit(note *CustomerNote) any {
	digest := sha256.Sum256([]byte(note.Remark))
	return struct {
		Revision     int64  `json:"revision"`
		RemarkDigest string `json:"remarkDigest"`
		Empty        bool   `json:"empty"`
	}{note.Revision, hex.EncodeToString(digest[:]), note.Remark == ""}
}
