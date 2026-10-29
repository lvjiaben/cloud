// This file exposes owner-checked persisted records without supplier calls.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerCommercialOperations(ctx context.Context, req *v1.NativeServerCommercialOperationsReq) (*v1.NativeServerCommercialOperationsRes, error) {
	if err := bindNativePath(ctx, "serverNo", &req.ServerNo); err != nil {
		return nil, publicError(err)
	}
	page, err := c.commercial.CommercialOperations(ctx, customeroperations.CommercialHistoryQuery{ServerNo: req.ServerNo, OperationNo: req.OperationNo, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	if page == nil {
		return nil, publicError(customeroperations.ErrUnavailable)
	}
	result := &v1.NativeServerCommercialOperationsRes{Items: make([]v1.NativeServerCommercialOperation, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, item := range page.Items {
		result.Items = append(result.Items, v1.NativeServerCommercialOperation{OperationNo: item.OperationNo, QuoteNo: item.QuoteNo, OrderNo: item.OrderNo, InvoiceNo: item.InvoiceNo, CurrencyCode: item.CurrencyCode, TotalAmountMinor: item.TotalAmountMinor, Status: item.Status, LastErrorCode: item.LastErrorCode, Action: string(item.Action), CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), CompletedAt: nativeNullableTime(item.CompletedAt)})
	}
	return result, nil
}
