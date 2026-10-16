package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

func (c *ControllerV1) CatalogConfiguration(ctx context.Context, req *v1.CatalogConfigurationReq) (*v1.CatalogConfigurationRes, error) {
	value, err := c.delegate.svc.ReadCatalogConfiguration(ctx, channel.CredentialChannelID(ctx), req.PublicSlug)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogConfigurationRes{Configuration: *value}, nil
}
