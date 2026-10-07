// This file implements transaction-required, idempotent, append-only audit persistence.

package audit

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	entryKindTransition      = "transition"
	entryKindObservation     = "observation"
	businessNumberTypeAudit  = "business_audit_event"
	businessNumberOwnerAudit = "business_audit_event"
	auditNumberPrefix        = "AUD_"
	maximumNumberAttempts    = 4
	auditUnsealedContext     = "{}"
)

type normalizedEntry struct {
	kind                  string
	fields                Fields
	actor                 TrustedActor
	authority             authorizationEvidence
	beforeOriginal        json.RawMessage
	afterOriginal         json.RawMessage
	callerContextOriginal json.RawMessage
	beforeSnapshot        json.RawMessage
	afterSnapshot         json.RawMessage
	callerContext         map[string]any
	request               normalizedRequestEvidence
	reservationCandidates []reservationCandidate
	affectedCustomerNos   []string
}

type reservationCandidate struct {
	reservationScopeDigest string
	idempotencyKeyDigest   string
}

type evidenceFingerprint struct {
	Kind                   string          `json:"kind"`
	ActorType              ActorType       `json:"actor_type"`
	ActorReferenceHMAC     string          `json:"actor_reference_hmac"`
	ActorUserID            int             `json:"actor_user_id"`
	ActingUserID           int             `json:"acting_user_id"`
	ActorAudienceCode      string          `json:"actor_audience_code"`
	ActorCustomerNo        string          `json:"actor_customer_no"`
	CustomerNo             string          `json:"customer_no"`
	OperationCode          string          `json:"operation_code"`
	TargetType             string          `json:"target_type"`
	TargetNo               string          `json:"target_no"`
	ResultCode             ResultCode      `json:"result_code"`
	ReasonCode             string          `json:"reason_code"`
	RiskCode               RiskCode        `json:"risk_code"`
	RequiredPermission     string          `json:"required_permission"`
	RequiredGranted        bool            `json:"required_granted"`
	WildcardGranted        bool            `json:"wildcard_granted"`
	PlatformAdmin          bool            `json:"platform_admin"`
	PermissionSnapshotHMAC string          `json:"permission_snapshot_hmac"`
	DataScope              int             `json:"data_scope"`
	Delegated              bool            `json:"delegated"`
	PlatformBypass         bool            `json:"platform_bypass"`
	BeforeSnapshot         json.RawMessage `json:"before_snapshot"`
	AfterSnapshot          json.RawMessage `json:"after_snapshot"`
	CallerContext          json.RawMessage `json:"caller_context"`
	RequestID              string          `json:"request_id"`
	TraceID                string          `json:"trace_id"`
	CorrelationNo          string          `json:"correlation_no"`
	IPDigest               string          `json:"ip_digest"`
	UserAgentDigest        string          `json:"user_agent_digest"`
}

type reservationScopeFingerprint struct {
	Version            int       `json:"version"`
	CallerScope        string    `json:"caller_scope"`
	ActorType          ActorType `json:"actor_type"`
	ActorReferenceHMAC string    `json:"actor_reference_hmac"`
	ActorUserID        int       `json:"actor_user_id"`
	ActingUserID       int       `json:"acting_user_id"`
	ActorAudienceCode  string    `json:"actor_audience_code"`
	ActorCustomerNo    string    `json:"actor_customer_no"`
	CustomerNo         string    `json:"customer_no"`
}

// AppendTransition appends inside the caller's existing authoritative transaction.
func (s *serviceImpl) AppendTransition(
	ctx context.Context,
	input TransitionInput,
) (*Event, error) {
	if gdb.TXFromCtx(ctx, dao.BusinessAuditEvent.Group()) == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := s.normalizeEntry(
		ctx,
		entryKindTransition,
		input.Fields,
		input.BeforeSnapshot,
		input.AfterSnapshot,
	)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	event, err := s.appendInTransaction(ctx, normalized)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return event, nil
}

