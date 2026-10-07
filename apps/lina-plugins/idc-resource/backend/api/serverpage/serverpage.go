// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package serverpage

import (
	"context"

	"lina-plugin-idc-resource/backend/api/serverpage/v1"
)

type IServerpageV1 interface {
	ServerDetail(ctx context.Context, req *v1.ServerDetailReq) (res *v1.ServerDetailRes, err error)
	ServerList(ctx context.Context, req *v1.ServerListReq) (res *v1.ServerListRes, err error)
}
