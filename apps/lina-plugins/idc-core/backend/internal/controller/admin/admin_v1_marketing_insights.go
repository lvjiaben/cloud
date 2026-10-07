package admin

import (
	"context"
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingPerformanceList(ctx context.Context, req *v1.MarketingPerformanceListReq) (*v1.MarketingPerformanceListRes, error) {
	report, err := c.marketingInsights.Performance(ctx, marketing.MarketingPerformanceInput{
		StartedAt: time.UnixMilli(req.StartedAt), EndedAt: time.UnixMilli(req.EndedAt),
		Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]*v1.MarketingPerformanceItem, 0, len(report.Rows))
	for _, item := range report.Rows {
		rows = append(rows, &v1.MarketingPerformanceItem{
			CampaignNo: item.CampaignNo, NameZhCn: item.NameZhCN, NameEnUs: item.NameEnUS,
			CurrencyCode: item.CurrencyCode, Exposure: item.Exposure, Claim: item.Claim, Use: item.Use, Order: item.Order,
			ReceiptMinor: strconv.FormatInt(item.ReceiptMinor, 10), RefundMinor: strconv.FormatInt(item.RefundMinor, 10),
			CostMinor: strconv.FormatInt(item.CostMinor, 10), NetMinor: strconv.FormatInt(item.NetMinor, 10),
			CostComplete: item.CostComplete,
		})
	}
	return &v1.MarketingPerformanceListRes{StartedAt: report.StartedAt.UnixMilli(), EndedAt: report.EndedAt.UnixMilli(), Rows: rows, Total: report.Total}, nil
}

func (c *ControllerV1) MarketingAbuseCaseList(ctx context.Context, req *v1.MarketingAbuseCaseListReq) (*v1.MarketingAbuseCaseListRes, error) {
	page, err := c.marketingInsights.AbuseCases(ctx, marketing.MarketingAbuseSearchInput{Status: req.Status, Limit: req.Limit, Page: req.Page})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingAbuseCaseItem, 0, len(page.Cases))
	for i := range page.Cases {
		items = append(items, toAPIMarketingAbuseCase(&page.Cases[i]))
	}
	return &v1.MarketingAbuseCaseListRes{Cases: items, Total: page.Total}, nil
}

func (c *ControllerV1) MarketingAbuseCaseReview(ctx context.Context, req *v1.MarketingAbuseCaseReviewReq) (*v1.MarketingAbuseCaseReviewRes, error) {
	item, err := c.marketingInsights.ReviewAbuse(ctx, marketing.MarketingAbuseReviewInput{
		CaseNo: req.CaseNo, Decision: req.Decision, Reason: req.Reason, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingAbuseCaseReviewRes{Case: toAPIMarketingAbuseCase(item)}, nil
}

func (c *ControllerV1) MarketingMissingCostList(ctx context.Context, req *v1.MarketingMissingCostListReq) (*v1.MarketingMissingCostListRes, error) {
	page, err := c.marketingInsights.MissingCosts(ctx, marketing.MarketingMissingCostSearchInput{Limit: req.Limit, Page: req.Page})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingMissingCostItem, 0, len(page.Items))
	for index := range page.Items {
		items = append(items, toAPIMarketingMissingCost(&page.Items[index]))
	}
	return &v1.MarketingMissingCostListRes{Items: items, Total: page.Total}, nil
}

func (c *ControllerV1) MarketingCostCorrection(ctx context.Context, req *v1.MarketingCostCorrectionReq) (*v1.MarketingCostCorrectionRes, error) {
	unitCost, err := strconv.ParseInt(req.UnitCostMinor, 10, 64)
	if err != nil || unitCost < 0 {
		return nil, marketingInvalidControllerInput()
	}
	item, err := c.marketingInsights.CorrectCost(ctx, marketing.MarketingCostCorrectionInput{
		RequestNo: req.RequestNo, OrderItemNo: req.OrderItemNo, UnitCostMinor: unitCost,
		Reason: req.Reason, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingCostCorrectionRes{Item: toAPIMarketingMissingCost(item)}, nil
}

func (c *ControllerV1) MarketingCampaignNewcomerUpdate(ctx context.Context, req *v1.MarketingCampaignNewcomerUpdateReq) (*v1.MarketingCampaignNewcomerUpdateRes, error) {
	setting, err := c.marketingInsights.SetCampaignNewcomer(ctx, marketing.MarketingCampaignNewcomerInput{
		RequestNo: req.RequestNo, CampaignNo: req.CampaignNo, Enabled: req.Enabled, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingCampaignNewcomerUpdateRes{
		CampaignNo: setting.CampaignNo, Enabled: setting.Enabled, Version: setting.Version, UpdatedAt: setting.UpdatedAt.UnixMilli(),
	}, nil
}

func marketingInvalidControllerInput() error {
	return bizerr.NewCode(marketing.CodeInsightsInvalid)
}

func toAPIMarketingMissingCost(item *marketing.MarketingMissingCostItem) *v1.MarketingMissingCostItem {
	if item == nil {
		return nil
	}
	result := &v1.MarketingMissingCostItem{
		OrderItemNo: item.OrderItemNo, OrderNo: item.OrderNo, VersionNo: item.VersionNo,
		CurrencyCode: item.CurrencyCode, PeriodUnitCode: item.PeriodUnitCode, PeriodCount: item.PeriodCount,
		Quantity: item.Quantity, PurchaseCycles: item.PurchaseCycles,
		ProductNameZhCn: item.ProductNameZhCN, ProductNameEnUs: item.ProductNameEnUS,
		StatusCode: item.StatusCode, CorrectionReason: item.CorrectionReason, Version: item.Version,
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
	if item.UnitCostMinor != nil {
		value := strconv.FormatInt(*item.UnitCostMinor, 10)
		result.UnitCostMinor = &value
	}
	if item.TotalCostMinor != nil {
		value := strconv.FormatInt(*item.TotalCostMinor, 10)
		result.TotalCostMinor = &value
	}
	return result
}

func toAPIMarketingAbuseCase(item *marketing.MarketingAbuseCase) *v1.MarketingAbuseCaseItem {
	if item == nil {
		return nil
	}
	result := &v1.MarketingAbuseCaseItem{
		CaseNo: item.CaseNo, CampaignNo: item.CampaignNo, CustomerNo: item.CustomerNo,
		SignalCode: item.SignalCode, StatusCode: item.StatusCode, RelatedAccountCount: item.RelatedAccountCount,
		EvidenceSummaryCode: item.EvidenceSummaryCode, WindowStartedAt: item.WindowStartedAt.UnixMilli(),
		WindowEndedAt: item.WindowEndedAt.UnixMilli(), ObservedCount: item.ObservedCount,
		ReviewReason: item.ReviewReason, ReviewedBy: item.ReviewedBy, Version: item.Version,
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
	if item.BlockedUntil != nil {
		value := item.BlockedUntil.UnixMilli()
		result.BlockedUntil = &value
	}
	if item.ReviewedAt != nil {
		value := item.ReviewedAt.UnixMilli()
		result.ReviewedAt = &value
	}
	return result
}
