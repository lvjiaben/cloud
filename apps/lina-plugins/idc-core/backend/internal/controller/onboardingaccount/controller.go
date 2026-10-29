package onboardingaccount

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	api "lina-plugin-idc-core/backend/api/onboardingaccount"
	v1 "lina-plugin-idc-core/backend/api/onboardingaccount/v1"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type ControllerV1 struct {
	directory customerlogincap.Directory
	principal principal.Service
	signals   marketing.RegistrationSignalRecorder
}

func NewV1(service principal.Service, signals marketing.RegistrationSignalRecorder, directory customerlogincap.Directory) (*ControllerV1, error) {
	if service == nil || signals == nil || directory == nil {
		return nil, gerror.New("customer onboarding controller requires principal and registration signal services")
	}
	return &ControllerV1{principal: service, signals: signals, directory: directory}, nil
}

var _ api.IOnboardingAccountV1 = (*ControllerV1)(nil)

func (c *ControllerV1) Status(ctx context.Context, _ *v1.StatusReq) (*v1.StatusRes, error) {
	state, err := c.principal.OnboardingStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.StatusRes{Required: state.Required, CustomerNo: state.CustomerNo}, nil
}

func (c *ControllerV1) Redeem(ctx context.Context, req *v1.RedeemReq) (*v1.RedeemRes, error) {
	result, err := c.principal.RedeemCustomerOnboarding(ctx, req.HostRegistrationUserID, req.Token, req.BrowserNonce, req.HostRegistrationUserID)
	if err != nil {
		return nil, err
	}
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, gerror.New("customer onboarding request context is unavailable")
	}
	if _, err = c.signals.RecordRegistration(ctx, marketing.RegistrationSignalInput{
		CustomerNo: result.CustomerNo, DeviceID: req.DeviceId, SourceIP: request.GetClientIp(),
	}); err != nil {
		return nil, err
	}
	if err = c.directory.RefreshCurrent(ctx); err != nil {
		return nil, err
	}
	return &v1.RedeemRes{CustomerNo: result.CustomerNo, CustomerType: string(result.CustomerType), Reused: result.Reused}, nil
}
