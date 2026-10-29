// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package directoryaccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/directoryaccount/v1"
)

type IDirectoryaccountV1 interface {
	CustomerNoteGet(ctx context.Context, req *v1.CustomerNoteGetReq) (res *v1.CustomerNoteGetRes, err error)
	CustomerNoteUpdate(ctx context.Context, req *v1.CustomerNoteUpdateReq) (res *v1.CustomerNoteUpdateRes, err error)
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Services(ctx context.Context, req *v1.ServicesReq) (res *v1.ServicesRes, err error)
}
