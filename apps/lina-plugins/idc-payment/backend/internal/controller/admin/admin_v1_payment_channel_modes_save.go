// This file applies mode-only channel changes using direct staff authorization.
package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-payment/backend/api/admin/v1"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

func (c *ControllerV1) PaymentChannelModesSave(ctx context.Context, req *v1.PaymentChannelModesSaveReq) (*v1.PaymentChannelModesSaveRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, channelconfig.CodeChannelModesForbidden)
	}
	modes := make([]paymentcap.Mode, len(req.EnabledModes))
	for i, mode := range req.EnabledModes {
		modes[i] = paymentcap.Mode(mode)
	}
	channel, err := c.channels.SaveModes(ctx, req.ChannelNo, modes, req.ExpectedVersion, actor)
	if err != nil {
		return nil, err
	}
	return &v1.PaymentChannelModesSaveRes{Channel: mapPaymentChannel(channel)}, nil
}
