package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) ListChannels(ctx context.Context, req *v1.ListChannelsReq) (*v1.ListChannelsRes, error) {
	return c.delegate.ListChannels(ctx, req)
}

func (c *ControllerV1) SaveChannel(ctx context.Context, req *v1.SaveChannelReq) (*v1.SaveChannelRes, error) {
	return c.delegate.SaveChannel(ctx, req)
}

func (c *ControllerV1) SetChannelStatus(ctx context.Context, req *v1.SetChannelStatusReq) (*v1.SetChannelStatusRes, error) {
	return c.delegate.SetChannelStatus(ctx, req)
}

func (c *ControllerV1) ListPublications(ctx context.Context, req *v1.ListPublicationsReq) (*v1.ListPublicationsRes, error) {
	return c.delegate.ListPublications(ctx, req)
}

func (c *ControllerV1) SavePublication(ctx context.Context, req *v1.SavePublicationReq) (*v1.SavePublicationRes, error) {
	return c.delegate.SavePublication(ctx, req)
}

func (c *ControllerV1) ListRequestAudit(ctx context.Context, req *v1.ListRequestAuditReq) (*v1.ListRequestAuditRes, error) {
	return c.delegate.ListRequestAudit(ctx, req)
}

func (c *ControllerV1) Health(ctx context.Context, req *v1.HealthReq) (*v1.HealthRes, error) {
	return c.delegate.Health(ctx, req)
}

func (c *ControllerV1) Finance(ctx context.Context, req *v1.FinanceReq) (*v1.FinanceRes, error) {
	return c.delegate.Finance(ctx, req)
}
