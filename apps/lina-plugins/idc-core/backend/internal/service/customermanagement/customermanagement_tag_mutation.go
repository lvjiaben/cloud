// This file defines bounded, staff-only batch customer tag mutations.

package customermanagement

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// TagBatchAction selects an idempotent relationship transition.
type TagBatchAction string

const (
	TagBatchAssign TagBatchAction = "assign"
	TagBatchRevoke TagBatchAction = "revoke"
)

// TagBatchInput is bounded before any customer or tag row is read.
type TagBatchInput struct {
	Action         TagBatchAction
	CustomerNos    []string
	TagNos         []string
	Reason         string
	IdempotencyKey string
}

// TagBatchResult identifies the sealed batch evidence and normalized scope.
type TagBatchResult struct {
	AuditNo       string
	Replayed      bool
	CustomerCount int
	TagCount      int
}

// MutationService owns staff-only customer management writes separately from
// the read-only list and customer 360 service.
type MutationService interface {
	BatchUpdateTags(ctx context.Context, input TagBatchInput) (*TagBatchResult, error)
	AddNote(ctx context.Context, input NoteInput) (*NoteResult, error)
	PlanFollowUp(ctx context.Context, input FollowUpPlanInput) (*FollowUpResult, error)
	TransitionFollowUp(ctx context.Context, input FollowUpTransitionInput) (*FollowUpResult, error)
	CreateBlacklist(ctx context.Context, input BlacklistCreateInput) (*BlacklistResult, error)
	DecideBlacklist(ctx context.Context, input BlacklistDecisionInput) (*BlacklistResult, error)
}

type tagMutationPersistence interface {
	applyTagBatch(
		ctx context.Context,
		input normalizedTagBatch,
		auditWriter idccap.BusinessAuditWriter,
	) (*TagBatchResult, error)
}

type mutationServiceImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	auditWriter   idccap.BusinessAuditWriter
	persistence   tagMutationPersistence
}

var _ MutationService = (*mutationServiceImpl)(nil)

// NewMutation creates the production batch customer mutation service.
func NewMutation(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	auditWriter idccap.BusinessAuditWriter,
) (MutationService, error) {
	return newMutationService(
		bizCtxSvc,
		principalSvc,
		authorization,
		auditWriter,
		&daoTagMutationPersistence{},
	)
}

func newMutationService(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	auditWriter idccap.BusinessAuditWriter,
	persistence tagMutationPersistence,
) (MutationService, error) {
	if nilInterface(bizCtxSvc) || nilInterface(principalSvc) || nilInterface(authorization) ||
		nilInterface(auditWriter) || nilInterface(persistence) {
		return nil, gerror.New("customer tag mutation requires context, principal, authorization, audit, and persistence services")
	}
	return &mutationServiceImpl{
		bizCtxSvc: bizCtxSvc, principal: principalSvc, authorization: authorization,
		auditWriter: auditWriter, persistence: persistence,
	}, nil
}

func (s *mutationServiceImpl) BatchUpdateTags(
	ctx context.Context,
	input TagBatchInput,
) (*TagBatchResult, error) {
	if err := authorizeStaff(
		ctx,
		s.bizCtxSvc,
		s.principal,
		s.authorization,
		permissionCustomerTagBatch,
	); err != nil {
		return nil, err
	}
	normalized, err := normalizeTagBatch(input)
	if err != nil {
		return nil, err
	}
	normalized.actorUserID = s.bizCtxSvc.Current(ctx).UserID
	result, err := s.persistence.applyTagBatch(ctx, normalized, s.auditWriter)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return result, nil
}
