// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package supportaccount

import (
	"context"

	"lina-plugin-idc-core/backend/api/supportaccount/v1"
)

type ISupportaccountV1 interface {
	CategoryList(ctx context.Context, req *v1.CategoryListReq) (res *v1.CategoryListRes, err error)
	TicketList(ctx context.Context, req *v1.TicketListReq) (res *v1.TicketListRes, err error)
	TicketDetail(ctx context.Context, req *v1.TicketDetailReq) (res *v1.TicketDetailRes, err error)
	TicketCreate(ctx context.Context, req *v1.TicketCreateReq) (res *v1.TicketCreateRes, err error)
	TicketReply(ctx context.Context, req *v1.TicketReplyReq) (res *v1.TicketReplyRes, err error)
	TicketClose(ctx context.Context, req *v1.TicketCloseReq) (res *v1.TicketCloseRes, err error)
	TicketReopen(ctx context.Context, req *v1.TicketReopenReq) (res *v1.TicketReopenRes, err error)
	TicketRating(ctx context.Context, req *v1.TicketRatingReq) (res *v1.TicketRatingRes, err error)
	AttachmentUploadCreate(ctx context.Context, req *v1.AttachmentUploadCreateReq) (res *v1.AttachmentUploadCreateRes, err error)
	InitialAttachmentUploadCreate(ctx context.Context, req *v1.InitialAttachmentUploadCreateReq) (res *v1.InitialAttachmentUploadCreateRes, err error)
	AttachmentUploadContentUpdate(ctx context.Context, req *v1.AttachmentUploadContentUpdateReq) (res *v1.AttachmentUploadContentUpdateRes, err error)
	InitialAttachmentUploadContentUpdate(ctx context.Context, req *v1.InitialAttachmentUploadContentUpdateReq) (res *v1.InitialAttachmentUploadContentUpdateRes, err error)
	AttachmentOpen(ctx context.Context, req *v1.AttachmentOpenReq) (res *v1.AttachmentOpenRes, err error)
	SupportCustomFieldList(ctx context.Context, req *v1.SupportCustomFieldListReq) (res *v1.SupportCustomFieldListRes, err error)
	SupportEntitlementCandidateList(ctx context.Context, req *v1.SupportEntitlementCandidateListReq) (res *v1.SupportEntitlementCandidateListRes, err error)
}
