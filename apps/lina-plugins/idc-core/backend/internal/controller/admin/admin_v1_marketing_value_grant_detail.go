package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) MarketingValueGrantDetail(ctx context.Context, req *v1.MarketingValueGrantDetailReq) (*v1.MarketingValueGrantDetailRes, error) {
	detail, err := c.marketingAssetDirectory.GetValueGrant(ctx, req.GrantNo)
	if err != nil {
		return nil, err
	}
	entries := make([]*v1.MarketingValueEntryItem, 0, len(detail.Entries))
	for _, item := range detail.Entries {
		entries = append(entries, &v1.MarketingValueEntryItem{
			EntryNo: item.EntryNo, Type: item.Type, PendingDeltaMinor: item.PendingDeltaMinor,
			AvailableDeltaMinor: item.AvailableDeltaMinor, FrozenDeltaMinor: item.FrozenDeltaMinor,
			ConsumedDeltaMinor: item.ConsumedDeltaMinor, ExpiredDeltaMinor: item.ExpiredDeltaMinor,
			ReversedDeltaMinor: item.ReversedDeltaMinor, SourceType: item.SourceType, SourceReference: item.SourceReference,
			ReversalOfEntryNo: item.ReversalOfEntryNo, CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return &v1.MarketingValueGrantDetailRes{Grant: toAPIMarketingValueGrant(detail.ValueGrantSummary), Entries: entries}, nil
}
