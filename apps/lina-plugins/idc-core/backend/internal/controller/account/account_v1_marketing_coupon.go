package account

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingCouponList(ctx context.Context, _ *v1.MarketingCouponListReq) (*v1.MarketingCouponListRes, error) {
	if c.coupons == nil {
		return nil, gerror.New("customer coupon service is unavailable")
	}
	items, err := c.coupons.ListMine(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.MarketingCouponItem, 0, len(items))
	for index := range items {
		result = append(result, toAPIMarketingCoupon(&items[index]))
	}
	return &v1.MarketingCouponListRes{Items: result}, nil
}

func (c *ControllerV1) MarketingCouponClaim(ctx context.Context, req *v1.MarketingCouponClaimReq) (*v1.MarketingCouponClaimRes, error) {
	if c.coupons == nil {
		return nil, gerror.New("customer coupon service is unavailable")
	}
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, gerror.New("customer coupon request context is unavailable")
	}
	item, err := c.coupons.Claim(ctx, marketing.CouponClaimInput{
		Code: req.Code, DeviceID: req.DeviceID, SourceIP: request.GetClientIp(),
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingCouponClaimRes{Item: toAPIMarketingCoupon(item)}, nil
}

func toAPIMarketingCoupon(item *marketing.CouponItem) *v1.MarketingCouponItem {
	if item == nil {
		return nil
	}
	result := &v1.MarketingCouponItem{
		AssetNo: item.AssetNo, CampaignNo: item.CampaignNo, NameZhCn: item.NameZhCN, NameEnUs: item.NameEnUS,
		AssetType: item.AssetType, CurrencyCode: item.CurrencyCode, FaceAmountMinor: item.FaceAmountMinor,
		RateBasisPoints: item.RateBasisPoints, UsageLimit: item.UsageLimit, ConsumedUses: item.ConsumedUses,
		Status: item.Status, ValidFrom: item.ValidFrom.UnixMilli(), ValidUntil: item.ValidUntil.UnixMilli(),
	}
	if item.ClaimedAt != nil {
		value := item.ClaimedAt.UnixMilli()
		result.ClaimedAt = &value
	}
	return result
}