// RecordObservation owns one short transaction and never calls an ambient
// non-transactional insert path.
func (s *serviceImpl) RecordObservation(
	ctx context.Context,
	input ObservationInput,
) (*Event, error) {
	if gdb.TXFromCtx(ctx, dao.BusinessAuditEvent.Group()) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := s.normalizeEntry(
		ctx,
		entryKindObservation,
		input.Fields,
		map[string]any{},
		input.Snapshot,
	)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var event *Event
	err = dao.BusinessAuditEvent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var appendErr error
		event, appendErr = s.appendInTransaction(ctx, normalized)
		return appendErr
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return event, nil
}

// normalizeEntry resolves actor authority, redacts snapshots, and builds one
// complete keyed semantic fingerprint before persistence.
func (s *serviceImpl) normalizeEntry(
	ctx context.Context,
	kind string,
	fields Fields,
	beforeValue any,
	afterValue any,
) (normalizedEntry, error) {
	fields, err := normalizeFields(fields)
	if err != nil {
		return normalizedEntry{}, err
	}
	actor, err := s.resolveActor(ctx)
	if err != nil {
		return normalizedEntry{}, err
	}
	authority, err := s.validateWriteAuthority(ctx, actor, &fields)
	if err != nil {
		return normalizedEntry{}, err
	}
	if err = validateNormalizedFields(fields); err != nil {
		return normalizedEntry{}, err
	}
	request, err := s.normalizeRequestEvidence(fields.Request)
	if err != nil {
		return normalizedEntry{}, err
	}
	beforeObject, err := s.normalizeObject(beforeValue)
	if err != nil {
		return normalizedEntry{}, err
	}
	afterObject, err := s.normalizeObject(afterValue)
	if err != nil {
		return normalizedEntry{}, err
	}
	callerContext, err := s.normalizeObject(fields.ContextSnapshot)
	if err != nil {
		return normalizedEntry{}, err
	}
	if _, exists := callerContext.redacted[batchCustomerCountContextKey]; exists {
		return normalizedEntry{}, bizerr.NewCode(CodeInvalid)
	}
	if _, exists := callerContext.redacted[batchCustomerDigestContextKey]; exists {
		return normalizedEntry{}, bizerr.NewCode(CodeInvalid)
	}
	beforeSnapshot, err := s.versionedSnapshot(beforeObject.redacted)
	if err != nil {
		return normalizedEntry{}, err
	}
	afterSnapshot, err := s.versionedSnapshot(afterObject.redacted)
	if err != nil {
		return normalizedEntry{}, err
	}
	reservationCandidates, err := s.auditReservationCandidates(fields, actor)
	if err != nil {
		return normalizedEntry{}, err
	}
	return normalizedEntry{
		kind:                  kind,
		fields:                fields,
		actor:                 actor,
		authority:             authority,
		beforeOriginal:        beforeObject.original,
		afterOriginal:         afterObject.original,
		callerContextOriginal: callerContext.original,
		beforeSnapshot:        beforeSnapshot,
		afterSnapshot:         afterSnapshot,
		callerContext:         callerContext.redacted,
		request:               request,
		reservationCandidates: reservationCandidates,
	}, nil
}

