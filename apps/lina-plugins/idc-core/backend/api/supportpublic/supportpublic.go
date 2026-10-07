// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package supportpublic

import (
	"context"

	"lina-plugin-idc-core/backend/api/supportpublic/v1"
)

type ISupportpublicV1 interface {
	SupportCategoryList(ctx context.Context, req *v1.SupportCategoryListReq) (res *v1.SupportCategoryListRes, err error)
	PresalesAccessVerify(ctx context.Context, req *v1.PresalesAccessVerifyReq) (res *v1.PresalesAccessVerifyRes, err error)
	PresalesReply(ctx context.Context, req *v1.PresalesReplyReq) (res *v1.PresalesReplyRes, err error)
	PresalesClose(ctx context.Context, req *v1.PresalesCloseReq) (res *v1.PresalesCloseRes, err error)
	PresalesReopen(ctx context.Context, req *v1.PresalesReopenReq) (res *v1.PresalesReopenRes, err error)
	PresalesRating(ctx context.Context, req *v1.PresalesRatingReq) (res *v1.PresalesRatingRes, err error)
	PresalesAttachmentUploadCreate(ctx context.Context, req *v1.PresalesAttachmentUploadCreateReq) (res *v1.PresalesAttachmentUploadCreateRes, err error)
	PresalesAttachmentUploadContentUpdate(ctx context.Context, req *v1.PresalesAttachmentUploadContentUpdateReq) (res *v1.PresalesAttachmentUploadContentUpdateRes, err error)
	PresalesAttachmentOpen(ctx context.Context, req *v1.PresalesAttachmentOpenReq) (res *v1.PresalesAttachmentOpenRes, err error)
	PresalesCustomFieldList(ctx context.Context, req *v1.PresalesCustomFieldListReq) (res *v1.PresalesCustomFieldListRes, err error)
}
