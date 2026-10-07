// This file applies authenticated ownership before customer-profile persistence access.

package customer

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type persistence interface {
	load(ctx context.Context, customerNo string) (*Profile, error)
	updateOwnBasics(ctx context.Context, actorUserID int, input normalizedUpdate) (*Profile, error)
}

type workflowPersistence interface {
	requestOwnLegalChange(
		ctx context.Context,
		actorUserID int,
		input normalizedLegalChange,
	) (*LegalChangeRequest, error)
	transitionAccount(
		ctx context.Context,
		actorUserID int,
		input normalizedAccountTransition,
	) (*AccountStateTransition, error)
	requestOwnClosure(
		ctx context.Context,
		actorUserID int,
		input normalizedClosureRequest,
		precheck ClosurePrecheckResult,
	) (*ClosureRequest, error)
	findOwnDataExportReplay(
		ctx context.Context,
		actorUserID int,
		input normalizedDataExport,
	) (*DataExportRequest, error)
	requestOwnDataExport(
		ctx context.Context,
		actorUserID int,
		input normalizedDataExport,
		proof *reauth.Proof,
	) (*DataExportRequest, error)
}

type legalChangePersistence interface {
	currentOwnLegalChange(ctx context.Context, customerNo string) (*LegalChangeRequest, error)
}

type serviceImpl struct {
	access      *AccessGuard
	persistence persistence
	workflow    workflowPersistence
	legalChange legalChangePersistence
	closure     ClosurePrecheckService
	reauth      reauth.Service
	now         func() time.Time
}

type accountLifecycleServiceImpl struct {
	bizCtxSvc  bizctxcap.Service
	principal  principal.Service
	workflow   workflowPersistence
	authorizer AccountStateAuthorizer
}

var _ Service = (*serviceImpl)(nil)
var _ AccountLifecycleService = (*accountLifecycleServiceImpl)(nil)

// New creates the self-service customer profile and workflow boundary. A
// shared customer access guard, transaction-aware audit, task,
// recent-authentication, and closure-precheck services are mandatory.
func New(
	access *AccessGuard,
	auditWriter transitionAuditWriter,
	taskSvc asynctask.Service,
	reauthSvc reauth.Service,
	closurePrecheck ClosurePrecheckService,
) (Service, error) {
	if nilInterface(access) {
		return nil, gerror.New("customer service requires shared customer access guard")
	}
	if nilInterface(auditWriter) {
		return nil, gerror.New("customer service requires audit writer")
	}
	if nilInterface(taskSvc) {
		return nil, gerror.New("customer service requires async task service")
	}
	if nilInterface(reauthSvc) {
		return nil, gerror.New("customer service requires recent authentication service")
	}
	if nilInterface(closurePrecheck) {
		return nil, gerror.New("customer service requires closure precheck service")
	}
	store := newDAOPersistence(auditWriter, taskSvc)
	return &serviceImpl{
		access:      access,
		persistence: store,
		workflow:    store,
		legalChange: store,
		closure:     closurePrecheck,
		reauth:      reauthSvc,
		now:         time.Now,
	}, nil
}

// NewOwnLegalChangeService creates the focused current-customer legal-change
// boundary used by the source-only enterprise verification workflow.
func NewOwnLegalChangeService(
	access *AccessGuard,
	auditWriter audit.Writer,
) (OwnLegalChangeService, error) {
	if nilInterface(access) || nilInterface(auditWriter) {
		return nil, gerror.New("customer legal change service requires access and audit dependencies")
	}
	store := newDAOPersistence(auditWriter, nil)
	return &serviceImpl{
		access: access, persistence: store, workflow: store, legalChange: store,
	}, nil
}

// NewAccountLifecycleService creates the separate staff-side state boundary.
// The injected authorizer remains mandatory until the plugin has a production
// exact-permission and target-data-scope adapter in a later task.
func NewAccountLifecycleService(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Service,
	auditWriter transitionAuditWriter,
	authorizer AccountStateAuthorizer,
) (AccountLifecycleService, error) {
	if nilInterface(bizCtxSvc) || nilInterface(principalSvc) || nilInterface(auditWriter) ||
		nilInterface(authorizer) {
		return nil, gerror.New("customer account lifecycle requires all authorization and persistence dependencies")
	}
	return &accountLifecycleServiceImpl{
		bizCtxSvc:  bizCtxSvc,
		principal:  principalSvc,
		workflow:   newDAOPersistence(auditWriter, nil),
		authorizer: authorizer,
	}, nil
}

