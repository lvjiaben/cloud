// Package providerconfig owns the installation-wide Aliyun FACE credential configuration.
package providerconfig

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// PermissionQuery authorizes the installation-wide safe configuration projection.
	PermissionQuery = "idc:admin:compliance:provider:query"
	// PermissionWrite authorizes a high-risk merchant credential replacement.
	PermissionWrite = "idc:admin:compliance:provider:write"
	// PermissionValidate authorizes a read-only external credential connectivity probe.
	PermissionValidate = "idc:admin:compliance:provider:validate"

	configurationNo                    = "compliance-provider-aliyun-face"
	providerCode                       = "aliyun_face"
	freshLoginWindow                   = 5 * time.Minute
	allowedLoginClockSkew              = 15 * time.Second
	statusDraft                 Status = "draft"
	statusUpdating              Status = "updating"
	statusConfigured            Status = "configured"
	statusDisabled              Status = "disabled"
	auditTargetType                    = "compliance_provider_configuration"
	auditRiskHigh                      = "high"
	auditResultAccepted                = "succeeded"
	auditResultSucceeded               = "succeeded"
	auditResultFailed                  = "failed"
	auditReasonCredentialUpdate        = "merchant_credential_update"
	auditReasonValidation              = "provider_connectivity_validation"
)

// Status is the bounded provider-configuration lifecycle state.
type Status string

// ValidationStatus is the stable, non-secret connectivity projection.
type ValidationStatus string

const (
	// ValidationStatusUntested means the current credential generation has not been probed.
	ValidationStatusUntested ValidationStatus = "untested"
	// ValidationStatusReachable means the signed read-only FACE query reached the product.
	ValidationStatusReachable ValidationStatus = "reachable"
	// ValidationStatusRejected means credentials, entitlement, or scene were rejected.
	ValidationStatusRejected ValidationStatus = "rejected"
	// ValidationStatusUnavailable means the provider result was transport-ambiguous.
	ValidationStatusUnavailable ValidationStatus = "unavailable"
)

// Configuration is the only safe staff projection. It deliberately omits the
// secretcap handle, ciphertext, credentials, validation payloads, and identity data.
type Configuration struct {
	ProviderCode         string
	Status               Status
	SceneID              string
	CredentialConfigured bool
	CredentialMaskedTail string
	CredentialGeneration int64
	ValidationStatus     ValidationStatus
	Version              int64
	UpdatedAt            *time.Time
	ValidatedAt          *time.Time
}

// ConnectivityChecker performs only a synthetic read-only provider query and
// returns a bounded status without provider messages or credential material.
type ConnectivityChecker interface {
	Check(context.Context, secretcap.Handle, string) ValidationStatus
}

// SaveInput consumes and clears both credential byte slices before returning.
// ExpectedVersion is zero only for the first configuration.
type SaveInput struct {
	SceneID         string
	AccessKeyID     []byte
	AccessKeySecret []byte
	ExpectedVersion int64
	IdempotencyKey  string
}

// RecoveryInput consumes and clears a replacement credential pair used to
// resume the single persisted updating mutation after an interrupted request.
type RecoveryInput struct {
	AccessKeyID     []byte
	AccessKeySecret []byte
}

// Recovery reports the safe configured projection and whether secretcap
// reused the already completed secret mutation.
type Recovery struct {
	Configuration          *Configuration
	ReusedExistingMutation bool
}

// Service owns authorized query and idempotent two-phase credential replacement.
type Service interface {
	// Get authorizes the fixed installation target and returns only safe local
	// configuration fields plus secretcap status metadata.
	Get(context.Context) (*Configuration, error)
	// Save consumes credential bytes, stages an audited fail-closed mutation,
	// updates secretcap idempotently, and enables the new generation only after
	// its completion audit commits. A recent full-login session is required.
	Save(context.Context, SaveInput) (*Configuration, error)
	// Recover consumes credential bytes and resumes only the persisted updating
	// mutation with its server-generated operation key. It never marks an
	// unverified secret mutation successful. A recent full-login session is required.
	Recover(context.Context, RecoveryInput) (*Recovery, error)
	// ValidateConnectivity requires a recent full-login session, performs one
	// synthetic read-only query, and persists only its bounded status and time.
	ValidateConnectivity(context.Context) (*Configuration, error)
}

type serviceImpl struct {
	manager  secretcap.Manager
	runtime  managementcap.Runtime
	sessions sessioncap.Service
	checker  ConnectivityChecker
	now      func() time.Time
}

var _ Service = (*serviceImpl)(nil)

// New constructs the credential service from explicit startup capabilities.
func New(
	manager secretcap.Manager,
	runtime managementcap.Runtime,
	sessions sessioncap.Service,
	checker ConnectivityChecker,
) (Service, error) {
	if nilDependency(manager) || managementcap.ValidateRuntime(runtime) != nil ||
		nilDependency(sessions) || nilDependency(checker) {
		return nil, gerror.New("compliance provider configuration dependencies are invalid")
	}
	return &serviceImpl{
		manager: manager, runtime: runtime, sessions: sessions, checker: checker, now: time.Now,
	}, nil
}
