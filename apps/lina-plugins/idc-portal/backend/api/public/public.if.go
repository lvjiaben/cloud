// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package public

import (
	"context"

	"lina-plugin-idc-portal/backend/api/public/v1"
)

type IPublicV1 interface {
	BrandGet(ctx context.Context, req *v1.BrandGetReq) (res *v1.BrandGetRes, err error)
	ExternalMenuList(ctx context.Context, req *v1.ExternalMenuListReq) (res *v1.ExternalMenuListRes, err error)
}
