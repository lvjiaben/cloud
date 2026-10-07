// Package electroniccontractonboarding owns the management-side Tencent setup projection.
package electroniccontractonboarding

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// providerCode is the only supplier supported by this managed onboarding flow.
const providerCode = supplierconfig.ProviderTencentESSBasic

// Merchant is the read-only enterprise profile shown in step one.
type Merchant struct {
	LegalName   string
	CountryCode string
	MerchantNo  string
}

// View is the secret-free four-step onboarding state.
type View struct {
	Merchant          Merchant
	State             string
	ReasonCode        string
	ConsoleURL        string
	OrganizationReady bool
	OperatorReady     bool
	BindingNo         string
	SubjectStatus     string
	Capabilities      []localgatewayclient.TencentOnboardingCapability
}

// Service exposes only page loading, official-link creation, and explicit recheck.
type Service interface {
	// Get returns the current secret-free local projection. A missing gateway is
	// represented as a closed state; invalid local dependencies return an error.
	Get(context.Context) (*View, error)
	// CreateLink accepts an 8-to-128-character retry key and creates one official
	// short-lived Tencent URL after permission and recent-login checks.
	CreateLink(context.Context, string) (*View, error)
	// Recheck accepts an 8-to-128-character retry key and refreshes Tencent's
	// authoritative state without returning a newly generated URL.
	Recheck(context.Context, string) (*View, error)
}

type gateway interface {
	// TencentOnboardingStatus reads only the installation-local projection and
	// returns an error when the signed local call cannot be completed.
	TencentOnboardingStatus(context.Context) (localgatewayclient.TencentOnboardingResult, error)
	// CreateTencentOnboardingLink requests one official short-lived Tencent URL;
	// an empty result is invalid and transport or gateway failures return an error.
	CreateTencentOnboardingLink(context.Context, string, string) (localgatewayclient.TencentOnboardingResult, error)
	// RecheckTencentOnboarding refreshes activation flags without exposing a URL;
	// transport or gateway failures return an error.
	RecheckTencentOnboarding(context.Context, string, string) (localgatewayclient.TencentOnboardingResult, error)
}

type serviceImpl struct {
	merchants idccap.Service
	suppliers supplierconfig.Service
	gateway   gateway
}

var _ Service = (*serviceImpl)(nil)

// New constructs the installation-scoped onboarding service.
func New(
	merchants idccap.Service,
	suppliers supplierconfig.Service,
	localGateway gateway,
) (Service, error) {
	if merchants == nil || suppliers == nil || localGateway == nil {
		return nil, gerror.New("electronic contract onboarding dependencies are invalid")
	}
	return &serviceImpl{merchants: merchants, suppliers: suppliers, gateway: localGateway}, nil
}
