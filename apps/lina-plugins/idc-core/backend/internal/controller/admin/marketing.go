// This file maps internal marketing read models to public administration DTOs.

package admin

import (
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

// toAPIMarketingOverview converts one validated service snapshot to Unix-millisecond API values.
func toAPIMarketingOverview(overview *marketing.ManagementOverview) *v1.MarketingOverviewGetRes {
	if overview == nil {
		return nil
	}
	result := &v1.MarketingOverviewGetRes{
		GeneratedAt: overview.GeneratedAt.UnixMilli(),
		Summary: &v1.MarketingOverviewSummary{
			SegmentTotal: overview.Summary.SegmentTotal, SegmentActive: overview.Summary.SegmentActive,
			SegmentDynamic: overview.Summary.SegmentDynamic, ActiveSegmentMembers: overview.Summary.ActiveSegmentMembers,
			CampaignTotal: overview.Summary.CampaignTotal, CampaignActive: overview.Summary.CampaignActive,
			CampaignScheduled: overview.Summary.CampaignScheduled, CampaignPaused: overview.Summary.CampaignPaused,
			CampaignReservedUnits: overview.Summary.CampaignReservedUnits,
			CampaignConsumedUnits: overview.Summary.CampaignConsumedUnits,
			AssetTotal:            overview.Summary.AssetTotal, AssetIssued: overview.Summary.AssetIssued,
			AssetActive: overview.Summary.AssetActive, AssetReservedUses: overview.Summary.AssetReservedUses,
			AssetConsumedUses:  overview.Summary.AssetConsumedUses,
			PointsAccountCount: overview.Summary.PointsAccountCount,
			PointsAvailable:    overview.Summary.PointsAvailable, PointsFrozen: overview.Summary.PointsFrozen,
			PointsDebt: overview.Summary.PointsDebt, ConsentOptedIn: overview.Summary.ConsentOptedIn,
			ConsentOptedOut: overview.Summary.ConsentOptedOut,
		},
		Currencies:      make([]*v1.MarketingCurrencySummary, 0, len(overview.Currencies)),
		RecentCampaigns: make([]*v1.MarketingCampaignSummary, 0, len(overview.RecentCampaigns)),
		RecentGrants:    make([]*v1.MarketingGrantSummary, 0, len(overview.RecentGrants)),
	}
	for _, item := range overview.Currencies {
		result.Currencies = append(result.Currencies, &v1.MarketingCurrencySummary{
			CurrencyCode: item.CurrencyCode, GrantCount: item.GrantCount,
			PendingMinor: item.PendingMinor, AvailableMinor: item.AvailableMinor,
			FrozenMinor: item.FrozenMinor, ConsumedMinor: item.ConsumedMinor,
			ExpiredMinor: item.ExpiredMinor, ReversedMinor: item.ReversedMinor,
		})
	}
	for _, item := range overview.RecentCampaigns {
		result.RecentCampaigns = append(result.RecentCampaigns, &v1.MarketingCampaignSummary{
			CampaignNo: item.CampaignNo, NameZhCn: item.NameZhCN, NameEnUs: item.NameEnUS,
			Type: item.Type, Status: item.Status, TargetSegmentNo: item.TargetSegmentNo,
			TargetSnapshotNo: item.TargetSnapshotNo, Priority: item.Priority,
			GlobalQuotaLimit: item.GlobalQuotaLimit, GlobalReservedCount: item.GlobalReservedCount,
			GlobalConsumedCount: item.GlobalConsumedCount, StartsAt: item.StartsAt.UnixMilli(),
			EndsAt: item.EndsAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
		})
	}
	for _, item := range overview.RecentGrants {
		apiItem := &v1.MarketingGrantSummary{
			GrantNo: item.GrantNo, GrantType: item.GrantType, CampaignNo: item.CampaignNo,
			CustomerNo: item.CustomerNo, CurrencyCode: item.CurrencyCode,
			OriginalMinor: item.OriginalMinor, PendingMinor: item.PendingMinor,
			AvailableMinor: item.AvailableMinor, FrozenMinor: item.FrozenMinor,
			ConsumedMinor: item.ConsumedMinor, ExpiredMinor: item.ExpiredMinor,
			ReversedMinor: item.ReversedMinor, Withdrawable: item.Withdrawable,
			Status: item.Status, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
		}
		if item.ExpiresAt != nil {
			value := item.ExpiresAt.UnixMilli()
			apiItem.ExpiresAt = &value
		}
		result.RecentGrants = append(result.RecentGrants, apiItem)
	}
	return result
}
