// Segment mutations create or assign one named group/tag for a scoped customer and retain audit history.
package customermanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

// Segment definition and assignment states stay independent from customer account states.
type segmentState string

const (
	segmentActive         segmentState = "active"
	segmentRevoked        segmentState = "revoked"
	segmentManualSource                = "manual"
	segmentAuditTarget                 = "customer"
	segmentAuditSucceeded              = "succeeded"
	segmentAuditReason                 = "manual_segment_update"
	segmentAuditRisk                   = "low"
)

// SegmentMutationInput identifies a customer, existing definition or new display name, and retry key.
type SegmentMutationInput struct {
	CustomerNo     string
	Kind           SegmentKind
	SegmentNo      string
	Name           string
	IdempotencyKey string
}

// SegmentMutationService owns simple single-customer additions and revocations.
type SegmentMutationService interface {
	// AddSegment checks staff permission and customer scope, atomically creates or assigns, and safely replays.
	AddSegment(context.Context, SegmentMutationInput) (string, error)
	// RemoveSegment revokes only the selected customer's assignment; shared definitions and audit history survive.
	RemoveSegment(context.Context, SegmentMutationInput) error
}

func normalizeSegmentMutation(input SegmentMutationInput, remove bool) (SegmentMutationInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.Name = strings.TrimSpace(input.Name)
	input.SegmentNo = strings.TrimSpace(input.SegmentNo)
	if !validPersistedCustomerNo(input.CustomerNo) || (input.Kind != SegmentKindGroup && input.Kind != SegmentKindTag) || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return input, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	prefix := "GRP_"
	if input.Kind == SegmentKindTag {
		prefix = "TAG_"
	}
	if input.SegmentNo != "" {
		suffix, err := hex.DecodeString(strings.TrimPrefix(input.SegmentNo, prefix))
		if !strings.HasPrefix(input.SegmentNo, prefix) || err != nil || len(suffix) != 16 || len(input.SegmentNo) != 36 || input.Name != "" {
			return input, bizerr.NewCode(customer.CodeProfileInvalid)
		}
	} else if remove || !utf8.ValidString(input.Name) || utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 128 {
		return input, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	return input, nil
}

// AddSegment returns the assigned stable number only after the transaction commits.
func (s *mutationServiceImpl) AddSegment(ctx context.Context, input SegmentMutationInput) (string, error) {
	return s.applySegment(ctx, input, false)
}

// RemoveSegment preserves the shared definition and previous assignments.
func (s *mutationServiceImpl) RemoveSegment(ctx context.Context, input SegmentMutationInput) error {
	_, err := s.applySegment(ctx, input, true)
	return err
}

func (s *mutationServiceImpl) applySegment(ctx context.Context, input SegmentMutationInput, remove bool) (string, error) {
	if err := authorizeStaff(ctx, s.bizCtxSvc, s.principal, s.authorization, permissionCustomerUpdate); err != nil {
		return "", err
	}
	input, err := normalizeSegmentMutation(input, remove)
	if err != nil {
		return "", err
	}
	actor := s.bizCtxSvc.Current(ctx).UserID
	create := input.SegmentNo == ""
	if create {
		digest := sha256.Sum256([]byte(input.CustomerNo + "\x00" + input.IdempotencyKey))
		prefix := "GRP_"
		if input.Kind == SegmentKindTag {
			prefix = "TAG_"
		}
		input.SegmentNo = prefix + hex.EncodeToString(digest[:16])
	}
	action := TagBatchAssign
	if remove {
		action = TagBatchRevoke
	}
	operation := "customer.segment." + string(action)
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		customers, err := lockTagBatchCustomers(ctx, []string{input.CustomerNo})
		if err != nil {
			return err
		}
		if err = validateTagBatchCustomers(customers); err != nil {
			return err
		}
		if create && input.Kind == SegmentKindTag {
			numbers, resolveErr := ensureNamedTags(ctx, []string{input.Name}, actor)
			if resolveErr != nil {
				return resolveErr
			}
			input.SegmentNo = numbers[0]
			create = false
		}
		evidence, err := s.auditWriter.AppendBusinessBatchTransition(ctx, idccap.BusinessAuditBatchTransition{
			IdempotencyScope: operation, IdempotencyKey: input.IdempotencyKey, OperationCode: operation,
			TargetType: segmentAuditTarget, TargetNo: input.CustomerNo, AffectedCustomerNos: []string{input.CustomerNo},
			ResultCode: segmentAuditSucceeded, ReasonCode: segmentAuditReason, RiskCode: segmentAuditRisk, RequiredPermission: permissionCustomerUpdate,
			CorrelationNo: input.IdempotencyKey, BeforeSnapshot: map[string]any{"kind": input.Kind, "segment_no": input.SegmentNo, "name": input.Name}, AfterSnapshot: map[string]any{"action": action},
		})
		if err != nil {
			return err
		}
		if evidence.Replayed {
			return nil
		}
		code := "segment_" + input.SegmentNo[4:]
		var definitionStatus string
		if input.Kind == SegmentKindGroup {
			if create {
				_, err = dao.CustomerGroup.Ctx(ctx).Data(do.CustomerGroup{TenantId: idccap.DefaultTenantID, GroupNo: input.SegmentNo, GroupCode: code, NameZhCn: input.Name, NameEnUs: input.Name, StatusCode: segmentActive, Version: 1, CreatedBy: actor, UpdatedBy: actor}).Insert()
				if err != nil {
					return err
				}
			}
			var row *entity.CustomerGroup
			err = dao.CustomerGroup.Ctx(ctx).Where(do.CustomerGroup{TenantId: idccap.DefaultTenantID, GroupNo: input.SegmentNo}).LockUpdate().Scan(&row)
			if err != nil {
				return err
			}
			if row != nil {
				definitionStatus = row.StatusCode
			}
		} else {
			if create {
				_, err = dao.CustomerTag.Ctx(ctx).Data(do.CustomerTag{TenantId: idccap.DefaultTenantID, TagNo: input.SegmentNo, TagCode: code, NameZhCn: input.Name, NameEnUs: input.Name, ColorToken: "blue", StatusCode: segmentActive, Version: 1, CreatedBy: actor, UpdatedBy: actor}).Insert()
				if err != nil {
					return err
				}
			}
			var row *entity.CustomerTag
			err = dao.CustomerTag.Ctx(ctx).Where(do.CustomerTag{TenantId: idccap.DefaultTenantID, TagNo: input.SegmentNo}).LockUpdate().Scan(&row)
			if err != nil {
				return err
			}
			if row != nil {
				definitionStatus = row.StatusCode
			}
		}
		if definitionStatus != string(segmentActive) {
			return bizerr.NewCode(customer.CodeProfileInvalid)
		}
		// The target customer is locked, serializing assignment/revocation for this customer.
		now := time.Now()
		if input.Kind == SegmentKindGroup {
			condition := do.CustomerGroupAssignment{TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo, GroupNo: input.SegmentNo, StatusCode: segmentActive}
			if remove {
				_, err = dao.CustomerGroupAssignment.Ctx(ctx).Where(condition).Data(do.CustomerGroupAssignment{StatusCode: segmentRevoked, RevokedBy: actor, RevokedAt: &now}).Update()
				return err
			}
			count, err := dao.CustomerGroupAssignment.Ctx(ctx).Where(condition).Count()
			if err != nil || count > 0 {
				return err
			}
			_, err = dao.CustomerGroupAssignment.Ctx(ctx).Data(do.CustomerGroupAssignment{TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo, GroupNo: input.SegmentNo, StatusCode: segmentActive, SourceCode: segmentManualSource, AssignedBy: actor, AssignedAt: &now}).Insert()
			return err
		}
		condition := do.CustomerTagAssignment{TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo, TagNo: input.SegmentNo, StatusCode: segmentActive}
		if remove {
			_, err = dao.CustomerTagAssignment.Ctx(ctx).Where(condition).Data(do.CustomerTagAssignment{StatusCode: segmentRevoked, RevokedBy: actor, RevokedAt: &now}).Update()
			return err
		}
		count, err := dao.CustomerTagAssignment.Ctx(ctx).Where(condition).Count()
		if err != nil || count > 0 {
			return err
		}
		_, err = dao.CustomerTagAssignment.Ctx(ctx).Data(do.CustomerTagAssignment{TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo, TagNo: input.SegmentNo, StatusCode: segmentActive, SourceCode: segmentManualSource, AssignedBy: actor, AssignedAt: &now}).Insert()
		return err
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return "", err
		}
		return "", bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return input.SegmentNo, nil
}

var _ SegmentMutationService = (*mutationServiceImpl)(nil)
