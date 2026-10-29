// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package notice

import (
	"context"

	"lina-plugin-content-notice/backend/api/notice/v1"
)

type INoticeV1 interface {
	Announcements(ctx context.Context, req *v1.AnnouncementsReq) (res *v1.AnnouncementsRes, err error)
	Announcement(ctx context.Context, req *v1.AnnouncementReq) (res *v1.AnnouncementRes, err error)
	AnnouncementRead(ctx context.Context, req *v1.AnnouncementReadReq) (res *v1.AnnouncementReadRes, err error)
	Create(ctx context.Context, req *v1.CreateReq) (res *v1.CreateRes, err error)
	Delete(ctx context.Context, req *v1.DeleteReq) (res *v1.DeleteRes, err error)
	Get(ctx context.Context, req *v1.GetReq) (res *v1.GetRes, err error)
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Update(ctx context.Context, req *v1.UpdateReq) (res *v1.UpdateRes, err error)
}
