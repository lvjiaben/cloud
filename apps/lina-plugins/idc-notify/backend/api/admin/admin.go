// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

type IAdminV1 interface {
	ChannelAliyunSMSConfigurationUpdate(ctx context.Context, req *v1.ChannelAliyunSMSConfigurationUpdateReq) (res *v1.ChannelAliyunSMSConfigurationUpdateRes, err error)
	ChannelConfigurationClearanceCreate(ctx context.Context, req *v1.ChannelConfigurationClearanceCreateReq) (res *v1.ChannelConfigurationClearanceCreateRes, err error)
	ChannelDisablementCreate(ctx context.Context, req *v1.ChannelDisablementCreateReq) (res *v1.ChannelDisablementCreateRes, err error)
	ChannelEnablementCreate(ctx context.Context, req *v1.ChannelEnablementCreateReq) (res *v1.ChannelEnablementCreateRes, err error)
	ChannelList(ctx context.Context, req *v1.ChannelListReq) (res *v1.ChannelListRes, err error)
	ChannelSMSBaoConfigurationUpdate(ctx context.Context, req *v1.ChannelSMSBaoConfigurationUpdateReq) (res *v1.ChannelSMSBaoConfigurationUpdateRes, err error)
	ChannelSMTPConfigurationUpdate(ctx context.Context, req *v1.ChannelSMTPConfigurationUpdateReq) (res *v1.ChannelSMTPConfigurationUpdateRes, err error)
	ChannelTencentSMSConfigurationUpdate(ctx context.Context, req *v1.ChannelTencentSMSConfigurationUpdateReq) (res *v1.ChannelTencentSMSConfigurationUpdateRes, err error)
	ChannelTestCreate(ctx context.Context, req *v1.ChannelTestCreateReq) (res *v1.ChannelTestCreateRes, err error)
	DeliveryGet(ctx context.Context, req *v1.DeliveryGetReq) (res *v1.DeliveryGetRes, err error)
	DeliveryList(ctx context.Context, req *v1.DeliveryListReq) (res *v1.DeliveryListRes, err error)
	DeliveryResendCreate(ctx context.Context, req *v1.DeliveryResendCreateReq) (res *v1.DeliveryResendCreateRes, err error)
	DeliveryResendEligibilityGet(ctx context.Context, req *v1.DeliveryResendEligibilityGetReq) (res *v1.DeliveryResendEligibilityGetRes, err error)
	DeliveryUnknownClosureCreate(ctx context.Context, req *v1.DeliveryUnknownClosureCreateReq) (res *v1.DeliveryUnknownClosureCreateRes, err error)
	EventList(ctx context.Context, req *v1.EventListReq) (res *v1.EventListRes, err error)
	EventPolicyList(ctx context.Context, req *v1.EventPolicyListReq) (res *v1.EventPolicyListRes, err error)
	EventPolicyUpdate(ctx context.Context, req *v1.EventPolicyUpdateReq) (res *v1.EventPolicyUpdateRes, err error)
	TemplateGet(ctx context.Context, req *v1.TemplateGetReq) (res *v1.TemplateGetRes, err error)
	TemplateList(ctx context.Context, req *v1.TemplateListReq) (res *v1.TemplateListRes, err error)
	TemplatePreviewCreate(ctx context.Context, req *v1.TemplatePreviewCreateReq) (res *v1.TemplatePreviewCreateRes, err error)
	TemplateUpdate(ctx context.Context, req *v1.TemplateUpdateReq) (res *v1.TemplateUpdateRes, err error)
}
