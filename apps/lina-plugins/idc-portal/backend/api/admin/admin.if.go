// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-portal/backend/api/admin/v1"
)

type IAdminV1 interface {
	BrandGet(ctx context.Context, req *v1.BrandGetReq) (res *v1.BrandGetRes, err error)
	BrandSave(ctx context.Context, req *v1.BrandSaveReq) (res *v1.BrandSaveRes, err error)
	BrandPreview(ctx context.Context, req *v1.BrandPreviewReq) (res *v1.BrandPreviewRes, err error)
	BrandPublish(ctx context.Context, req *v1.BrandPublishReq) (res *v1.BrandPublishRes, err error)
	BrandRollback(ctx context.Context, req *v1.BrandRollbackReq) (res *v1.BrandRollbackRes, err error)
	ExternalMenuList(ctx context.Context, req *v1.ExternalMenuListReq) (res *v1.ExternalMenuListRes, err error)
	ExternalMenuSave(ctx context.Context, req *v1.ExternalMenuSaveReq) (res *v1.ExternalMenuSaveRes, err error)
	ExternalMenuStatus(ctx context.Context, req *v1.ExternalMenuStatusReq) (res *v1.ExternalMenuStatusRes, err error)
}
