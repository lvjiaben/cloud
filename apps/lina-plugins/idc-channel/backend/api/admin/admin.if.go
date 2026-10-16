// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

type IAdminV1 interface {
	ListChannels(ctx context.Context, req *v1.ListChannelsReq) (res *v1.ListChannelsRes, err error)
	SaveChannel(ctx context.Context, req *v1.SaveChannelReq) (res *v1.SaveChannelRes, err error)
	SetChannelStatus(ctx context.Context, req *v1.SetChannelStatusReq) (res *v1.SetChannelStatusRes, err error)
	ListPublications(ctx context.Context, req *v1.ListPublicationsReq) (res *v1.ListPublicationsRes, err error)
	SavePublication(ctx context.Context, req *v1.SavePublicationReq) (res *v1.SavePublicationRes, err error)
	ListRequestAudit(ctx context.Context, req *v1.ListRequestAuditReq) (res *v1.ListRequestAuditRes, err error)
	Health(ctx context.Context, req *v1.HealthReq) (res *v1.HealthRes, err error)
	Finance(ctx context.Context, req *v1.FinanceReq) (res *v1.FinanceRes, err error)
	Capabilities(ctx context.Context, req *v1.CapabilitiesReq) (res *v1.CapabilitiesRes, err error)
	Catalog(ctx context.Context, req *v1.CatalogReq) (res *v1.CatalogRes, err error)
	ListTokens(ctx context.Context, req *v1.ListTokensReq) (res *v1.ListTokensRes, err error)
	CreateToken(ctx context.Context, req *v1.CreateTokenReq) (res *v1.CreateTokenRes, err error)
	RevokeToken(ctx context.Context, req *v1.RevokeTokenReq) (res *v1.RevokeTokenRes, err error)
	CustomerPlatform(ctx context.Context, req *v1.CustomerPlatformReq) (res *v1.CustomerPlatformRes, err error)
	CustomerAPICalls(ctx context.Context, req *v1.CustomerAPICallsReq) (res *v1.CustomerAPICallsRes, err error)
	CustomerSecretRotate(ctx context.Context, req *v1.CustomerSecretRotateReq) (res *v1.CustomerSecretRotateRes, err error)
}
