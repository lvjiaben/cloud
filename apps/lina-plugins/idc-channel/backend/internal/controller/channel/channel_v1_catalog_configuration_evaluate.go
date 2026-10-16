package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (c *ControllerV1) CatalogConfigurationEvaluate(ctx context.Context, req *v1.CatalogConfigurationEvaluateReq) (*v1.CatalogConfigurationEvaluateRes, error) {
	values, err := configurationQueryValues(req.Values)
	if err != nil {
		return nil, err
	}
	value, err := c.delegate.svc.EvaluateCatalogConfiguration(ctx, channel.CredentialChannelID(ctx), channelcommercecap.ConfigurationInput{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogConfigurationEvaluateRes{ConfigurationEvaluation: *value}, nil
}
