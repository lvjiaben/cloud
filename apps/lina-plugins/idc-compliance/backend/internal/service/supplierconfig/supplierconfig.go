// Package supplierconfig owns installation-local merchant supplier credentials.
// Real supplier clients remain deliberately absent until external evidence gates pass.
package supplierconfig

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// ProviderNuonuoInvoice identifies the reviewed NuoNuo supplier target.
	ProviderNuonuoInvoice = "nuonuo_invoice"

	// PermissionQuery allows reading the secret-safe supplier projection.
	PermissionQuery = "idc:admin:compliance:supplier:query"
	// PermissionCredentialWrite allows replacing merchant-owned credentials.
	PermissionCredentialWrite = "idc:admin:compliance:supplier:credential:write"
	// PermissionSubjectBind allows starting a new supplier-subject binding.
	PermissionSubjectBind = "idc:admin:compliance:supplier:subject:bind"
	// PermissionSubjectRevoke allows revoking one local opaque binding.
	PermissionSubjectRevoke = "idc:admin:compliance:supplier:subject:revoke"
	// PermissionSubjectReopen allows reopening one revoked local binding.
	PermissionSubjectReopen = "idc:admin:compliance:supplier:subject:reopen"

	// EvidenceExternalConfirmationRequired keeps real provider clients disabled.
	EvidenceExternalConfirmationRequired = "external_confirmation_required"
	// EvidenceOfficialPublicContract identifies Tencent's reviewed public API contract.
	EvidenceOfficialPublicContract = "official_public_contract"

	// CredentialUnconfigured means no merchant credential is stored.
	CredentialUnconfigured CredentialStatus = "unconfigured"
	// CredentialUpdating means a governed secret mutation is in progress.
	CredentialUpdating CredentialStatus = "updating"
	// CredentialConfigured means the governed secret reference is ready.
	CredentialConfigured CredentialStatus = "configured"
	// CredentialRetryableFailed means the last credential mutation may be retried.
	CredentialRetryableFailed CredentialStatus = "retryable_failed"

	// SubjectUnbound means no supplier subject has been bound.
	SubjectUnbound SubjectStatus = "unbound"
	// SubjectBinding means one binding operation is pending completion.
	SubjectBinding SubjectStatus = "binding"
	// SubjectActive means the local opaque supplier binding is active.
	SubjectActive SubjectStatus = "active"
	// SubjectRevoked means the local opaque supplier binding is revoked.
	SubjectRevoked SubjectStatus = "revoked"
	// SubjectReopening means a reviewed reopen operation is pending.
	SubjectReopening SubjectStatus = "reopening"
	// SubjectRetryableFailed means the last subject operation may be retried.
	SubjectRetryableFailed SubjectStatus = "retryable_failed"

	// OperationNone means no supplier-subject operation has been recorded.
	OperationNone OperationKind = "none"
	// OperationSubjectBind identifies a supplier-subject binding operation.
	OperationSubjectBind OperationKind = "subject_bind"
	// OperationSubjectRevoke identifies a supplier-subject revocation operation.
	OperationSubjectRevoke OperationKind = "subject_revoke"
	// OperationSubjectReopen identifies a supplier-subject reopening operation.
	OperationSubjectReopen OperationKind = "subject_reopen"

	// OperationResultNone means no supplier-subject operation result exists.
	OperationResultNone OperationResult = "none"
	// OperationResultPending means the operation awaits gateway completion.
	OperationResultPending OperationResult = "pending"
	// OperationResultSucceeded means the gateway operation completed.
	OperationResultSucceeded OperationResult = "succeeded"
	// OperationResultRetryableFailed means the operation can be safely retried.
	OperationResultRetryableFailed OperationResult = "retryable_failed"

	freshLoginWindow      = 5 * time.Minute
	allowedLoginClockSkew = 15 * time.Second
)

// CredentialStatus is the independently persisted merchant credential state.
type CredentialStatus string

// SubjectStatus is the independently persisted local supplier-subject state.
type SubjectStatus string

// OperationKind identifies the last supplier-subject write kind.
type OperationKind string

// OperationResult identifies the last supplier-subject write outcome.
type OperationResult string

// Configuration is a secret-safe, installation-local projection.
type Configuration struct {
	ProviderCode           string
	CredentialStatus       CredentialStatus
	CredentialConfigured   bool
	CredentialMaskedTail   string
	CredentialGeneration   int64
	SubjectStatus          SubjectStatus
	SubjectBindingNo       string
	LastOperationKind      OperationKind
	LastOperationResult    OperationResult
	LastOperationNo        string
	LastOperationAt        *time.Time
	CapabilityEvidenceCode string
	AdapterEnabled         bool
	Version                int64
	UpdatedAt              *time.Time
}

