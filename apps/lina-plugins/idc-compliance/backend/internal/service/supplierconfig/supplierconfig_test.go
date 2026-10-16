// This file verifies exact permissions, recent-login gates, and proof intent binding.

package supplierconfig

import (
	"context"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type supplierAuthorizer struct {
	request managementcap.AuthorizationRequest
	actor   int
}

func (a *supplierAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	a.request = request
	return a.actor, nil
}

type supplierSessions struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
}

func (s *supplierSessions) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return s.current, nil
}

type supplierAudit struct {
	idccap.BusinessAuditWriter
	transitions int
}
type supplierManager struct{ secretcap.Manager }
type supplierSensitiveAccess struct{ issued *int }
type supplierGateway struct{ available bool }

func (s supplierSensitiveAccess) IssueSensitiveAccess(
	context.Context,
	managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessContext, error) {
	if s.issued != nil {
		*s.issued++
	}
	return &managementcap.SensitiveAccessContext{Token: "proof", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (a *supplierAudit) AppendBusinessTransition(
	context.Context,
	idccap.BusinessAuditTransition,
) error {
	a.transitions++
	return nil
}

func (supplierSensitiveAccess) ConsumeSensitiveAccess(
	context.Context,
	string,
	managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessProof, error) {
	return &managementcap.SensitiveAccessProof{ActorUserID: 42, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func TestUnavailableRecognizerDoesNotIssueProofOrCreatePendingState(t *testing.T) {
	now := time.Date(2026, time.August, 14, 8, 0, 0, 0, time.UTC)
	issued := 0
	audit := &supplierAudit{}
	service, err := New(
		&supplierManager{},
		managementcap.Runtime{
			Authorizer: &supplierAuthorizer{actor: 42}, Audit: audit,
			SensitiveAccess: supplierSensitiveAccess{issued: &issued},
		},
		&supplierSessions{current: &sessioncap.SessionInfo{
			ID: "session-1", UserID: "42", TenantID: capmodel.DomainID("0"),
			LoginAt: timePointer(now.Add(-30 * 24 * time.Hour)),
		}},
		&supplierGateway{available: false},
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.(*serviceImpl).now = func() time.Time { return now }
	err = service.StartSubjectBinding(context.Background(), ProviderTencentESSBasic)
	if !bizerr.Is(err, CodeExternalEvidenceRequired) || issued != 0 || audit.transitions != 1 {
		t.Fatalf("error=%v issued=%d audits=%d", err, issued, audit.transitions)
	}
}

func TestAllSupplierWritesRequireActiveSessionBeforeStateAccess(t *testing.T) {
	now := time.Date(2026, time.August, 14, 8, 0, 0, 0, time.UTC)
	authorizer := &supplierAuthorizer{actor: 42}
	service, err := New(
		&supplierManager{},
		managementcap.Runtime{
			Authorizer: authorizer, Audit: &supplierAudit{},
			SensitiveAccess: supplierSensitiveAccess{},
		},
		&supplierSessions{current: &sessioncap.SessionInfo{
			ID: "session-1", UserID: "42", TenantID: capmodel.DomainID("0"),
			LoginAt: nil,
		}},
		&supplierGateway{available: false},
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.(*serviceImpl).now = func() time.Time { return now }
	tests := []struct {
		name       string
		permission string
		run        func() error
	}{
		{name: "credential", permission: PermissionCredentialWrite, run: func() error {
			_, runErr := service.SaveCredential(context.Background(), CredentialSaveInput{
				ProviderCode:       ProviderTencentESSBasic,
				CredentialEnvelope: []byte("opaque-local-envelope"),
				IdempotencyKey:     "supplier-credential-0001",
			})
			return runErr
		}},
		{name: "bind", permission: PermissionSubjectBind, run: func() error {
			return service.StartSubjectBinding(context.Background(), ProviderTencentESSBasic)
		}},
		{name: "revoke", permission: PermissionSubjectRevoke, run: func() error {
			_, runErr := service.RevokeSubject(context.Background(), SubjectRevokeInput{
				ProviderCode: ProviderTencentESSBasic,
				BindingNo:    "lsb_0123456789abcdef0123456789abcdef",
				ReasonCode:   "supplier_authorization_revoked", ExpectedVersion: 1,
				IdempotencyKey: "supplier-revoke-0001",
			})
			return runErr
		}},
		{name: "reopen", permission: PermissionSubjectReopen, run: func() error {
			return service.StartSubjectReopening(context.Background(), ProviderTencentESSBasic)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if runErr := test.run(); !bizerr.Is(runErr, CodeRecentAuthenticationRequired) {
				t.Fatalf("error=%v", runErr)
			}
			if authorizer.request.Permission != test.permission ||
				authorizer.request.TargetNo != configurationNo(ProviderTencentESSBasic) {
				t.Fatalf("authorization request=%+v", authorizer.request)
			}
		})
	}
}

func TestSubjectProofBindingSeparatesEveryOperationIntent(t *testing.T) {
	base := SubjectAuthorizationRequest{
		ProviderCode: ProviderTencentESSBasic, Operation: OperationSubjectBind,
		IdempotencyKey: "supplier-subject-0001",
	}
	baseDigest := subjectProofBinding(base).TargetDigest
	variants := []SubjectAuthorizationRequest{
		{ProviderCode: ProviderNuonuoInvoice, Operation: OperationSubjectBind, IdempotencyKey: base.IdempotencyKey},
		{ProviderCode: base.ProviderCode, Operation: OperationSubjectReopen,
			PreviousBindingNo: "lsb_0123456789abcdef0123456789abcdef", IdempotencyKey: base.IdempotencyKey},
		{ProviderCode: base.ProviderCode, Operation: base.Operation, IdempotencyKey: "supplier-subject-0002"},
		{ProviderCode: base.ProviderCode, Operation: base.Operation,
			ExpectedVersion: 1, IdempotencyKey: base.IdempotencyKey},
	}
	if managementcap.ValidateSensitiveAccessBinding(subjectProofBinding(base)) != nil {
		t.Fatal("base subject proof binding is invalid")
	}
	for _, variant := range variants {
		if digest := subjectProofBinding(variant).TargetDigest; digest == baseDigest {
			t.Fatalf("variant reused proof digest: %+v", variant)
		}
	}
}

func (g *supplierGateway) RecognizerAvailable(string) bool { return g.available }
func (g *supplierGateway) Bind(context.Context, string, []byte, string) (GatewayResult, error) {
	return GatewayResult{}, nil
}
func (g *supplierGateway) Revoke(
	context.Context, string, string, string, string, string, string,
) (GatewayResult, error) {
	return GatewayResult{}, nil
}
func (g *supplierGateway) Reopen(
	context.Context, string, string, string, []byte, string, string, string,
) (GatewayResult, error) {
	return GatewayResult{}, nil
}

func TestSupplierAuthorizationUsesExactInstallationTargetAndRecentLogin(t *testing.T) {
	now := time.Date(2026, time.August, 13, 8, 0, 0, 0, time.UTC)
	authorizer := &supplierAuthorizer{actor: 42}
	sessions := &supplierSessions{current: &sessioncap.SessionInfo{
		ID: "session-1", UserID: "42", TenantID: capmodel.DomainID("0"),
		LoginAt: timePointer(now.Add(-time.Minute)),
	}}
	service, err := New(
		&supplierManager{},
		managementcap.Runtime{
			Authorizer: authorizer, Audit: &supplierAudit{}, SensitiveAccess: supplierSensitiveAccess{},
		},
		sessions, &supplierGateway{},
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	implementation := service.(*serviceImpl)
	implementation.now = func() time.Time { return now }
	actor, err := implementation.authorize(
		context.Background(), PermissionCredentialWrite, ProviderTencentESSBasic,
	)
	if err != nil || actor != 42 {
		t.Fatalf("authorize actor=%d err=%v", actor, err)
	}
	if authorizer.request.Permission != PermissionCredentialWrite ||
		authorizer.request.TargetNo != "local-supplier-tencent_essbasic" {
		t.Fatalf("authorization request=%+v", authorizer.request)
	}
	if err = implementation.ensureRecentLogin(context.Background(), actor); err != nil {
		t.Fatalf("recent login: %v", err)
	}
	if !idccap.IsCatalogedAdminPermission(PermissionQuery) ||
		!idccap.IsCatalogedAdminPermission(PermissionCredentialWrite) ||
		!idccap.IsCatalogedAdminPermission(PermissionSubjectBind) ||
		!idccap.IsCatalogedAdminPermission(PermissionSubjectRevoke) ||
		!idccap.IsCatalogedAdminPermission(PermissionSubjectReopen) {
		t.Fatal("local supplier permissions are not exact catalog entries")
	}
}

func TestNormalizeSupplierSaveKeepsAdapterContractProviderNeutral(t *testing.T) {
	input := CredentialSaveInput{
		ProviderCode:       ProviderNuonuoInvoice,
		CredentialEnvelope: []byte("opaque-local-envelope"), ExpectedVersion: 0,
		IdempotencyKey: "supplier-config-0001",
	}
	normalized, digest, err := normalizeCredentialSave(input)
	if err != nil || normalized.ProviderCode != ProviderNuonuoInvoice ||
		!lowerHex64Pattern.MatchString(digest) {
		t.Fatalf("normalized=%+v digest=%q err=%v", normalized, digest, err)
	}
	invalid := input
	invalid.ProviderCode = "vendor_central_proxy"
	if _, _, err = normalizeCredentialSave(invalid); err == nil {
		t.Fatal("central proxy provider was accepted")
	}
}

func TestRecordManagedBindingRejectsNuoNuoProvider(t *testing.T) {
	implementation := &serviceImpl{}
	configuration, err := implementation.RecordManagedBinding(
		context.Background(),
		ProviderNuonuoInvoice,
		GatewayResult{BindingNo: "lsb_0123456789abcdef0123456789abcdef"},
	)
	if configuration != nil || !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("configuration=%+v error=%v", configuration, err)
	}
}

func timePointer(value time.Time) *time.Time { return &value }
