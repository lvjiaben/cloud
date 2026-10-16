package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) GetOrderByRequest(ctx context.Context, req *v1.GetOrderByRequestReq) (res *v1.GetOrderByRequestRes, err error) {
	o, err := c.delegate.svc.GetOrderByRequest(ctx, channel.CredentialChannelID(ctx), req.RequestId)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return &v1.GetOrderByRequestRes{Found: false}, nil
	}
	return &v1.GetOrderByRequestRes{Found: true, Order: &v1.GetOrderRes{Id: o.ID, ChannelId: o.ChannelID, OwnershipMode: o.OwnershipMode, Currency: o.Currency, Status: o.Status, FinanciallySettled: o.FinanciallySettled, IdentityReady: o.IdentityReady, CoreOrderNo: o.CoreOrderNo, PayerCustomerNo: o.PayerCustomerNo, ServiceOwnerCustomerNo: o.ServiceOwnerCustomerNo, TotalAmountMinor: o.TotalAmountMinor}}, nil
}
