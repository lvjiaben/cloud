// This generated controller projects independent payment performance to the API.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementPerformanceList(ctx context.Context, req *v1.SalesManagementPerformanceListReq) (*v1.SalesManagementPerformanceListRes, error) {
	owner, ok := c.salesManagement.(salesmanagement.PerformanceService)
	if !ok {
		return nil, gerror.New("sales performance service unavailable")
	}
	page, err := owner.ListPerformance(ctx, salesmanagement.PerformanceInput{PageInput: salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search}, Month: req.Month, BeneficiaryType: req.BeneficiaryType, CurrencyCode: req.CurrencyCode})
	if err != nil {
		return nil, err
	}
	result := &v1.SalesManagementPerformanceListRes{Items: []v1.SalesManagementPerformance{}, Total: page.Total, Timezone: page.Timezone, AsOf: page.AsOf}
	for _, item := range page.Items {
		result.Items = append(result.Items, v1.SalesManagementPerformance{BeneficiaryType: item.BeneficiaryType, BeneficiaryReference: item.BeneficiaryReference, BeneficiaryName: item.BeneficiaryName, CurrencyCode: item.CurrencyCode, PaidOrderCount: item.PaidOrderCount, PaidRenewalCount: item.PaidRenewalCount, ReceivedMinor: item.ReceivedMinor, RefundedMinor: item.RefundedMinor, NetReceivedMinor: item.NetReceivedMinor})
	}
	return result, nil
}
