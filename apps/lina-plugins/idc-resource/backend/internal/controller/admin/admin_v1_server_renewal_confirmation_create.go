package admin

import (
	"context"
	"strconv"

	"lina-plugin-idc-resource/backend/api/admin/v1"
)

func (c *ControllerV1) ServerRenewalConfirmationCreate(ctx context.Context, req *v1.ServerRenewalConfirmationCreateReq) (res *v1.ServerRenewalConfirmationCreateRes, err error) {
	confirmation, err := c.renewals.Confirm(ctx, req.RequestKey, req.RenewalCycles, req.ServerNos, req.ConfirmationDigest, req.EntitlementNo)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ServerRenewalResultItem, len(confirmation.Items))
	for index, item := range confirmation.Items {
		items[index] = &v1.ServerRenewalResultItem{ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN,
			ProductNameEnUS: item.ProductNameEnUS, Succeeded: item.Succeeded, FailureCode: item.FailureCode,
			Status: item.Status, CurrencyCode: item.CurrencyCode, TotalAmountMinor: strconv.FormatInt(item.TotalAmountMinor, 10)}
	}
	return &v1.ServerRenewalConfirmationCreateRes{RequestKey: confirmation.RequestKey,
		RenewalCycles: confirmation.RenewalCycles, Items: items}, nil
}