// auditReservationCandidates derives one stable, keyed reservation locator that
// is independent of the rotatable replay-evidence keyring. The plaintext caller
// scope, actor reference, and idempotency key are never persisted.
func (s *serviceImpl) auditReservationCandidates(
	fields Fields,
	actor TrustedActor,
) ([]reservationCandidate, error) {
	lookupKey := s.config.ReservationLookupKey.Secret
	actorReferenceHMAC := ""
	var err error
	if actor.Reference != "" {
		actorReferenceHMAC, err = keyedDigestWithKey(
			lookupKey,
			"audit-reservation-actor-reference-v2",
			actor.Reference,
		)
		if err != nil {
			return nil, err
		}
	}
	document := reservationScopeFingerprint{
		Version:            integritySchemaVersion,
		CallerScope:        fields.IdempotencyScope,
		ActorType:          actor.Type,
		ActorReferenceHMAC: actorReferenceHMAC,
		ActorUserID:        actor.UserID,
		ActingUserID:       actor.ActingUserID,
		ActorAudienceCode:  string(actor.AudienceCode),
		ActorCustomerNo:    actor.CustomerNo,
		CustomerNo:         fields.CustomerNo,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, gerror.Wrap(err, "encode audit reservation scope")
	}
	scopeDigest, err := keyedDigestWithKey(
		lookupKey,
		"audit-reservation-scope-v2",
		string(encoded),
	)
	if err != nil {
		return nil, err
	}
	idempotencyDigest, err := keyedDigestWithKey(
		lookupKey,
		"audit-idempotency-key-v2",
		fields.IdempotencyKey,
	)
	if err != nil {
		return nil, err
	}
	return []reservationCandidate{{
		reservationScopeDigest: scopeDigest,
		idempotencyKeyDigest:   idempotencyDigest,
	}}, nil
}

// securityForEntry derives persistable security evidence only from trusted
// actor and host authorization state using the row's replay-key version.
func (s *serviceImpl) securityForEntry(
	input normalizedEntry,
	replayKey []byte,
) (securitySnapshot, error) {
	permissionSnapshotJSON, err := json.Marshal(input.authority)
	if err != nil {
		return securitySnapshot{}, gerror.Wrap(err, "encode audit permission snapshot")
	}
	permissionSnapshotHMAC, err := keyedDigestWithKey(
		replayKey,
		"audit-permissions-v1",
		string(permissionSnapshotJSON),
	)
	if err != nil {
		return securitySnapshot{}, err
	}
	actorReferenceHMAC := ""
	if input.actor.Reference != "" {
		actorReferenceHMAC, err = keyedDigestWithKey(
			replayKey,
			"audit-actor-reference-v1",
			input.actor.Reference,
		)
		if err != nil {
			return securitySnapshot{}, err
		}
	}
	return securitySnapshot{
		EntryKind:              input.kind,
		ActorAudienceCode:      string(input.actor.AudienceCode),
		ActorReferenceHMAC:     actorReferenceHMAC,
		DataScope:              input.actor.DataScope,
		Delegated:              input.actor.Delegated,
		RequiredPermission:     input.authority.RequiredPermission,
		RequiredGranted:        input.authority.RequiredGranted,
		WildcardGranted:        input.authority.WildcardGranted,
		PlatformAdmin:          input.authority.PlatformAdmin,
		PermissionSnapshotHMAC: permissionSnapshotHMAC,
		PlatformBypass:         input.actor.PlatformBypass,
	}, nil
}

// rawReplayHMAC fingerprints original request evidence with the row's
// versioned replay key; it is independent of the persisted-row MAC keyring.
func rawReplayHMAC(
	input normalizedEntry,
	security securitySnapshot,
	request normalizedRequestEvidence,
	replayKey []byte,
) (string, error) {
	fingerprintDocument := evidenceFingerprint{
		Kind:                   input.kind,
		ActorType:              input.actor.Type,
		ActorReferenceHMAC:     security.ActorReferenceHMAC,
		ActorUserID:            input.actor.UserID,
		ActingUserID:           input.actor.ActingUserID,
		ActorAudienceCode:      string(input.actor.AudienceCode),
		ActorCustomerNo:        input.actor.CustomerNo,
		CustomerNo:             input.fields.CustomerNo,
		OperationCode:          input.fields.OperationCode,
		TargetType:             input.fields.TargetType,
		TargetNo:               input.fields.TargetNo,
		ResultCode:             input.fields.Result,
		ReasonCode:             input.fields.ReasonCode,
		RiskCode:               input.fields.Risk,
		RequiredPermission:     input.fields.RequiredPermission,
		RequiredGranted:        input.authority.RequiredGranted,
		WildcardGranted:        input.authority.WildcardGranted,
		PlatformAdmin:          input.authority.PlatformAdmin,
		PermissionSnapshotHMAC: security.PermissionSnapshotHMAC,
		DataScope:              input.actor.DataScope,
		Delegated:              input.actor.Delegated,
		PlatformBypass:         input.actor.PlatformBypass,
		BeforeSnapshot:         input.beforeOriginal,
		AfterSnapshot:          input.afterOriginal,
		CallerContext:          input.callerContextOriginal,
		RequestID:              request.requestID,
		TraceID:                request.traceID,
		CorrelationNo:          input.fields.CorrelationNo,
		IPDigest:               request.ipDigest,
		UserAgentDigest:        request.userAgentDigest,
	}
	fingerprintJSON, err := json.Marshal(fingerprintDocument)
	if err != nil {
		return "", gerror.Wrap(err, "encode audit raw replay fingerprint")
	}
	return keyedDigestWithKey(replayKey, rawReplayDomain, string(fingerprintJSON))
}

