// This file verifies ownership-first workflow orchestration and capability separation.

package customer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type recordingWorkflowPersistence struct {
	legalCalls       int
	transitionCalls  int
	closureCalls     int
	findExportCalls  int
	exportCalls      int
	legalResult      *LegalChangeRequest
	transitionResult *AccountStateTransition
	closureResult    *ClosureRequest
	findExportResult *DataExportRequest
	exportResult     *DataExportRequest
	err              error
	actorUserID      int
	legalInput       normalizedLegalChange
	transitionInput  normalizedAccountTransition
	closureInput     normalizedClosureRequest
	precheck         ClosurePrecheckResult
	exportInput      normalizedDataExport
	proof            *reauth.Proof
	events           *[]string
}

func (p *recordingWorkflowPersistence) requestOwnLegalChange(
	_ context.Context,
	actorUserID int,
	input normalizedLegalChange,
) (*LegalChangeRequest, error) {
	p.legalCalls++
	p.actorUserID = actorUserID
	p.legalInput = input
	return p.legalResult, p.err
}

func (p *recordingWorkflowPersistence) transitionAccount(
	_ context.Context,
	actorUserID int,
	input normalizedAccountTransition,
) (*AccountStateTransition, error) {
	p.transitionCalls++
	p.actorUserID = actorUserID
	p.transitionInput = input
	return p.transitionResult, p.err
}

func (p *recordingWorkflowPersistence) requestOwnClosure(
	_ context.Context,
	actorUserID int,
	input normalizedClosureRequest,
	precheck ClosurePrecheckResult,
) (*ClosureRequest, error) {
	p.closureCalls++
	p.actorUserID = actorUserID
	p.closureInput = input
	p.precheck = precheck
	return p.closureResult, p.err
}

func (p *recordingWorkflowPersistence) findOwnDataExportReplay(
	_ context.Context,
	actorUserID int,
	input normalizedDataExport,
) (*DataExportRequest, error) {
	p.findExportCalls++
	if p.events != nil {
		*p.events = append(*p.events, "find_export")
	}
	p.actorUserID = actorUserID
	p.exportInput = input
	return p.findExportResult, p.err
}

func (p *recordingWorkflowPersistence) requestOwnDataExport(
	_ context.Context,
	actorUserID int,
	input normalizedDataExport,
	proof *reauth.Proof,
) (*DataExportRequest, error) {
	p.exportCalls++
	if p.events != nil {
		*p.events = append(*p.events, "request_export")
	}
	p.actorUserID = actorUserID
	p.exportInput = input
	p.proof = proof
	return p.exportResult, p.err
}

type recordingClosurePrecheck struct {
	results []ClosurePrecheckResult
	err     error
	calls   int
	inputs  [][]string
}

func (p *recordingClosurePrecheck) BatchCheck(
	_ context.Context,
	customerNos []string,
) ([]ClosurePrecheckResult, error) {
	p.calls++
	p.inputs = append(p.inputs, append([]string(nil), customerNos...))
	return p.results, p.err
}

type recordingReauth struct {
	issueResult    *reauth.ProofContext
	consumeResult  *reauth.Proof
	err            error
	issueCalls     int
	consumeCalls   int
	issueBinding   reauth.Binding
	consumeBinding reauth.Binding
	consumeToken   string
	events         *[]string
}

func (r *recordingReauth) Issue(
	_ context.Context,
	binding reauth.Binding,
) (*reauth.ProofContext, error) {
	r.issueCalls++
	if r.events != nil {
		*r.events = append(*r.events, "issue")
	}
	r.issueBinding = binding
	return r.issueResult, r.err
}

func (r *recordingReauth) Consume(
	_ context.Context,
	token string,
	binding reauth.Binding,
) (*reauth.Proof, error) {
	r.consumeCalls++
	if r.events != nil {
		*r.events = append(*r.events, "consume")
	}
	r.consumeToken = token
	r.consumeBinding = binding
	return r.consumeResult, r.err
}

type recordingAccountAuthorizer struct {
	err        error
	calls      int
	customerNo string
}

func (a *recordingAccountAuthorizer) AuthorizeTransition(
	_ context.Context,
	customerNo string,
) error {
	a.calls++
	a.customerNo = customerNo
	return a.err
}

