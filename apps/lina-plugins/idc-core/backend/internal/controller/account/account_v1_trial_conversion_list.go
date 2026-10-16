// This file handles owned trial conversion list using the startup renewal service.

package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) TrialConversionList(ctx context.Context, req *v1.TrialConversionListReq) (res *v1.TrialConversionListRes, err error) {
	page, err := c.renewals.ListTrialConversions(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]v1.TrialConversionItem, 0, len(page.Items))
	for _, item := range page.Items {
		periods := make([]v1.TrialConversionPeriod, 0, len(item.Periods))
		for _, period := range item.Periods {
			periods = append(periods, v1.TrialConversionPeriod{BillingRuleNo: period.BillingRuleNo, PeriodUnitCode: period.PeriodUnitCode, PeriodCount: period.PeriodCount})
		}
		items = append(items, v1.TrialConversionItem{EntitlementNo: item.EntitlementNo, ProductNameZhCN: item.ProductNameZhCN,
			ProductNameEnUS: item.ProductNameEnUS, CurrencyCode: item.CurrencyCode, StatusCode: item.StatusCode, ExpiresAt: unixMillis(item.ExpiresAt), Periods: periods})
	}
	return &v1.TrialConversionListRes{Items: items, Total: page.Total}, nil
}