// buildUnsealedAuditRow creates all immutable business evidence using the
// active replay key. Context remains an explicit unsealed marker until the
// generated row ID and reservation owner ID can enter the final row MAC.
func (s *serviceImpl) buildUnsealedAuditRow(
	input normalizedEntry,
	reservation *entity.BusinessNumber,
	occurredAt time.Time,
) (*entity.BusinessAuditEvent, error) {
	request, err := s.requestEvidenceForKey(
		input.request,
		s.config.ActiveReplayDigestKey.Secret,
	)
	if err != nil {
		return nil, err
	}
	return &entity.BusinessAuditEvent{
		TenantId:          idccap.DefaultTenantID,
		AuditNo:           reservation.BusinessNo,
		OccurredAt:        &occurredAt,
		ActorType:         string(input.actor.Type),
		ActorUserId:       input.actor.UserID,
		ActingUserId:      input.actor.ActingUserID,
		ActorAudienceCode: string(input.actor.AudienceCode),
		ActorCustomerNo:   input.actor.CustomerNo,
		CustomerNo:        input.fields.CustomerNo,
		OperationCode:     input.fields.OperationCode,
		TargetType:        input.fields.TargetType,
		TargetNo:          input.fields.TargetNo,
		ResultCode:        string(input.fields.Result),
		ReasonCode:        input.fields.ReasonCode,
		RiskCode:          string(input.fields.Risk),
		BeforeSnapshot:    string(input.beforeSnapshot),
		AfterSnapshot:     string(input.afterSnapshot),
		ContextSnapshot:   auditUnsealedContext,
		RequestId:         request.requestID,
		TraceId:           request.traceID,
		CorrelationNo:     input.fields.CorrelationNo,
		IpDigest:          request.ipDigest,
		UserAgentDigest:   request.userAgentDigest,
	}, nil
}

// buildSealedContextSnapshot authenticates the final database identity,
// reservation owner identity, all scalars, and all evidence envelopes.
func (s *serviceImpl) buildSealedContextSnapshot(
	input normalizedEntry,
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
) (json.RawMessage, error) {
	replayKey := s.config.ActiveReplayDigestKey
	request, err := s.requestEvidenceForKey(input.request, replayKey.Secret)
	if err != nil {
		return nil, err
	}
	security, err := s.securityForEntry(input, replayKey.Secret)
	if err != nil {
		return nil, err
	}
	rawReplayHMAC, err := rawReplayHMAC(input, security, request, replayKey.Secret)
	if err != nil {
		return nil, err
	}
	integrity := integritySnapshot{
		Version:       integritySchemaVersion,
		KeyID:         s.config.ActiveIntegrityKey.ID,
		ReplayKeyID:   replayKey.ID,
		RawReplayHMAC: rawReplayHMAC,
	}
	contextCore, err := encodeContextCore(input.callerContext, security, integrity)
	if err != nil {
		return nil, err
	}
	integrity.RowHMAC, err = rowIntegrityHMAC(
		s.config.ActiveIntegrityKey.Secret,
		row,
		reservation,
		contextCore,
	)
	if err != nil {
		return nil, err
	}
	return s.encodeContextSnapshot(input.callerContext, security, integrity)
}

