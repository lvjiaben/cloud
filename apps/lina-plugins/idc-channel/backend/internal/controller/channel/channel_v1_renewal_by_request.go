// This file maps read-only renewal recovery to the authenticated application.
package channel

import (
	"context"
	"time"

	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) RenewalByRequest(ctx context.Context, req *v1.RenewalByRequestReq) (res *v1.RenewalByRequestRes, err error) {
	value, err := c.delegate.svc.ReadRenewalByRequest(ctx, channel.CredentialChannelID(ctx), req.ServiceNo, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &v1.RenewalByRequestRes{ObservedAt: time.Now().UnixMilli(), Found: value.Found, OperationNo: value.OperationNo, Status: value.Status, CurrencyCode: value.CurrencyCode, AmountMinor: value.AmountMinor, CurrentExpiresAt: channelMilliseconds(value.CurrentExpiresAt)}, nil
}