func TestWorkflowSelfServiceRejectsForeignOwnershipBeforeDependencies(t *testing.T) {
	foreignNo := "CUS_ffffffffffffffffffffffffffffffff"
	workflow := &recordingWorkflowPersistence{}
	checker := &recordingClosurePrecheck{}
	recentAuth := &recordingReauth{}
	service := &serviceImpl{
		access: testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()), workflow: workflow,
		closure: checker, reauth: recentAuth, now: time.Now,
	}
	ctx := context.Background()
	if _, err := service.RequestOwnLegalChange(ctx, RequestOwnLegalChangeInput{
		CustomerNo: foreignNo, Fields: []LegalChangeField{LegalChangeFieldName},
		Reason: LegalChangeReasonNameChange, ExpectedProfileVersion: 1, IdempotencyKey: "legal",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign legal request returned %v", err)
	}
	if _, err := service.RequestOwnClosure(ctx, RequestOwnClosureInput{
		CustomerNo: foreignNo, ExpectedVersion: 1, Confirmed: true, IdempotencyKey: "closure",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign closure request returned %v", err)
	}
	intent := OwnDataExportIntent{
		CustomerNo: foreignNo, Scope: DataExportScopeOwnDeliverable,
		Format: DataExportFormatJSONZip, ExpectedVersion: 1, IdempotencyKey: "export", Confirmed: true,
	}
	if _, err := service.IssueOwnDataExportProof(ctx, intent); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign export proof returned %v", err)
	}
	if _, err := service.RequestOwnDataExport(ctx, RequestOwnDataExportInput{
		OwnDataExportIntent: intent, ProofToken: "opaque-proof",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign export request returned %v", err)
	}
	if workflow.legalCalls != 0 || workflow.closureCalls != 0 || workflow.findExportCalls != 0 ||
		workflow.exportCalls != 0 || checker.calls != 0 || recentAuth.issueCalls != 0 ||
		recentAuth.consumeCalls != 0 {
		t.Fatalf("foreign ownership crossed dependency boundary: workflow=%+v checker=%+v auth=%+v", workflow, checker, recentAuth)
	}
}

func TestRequestOwnLegalChangePassesOnlyNormalizedCategories(t *testing.T) {
	expected := &LegalChangeRequest{RequestNo: "LCR_0123456789abcdef0123456789abcdef"}
	workflow := &recordingWorkflowPersistence{legalResult: expected}
	service := &serviceImpl{
		access: testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()), workflow: workflow,
	}
	result, err := service.RequestOwnLegalChange(context.Background(), RequestOwnLegalChangeInput{
		CustomerNo: testCustomerNo,
		Fields:     []LegalChangeField{LegalChangeFieldIdentifier, LegalChangeFieldName},
		Reason:     LegalChangeReasonOther, ExpectedProfileVersion: 7,
		IdempotencyKey: " legal-change-1 ",
	})
	if err != nil || result != expected || workflow.legalCalls != 1 || workflow.actorUserID != 81 {
		t.Fatalf("legal change result=%+v err=%v workflow=%+v", result, err, workflow)
	}
	if !workflow.legalInput.changeLegalName || !workflow.legalInput.changeLegalIdentifier ||
		workflow.legalInput.expectedProfileVersion != 7 ||
		workflow.legalInput.idempotencyKey != "legal-change-1" {
		t.Fatalf("unexpected normalized legal categories: %+v", workflow.legalInput)
	}
}

func TestRequestOwnClosureFailsClosedAndDoesNotMutateProviderSlice(t *testing.T) {
	now := time.Date(2026, 8, 7, 1, 0, 0, 0, time.UTC)
	providerFindings := []ClosureFinding{
		{Code: ClosureFindingLegalRetention, Count: 2},
		{Code: ClosureFindingActiveServices, Count: 1},
	}
	checker := &recordingClosurePrecheck{results: []ClosurePrecheckResult{{
		CustomerNo: testCustomerNo, ContractVersion: closurePrecheckContractVersion,
		CheckedAt: now, Findings: providerFindings,
	}}}
	workflow := &recordingWorkflowPersistence{}
	service := &serviceImpl{
		access:   testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()),
		workflow: workflow, closure: checker,
		now: func() time.Time { return now },
	}
	result, err := service.RequestOwnClosure(context.Background(), RequestOwnClosureInput{
		CustomerNo: testCustomerNo, ExpectedVersion: 3, Confirmed: true, IdempotencyKey: "closure-1",
	})
	if err != nil || result == nil || result.Status != ClosureStatusBlocked || result.RequestNo != "" {
		t.Fatalf("blocked closure result=%+v err=%v", result, err)
	}
	if workflow.closureCalls != 0 {
		t.Fatalf("blocked precheck persisted a closure request: %+v", workflow)
	}
	if !reflect.DeepEqual(providerFindings, []ClosureFinding{
		{Code: ClosureFindingLegalRetention, Count: 2},
		{Code: ClosureFindingActiveServices, Count: 1},
	}) {
		t.Fatalf("validation mutated provider-owned findings: %+v", providerFindings)
	}

	checker.results = nil
	if _, err = service.RequestOwnClosure(context.Background(), RequestOwnClosureInput{
		CustomerNo: testCustomerNo, ExpectedVersion: 3, Confirmed: true, IdempotencyKey: "closure-2",
	}); !bizerr.Is(err, CodeWorkflowUnavailable) {
		t.Fatalf("missing precheck result returned %v", err)
	}
	checker.err = errors.New("dependency unavailable")
	if _, err = service.RequestOwnClosure(context.Background(), RequestOwnClosureInput{
		CustomerNo: testCustomerNo, ExpectedVersion: 3, Confirmed: true, IdempotencyKey: "closure-3",
	}); !bizerr.Is(err, CodeWorkflowUnavailable) {
		t.Fatalf("precheck dependency error returned %v", err)
	}
}

