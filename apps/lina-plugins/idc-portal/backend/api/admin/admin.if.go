// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-portal/backend/api/admin/v1"
)

type IAdminV1 interface {
	CustomerCenterConfigurationGet(ctx context.Context, req *v1.CustomerCenterConfigurationGetReq) (res *v1.CustomerCenterConfigurationGetRes, err error)
	CustomerCenterConfigurationSave(ctx context.Context, req *v1.CustomerCenterConfigurationSaveReq) (res *v1.CustomerCenterConfigurationSaveRes, err error)
	ExternalMenuList(ctx context.Context, req *v1.ExternalMenuListReq) (res *v1.ExternalMenuListRes, err error)
	ExternalMenuSave(ctx context.Context, req *v1.ExternalMenuSaveReq) (res *v1.ExternalMenuSaveRes, err error)
	ExternalMenuStatus(ctx context.Context, req *v1.ExternalMenuStatusReq) (res *v1.ExternalMenuStatusRes, err error)
	SiteConfigurationGet(ctx context.Context, req *v1.SiteConfigurationGetReq) (res *v1.SiteConfigurationGetRes, err error)
	SiteConfigurationSave(ctx context.Context, req *v1.SiteConfigurationSaveReq) (res *v1.SiteConfigurationSaveRes, err error)
}
