package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (c *ControllerV1) CatalogBillingTerms(ctx context.Context, req *v1.CatalogBillingTermsReq) (*v1.CatalogBillingTermsRes, error) {
	values, err := configurationQueryValues(req.Values)
	if err != nil {
		return nil, err
	}
	value, err := c.delegate.svc.PreviewCatalogTerms(ctx, channel.CredentialChannelID(ctx), channelcommercecap.ConfigurationInput{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, CurrencyCode: req.CurrencyCode, Values: values})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogBillingTermsRes{BillingTerms: *value}, nil
}
