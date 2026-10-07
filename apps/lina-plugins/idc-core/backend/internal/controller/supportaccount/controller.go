// This file maps customer support requests to the owner service without weakening customer visibility.

package supportaccount

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/supportaccount/v1"
	"lina-plugin-idc-core/backend/internal/controller/supportstream"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

type ControllerV1 struct{ service supportdesk.Service }

func NewV1(service supportdesk.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("support account controller requires service")
	}
	return &ControllerV1{service: service}, nil
}
func (c *ControllerV1) CategoryList(ctx context.Context, req *v1.CategoryListReq) (*v1.CategoryListRes, error) {
	page, err := c.service.SearchPublicCategories(ctx, supportdesk.CategorySearchInput{Keyword: req.Keyword, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]v1.CategoryOption, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.CategoryOption{CategoryLabel: v1.CategoryLabel{CategoryNo: item.CategoryNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS}, Priority: item.Priority})
	}
	return &v1.CategoryListRes{Items: items, NextCursor: page.NextCursor}, nil
}
func (c *ControllerV1) TicketList(ctx context.Context, req *v1.TicketListReq) (*v1.TicketListRes, error) {
	page, err := c.service.SearchAccount(ctx, supportdesk.SearchInput{Stage: req.Stage, Query: req.Query, Cursor: req.Cursor, Page: req.Page, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.TicketItem, 0, len(page.Tickets))
	for _, item := range page.Tickets {
		items = append(items, ToSafeTicket(item))
	}
	return &v1.TicketListRes{Tickets: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
func (c *ControllerV1) TicketDetail(ctx context.Context, req *v1.TicketDetailReq) (*v1.TicketDetailRes, error) {
	item, err := c.service.DetailAccount(ctx, req.TicketNo)
	if err != nil {
		return nil, err
	}
	return &v1.TicketDetailRes{Item: ToSafeDetail(item)}, nil
}
func (c *ControllerV1) TicketCreate(ctx context.Context, req *v1.TicketCreateReq) (*v1.TicketCreateRes, error) {
	item, err := c.service.CreateAccount(ctx, supportdesk.CreateInput{CategoryNo: req.CategoryNo, Subject: req.Subject, Body: req.Body, UploadNos: req.UploadNos, AssociationReference: req.AssociationReference, CustomFields: fromAccountCustomFieldSubmissions(req.CustomFields)})
	if err != nil {
		return nil, err
	}
	return &v1.TicketCreateRes{Item: ToSafeDetail(item)}, nil
}
func (c *ControllerV1) TicketReply(ctx context.Context, req *v1.TicketReplyReq) (*v1.TicketReplyRes, error) {
	item, err := c.service.ReplyAccount(ctx, supportdesk.MessageInput{TicketNo: req.TicketNo, Body: req.Body, UploadNos: req.UploadNos, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketReplyRes{Item: ToSafeDetail(item)}, nil
}
func (c *ControllerV1) TicketClose(ctx context.Context, req *v1.TicketCloseReq) (*v1.TicketCloseRes, error) {
	if !req.Confirmed {
		return nil, bizerr.NewCode(supportdesk.CodeInvalid)
	}
	item, err := c.service.CloseAccount(ctx, supportdesk.TransitionInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketCloseRes{Item: ToSafeDetail(item)}, nil
}
func (c *ControllerV1) TicketReopen(ctx context.Context, req *v1.TicketReopenReq) (*v1.TicketReopenRes, error) {
	if !req.Confirmed {
		return nil, bizerr.NewCode(supportdesk.CodeInvalid)
	}
	item, err := c.service.ReopenAccount(ctx, supportdesk.TransitionInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketReopenRes{Item: ToSafeDetail(item)}, nil
}
func (c *ControllerV1) TicketRating(ctx context.Context, req *v1.TicketRatingReq) (*v1.TicketRatingRes, error) {
	item, err := c.service.RateAccount(ctx, supportdesk.RatingInput{TicketNo: req.TicketNo, Stars: req.Stars, RoundNo: req.RoundNo, Feedback: req.Feedback, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketRatingRes{Item: ToSafeDetail(item)}, nil
}

func ToSafeTicket(item *supportdesk.Ticket) *v1.TicketItem {
	if item == nil {
		return nil
	}
	result := &v1.TicketItem{TicketNo: item.TicketNo, Subject: item.Subject, Status: item.CustomerStage, Priority: item.Priority, CurrentRound: item.CurrentRound, Version: item.Version, LastActivityAt: item.LastActivityAt.UnixMilli(), CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), RelatedEntitlement: toAccountRelatedEntitlement(item)}
	if item.CategoryNo != "" {
		result.Category = &v1.CategoryLabel{CategoryNo: item.CategoryNo, NameZhCN: item.CategoryNameZhCN, NameEnUS: item.CategoryNameEnUS}
	}
	if item.ResolvedAt != nil {
		result.ResolvedAt = item.ResolvedAt.UnixMilli()
	}
	if item.ClosedAt != nil {
		result.ClosedAt = item.ClosedAt.UnixMilli()
	}
	return result
}

func ToSafeDetail(item *supportdesk.Detail) *v1.TicketDetail {
	if item == nil {
		return nil
	}
	result := &v1.TicketDetail{Ticket: ToSafeTicket(item.Ticket), AllowedActions: append([]string(nil), item.AvailableActions...), Messages: make([]v1.Message, 0, len(item.Messages)), CustomFields: toAccountCustomFieldValues(item.CustomFields)}
	if item.ReopenDeadline != nil {
		result.ReopenDeadline = item.ReopenDeadline.UnixMilli()
	}
	if item.Rating != nil {
		result.Rating = &v1.Rating{Stars: item.Rating.Stars, RoundNo: item.Rating.RoundNo, Feedback: item.Rating.Feedback, SubmittedAt: item.Rating.SubmittedAt.UnixMilli()}
	}
	for _, message := range item.Messages {
		author := "support"
		switch message.AuthorType {
		case "customer":
			author = "customer"
		case "anonymous":
			author = "visitor"
		case "automation":
			author = "system"
		}
		mapped := v1.Message{MessageNo: message.MessageNo, Kind: message.Type, Author: author, Body: message.Body, Automated: message.Automated, CreatedAt: message.CreatedAt.UnixMilli(), Attachments: make([]v1.Attachment, 0, len(message.Attachments))}
		for _, attachment := range message.Attachments {
			mapped.Attachments = append(mapped.Attachments, v1.Attachment{AttachmentNo: attachment.AttachmentNo, FileName: attachment.FileName, MediaType: attachment.MediaType, SizeBytes: attachment.SizeBytes, CreatedAt: attachment.CreatedAt.UnixMilli()})
		}
		result.Messages = append(result.Messages, mapped)
	}
	return result
}
func (c *ControllerV1) AttachmentUploadCreate(ctx context.Context, req *v1.AttachmentUploadCreateReq) (*v1.AttachmentUploadCreateRes, error) {
	item, err := c.service.ReserveAccountAttachment(ctx, supportdesk.AttachmentUploadInput{TicketNo: req.TicketNo, FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.AttachmentUploadCreateRes{UploadNo: item.UploadNo, NextOffset: item.NextOffset, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}, nil
}
func (c *ControllerV1) InitialAttachmentUploadCreate(ctx context.Context, req *v1.InitialAttachmentUploadCreateReq) (*v1.InitialAttachmentUploadCreateRes, error) {
	item, err := c.service.ReserveAccountInitialAttachment(ctx, supportdesk.AttachmentUploadInput{FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.InitialAttachmentUploadCreateRes{UploadNo: item.UploadNo, NextOffset: item.NextOffset, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}, nil
}
func (c *ControllerV1) AttachmentUploadContentUpdate(ctx context.Context, req *v1.AttachmentUploadContentUpdateReq) (*v1.AttachmentUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(supportdesk.CodeAttachmentRejected)
	}
	item, err := c.service.UploadAccountAttachment(ctx, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.AttachmentUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Version: item.Version, Ready: item.Ready}, nil
}
func (c *ControllerV1) InitialAttachmentUploadContentUpdate(ctx context.Context, req *v1.InitialAttachmentUploadContentUpdateReq) (*v1.InitialAttachmentUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(supportdesk.CodeAttachmentRejected)
	}
	item, err := c.service.UploadAccountAttachment(ctx, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.InitialAttachmentUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Version: item.Version, Ready: item.Ready}, nil
}
func (c *ControllerV1) AttachmentOpen(ctx context.Context, req *v1.AttachmentOpenReq) (*v1.AttachmentOpenRes, error) {
	file, err := c.service.OpenAccountAttachment(ctx, req.AttachmentNo)
	if err != nil {
		return nil, err
	}
	if err = supportstream.Write(ctx, file); err != nil {
		return nil, err
	}
	return &v1.AttachmentOpenRes{}, nil
}
