// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package adminserver

import (
	"context"

	"lina-plugin-idc-resource/backend/api/adminserver/v1"
)

type IAdminserverV1 interface {
	CloudServerDetail(ctx context.Context, req *v1.CloudServerDetailReq) (res *v1.CloudServerDetailRes, err error)
	CloudServerList(ctx context.Context, req *v1.CloudServerListReq) (res *v1.CloudServerListRes, err error)
}
