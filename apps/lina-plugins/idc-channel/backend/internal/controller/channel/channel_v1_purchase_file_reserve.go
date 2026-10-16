package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (c *ControllerV1) PurchaseFileReserve(ctx context.Context, req *v1.PurchaseFileReserveReq) (res *v1.PurchaseFileReserveRes, err error) {
	value, err := c.delegate.svc.ReservePurchaseFile(ctx, channel.CredentialChannelID(ctx), channelcommercecap.PurchaseFileInput{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, FieldToken: req.FieldToken, FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.PurchaseFileReserveRes{PurchaseFileReservation: *value}, nil
}
