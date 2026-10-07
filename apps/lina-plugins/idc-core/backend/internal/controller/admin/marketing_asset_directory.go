// This file maps bounded marketing asset directory models to API DTOs.

package admin

import (
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func toAPIMarketingPromotionAsset(item marketing.PromotionAssetSummary) *v1.MarketingPromotionAssetItem {
	return &v1.MarketingPromotionAssetItem{
		AssetNo: item.AssetNo, CampaignNo: item.CampaignNo, BenefitNo: item.BenefitNo, BenefitType: item.BenefitType,
		Type: item.Type, CustomerNo: item.CustomerNo, CurrencyCode: item.CurrencyCode, FaceAmountMinor: item.FaceAmountMinor,
		RateBasisPoints: item.RateBasisPoints, UsageLimit: item.UsageLimit, ReservedUses: item.ReservedUses, ConsumedUses: item.ConsumedUses,
		Status: item.Status, ValidFrom: item.ValidFrom.UnixMilli(), ValidUntil: item.ValidUntil.UnixMilli(), ClaimedAt: toUnixMilliseconds(item.ClaimedAt),
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}

func toAPIMarketingPointsAccount(item marketing.PointsAccountSummary) *v1.MarketingPointsAccountItem {
	return &v1.MarketingPointsAccountItem{
		CustomerNo: item.CustomerNo, AvailablePoints: item.AvailablePoints, FrozenPoints: item.FrozenPoints, DebtPoints: item.DebtPoints,
		Version: item.Version, LotCount: item.LotCount, EntryCount: item.EntryCount,
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}

func toAPIMarketingValueGrant(item marketing.ValueGrantSummary) *v1.MarketingValueGrantItem {
	return &v1.MarketingValueGrantItem{
		GrantNo: item.GrantNo, Type: item.Type, CampaignNo: item.CampaignNo, BenefitNo: item.BenefitNo,
		CustomerNo: item.CustomerNo, CurrencyCode: item.CurrencyCode, SourceReference: item.SourceReference,
		OriginalAmountMinor: item.OriginalAmountMinor, PendingAmountMinor: item.PendingAmountMinor,
		AvailableAmountMinor: item.AvailableAmountMinor, FrozenAmountMinor: item.FrozenAmountMinor,
		ConsumedAmountMinor: item.ConsumedAmountMinor, ExpiredAmountMinor: item.ExpiredAmountMinor,
		ReversedAmountMinor: item.ReversedAmountMinor, Withdrawable: item.Withdrawable,
		GlobalQuotaReserved: item.GlobalQuotaReserved, CustomerQuotaReserved: item.CustomerQuotaReserved,
		ExpiresAt: toUnixMilliseconds(item.ExpiresAt), Status: item.Status,
		FinancialPostingReference: item.FinancialPostingReference,
		CreatedAt:                 item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}
