package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) MarketingPointsAccountDetail(ctx context.Context, req *v1.MarketingPointsAccountDetailReq) (*v1.MarketingPointsAccountDetailRes, error) {
	detail, err := c.marketingAssetDirectory.GetPointsAccount(ctx, req.CustomerNo)
	if err != nil {
		return nil, err
	}
	lots := make([]*v1.MarketingPointsLotItem, 0, len(detail.Lots))
	for _, item := range detail.Lots {
		lots = append(lots, &v1.MarketingPointsLotItem{
			LotNo: item.LotNo, CampaignNo: item.CampaignNo, SourceType: item.SourceType, SourceReference: item.SourceReference,
			OriginalPoints: item.OriginalPoints, AvailablePoints: item.AvailablePoints, FrozenPoints: item.FrozenPoints,
			ConsumedPoints: item.ConsumedPoints, ExpiredPoints: item.ExpiredPoints, ReversedPoints: item.ReversedPoints,
			ReversalShortfallPoints: item.ReversalShortfallPoints, ExpiresAt: toUnixMilliseconds(item.ExpiresAt),
			Status: item.Status, CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	entries := make([]*v1.MarketingPointsEntryItem, 0, len(detail.Entries))
	for _, item := range detail.Entries {
		entries = append(entries, &v1.MarketingPointsEntryItem{
			EntryNo: item.EntryNo, LotNo: item.LotNo, Type: item.Type, AvailableDelta: item.AvailableDelta,
			FrozenDelta: item.FrozenDelta, DebtDelta: item.DebtDelta, SourceType: item.SourceType,
			SourceReference: item.SourceReference, ReversalOfEntryNo: item.ReversalOfEntryNo, CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return &v1.MarketingPointsAccountDetailRes{Account: toAPIMarketingPointsAccount(detail.PointsAccountSummary), Lots: lots, Entries: entries}, nil
}
