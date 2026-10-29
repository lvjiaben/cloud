package admin

import (
	"context"
	"strconv"

	"lina-plugin-idc-resource/backend/api/admin/v1"
)

func (c *ControllerV1) ServerRenewalPreviewCreate(ctx context.Context, req *v1.ServerRenewalPreviewCreateReq) (res *v1.ServerRenewalPreviewCreateRes, err error) {
	preview, err := c.renewals.Preview(ctx, req.RequestKey, req.RenewalCycles, req.ServerNos, req.EntitlementNo)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ServerRenewalPreviewItem, len(preview.Items))
	for index, item := range preview.Items {
		items[index] = &v1.ServerRenewalPreviewItem{ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN,
			ProductNameEnUS: item.ProductNameEnUS,
			Eligible:        item.Eligible, FailureCode: item.FailureCode, CurrencyCode: item.CurrencyCode,
			TotalAmountMinor: strconv.FormatInt(item.TotalAmountMinor, 10), OriginalExpiresAt: unixMilliPointer(item.OriginalExpiresAt),
			ProjectedExpiresAt: unixMilliPointer(item.ProjectedExpiresAt), QuoteExpiresAt: unixMilliPointer(item.QuoteExpiresAt)}
	}
	return &v1.ServerRenewalPreviewCreateRes{RequestKey: preview.RequestKey, RenewalCycles: preview.RenewalCycles,
		ConfirmationDigest: preview.ConfirmationDigest, Items: items}, nil
}
