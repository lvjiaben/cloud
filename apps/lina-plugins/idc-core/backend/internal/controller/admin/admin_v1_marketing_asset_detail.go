package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) MarketingAssetDetail(ctx context.Context, req *v1.MarketingAssetDetailReq) (*v1.MarketingAssetDetailRes, error) {
	detail, err := c.marketingAssetDirectory.GetAsset(ctx, req.AssetNo)
	if err != nil {
		return nil, err
	}
	reservations := make([]*v1.MarketingAssetReservationItem, 0, len(detail.Reservations))
	for _, item := range detail.Reservations {
		reservations = append(reservations, &v1.MarketingAssetReservationItem{
			ReservationNo: item.ReservationNo, ReservedUses: item.ReservedUses, DiscountAmountMinor: item.DiscountAmountMinor,
			Status: item.Status, ExpiresAt: item.ExpiresAt.UnixMilli(), CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return &v1.MarketingAssetDetailRes{Asset: toAPIMarketingPromotionAsset(detail.PromotionAssetSummary), Reservations: reservations}, nil
}
