package supportpublic

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"

	v1 "lina-plugin-idc-core/backend/api/supportpublic/v1"
	"lina-plugin-idc-core/backend/internal/controller/supportaccount"
	"lina-plugin-idc-core/backend/internal/controller/supportstream"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

type ControllerV1 struct{ service supportdesk.Service }

func NewV1(service supportdesk.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("public support controller requires service")
	}
	return &ControllerV1{service: service}, nil
}

func (c *ControllerV1) PresalesAccessVerify(ctx context.Context, req *v1.PresalesAccessVerifyReq) (*v1.PresalesAccessVerifyRes, error) {
	detail, err := c.service.DetailAnonymous(ctx, req.AccessToken)
	if err != nil {
		return nil, err
	}
	return &v1.PresalesAccessVerifyRes{Item: supportaccount.ToSafeDetail(detail), ExpiresAt: anonymousExpiresAt(detail)}, nil
}

func (c *ControllerV1) PresalesReply(ctx context.Context, req *v1.PresalesReplyReq) (*v1.PresalesReplyRes, error) {
	detail, err := c.service.ReplyAnonymous(ctx, req.AccessToken, supportdesk.MessageInput{Body: req.Body, UploadNos: req.UploadNos, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.PresalesReplyRes{Item: supportaccount.ToSafeDetail(detail), ExpiresAt: anonymousExpiresAt(detail)}, nil
}

func (c *ControllerV1) PresalesAttachmentUploadCreate(ctx context.Context, req *v1.PresalesAttachmentUploadCreateReq) (*v1.PresalesAttachmentUploadCreateRes, error) {
	item, err := c.service.ReserveAnonymousAttachment(ctx, req.AccessToken, supportdesk.AttachmentUploadInput{FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.PresalesAttachmentUploadCreateRes{UploadNo: item.UploadNo, NextOffset: item.NextOffset, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}, nil
}

func (c *ControllerV1) PresalesAttachmentUploadContentUpdate(ctx context.Context, req *v1.PresalesAttachmentUploadContentUpdateReq) (*v1.PresalesAttachmentUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(supportdesk.CodeAttachmentRejected)
	}
	item, err := c.service.UploadAnonymousAttachment(ctx, req.AccessToken, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.PresalesAttachmentUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Version: item.Version, Ready: item.Ready}, nil
}

func (c *ControllerV1) PresalesClose(ctx context.Context, req *v1.PresalesCloseReq) (*v1.PresalesCloseRes, error) {
	detail, err := c.service.CloseAnonymous(ctx, req.AccessToken, req.TicketVersion)
	if err != nil {
		return nil, err
	}
	return &v1.PresalesCloseRes{Item: supportaccount.ToSafeDetail(detail), ExpiresAt: anonymousExpiresAt(detail)}, nil
}

func (c *ControllerV1) PresalesReopen(ctx context.Context, req *v1.PresalesReopenReq) (*v1.PresalesReopenRes, error) {
	detail, err := c.service.ReopenAnonymous(ctx, req.AccessToken, req.TicketVersion)
	if err != nil {
		return nil, err
	}
	return &v1.PresalesReopenRes{Item: supportaccount.ToSafeDetail(detail), ExpiresAt: anonymousExpiresAt(detail)}, nil
}

func (c *ControllerV1) PresalesRating(ctx context.Context, req *v1.PresalesRatingReq) (*v1.PresalesRatingRes, error) {
	detail, err := c.service.RateAnonymous(ctx, req.AccessToken, supportdesk.RatingInput{TicketVersion: req.TicketVersion, RoundNo: req.RoundNo, Stars: req.Stars, Feedback: req.Feedback})
	if err != nil {
		return nil, err
	}
	return &v1.PresalesRatingRes{Item: supportaccount.ToSafeDetail(detail), ExpiresAt: anonymousExpiresAt(detail)}, nil
}

func anonymousExpiresAt(detail *supportdesk.Detail) int64 {
	if detail == nil || detail.AnonymousAccessExpiresAt == nil {
		return 0
	}
	return detail.AnonymousAccessExpiresAt.UnixMilli()
}

func (c *ControllerV1) PresalesAttachmentOpen(ctx context.Context, req *v1.PresalesAttachmentOpenReq) (*v1.PresalesAttachmentOpenRes, error) {
	file, err := c.service.OpenAnonymousAttachment(ctx, req.AccessToken, req.AttachmentNo)
	if err != nil {
		return nil, err
	}
	if err = supportstream.Write(ctx, file); err != nil {
		return nil, err
	}
	return &v1.PresalesAttachmentOpenRes{}, nil
}

func (c *ControllerV1) SupportCategoryList(ctx context.Context, req *v1.SupportCategoryListReq) (*v1.SupportCategoryListRes, error) {
	page, err := c.service.SearchPublicCategories(ctx, supportdesk.CategorySearchInput{Keyword: req.Keyword, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]v1.SupportCategoryOption, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.SupportCategoryOption{CategoryNo: item.CategoryNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, Priority: item.Priority})
	}
	return &v1.SupportCategoryListRes{Items: items, NextCursor: page.NextCursor}, nil
}

func (c *ControllerV1) PresalesCustomFieldList(ctx context.Context, _ *v1.PresalesCustomFieldListReq) (*v1.PresalesCustomFieldListRes, error) {
	definitions, err := c.service.PublicPresalesCustomFields(ctx)
	if err != nil {
		return nil, err
	}
	safe := supportaccount.ToSafeCustomFieldDefinitions(definitions)
	items := make([]v1.PresalesCustomFieldDefinition, 0, len(safe))
	for _, item := range safe {
		options := make([]v1.PresalesCustomFieldOption, 0, len(item.Options))
		for _, option := range item.Options {
			options = append(options, v1.PresalesCustomFieldOption{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder})
		}
		items = append(items, v1.PresalesCustomFieldDefinition{FieldNo: item.FieldNo, FieldType: item.FieldType, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS, Required: item.Required, SortOrder: item.SortOrder, Options: options})
	}
	return &v1.PresalesCustomFieldListRes{Items: items}, nil
}
