// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package activationaccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/activationaccount/v1"
)

type IActivationaccountV1 interface {
	CustomerActivationRedeem(ctx context.Context, req *v1.CustomerActivationRedeemReq) (res *v1.CustomerActivationRedeemRes, err error)
	CustomerInitialPasswordStatus(ctx context.Context, req *v1.CustomerInitialPasswordStatusReq) (res *v1.CustomerInitialPasswordStatusRes, err error)
	CustomerInitialPasswordChange(ctx context.Context, req *v1.CustomerInitialPasswordChangeReq) (res *v1.CustomerInitialPasswordChangeRes, err error)
}