// CredentialSaveInput carries an opaque provider-neutral credential envelope. External
// provider field contracts are not frozen until their evidence gates pass.
type CredentialSaveInput struct {
	ProviderCode       string
	CredentialEnvelope []byte
	ExpectedVersion    int64
	IdempotencyKey     string
}

// SubjectAuthorizationRequest binds a one-time recent-authentication proof to
// one exact provider operation. Supplier subject bytes are deliberately absent.
type SubjectAuthorizationRequest struct {
	ProviderCode      string
	Operation         OperationKind
	PreviousBindingNo string
	ExpectedVersion   int64
	IdempotencyKey    string
}

// SubjectAuthorizationContext is server-only and must never be returned by an
// ordinary management API.
type SubjectAuthorizationContext struct {
	Token     string
	ExpiresAt time.Time
}

// SubjectAuthorizationResult is supplied only by a future reviewed provider
// recognizer after it has completed supplier-side authorization.
type SubjectAuthorizationResult struct {
	Request             SubjectAuthorizationRequest
	ProofToken          string
	AuthorizationResult []byte
}

// SubjectRevokeInput selects one local opaque binding for revocation.
type SubjectRevokeInput struct {
	ProviderCode    string
	BindingNo       string
	ReasonCode      string
	ExpectedVersion int64
	IdempotencyKey  string
}

// GatewayResult contains only installation-local opaque identifiers.
type GatewayResult struct {
	OperationNo string
	BindingNo   string
	Replayed    bool
}

// SubjectGateway is the only plugin seam allowed to send supplier
// authorization results into the installation-local compliance gateway.
type SubjectGateway interface {
	// RecognizerAvailable reports whether external evidence has enabled a provider recognizer.
	RecognizerAvailable(provider string) bool
	// Bind sends a reviewed authorization result to the installation-local gateway.
	Bind(context.Context, string, []byte, string) (GatewayResult, error)
	// Revoke revokes one local opaque binding and never accepts supplier plaintext.
	Revoke(context.Context, string, string, string, string, string, string) (GatewayResult, error)
	// Reopen sends a reviewed authorization result and server-derived employee evidence.
	Reopen(context.Context, string, string, string, []byte, string, string, string) (GatewayResult, error)
}

// Service governs the two allowed local supplier targets.
type Service interface {
	// Get authorizes the caller and returns one secret-safe installation projection.
	Get(context.Context, string) (*Configuration, error)
	// SaveCredential requires recent full login and stores only a governed secret reference.
	SaveCredential(context.Context, CredentialSaveInput) (*Configuration, error)
	// StartSubjectBinding applies permission, recent-login, and external-evidence gates.
	StartSubjectBinding(context.Context, string) error
	// StartSubjectReopening applies permission, recent-login, and external-evidence gates.
	StartSubjectReopening(context.Context, string) error
	// IssueSubjectAuthorization issues a five-minute, single-use, exact-target proof.
	IssueSubjectAuthorization(context.Context, SubjectAuthorizationRequest) (*SubjectAuthorizationContext, error)
	// CompleteSubjectAuthorization consumes the proof and clears provider result bytes after use.
	CompleteSubjectAuthorization(context.Context, SubjectAuthorizationResult) (*Configuration, error)
	// RevokeSubject requires recent full login and revokes one local opaque binding.
	RevokeSubject(context.Context, SubjectRevokeInput) (*Configuration, error)
}

// serviceImpl owns the invoice supplier runtime dependencies.
type serviceImpl struct {
	manager  secretcap.Manager
	runtime  managementcap.Runtime
	sessions sessioncap.Service
	gateway  SubjectGateway
	now      func() time.Time
}

var _ Service = (*serviceImpl)(nil)

// New creates the installation-local supplier configuration service.
func New(
	manager secretcap.Manager,
	runtime managementcap.Runtime,
	sessions sessioncap.Service,
	gateway SubjectGateway,
) (Service, error) {
	if nilDependency(manager) || managementcap.ValidateRuntime(runtime) != nil ||
		nilDependency(runtime.SensitiveAccess) || nilDependency(sessions) || nilDependency(gateway) {
		return nil, gerror.New("local supplier configuration dependencies are invalid")
	}
	return &serviceImpl{
		manager: manager, runtime: runtime, sessions: sessions, gateway: gateway, now: time.Now,
	}, nil
}

func validProvider(provider string) bool {
	return provider == ProviderNuonuoInvoice
}

func configurationNo(provider string) string { return "local-supplier-" + provider }

func nilDependency(value any) bool {
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
