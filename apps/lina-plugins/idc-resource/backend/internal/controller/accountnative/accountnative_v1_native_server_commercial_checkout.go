package accountnative

import (
	"context"
	"strconv"

	"lina-plugin-idc-resource/backend/internal/service/customeroperations"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerCommercialCheckout(ctx context.Context, req *v1.NativeServerCommercialCheckoutReq) (res *v1.NativeServerCommercialCheckoutRes, err error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	purchase, err := c.commercial.Checkout(ctx, customeroperations.CommercialCheckoutCommand{ServerNo: req.ServerNo, QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey, ProofToken: req.ProofToken, Confirmed: req.Confirmed})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	return &v1.NativeServerCommercialCheckoutRes{OperationNo: purchase.OperationNo, QuoteNo: purchase.QuoteNo, OrderNo: purchase.OrderNo, InvoiceNo: purchase.InvoiceNo, Status: purchase.Status, CurrencyCode: purchase.CurrencyCode, TotalAmountMinor: strconv.FormatInt(purchase.TotalAmountMinor, 10), LastErrorCode: purchase.LastErrorCode, Reused: purchase.Reused}, nil
}
