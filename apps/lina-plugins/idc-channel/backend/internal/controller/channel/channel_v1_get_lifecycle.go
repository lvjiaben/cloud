// This file reads persisted operation progress through the scoped channel service.
package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) GetLifecycle(ctx context.Context, req *v1.GetLifecycleReq) (*v1.GetLifecycleRes, error) {
	result, err := c.delegate.svc.GetLifecycle(ctx, channel.CredentialChannelID(ctx), req.ServiceNo, req.OperationNo)
	if err != nil {
		return nil, err
	}
	return &v1.GetLifecycleRes{OperationNo: result.OperationNo, Status: result.Status, CurrencyCode: result.CurrencyCode, AmountMinor: result.AmountMinor}, nil
}
