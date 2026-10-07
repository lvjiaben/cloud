package customeraccount

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/api/customeraccount/v1"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

func (c *ControllerV1) ReferralLinkCreate(ctx context.Context, req *v1.ReferralLinkCreateReq) (*v1.ReferralLinkCreateRes, error) {
	profile, err := c.customers.CurrentProfile(ctx)
	if err != nil {
		return nil, err
	}
	input := saleschannel.CustomerReferralLinkRequest{
		PolicyCode: req.PolicyCode, AttributionWindowSeconds: req.AttributionWindowSeconds,
		IdempotencyKey: req.IdempotencyKey,
	}
	if req.ExpiresAtUnixMilli != nil {
		expiresAt := time.UnixMilli(*req.ExpiresAtUnixMilli)
		input.ExpiresAt = &expiresAt
	}
	result, err := c.salesChannel.CreateCustomerReferralLink(ctx, profile.Subject.CustomerNo, input)
	if err != nil {
		return nil, err
	}
	return &v1.ReferralLinkCreateRes{Item: &v1.CustomerReferralLink{
		LinkNo: result.Item.LinkNo, Policy: result.Item.Policy, Status: result.Item.Status,
	}, Reused: result.Reused}, nil
}