func (s *serviceImpl) CurrentProfile(ctx context.Context) (*Profile, error) {
	audience, err := s.currentDirectCustomer(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := s.persistence.load(ctx, audience.CustomerNo)
	return mapLoadResult(profile, err)
}

func (s *serviceImpl) GetOwnedProfile(ctx context.Context, customerNo string) (*Profile, error) {
	_, err := s.requireOwnedCustomer(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	profile, err := s.persistence.load(ctx, customerNo)
	return mapLoadResult(profile, err)
}

func (s *serviceImpl) UpdateOwnBasics(
	ctx context.Context,
	input UpdateOwnBasicsInput,
) (*Profile, error) {
	audience, err := s.requireOwnedCustomer(ctx, input.CustomerNo)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeUpdate(input)
	if err != nil {
		return nil, err
	}
	profile, err := s.persistence.updateOwnBasics(ctx, audience.UserID, normalized)
	if err == nil {
		return profile, nil
	}
	if bizerr.Is(err, CodeNotAccessible) ||
		bizerr.Is(err, CodeProfileInvalid) ||
		bizerr.Is(err, CodeStateDenied) ||
		bizerr.Is(err, CodeVersionConflict) ||
		bizerr.Is(err, CodeIdempotencyConflict) {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeUnavailable)
}

func (s *serviceImpl) RequestOwnLegalChange(
	ctx context.Context,
	input RequestOwnLegalChangeInput,
) (*LegalChangeRequest, error) {
	audience, err := s.requireOwnedCustomer(ctx, input.CustomerNo)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeLegalChange(input)
	if err != nil {
		return nil, err
	}
	result, err := s.workflow.requestOwnLegalChange(ctx, audience.UserID, normalized)
	return mapWorkflowResult(result, err)
}

// CurrentOwnLegalChange returns the current customer's one active legal-change
// workflow, if present, without accepting a customer number from the caller.
func (s *serviceImpl) CurrentOwnLegalChange(
	ctx context.Context,
) (*LegalChangeRequest, error) {
	audience, err := s.currentDirectCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if s.legalChange == nil {
		return nil, bizerr.NewCode(CodeWorkflowUnavailable)
	}
	result, err := s.legalChange.currentOwnLegalChange(ctx, audience.CustomerNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeWorkflowUnavailable)
	}
	return result, nil
}

func (s *accountLifecycleServiceImpl) TransitionAccount(
	ctx context.Context,
	input TransitionAccountInput,
) (*AccountStateTransition, error) {
	staff, err := s.currentDirectStaff(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeAccountTransition(input)
	if err != nil {
		return nil, err
	}
	if err = s.authorizer.AuthorizeTransition(ctx, normalized.customerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	result, err := s.workflow.transitionAccount(ctx, staff.UserID, normalized)
	return mapWorkflowResult(result, err)
}

func (s *serviceImpl) RequestOwnClosure(
	ctx context.Context,
	input RequestOwnClosureInput,
) (*ClosureRequest, error) {
	audience, err := s.requireOwnedCustomer(ctx, input.CustomerNo)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeClosureRequest(input)
	if err != nil {
		return nil, err
	}
	results, err := s.closure.BatchCheck(ctx, []string{audience.CustomerNo})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeWorkflowUnavailable)
	}
	precheck, err := validateClosureBatch(audience.CustomerNo, results, s.now())
	if err != nil {
		return nil, err
	}
	if closureHasHardBlocker(precheck.Findings) {
		// Blocked is a precheck outcome, not a retained workflow row. The caller
		// may retry after resolving blockers; no consent or closure authority is
		// created from a failed snapshot.
		return &ClosureRequest{
			CustomerNo: audience.CustomerNo, Status: ClosureStatusBlocked,
			SubjectVersion:          normalized.expectedVersion,
			Findings:                append([]ClosureFinding(nil), precheck.Findings...),
			PrecheckContractVersion: precheck.ContractVersion,
			PrecheckedAt:            precheck.CheckedAt,
		}, nil
	}
	result, err := s.workflow.requestOwnClosure(ctx, audience.UserID, normalized, precheck)
	return mapWorkflowResult(result, err)
}

func (s *serviceImpl) IssueOwnDataExportProof(
	ctx context.Context,
	input OwnDataExportIntent,
) (*reauth.ProofContext, error) {
	_, err := s.requireOwnedCustomer(ctx, input.CustomerNo)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeDataExport(input)
	if err != nil {
		return nil, err
	}
	binding, err := normalized.binding()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeWorkflowUnavailable)
	}
	return s.reauth.Issue(ctx, binding)
}

