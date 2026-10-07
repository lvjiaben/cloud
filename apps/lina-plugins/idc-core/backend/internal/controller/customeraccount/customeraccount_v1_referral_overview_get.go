package customeraccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
)

func (c *ControllerV1) ReferralOverviewGet(ctx context.Context, req *v1.ReferralOverviewGetReq) (res *v1.ReferralOverviewGetRes, err error) {
	profile, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.salesChannel.LoadCustomerReferral(ctx, profile.Subject.CustomerNo)
	if err != nil {
		return nil, err
	}
	return &v1.ReferralOverviewGetRes{ReferralCode: item.ReferralCode, ReferralURL: item.ReferralURL, AttributionPolicyCode: item.AttributionPolicyCode, AttributionExpiresAt: item.AttributionExpiresAt, ReferredCustomerCount: item.ReferredCustomerCount, AttributedOrderCount: item.AttributedOrderCount, QualifiedOrderCount: item.QualifiedOrderCount, RejectedAttributionCount: item.RejectedAttributionCount}, nil
}