// appendInTransaction converges retries through the business-number ledger.
func (s *serviceImpl) appendInTransaction(
	ctx context.Context,
	input normalizedEntry,
) (*Event, error) {
	event, _, err := s.appendInTransactionWithReplay(ctx, input)
	return event, err
}

func (s *serviceImpl) appendInTransactionWithReplay(
	ctx context.Context,
	input normalizedEntry,
) (*Event, bool, error) {
	reservation, err := s.reserveAuditNumber(ctx, input)
	if err != nil {
		return nil, false, err
	}
	existing, err := loadAuditByNumber(ctx, reservation.BusinessNo, true)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		event, reuseErr := s.reuseAudit(existing, reservation, input)
		if reuseErr != nil {
			return nil, false, reuseErr
		}
		if reuseErr = validateAuditCustomerLinks(ctx, existing.AuditNo, input); reuseErr != nil {
			return nil, false, reuseErr
		}
		return event, true, nil
	}
	if reservation.OwnerType != "" || reservation.OwnerInternalId != 0 || reservation.AssignedAt != nil {
		return nil, false, gerror.New("assigned audit number has no append-only event")
	}
	occurredAt := databaseTime(s.now())
	row, err := s.buildUnsealedAuditRow(input, reservation, occurredAt)
	if err != nil {
		return nil, false, err
	}
	auditID, err := dao.BusinessAuditEvent.Ctx(ctx).Data(do.BusinessAuditEvent{
		TenantId:          row.TenantId,
		AuditNo:           row.AuditNo,
		OccurredAt:        row.OccurredAt,
		ActorType:         row.ActorType,
		ActorUserId:       nullablePositiveInt(row.ActorUserId),
		ActingUserId:      nullablePositiveInt(row.ActingUserId),
		ActorAudienceCode: row.ActorAudienceCode,
		ActorCustomerNo:   row.ActorCustomerNo,
		CustomerNo:        row.CustomerNo,
		OperationCode:     row.OperationCode,
		TargetType:        row.TargetType,
		TargetNo:          row.TargetNo,
		ResultCode:        row.ResultCode,
		ReasonCode:        row.ReasonCode,
		RiskCode:          row.RiskCode,
		BeforeSnapshot:    row.BeforeSnapshot,
		AfterSnapshot:     row.AfterSnapshot,
		ContextSnapshot:   row.ContextSnapshot,
		RequestId:         row.RequestId,
		TraceId:           row.TraceId,
		CorrelationNo:     row.CorrelationNo,
		IpDigest:          row.IpDigest,
		UserAgentDigest:   row.UserAgentDigest,
	}).InsertAndGetId()
	if err != nil {
		return nil, false, gerror.Wrap(err, "insert append-only audit event")
	}
	row.Id = auditID
	if err = assignAuditNumber(ctx, reservation.BusinessNo, auditID, occurredAt); err != nil {
		return nil, false, err
	}
	if err = insertAuditCustomerLinks(ctx, row.AuditNo, input.affectedCustomerNos); err != nil {
		return nil, false, err
	}
	expectedReservation := *reservation
	expectedReservation.OwnerType = businessNumberOwnerAudit
	expectedReservation.OwnerInternalId = auditID
	expectedReservation.AssignedAt = &occurredAt
	contextSnapshot, err := s.buildSealedContextSnapshot(input, row, &expectedReservation)
	if err != nil {
		return nil, false, err
	}
	if err = sealAuditEvent(ctx, row, contextSnapshot); err != nil {
		return nil, false, err
	}
	assignedReservation, err := loadAuditNumberByBusinessNo(ctx, reservation.BusinessNo, false)
	if err != nil {
		return nil, false, err
	}
	created, err := loadAuditByNumber(ctx, reservation.BusinessNo, false)
	if err != nil {
		return nil, false, err
	}
	if created == nil || assignedReservation == nil {
		return nil, false, gerror.New("inserted audit event is not readable")
	}
	if err = validateAuditCustomerLinks(ctx, created.AuditNo, input); err != nil {
		return nil, false, err
	}
	event, err := projectEventWithCustomerLinks(
		created,
		assignedReservation,
		s,
		input.affectedCustomerNos,
	)
	return event, false, err
}

