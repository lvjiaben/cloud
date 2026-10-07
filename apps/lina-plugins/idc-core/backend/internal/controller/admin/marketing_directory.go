// This file maps bounded marketing directory read models to public DTOs.

package admin

import (
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func toAPIMarketingSegmentSummary(item marketing.SegmentManagementSummary) *v1.MarketingSegmentSummaryItem {
	return &v1.MarketingSegmentSummaryItem{
		SegmentNo: item.SegmentNo, SegmentCode: item.SegmentCode, NameZhCn: item.NameZhCN, NameEnUs: item.NameEnUS,
		Mode: item.Mode, RefreshIntervalMinutes: item.RefreshIntervalMinutes, Status: item.Status, Version: item.Version,
		ConditionCount: item.ConditionCount, ActiveMemberCount: item.ActiveMemberCount, LatestSnapshotNo: item.LatestSnapshotNo,
		LatestSnapshotStatus: item.LatestSnapshotStatus, LatestSnapshotMembers: item.LatestSnapshotMembers,
		LatestSnapshotAt: toUnixMilliseconds(item.LatestSnapshotAt), CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}

func toAPIMarketingSegmentDetail(detail *marketing.SegmentManagementDetail) *v1.MarketingSegmentDetailRes {
	if detail == nil {
		return nil
	}
	result := &v1.MarketingSegmentDetailRes{
		Segment:     toAPIMarketingSegmentSummary(detail.SegmentManagementSummary),
		Description: &v1.MarketingBilingualDescription{ZhCn: detail.DescriptionZhCN, EnUs: detail.DescriptionEnUS},
		Conditions:  make([]*v1.MarketingSegmentCondition, 0, len(detail.Conditions)),
		Snapshots:   make([]*v1.MarketingSegmentSnapshot, 0, len(detail.Snapshots)),
	}
	for _, item := range detail.Conditions {
		condition := &v1.MarketingSegmentCondition{
			ConditionNo: item.ConditionNo, OrGroupNo: item.OrGroupNo, SortOrder: item.SortOrder, Fact: item.Fact,
			Operator: item.Operator, ValueKind: item.ValueKind, ValueText: item.ValueText,
			ValueIntegerLower: item.ValueIntegerLower, ValueIntegerUpper: item.ValueIntegerUpper,
			ValueBoolean: item.ValueBoolean, CurrencyCode: item.CurrencyCode,
			Values: make([]*v1.MarketingSegmentConditionValue, 0, len(item.Values)),
		}
		for _, value := range item.Values {
			condition.Values = append(condition.Values, &v1.MarketingSegmentConditionValue{Text: value.Text, Integer: value.Integer})
		}
		result.Conditions = append(result.Conditions, condition)
	}
	for _, item := range detail.Snapshots {
		result.Snapshots = append(result.Snapshots, &v1.MarketingSegmentSnapshot{
			SnapshotNo: item.SnapshotNo, SegmentVersion: item.SegmentVersion, Status: item.Status,
			MemberCount: item.MemberCount, StartedAt: item.StartedAt.UnixMilli(), CompletedAt: toUnixMilliseconds(item.CompletedAt),
		})
	}
	return result
}

func toAPIMarketingCampaignSummary(item marketing.CampaignManagementSummary) *v1.MarketingCampaignManagementItem {
	return &v1.MarketingCampaignManagementItem{
		CampaignNo: item.CampaignNo, CampaignCode: item.CampaignCode, NameZhCn: item.NameZhCN, NameEnUs: item.NameEnUS,
		Type: item.Type, Status: item.Status, TargetSegmentNo: item.TargetSegmentNo, TargetSnapshotNo: item.TargetSnapshotNo,
		Priority: item.Priority, StackPolicy: item.StackPolicy, GlobalQuotaLimit: item.GlobalQuotaLimit,
		GlobalReservedCount: item.GlobalReservedCount, GlobalConsumedCount: item.GlobalConsumedCount,
		PerCustomerQuotaLimit: item.PerCustomerQuotaLimit, IsNewcomerExclusive: item.IsNewcomerExclusive,
		ScopeCount: item.ScopeCount, BenefitCount: item.BenefitCount,
		ReservationCount: item.ReservationCount, StartsAt: item.StartsAt.UnixMilli(), EndsAt: item.EndsAt.UnixMilli(),
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}

func toAPIMarketingCampaignDetail(detail *marketing.CampaignManagementDetail) *v1.MarketingCampaignDetailRes {
	if detail == nil {
		return nil
	}
	result := &v1.MarketingCampaignDetailRes{
		Campaign:    toAPIMarketingCampaignSummary(detail.CampaignManagementSummary),
		Description: &v1.MarketingBilingualDescription{ZhCn: detail.DescriptionZhCN, EnUs: detail.DescriptionEnUS},
		StackGroup:  detail.StackGroup, Version: detail.Version,
		Scopes:   make([]*v1.MarketingCampaignScope, 0, len(detail.Scopes)),
		Benefits: make([]*v1.MarketingCampaignBenefit, 0, len(detail.Benefits)),
	}
	for _, item := range detail.Scopes {
		result.Scopes = append(result.Scopes, &v1.MarketingCampaignScope{Kind: item.Kind, MatchMode: item.MatchMode, ReferenceCode: item.ReferenceCode})
	}
	for _, item := range detail.Benefits {
		result.Benefits = append(result.Benefits, &v1.MarketingCampaignBenefit{
			BenefitNo: item.BenefitNo, Type: item.Type, CurrencyCode: item.CurrencyCode, AmountMinor: item.AmountMinor,
			RateBasisPoints: item.RateBasisPoints, MaximumDiscountMinor: item.MaximumDiscountMinor,
			MinimumSpendMinor: item.MinimumSpendMinor, PointsQuantity: item.PointsQuantity,
			PurchasePhase: item.PurchasePhase, CycleScope: item.CycleScope, CycleCount: item.CycleCount,
			Status: item.Status, Version: item.Version,
		})
	}
	return result
}
