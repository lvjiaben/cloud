// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package onboardingaccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/onboardingaccount/v1"
)

type IOnboardingaccountV1 interface {
	Status(ctx context.Context, req *v1.StatusReq) (res *v1.StatusRes, err error)
	Redeem(ctx context.Context, req *v1.RedeemReq) (res *v1.RedeemRes, err error)
}