func (s *serviceImpl) RequestOwnDataExport(
	ctx context.Context,
	input RequestOwnDataExportInput,
) (*DataExportRequest, error) {
	audience, err := s.requireOwnedCustomer(ctx, input.CustomerNo)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeDataExport(input.OwnDataExportIntent)
	if err != nil || !validBoundedText(strings.TrimSpace(input.ProofToken), 4096) {
		if err != nil {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeWorkflowInvalid)
	}
	// Exact committed replay is resolved before consuming another single-use
	// proof. A changed request under the same idempotency key still conflicts.
	replay, err := s.workflow.findOwnDataExportReplay(ctx, audience.UserID, normalized)
	if err != nil {
		return mapWorkflowResult(replay, err)
	}
	if replay != nil {
		return replay, nil
	}
	binding, err := normalized.binding()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeWorkflowUnavailable)
	}
	proof, err := s.reauth.Consume(ctx, strings.TrimSpace(input.ProofToken), binding)
	if err != nil {
		return nil, err
	}
	if proof == nil || proof.UserID != audience.UserID || proof.TenantID != idccap.DefaultTenantID ||
		proof.Purpose != binding.Purpose || proof.TargetDigest != binding.TargetDigest ||
		!validDigest(proof.SessionDigest) || proof.LoginAt.IsZero() || proof.IssuedAt.IsZero() ||
		proof.ExpiresAt.IsZero() || !proof.ExpiresAt.After(proof.IssuedAt) {
		return nil, bizerr.NewCode(CodeWorkflowUnavailable)
	}
	result, err := s.workflow.requestOwnDataExport(ctx, audience.UserID, normalized, proof)
	return mapWorkflowResult(result, err)
}

func (s *serviceImpl) currentDirectCustomer(ctx context.Context) (*principal.Audience, error) {
	if s == nil || s.access == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.access.Resolve(ctx)
}

func (s *serviceImpl) requireOwnedCustomer(ctx context.Context, customerNo string) (AccessSubject, error) {
	if s == nil || s.access == nil {
		return AccessSubject{}, bizerr.NewCode(CodeNotAccessible)
	}
	return s.access.RequireOwned(ctx, customerNo)
}

func (s *accountLifecycleServiceImpl) currentDirectStaff(ctx context.Context) (*principal.Audience, error) {
	current := s.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.ActingUserID != 0 || current.IsImpersonation ||
		current.ActingAsTenant || !hasBroadCustomerDataScope(current) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return audience, nil
}

func mapWorkflowResult[T any](result *T, err error) (*T, error) {
	if err == nil {
		if result == nil {
			return nil, bizerr.NewCode(CodeWorkflowUnavailable)
		}
		return result, nil
	}
	if bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeWorkflowInvalid) ||
		bizerr.Is(err, CodeStateDenied) || bizerr.Is(err, CodeVersionConflict) ||
		bizerr.Is(err, CodeIdempotencyConflict) || bizerr.Is(err, CodeWorkflowUnavailable) {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeWorkflowUnavailable)
}

func mapLoadResult(profile *Profile, err error) (*Profile, error) {
	if err != nil {
		if bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if profile == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return profile, nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
