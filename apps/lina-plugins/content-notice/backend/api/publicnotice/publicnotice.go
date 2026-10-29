// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package publicnotice

import (
	"context"

	"lina-plugin-content-notice/backend/api/publicnotice/v1"
)

type IPublicnoticeV1 interface {
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Get(ctx context.Context, req *v1.GetReq) (res *v1.GetRes, err error)
}
