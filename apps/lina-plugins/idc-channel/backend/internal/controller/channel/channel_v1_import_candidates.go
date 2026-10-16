// This file adapts bounded dynamic import candidate reads to the customer API.
package channel

import (
	"context"
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (c *ControllerV1) ImportCandidates(ctx context.Context, req *v1.ImportCandidatesReq) (*v1.ImportCandidatesRes, error) {
	values, err := configurationQueryValues(req.Values)
	if err != nil {
		return nil, err
	}
	value, err := c.delegate.svc.ReadImportCandidates(ctx, channel.CredentialChannelID(ctx), channelcommercecap.ImportCandidatesInput{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Values: values})
	if err != nil {
		return nil, err
	}
	return &v1.ImportCandidatesRes{ImportCandidates: *value}, nil
}
