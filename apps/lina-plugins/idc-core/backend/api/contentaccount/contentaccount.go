// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package contentaccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/contentaccount/v1"
)

type IContentaccountV1 interface {
	ContentList(ctx context.Context, req *v1.ContentListReq) (res *v1.ContentListRes, err error)
	ContentDetail(ctx context.Context, req *v1.ContentDetailReq) (res *v1.ContentDetailRes, err error)
	DownloadTicketCreate(ctx context.Context, req *v1.DownloadTicketCreateReq) (res *v1.DownloadTicketCreateRes, err error)
	DownloadOpen(ctx context.Context, req *v1.DownloadOpenReq) (res *v1.DownloadOpenRes, err error)
}
