// This file maps authenticated API requests to the customer-owned core projection.
package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"time"
)

func (c *ControllerV1) OrderDeliveries(ctx context.Context, req *v1.OrderDeliveriesReq) (*v1.OrderDeliveriesRes, error) {
	value, err := c.delegate.svc.ReadOrderDeliveries(ctx, channel.CredentialChannelID(ctx), req.Id)
	if err != nil {
		return nil, err
	}
	result := projectChannelDeliveries(value)
	result.ObservedAt = time.Now().UnixMilli()
	return result, nil
}
