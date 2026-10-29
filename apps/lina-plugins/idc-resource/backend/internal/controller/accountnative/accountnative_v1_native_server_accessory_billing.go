// This file exposes owner-checked persisted records without supplier calls.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerAccessoryBilling(ctx context.Context, req *v1.NativeServerAccessoryBillingReq) (*v1.NativeServerAccessoryBillingRes, error) {
	if err := bindNativePath(ctx, "serverNo", &req.ServerNo); err != nil {
		return nil, publicError(err)
	}
	page, err := c.commercial.AccessoryBilling(ctx, customeroperations.AccessoryBillingQuery{ServerNo: req.ServerNo, ObjectNo: req.ObjectNo, Kind: req.Kind, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	if page == nil {
		return nil, publicError(customeroperations.ErrUnavailable)
	}
	result := &v1.NativeServerAccessoryBillingRes{Items: make([]v1.NativeServerAccessoryBill, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, item := range page.Items {
		result.Items = append(result.Items, v1.NativeServerAccessoryBill{AccessoryEntitlementNo: item.AccessoryEntitlementNo, ObjectNo: item.ObjectNo, BillingMode: item.BillingMode, Status: item.Status, CurrencyCode: item.CurrencyCode, PeriodUnit: item.PeriodUnit, PeriodCount: item.PeriodCount, AutoRenewEnabled: item.AutoRenewEnabled, PendingInvoiceNo: item.PendingInvoiceNo, PendingInvoiceStatus: item.PendingInvoiceStatus, PendingAmountMinor: item.PendingAmountMinor, Kind: string(item.Kind), CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), CurrentPeriodStartAt: nativeNullableTime(item.CurrentPeriodStartAt), CurrentPeriodEndAt: nativeNullableTime(item.CurrentPeriodEndAt)})
	}
	return result, nil
}
