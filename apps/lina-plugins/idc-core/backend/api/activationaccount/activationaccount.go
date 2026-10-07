package activationaccount

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/activationaccount/v1"
)

type IActivationAccountV1 interface {
	CustomerActivationRedeem(context.Context, *v1.CustomerActivationRedeemReq) (*v1.CustomerActivationRedeemRes, error)
	CustomerInitialPasswordStatus(context.Context, *v1.CustomerInitialPasswordStatusReq) (*v1.CustomerInitialPasswordStatusRes, error)
	CustomerInitialPasswordChange(context.Context, *v1.CustomerInitialPasswordChangeReq) (*v1.CustomerInitialPasswordChangeRes, error)
}
