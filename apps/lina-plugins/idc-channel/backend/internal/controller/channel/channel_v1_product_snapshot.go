package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) ProductSnapshot(ctx context.Context, req *v1.ProductSnapshotReq) (*v1.ProductSnapshotRes, error) {
	value, err := c.delegate.svc.ReadProductSnapshot(ctx, channel.CredentialChannelID(ctx), req.PublicSlug)
	if err != nil {
		return nil, err
	}
	return &v1.ProductSnapshotRes{ProductSnapshot: *value}, nil
}
