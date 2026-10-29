// This file saves independent notification and browser return origins immediately.
package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/api/admin/v1"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

func (c *ControllerV1) PaymentPublicAddressSave(ctx context.Context, req *v1.PaymentPublicAddressSaveReq) (*v1.PaymentPublicAddressSaveRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, channelconfig.CodePublicAddressForbidden)
	}
	item, err := c.channels.SavePublicAddress(ctx, channelconfig.PublicAddress{NotificationOrigin: req.NotificationOrigin, CustomerReturnOrigin: req.CustomerReturnOrigin}, req.ExpectedVersion, actor)
	if err != nil {
		return nil, err
	}
	return &v1.PaymentPublicAddressSaveRes{Addresses: v1.PaymentPublicAddress{NotificationOrigin: item.NotificationOrigin, CustomerReturnOrigin: item.CustomerReturnOrigin, Version: item.Version}}, nil
}
