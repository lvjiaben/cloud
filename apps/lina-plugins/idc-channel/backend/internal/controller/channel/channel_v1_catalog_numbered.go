package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (c *ControllerV1) CatalogNumbered(ctx context.Context, req *v1.CatalogNumberedReq) (*v1.CatalogNumberedRes, error) {
	value, err := c.delegate.svc.BrowseCatalogNumbered(ctx, channel.CredentialChannelID(ctx), channelcommercecap.CatalogNumberedQuery{NamePrefix: req.NamePrefix, Currency: req.Currency, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogNumberedRes{CatalogNumberedPage: *value}, nil
}
