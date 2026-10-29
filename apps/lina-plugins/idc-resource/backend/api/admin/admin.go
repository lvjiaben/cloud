// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
)

type IAdminV1 interface {
	ChannelSecretAccessCreate(ctx context.Context, req *v1.ChannelSecretAccessCreateReq) (res *v1.ChannelSecretAccessCreateRes, err error)
	ChannelSecretCreate(ctx context.Context, req *v1.ChannelSecretCreateReq) (res *v1.ChannelSecretCreateRes, err error)
	ChannelSecretReplace(ctx context.Context, req *v1.ChannelSecretReplaceReq) (res *v1.ChannelSecretReplaceRes, err error)
	ChannelSecretRevoke(ctx context.Context, req *v1.ChannelSecretRevokeReq) (res *v1.ChannelSecretRevokeRes, err error)
	ChannelSecretRotate(ctx context.Context, req *v1.ChannelSecretRotateReq) (res *v1.ChannelSecretRotateRes, err error)
	ProviderAPILogList(ctx context.Context, req *v1.ProviderAPILogListReq) (res *v1.ProviderAPILogListRes, err error)
	ProviderCapacityObservationList(ctx context.Context, req *v1.ProviderCapacityObservationListReq) (res *v1.ProviderCapacityObservationListRes, err error)
	ProviderCapacityRefreshCreate(ctx context.Context, req *v1.ProviderCapacityRefreshCreateReq) (res *v1.ProviderCapacityRefreshCreateRes, err error)
	ProviderCatalogList(ctx context.Context, req *v1.ProviderCatalogListReq) (res *v1.ProviderCatalogListRes, err error)
	ProviderCatalogProduct(ctx context.Context, req *v1.ProviderCatalogProductReq) (res *v1.ProviderCatalogProductRes, err error)
	ProviderConfigurationList(ctx context.Context, req *v1.ProviderConfigurationListReq) (res *v1.ProviderConfigurationListRes, err error)
	ProviderChannelCreate(ctx context.Context, req *v1.ProviderChannelCreateReq) (res *v1.ProviderChannelCreateRes, err error)
	ProviderChannelUpdate(ctx context.Context, req *v1.ProviderChannelUpdateReq) (res *v1.ProviderChannelUpdateRes, err error)
	ProviderChannelVerificationCreate(ctx context.Context, req *v1.ProviderChannelVerificationCreateReq) (res *v1.ProviderChannelVerificationCreateRes, err error)
	ResourcePoolCreate(ctx context.Context, req *v1.ResourcePoolCreateReq) (res *v1.ResourcePoolCreateRes, err error)
	ResourcePoolUpdate(ctx context.Context, req *v1.ResourcePoolUpdateReq) (res *v1.ResourcePoolUpdateRes, err error)
	ProviderHealthList(ctx context.Context, req *v1.ProviderHealthListReq) (res *v1.ProviderHealthListRes, err error)
	ProviderHealthPolicyUpdate(ctx context.Context, req *v1.ProviderHealthPolicyUpdateReq) (res *v1.ProviderHealthPolicyUpdateRes, err error)
	ProviderTaskDetail(ctx context.Context, req *v1.ProviderTaskDetailReq) (res *v1.ProviderTaskDetailRes, err error)
	ProviderTaskList(ctx context.Context, req *v1.ProviderTaskListReq) (res *v1.ProviderTaskListRes, err error)
	ProviderTaskManualTakeover(ctx context.Context, req *v1.ProviderTaskManualTakeoverReq) (res *v1.ProviderTaskManualTakeoverRes, err error)
	ResourceOverview(ctx context.Context, req *v1.ResourceOverviewReq) (res *v1.ResourceOverviewRes, err error)
	ServerRenewalConfirmationCreate(ctx context.Context, req *v1.ServerRenewalConfirmationCreateReq) (res *v1.ServerRenewalConfirmationCreateRes, err error)
	ServerRenewalList(ctx context.Context, req *v1.ServerRenewalListReq) (res *v1.ServerRenewalListRes, err error)
	ServerRenewalPreviewCreate(ctx context.Context, req *v1.ServerRenewalPreviewCreateReq) (res *v1.ServerRenewalPreviewCreateRes, err error)
}
