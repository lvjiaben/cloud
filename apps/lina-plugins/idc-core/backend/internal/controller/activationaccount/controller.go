package activationaccount

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/activationaccount/v1"
	"lina-plugin-idc-core/backend/internal/service/customeractivation"
)

type ControllerV1 struct{ service customeractivation.Service }

func NewV1(service customeractivation.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("customer activation account controller requires service")
	}
	return &ControllerV1{service: service}, nil
}

func (c *ControllerV1) CustomerActivationRedeem(ctx context.Context, req *v1.CustomerActivationRedeemReq) (*v1.CustomerActivationRedeemRes, error) {
	item, err := c.service.Redeem(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerActivationRedeemRes{CustomerNo: item.CustomerNo, CustomerType: item.CustomerType, Status: item.Status, Reused: item.Reused}, nil
}

// CustomerInitialPasswordStatus returns the current customer's first-login gate.
func (c *ControllerV1) CustomerInitialPasswordStatus(ctx context.Context, _ *v1.CustomerInitialPasswordStatusReq) (*v1.CustomerInitialPasswordStatusRes, error) {
	required, err := c.service.InitialPasswordChangeRequired(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerInitialPasswordStatusRes{Required: required}, nil
}

// CustomerInitialPasswordChange replaces the initial password and clears the gate.
func (c *ControllerV1) CustomerInitialPasswordChange(ctx context.Context, req *v1.CustomerInitialPasswordChangeReq) (*v1.CustomerInitialPasswordChangeRes, error) {
	if err := c.service.ChangeInitialPassword(ctx, req.NewPassword); err != nil {
		return nil, err
	}
	return &v1.CustomerInitialPasswordChangeRes{Required: false}, nil
}
