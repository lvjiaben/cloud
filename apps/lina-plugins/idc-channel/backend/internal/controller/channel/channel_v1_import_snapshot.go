// This file maps authenticated API requests to the customer-owned core projection.
package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) ImportSnapshot(ctx context.Context, req *v1.ImportSnapshotReq) (*v1.ImportSnapshotRes, error) {
	value, err := c.delegate.svc.ReadCatalogImportSnapshot(ctx, channel.CredentialChannelID(ctx), req.PublicSlug)
	if err != nil {
		return nil, err
	}
	return &v1.ImportSnapshotRes{ImportSnapshot: *value}, nil
}