// sealAuditEvent performs the sole allowed audit-row update. The exact
// unsealed marker and immutable identity must still match, so a second seal or
// any intervening mutation fails closed and causes the surrounding transaction
// to roll back. A deferred database constraint separately rejects commit of an
// unsealed row even if a caller incorrectly ignores the returned error.
func sealAuditEvent(
	ctx context.Context,
	row *entity.BusinessAuditEvent,
	contextSnapshot json.RawMessage,
) error {
	if row == nil || row.Id <= 0 || len(contextSnapshot) == 0 {
		return gerror.New("audit event sealing input is incomplete")
	}
	result, err := dao.BusinessAuditEvent.Ctx(ctx).
		Where(do.BusinessAuditEvent{
			Id:              row.Id,
			TenantId:        row.TenantId,
			AuditNo:         row.AuditNo,
			ContextSnapshot: auditUnsealedContext,
		}).
		Data(do.BusinessAuditEvent{ContextSnapshot: string(contextSnapshot)}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "seal append-only audit event")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read audit event sealing result")
	}
	if affected != 1 {
		return gerror.New("audit event could not be sealed exactly once")
	}
	row.ContextSnapshot = string(contextSnapshot)
	return nil
}

// reserveAuditNumber reserves or loads one opaque audit number by caller scope and key.
func (s *serviceImpl) reserveAuditNumber(
	ctx context.Context,
	input normalizedEntry,
) (*entity.BusinessNumber, error) {
	if len(input.reservationCandidates) != 1 {
		return nil, gerror.New("audit reservation candidates are unavailable")
	}
	for _, candidate := range input.reservationCandidates {
		if !isCanonicalDigest(candidate.reservationScopeDigest) ||
			!isCanonicalDigest(candidate.idempotencyKeyDigest) {
			return nil, gerror.New("audit reservation candidate is invalid")
		}
	}
	if existing, err := loadAuditNumberReservation(
		ctx,
		input.reservationCandidates,
		true,
	); err != nil || existing != nil {
		return existing, err
	}
	active := input.reservationCandidates[0]
	for range maximumNumberAttempts {
		candidate, err := newOpaqueAuditNumber(s.random)
		if err != nil {
			return nil, err
		}
		result, err := dao.BusinessNumber.Ctx(ctx).Data(do.BusinessNumber{
			TenantId:               idccap.DefaultTenantID,
			NumberType:             businessNumberTypeAudit,
			BusinessNo:             candidate,
			ReservationScopeDigest: active.reservationScopeDigest,
			IdempotencyKeyDigest:   active.idempotencyKeyDigest,
		}).InsertIgnore()
		if err != nil {
			return nil, gerror.Wrap(err, "reserve audit business number")
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, gerror.Wrap(err, "read audit number reservation result")
		}
		if affected < 0 || affected > 1 {
			return nil, gerror.New("audit number reservation affected an invalid row count")
		}
		if existing, err := loadAuditNumberReservation(
			ctx,
			input.reservationCandidates,
			true,
		); err != nil || existing != nil {
			return existing, err
		}
	}
	return nil, gerror.New("audit business number collision retry limit reached")
}