func TestDataExportUsesExactBindingAndSkipsProofConsumptionOnlyForReplay(t *testing.T) {
	now := time.Date(2026, 8, 7, 1, 0, 0, 0, time.UTC)
	intent := OwnDataExportIntent{
		CustomerNo: testCustomerNo, Scope: DataExportScopeOwnDeliverable,
		Format: DataExportFormatJSONZip, ExpectedVersion: 4,
		IdempotencyKey: "export-1", Confirmed: true,
	}
	expected := &DataExportRequest{RequestNo: "DEX_0123456789abcdef0123456789abcdef"}
	events := make([]string, 0, 5)
	workflow := &recordingWorkflowPersistence{exportResult: expected, events: &events}
	recentAuth := &recordingReauth{
		issueResult: &reauth.ProofContext{Token: "proof"},
		consumeResult: &reauth.Proof{
			Purpose: reauth.PurposeSensitiveExport, UserID: 81, TenantID: idccap.DefaultTenantID,
			LoginAt: now.Add(-time.Minute), IssuedAt: now, ExpiresAt: now.Add(time.Minute),
			SessionDigest: strings.Repeat("a", 64),
		},
		events: &events,
	}
	service := &serviceImpl{
		access:   testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()),
		workflow: workflow, reauth: recentAuth, now: func() time.Time { return now },
	}
	proofContext, err := service.IssueOwnDataExportProof(context.Background(), intent)
	if err != nil || proofContext != recentAuth.issueResult {
		t.Fatalf("issue export proof result=%+v err=%v", proofContext, err)
	}
	recentAuth.consumeResult.TargetDigest = recentAuth.issueBinding.TargetDigest
	result, err := service.RequestOwnDataExport(context.Background(), RequestOwnDataExportInput{
		OwnDataExportIntent: intent, ProofToken: " opaque-proof ",
	})
	if err != nil || result != expected || workflow.exportCalls != 1 || recentAuth.consumeCalls != 1 {
		t.Fatalf("new export result=%+v err=%v workflow=%+v auth=%+v", result, err, workflow, recentAuth)
	}
	if recentAuth.issueBinding != recentAuth.consumeBinding || recentAuth.consumeToken != "opaque-proof" ||
		recentAuth.issueBinding.Purpose != reauth.PurposeSensitiveExport {
		t.Fatalf("export binding diverged: issue=%+v consume=%+v token=%q", recentAuth.issueBinding, recentAuth.consumeBinding, recentAuth.consumeToken)
	}
	if !reflect.DeepEqual(events, []string{"issue", "find_export", "consume", "request_export"}) {
		t.Fatalf("export orchestration order=%v", events)
	}

	replay := &DataExportRequest{RequestNo: "DEX_ffffffffffffffffffffffffffffffff"}
	workflow.findExportResult = replay
	events = events[:0]
	result, err = service.RequestOwnDataExport(context.Background(), RequestOwnDataExportInput{
		OwnDataExportIntent: intent, ProofToken: "already-consumed-proof",
	})
	if err != nil || result != replay || recentAuth.consumeCalls != 1 || workflow.exportCalls != 1 {
		t.Fatalf("replay consumed proof or recreated task: result=%+v err=%v workflow=%+v auth=%+v", result, err, workflow, recentAuth)
	}
	if !reflect.DeepEqual(events, []string{"find_export"}) {
		t.Fatalf("export replay crossed proof or mutation boundary: %v", events)
	}
}

