// Package principal owns the single-merchant customer/staff audience boundary and first-customer redemption transaction.
package principal

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// AudienceCode is the mutually exclusive business audience of one host identity.
type AudienceCode string

// Supported principal audience codes.
const (
	AudienceCustomer AudienceCode = "customer"
	AudienceStaff    AudienceCode = "staff"
)

// AudienceStatus is the lifecycle status of one audience binding.
type AudienceStatus string

// Supported principal audience statuses.
const (
	AudienceStatusActive  AudienceStatus = "active"
	AudienceStatusRevoked AudienceStatus = "revoked"
)

// CustomerType is the first-release customer subject type.
type CustomerType string

// Supported customer subject types.
const (
	CustomerTypeIndividual CustomerType = "individual"
	CustomerTypeEnterprise CustomerType = "enterprise"
)

// Audience is the minimal current-identity audience projection. CustomerNo is
// populated only for a consistent customer audience and is never inferred.
type Audience struct {
	UserID     int
	Code       AudienceCode
	Status     AudienceStatus
	CustomerNo string
	Version    int64
}

// CustomerBinding is the result of a successful or safely replayed onboarding redemption.
type CustomerBinding struct {
	UserID       int
	CustomerNo   string
	CustomerType CustomerType
	Reused       bool
}

type OnboardingState struct {
	Required   bool
	CustomerNo string
}

// Service defines the single-merchant identity audience operations.
type Service interface {
	// Current returns the current authenticated identity's persisted audience.
	// It returns nil without writing when the identity is not yet bound, rejects
	// non-default tenants, and reports inconsistent customer bindings as errors.
	Current(ctx context.Context) (*Audience, error)
	// EnsureCurrentStaff idempotently binds the current identity as active staff
	// only after the host snapshot proves super-administrator status or a concrete
	// idc:admin:* permission with supported data scope. Existing customer or
	// revoked bindings are never converted implicitly.
	EnsureCurrentStaff(ctx context.Context) (*Audience, error)
	// OnboardingStatus performs a write-free check for the one narrow account
	// route that must be callable before a customer audience exists.
	OnboardingStatus(ctx context.Context) (*OnboardingState, error)
	// RedeemCustomerOnboarding consumes one already-verified, unexpired intent in
	// a transaction and creates exactly one customer subject and identity binding.
	// currentUserID must equal the authenticated bizctx identity; the raw token is
	// hashed before persistence access, and safe replay returns the original result.
	RedeemCustomerOnboarding(ctx context.Context, currentUserID int, intentToken string, browserNonce string, hostRegistrationUserID int) (*CustomerBinding, error)
}

// Reader exposes the write-free audience projection required by authorization
// and audit actor resolution without requiring onboarding write dependencies.
type Reader interface {
	Current(ctx context.Context) (*Audience, error)
}

// persistence is the atomic database boundary used by the principal service.
type persistence interface {
	// current reads one audience projection without creating business records.
	current(ctx context.Context, userID int) (*Audience, error)
	// ensureStaff atomically creates or reuses the active staff audience.
	ensureStaff(ctx context.Context, userID int) (*Audience, error)
	// redeem atomically consumes an intent and returns its unique customer binding.
	redeem(ctx context.Context, input redeemInput) (*CustomerBinding, error)
}

// redeemInput contains only normalized or digested values accepted by persistence.
type redeemInput struct {
	UserID                 int
	HostRegistrationUserID int
	DisplayName            string
	UsernameDigest         string
	TokenDigest            string
	BrowserDigest          string
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)

// serviceImpl implements the principal audience service.
type serviceImpl struct {
	bizCtxSvc          bizctxcap.Service
	authorization      authz.Service
	adminPermissionKey []authz.PermissionKey
	persistence        persistence
}

// New creates a principal service from host-published business-context and
// authorization services. Authorization is mandatory so a missing source-plugin
// bizctx permission projection can never be interpreted as customer eligibility.
func New(
	bizCtxSvc bizctxcap.Service,
	authorization authz.Service,
	auditWriter idccap.BusinessAuditWriter,
) (Service, error) {
	if bizCtxSvc == nil {
		return nil, gerror.New("principal service requires host bizctx service")
	}
	if authorization == nil {
		return nil, gerror.New("principal service requires host authorization service")
	}
	if auditWriter == nil {
		return nil, gerror.New("principal service requires business audit writer")
	}
	catalog := idccap.AdminPermissionCatalog()
	adminPermissionKey := make([]authz.PermissionKey, 0, len(catalog))
	for _, permission := range catalog {
		adminPermissionKey = append(adminPermissionKey, authz.PermissionKey(permission))
	}
	return &serviceImpl{
		bizCtxSvc:          bizCtxSvc,
		authorization:      authorization,
		adminPermissionKey: adminPermissionKey,
		persistence:        newDAOPersistence(auditWriter),
	}, nil
}

// NewReader creates the read-only audience projection used during startup
// composition before the audited onboarding writer is available.
func NewReader(bizCtxSvc bizctxcap.Service) (Reader, error) {
	if bizCtxSvc == nil {
		return nil, gerror.New("principal reader requires host bizctx service")
	}
	return &serviceImpl{bizCtxSvc: bizCtxSvc, persistence: newDAOPersistence()}, nil
}

// NewAuthorizedReader creates a read-only audience projection that can also
// prove a customer has not acquired effective backoffice authority when the
// source-plugin context omits a complete permission snapshot.
func NewAuthorizedReader(
	bizCtxSvc bizctxcap.Service,
	authorization authz.Service,
) (Reader, error) {
	if bizCtxSvc == nil || authorization == nil {
		return nil, gerror.New("authorized principal reader requires host identity services")
	}
	catalog := idccap.AdminPermissionCatalog()
	keys := make([]authz.PermissionKey, 0, len(catalog))
	for _, permission := range catalog {
		keys = append(keys, authz.PermissionKey(permission))
	}
	return &serviceImpl{
		bizCtxSvc: bizCtxSvc, authorization: authorization,
		adminPermissionKey: keys, persistence: newDAOPersistence(),
	}, nil
}