// loadAuditNumberReservation reads at most one reservation through the stable
// installation-keyed locator in one indexed database query.
func loadAuditNumberReservation(
	ctx context.Context,
	candidates []reservationCandidate,
	lock bool,
) (*entity.BusinessNumber, error) {
	if len(candidates) != 1 {
		return nil, gerror.New("audit reservation query candidates are invalid")
	}
	candidate := candidates[0]
	model := dao.BusinessNumber.Ctx(ctx).
		Where(do.BusinessNumber{
			TenantId:               idccap.DefaultTenantID,
			NumberType:             businessNumberTypeAudit,
			ReservationScopeDigest: candidate.reservationScopeDigest,
			IdempotencyKeyDigest:   candidate.idempotencyKeyDigest,
		}).
		Limit(2)
	if lock {
		model = model.LockUpdate()
	}
	var rows []entity.BusinessNumber
	if err := model.Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "load audit number reservation")
	}
	if len(rows) > 1 {
		return nil, gerror.New("multiple stable reservations match one audit request")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// loadAuditNumberByBusinessNo reads the immutable owner projection for one
// audit number and optionally locks it inside the current transaction.
func loadAuditNumberByBusinessNo(
	ctx context.Context,
	auditNo string,
	lock bool,
) (*entity.BusinessNumber, error) {
	var row *entity.BusinessNumber
	model := dao.BusinessNumber.Ctx(ctx).Where(do.BusinessNumber{
		TenantId:   idccap.DefaultTenantID,
		NumberType: businessNumberTypeAudit,
		BusinessNo: auditNo,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load audit number by business number")
	}
	return row, nil
}

// assignAuditNumber freezes ownership of the append-only event number.
func assignAuditNumber(ctx context.Context, auditNo string, auditID int64, assignedAt time.Time) error {
	result, err := dao.BusinessNumber.Ctx(ctx).
		Where(do.BusinessNumber{
			TenantId:   idccap.DefaultTenantID,
			BusinessNo: auditNo,
			OwnerType:  "",
		}).
		Data(do.BusinessNumber{
			OwnerType:       businessNumberOwnerAudit,
			OwnerInternalId: auditID,
			AssignedAt:      &assignedAt,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "assign audit business number")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read audit number assignment result")
	}
	if affected != 1 {
		return gerror.New("audit number reservation disappeared before assignment")
	}
	return nil
}

// loadAuditByNumber reads one tenant-scoped append-only event and optionally locks it.
func loadAuditByNumber(ctx context.Context, auditNo string, lock bool) (*entity.BusinessAuditEvent, error) {
	var row *entity.BusinessAuditEvent
	model := dao.BusinessAuditEvent.Ctx(ctx).Where(do.BusinessAuditEvent{
		TenantId: idccap.DefaultTenantID,
		AuditNo:  auditNo,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load audit event by number")
	}
	return row, nil
}

// reuseAudit returns an existing event only when every normalized immutable
// semantic, including the keyed raw-evidence fingerprint, is identical.
func (s *serviceImpl) reuseAudit(
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
	input normalizedEntry,
) (*Event, error) {
	verified, err := s.verifyPersistedEvent(row, reservation)
	if err != nil {
		// Persisted evidence is an integrity boundary. Do not leak whether
		// corruption failed JSON/schema validation or MAC verification.
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	replayKey, exists := s.replayKeys[verified.parsed.integrity.ReplayKeyID]
	if !exists {
		return nil, bizerr.WrapCode(
			gerror.New("persisted audit replay key is unavailable"),
			CodeUnavailable,
		)
	}
	request, err := s.requestEvidenceForKey(input.request, replayKey)
	if err != nil {
		return nil, err
	}
	security, err := s.securityForEntry(input, replayKey)
	if err != nil {
		return nil, err
	}
	rawReplayHMAC, err := rawReplayHMAC(input, security, request, replayKey)
	if err != nil {
		return nil, err
	}
	reservationMatched := false
	for _, candidate := range input.reservationCandidates {
		if candidate.reservationScopeDigest == reservation.ReservationScopeDigest &&
			candidate.idempotencyKeyDigest == reservation.IdempotencyKeyDigest {
			reservationMatched = true
			break
		}
	}
	storedContextData, err := json.Marshal(verified.parsed.data)
	if err != nil {
		return nil, gerror.Wrap(err, "canonicalize persisted audit caller context")
	}
	inputContextData, err := json.Marshal(input.callerContext)
	if err != nil {
		return nil, gerror.Wrap(err, "canonicalize replay audit caller context")
	}
	if row.ActorType != string(input.actor.Type) ||
		row.ActorUserId != input.actor.UserID ||
		row.ActingUserId != input.actor.ActingUserID ||
		row.ActorAudienceCode != string(input.actor.AudienceCode) ||
		row.ActorCustomerNo != input.actor.CustomerNo ||
		row.CustomerNo != input.fields.CustomerNo ||
		row.OperationCode != input.fields.OperationCode ||
		row.TargetType != input.fields.TargetType ||
		row.TargetNo != input.fields.TargetNo ||
		row.ResultCode != string(input.fields.Result) ||
		row.ReasonCode != input.fields.ReasonCode ||
		row.RiskCode != string(input.fields.Risk) ||
		row.RequestId != request.requestID ||
		row.TraceId != request.traceID ||
		row.CorrelationNo != input.fields.CorrelationNo ||
		normalizePersistedOptionalDigest(row.IpDigest) != request.ipDigest ||
		normalizePersistedOptionalDigest(row.UserAgentDigest) != request.userAgentDigest ||
		!bytes.Equal(verified.before, input.beforeSnapshot) ||
		!bytes.Equal(verified.after, input.afterSnapshot) ||
		!bytes.Equal(storedContextData, inputContextData) ||
		!reservationMatched ||
		!equalHMAC(rawReplayHMAC, verified.parsed.integrity.RawReplayHMAC) {
		return nil, bizerr.NewCode(CodeIdempotencyConflict)
	}
	return projectVerifiedEvent(row, verified)
}

func projectEventWithCustomerLinks(
	row *entity.BusinessAuditEvent,
	reservation *entity.BusinessNumber,
	service *serviceImpl,
	customers []string,
) (*Event, error) {
	verified, err := service.verifyPersistedEvent(row, reservation)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if err = validateVerifiedAffectedCustomers(verified, customers); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return projectVerifiedEvent(row, verified)
}

// projectVerifiedEvent maps only already-authenticated persistence state.
func projectVerifiedEvent(
	row *entity.BusinessAuditEvent,
	verified *verifiedEvidence,
) (*Event, error) {
	if row == nil || row.OccurredAt == nil || verified == nil {
		return nil, gerror.New("audit event is missing verified persisted fields")
	}
	return &Event{
		AuditNo:           row.AuditNo,
		OccurredAt:        databaseTime(*row.OccurredAt),
		ActorType:         ActorType(row.ActorType),
		ActorUserID:       row.ActorUserId,
		ActingUserID:      row.ActingUserId,
		ActorAudienceCode: principal.AudienceCode(row.ActorAudienceCode),
		ActorCustomerNo:   row.ActorCustomerNo,
		CustomerNo:        row.CustomerNo,
		OperationCode:     row.OperationCode,
		TargetType:        row.TargetType,
		TargetNo:          row.TargetNo,
		Result:            ResultCode(row.ResultCode),
		ReasonCode:        row.ReasonCode,
		Risk:              RiskCode(row.RiskCode),
		BeforeSnapshot:    verified.before,
		AfterSnapshot:     verified.after,
		ContextSnapshot:   verified.context,
		RequestID:         row.RequestId,
		TraceID:           row.TraceId,
		CorrelationNo:     row.CorrelationNo,
		IPDigest:          normalizePersistedOptionalDigest(row.IpDigest),
		UserAgentDigest:   normalizePersistedOptionalDigest(row.UserAgentDigest),
	}, nil
}

// nullablePositiveInt writes SQL NULL instead of a misleading zero identity.
func nullablePositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

// newOpaqueAuditNumber returns one cryptographically random 128-bit identifier.
func newOpaqueAuditNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate opaque audit number")
	}
	return auditNumberPrefix + hex.EncodeToString(content[:]), nil
}