func TestAccountLifecycleRequiresSeparateExactTargetAuthorization(t *testing.T) {
	workflow := &recordingWorkflowPersistence{transitionResult: &AccountStateTransition{
		CustomerNo: testCustomerNo, ToStatus: StatusRestricted,
	}}
	authorizer := &recordingAccountAuthorizer{}
	principalSvc := &staticPrincipal{audience: &principal.Audience{
		UserID: 99, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	service := &accountLifecycleServiceImpl{
		bizCtxSvc: staticBizCtx{current: bizctxcap.CurrentContext{
			UserID: 99, DataScope: 2, PlatformBypass: true,
		}},
		principal: principalSvc, workflow: workflow, authorizer: authorizer,
	}
	input := TransitionAccountInput{
		CustomerNo: testCustomerNo, ToStatus: StatusRestricted,
		Reason: AccountReasonRiskRestriction, ExpectedVersion: 1, IdempotencyKey: "state-1",
	}
	result, err := service.TransitionAccount(context.Background(), input)
	if err != nil || result != workflow.transitionResult || authorizer.calls != 1 ||
		authorizer.customerNo != testCustomerNo || workflow.transitionCalls != 1 ||
		workflow.actorUserID != 99 {
		t.Fatalf("authorized transition result=%+v err=%v authz=%+v workflow=%+v", result, err, authorizer, workflow)
	}

	authorizer.err = errors.New("outside target data scope")
	if _, err = service.TransitionAccount(context.Background(), TransitionAccountInput{
		CustomerNo: testCustomerNo, ToStatus: StatusDisabled,
		Reason: AccountReasonManualDisable, ExpectedVersion: 2, IdempotencyKey: "state-2",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("target data-scope denial returned %v", err)
	}
	if workflow.transitionCalls != 1 {
		t.Fatalf("denied target reached persistence: %+v", workflow)
	}
	if principalSvc.calls != 2 || principalSvc.staffCalls != 0 {
		t.Fatalf("state path created or re-bound staff audience: %+v", principalSvc)
	}

	service.bizCtxSvc = staticBizCtx{current: bizctxcap.CurrentContext{
		UserID: 99, Permissions: []string{"idc:admin:customer:update"}, DataScope: 2,
		ActingUserID: 100,
	}}
	if _, err = service.TransitionAccount(context.Background(), input); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("delegated staff transition returned %v", err)
	}
	if authorizer.calls != 2 || workflow.transitionCalls != 1 || principalSvc.calls != 2 ||
		principalSvc.staffCalls != 0 {
		t.Fatalf("delegated transition crossed staff guard: authz=%+v workflow=%+v", authorizer, workflow)
	}
}

func TestNewAccountLifecycleRequiresExactAuthorizationDependency(t *testing.T) {
	principalSvc := &staticPrincipal{}
	auditWriter := &stubAuditWriter{}
	authorizer := &recordingAccountAuthorizer{}
	if _, err := NewAccountLifecycleService(nil, principalSvc, auditWriter, authorizer); err == nil {
		t.Fatal("missing lifecycle bizctx must fail")
	}
	if _, err := NewAccountLifecycleService(staticBizCtx{}, nil, auditWriter, authorizer); err == nil {
		t.Fatal("missing lifecycle principal must fail")
	}
	if _, err := NewAccountLifecycleService(staticBizCtx{}, principalSvc, nil, authorizer); err == nil {
		t.Fatal("missing lifecycle audit must fail")
	}
	if _, err := NewAccountLifecycleService(staticBizCtx{}, principalSvc, auditWriter, nil); err == nil {
		t.Fatal("missing lifecycle authorizer must fail")
	}
	var typedNil *recordingAccountAuthorizer
	if _, err := NewAccountLifecycleService(staticBizCtx{}, principalSvc, auditWriter, typedNil); err == nil {
		t.Fatal("typed-nil lifecycle authorizer must fail")
	}
}

func TestAccountTransitionMatrixNeverExposesAnonymizedSetter(t *testing.T) {
	if _, err := normalizeAccountTransition(TransitionAccountInput{
		CustomerNo: testCustomerNo, ToStatus: StatusAnonymized,
		Reason: AccountReasonManualDisable, ExpectedVersion: 1, IdempotencyKey: "state",
	}); !bizerr.Is(err, CodeWorkflowInvalid) {
		t.Fatalf("generic anonymized transition returned %v", err)
	}
	valid, err := normalizeAccountTransition(TransitionAccountInput{
		CustomerNo: testCustomerNo, ToStatus: StatusRestricted,
		Reason: AccountReasonRiskRestriction, ExpectedVersion: 1, IdempotencyKey: "state",
	})
	if err != nil || !validAccountTransition(StatusActive, valid) ||
		validAccountTransition(StatusDisabled, valid) {
		t.Fatalf("unexpected account transition matrix result input=%+v err=%v", valid, err)
	}
}
