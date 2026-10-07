package onboardingaccount

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/onboardingaccount/v1"
)

type IOnboardingAccountV1 interface {
	Status(context.Context, *v1.StatusReq) (*v1.StatusRes, error)
	Redeem(context.Context, *v1.RedeemReq) (*v1.RedeemRes, error)
}
