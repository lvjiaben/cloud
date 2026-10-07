// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package serviceaccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/serviceaccount/v1"
)

type IServiceaccountV1 interface {
	ActionCreate(ctx context.Context, req *v1.ActionCreateReq) (res *v1.ActionCreateRes, err error)
	EntitlementList(ctx context.Context, req *v1.EntitlementListReq) (res *v1.EntitlementListRes, err error)
	EntitlementGet(ctx context.Context, req *v1.EntitlementGetReq) (res *v1.EntitlementGetRes, err error)
	ActionTaskGet(ctx context.Context, req *v1.ActionTaskGetReq) (res *v1.ActionTaskGetRes, err error)
	DownloadTicketCreate(ctx context.Context, req *v1.DownloadTicketCreateReq) (res *v1.DownloadTicketCreateRes, err error)
	DownloadOpen(ctx context.Context, req *v1.DownloadOpenReq) (res *v1.DownloadOpenRes, err error)
}
