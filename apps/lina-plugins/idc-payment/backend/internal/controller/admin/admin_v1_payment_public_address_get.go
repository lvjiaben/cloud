// This file returns current merchant-wide payment routing settings.
package admin

import (
	"context"
	"lina-plugin-idc-payment/backend/api/admin/v1"
)

func (c *ControllerV1) PaymentPublicAddressGet(ctx context.Context, req *v1.PaymentPublicAddressGetReq) (*v1.PaymentPublicAddressGetRes, error) {
	item, err := c.channels.ReadPublicAddress(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.PaymentPublicAddressGetRes{Addresses: v1.PaymentPublicAddress{NotificationOrigin: item.NotificationOrigin, CustomerReturnOrigin: item.CustomerReturnOrigin, Version: item.Version}}, nil
}
