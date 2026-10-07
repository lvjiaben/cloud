package supportadmin

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/controller/supportstream"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

type ControllerV1 struct{ service supportdesk.Service }

func NewV1(service supportdesk.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("support admin controller requires service")
	}
	return &ControllerV1{service: service}, nil
}
func (c *ControllerV1) TicketList(ctx context.Context, req *v1.TicketListReq) (*v1.TicketListRes, error) {
	page, err := c.service.SearchStaff(ctx, supportdesk.SearchInput{Stage: req.Stage, Status: req.Status, Priority: req.Priority, Type: req.Type, DepartmentNo: req.DepartmentNo, AssigneeUserID: req.AssigneeUserID, FollowUp: req.FollowUp, Query: req.Query, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.TicketItem, 0, len(page.Tickets))
	for _, item := range page.Tickets {
		items = append(items, ToTicket(item))
	}
	return &v1.TicketListRes{Tickets: items, NextCursor: page.NextCursor, PageActions: append([]string(nil), page.Actions...)}, nil
}
func (c *ControllerV1) TicketGlobalList(ctx context.Context, req *v1.TicketGlobalListReq) (*v1.TicketGlobalListRes, error) {
	page, err := c.service.SearchAllStaff(ctx, supportdesk.SearchInput{Stage: req.Stage, Status: req.Status, Priority: req.Priority, Type: req.Type, DepartmentNo: req.DepartmentNo, AssigneeUserID: req.AssigneeUserID, FollowUp: req.FollowUp, Query: req.Query, Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.TicketItem, 0, len(page.Tickets))
	for _, item := range page.Tickets {
		items = append(items, ToTicket(item))
	}
	return &v1.TicketGlobalListRes{Tickets: items, NextCursor: page.NextCursor, PageActions: append([]string(nil), page.Actions...)}, nil
}
func (c *ControllerV1) TicketDetail(ctx context.Context, req *v1.TicketDetailReq) (*v1.TicketDetailRes, error) {
	item, err := c.service.DetailStaff(ctx, req.TicketNo)
	if err != nil {
		return nil, err
	}
	return &v1.TicketDetailRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketCreate(ctx context.Context, req *v1.TicketCreateReq) (*v1.TicketCreateRes, error) {
	item, err := c.service.CreateStaff(ctx, supportdesk.CreateInput{DepartmentNo: req.DepartmentNo, CategoryNo: req.CategoryNo, Subject: req.Subject, Body: req.Body, Priority: req.Priority, UploadNos: req.UploadNos, CustomFields: fromAdminCustomFieldSubmissions(req.CustomFields)})
	if err != nil {
		return nil, err
	}
	return &v1.TicketCreateRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketPublicReply(ctx context.Context, req *v1.TicketPublicReplyReq) (*v1.TicketPublicReplyRes, error) {
	item, err := c.service.ReplyStaff(ctx, supportdesk.MessageInput{TicketNo: req.TicketNo, Body: req.Body, UploadNos: req.UploadNos, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketPublicReplyRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketInternalNote(ctx context.Context, req *v1.TicketInternalNoteReq) (*v1.TicketInternalNoteRes, error) {
	item, err := c.service.NoteStaff(ctx, supportdesk.MessageInput{TicketNo: req.TicketNo, Body: req.Body, UploadNos: req.UploadNos, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketInternalNoteRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketClaim(ctx context.Context, req *v1.TicketClaimReq) (*v1.TicketClaimRes, error) {
	item, err := c.service.Claim(ctx, supportdesk.ClaimInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketClaimRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketTransfer(ctx context.Context, req *v1.TicketTransferReq) (*v1.TicketTransferRes, error) {
	item, err := c.service.Transfer(ctx, supportdesk.TransferInput{TicketNo: req.TicketNo, DepartmentNo: req.DepartmentNo, Reason: req.Reason, AssigneeUserID: req.AssigneeUserID, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketTransferRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketResolve(ctx context.Context, req *v1.TicketResolveReq) (*v1.TicketResolveRes, error) {
	item, err := c.service.Resolve(ctx, supportdesk.TransitionInput{TicketNo: req.TicketNo, Reason: req.Reason, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketResolveRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketClose(ctx context.Context, req *v1.TicketCloseReq) (*v1.TicketCloseRes, error) {
	item, err := c.service.CloseStaff(ctx, supportdesk.TransitionInput{TicketNo: req.TicketNo, Reason: req.Reason, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketCloseRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketReopen(ctx context.Context, req *v1.TicketReopenReq) (*v1.TicketReopenRes, error) {
	item, err := c.service.ReopenStaff(ctx, supportdesk.TransitionInput{TicketNo: req.TicketNo, Reason: req.Reason, TicketVersion: req.TicketVersion})
	if err != nil {
		return nil, err
	}
	return &v1.TicketReopenRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketMerge(ctx context.Context, req *v1.TicketMergeReq) (*v1.TicketMergeRes, error) {
	sources := make([]supportdesk.VersionedTicket, 0, len(req.Sources))
	for _, source := range req.Sources {
		sources = append(sources, supportdesk.VersionedTicket{TicketNo: source.TicketNo, Version: source.Version})
	}
	item, err := c.service.Merge(ctx, supportdesk.MergeInput{Main: supportdesk.VersionedTicket{TicketNo: req.TicketNo, Version: req.MainVersion}, Sources: sources, Reason: req.Reason}, false)
	if err != nil {
		return nil, err
	}
	return &v1.TicketMergeRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketMergeCandidateList(ctx context.Context, req *v1.TicketMergeCandidateListReq) (*v1.TicketMergeCandidateListRes, error) {
	page, err := c.service.MergeCandidates(ctx, req.TicketNo, req.Cursor, req.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]v1.MergeCandidateItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.MergeCandidateItem{TicketNo: item.TicketNo, Subject: item.Subject, Stage: item.Stage, Version: item.Version})
	}
	return &v1.TicketMergeCandidateListRes{Items: items, NextCursor: page.NextCursor}, nil
}
func (c *ControllerV1) TicketMergeAllCandidateList(ctx context.Context, req *v1.TicketMergeAllCandidateListReq) (*v1.TicketMergeAllCandidateListRes, error) {
	page, err := c.service.MergeAllCandidates(ctx, req.TicketNo, req.Cursor, req.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]v1.MergeCandidateItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.MergeCandidateItem{TicketNo: item.TicketNo, Subject: item.Subject, Stage: item.Stage, Version: item.Version})
	}
	return &v1.TicketMergeAllCandidateListRes{Items: items, NextCursor: page.NextCursor}, nil
}
func (c *ControllerV1) TicketMergeAll(ctx context.Context, req *v1.TicketMergeAllReq) (*v1.TicketMergeAllRes, error) {
	sources := make([]supportdesk.VersionedTicket, 0, len(req.Sources))
	for _, source := range req.Sources {
		sources = append(sources, supportdesk.VersionedTicket{TicketNo: source.TicketNo, Version: source.Version})
	}
	item, err := c.service.Merge(ctx, supportdesk.MergeInput{Main: supportdesk.VersionedTicket{TicketNo: req.TicketNo, Version: req.MainVersion}, Sources: sources, Reason: req.Reason}, true)
	if err != nil {
		return nil, err
	}
	return &v1.TicketMergeAllRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketSplit(ctx context.Context, req *v1.TicketSplitReq) (*v1.TicketSplitRes, error) {
	item, err := c.service.Split(ctx, supportdesk.SplitInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion, CategoryNo: req.CategoryNo, MessageNos: req.MessageNos, AttachmentNos: req.AttachmentNos, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &v1.TicketSplitRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketEscalate(ctx context.Context, req *v1.TicketEscalateReq) (*v1.TicketEscalateRes, error) {
	item, err := c.service.Escalate(ctx, supportdesk.EscalateInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion, ToUserID: req.ToUserID, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &v1.TicketEscalateRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) TicketHide(ctx context.Context, req *v1.TicketHideReq) (*v1.TicketHideRes, error) {
	item, err := c.service.Hide(ctx, supportdesk.HideInput{TicketNo: req.TicketNo, TicketVersion: req.TicketVersion, TargetType: req.TargetType, TargetNo: req.TargetNo, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &v1.TicketHideRes{Item: ToDetail(item)}, nil
}
func (c *ControllerV1) AttachmentUploadCreate(ctx context.Context, req *v1.AttachmentUploadCreateReq) (*v1.AttachmentUploadCreateRes, error) {
	if req.Context != "" && req.Context != "public_reply" {
		return nil, bizerr.NewCode(supportdesk.CodeNotAccessible)
	}
	item, err := c.service.ReserveStaffAttachment(ctx, supportdesk.AttachmentUploadInput{TicketNo: req.TicketNo, Context: "public_reply", FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.AttachmentUploadCreateRes{UploadNo: item.UploadNo, NextOffset: item.NextOffset, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}, nil
}
func (c *ControllerV1) InternalAttachmentUploadCreate(ctx context.Context, req *v1.InternalAttachmentUploadCreateReq) (*v1.InternalAttachmentUploadCreateRes, error) {
	item, err := c.service.ReserveStaffAttachment(ctx, supportdesk.AttachmentUploadInput{TicketNo: req.TicketNo, Context: "internal_note", FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.InternalAttachmentUploadCreateRes{UploadNo: item.UploadNo, NextOffset: item.NextOffset, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}, nil
}
func (c *ControllerV1) InternalTicketAttachmentUploadCreate(ctx context.Context, req *v1.InternalTicketAttachmentUploadCreateReq) (*v1.InternalTicketAttachmentUploadCreateRes, error) {
	item, err := c.service.ReserveStaffInitialAttachment(ctx, supportdesk.AttachmentUploadInput{FileName: req.FileName, SizeBytes: req.SizeBytes})
	if err != nil {
		return nil, err
	}
	return &v1.InternalTicketAttachmentUploadCreateRes{UploadNo: item.UploadNo, NextOffset: item.NextOffset, ExpiresAt: item.ExpiresAt.UnixMilli(), Version: item.Version}, nil
}
func (c *ControllerV1) AttachmentUploadContentUpdate(ctx context.Context, req *v1.AttachmentUploadContentUpdateReq) (*v1.AttachmentUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(supportdesk.CodeAttachmentRejected)
	}
	item, err := c.service.UploadStaffAttachment(ctx, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.AttachmentUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Version: item.Version, Ready: item.Ready}, nil
}
func (c *ControllerV1) InternalAttachmentUploadContentUpdate(ctx context.Context, req *v1.InternalAttachmentUploadContentUpdateReq) (*v1.InternalAttachmentUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(supportdesk.CodeAttachmentRejected)
	}
	item, err := c.service.UploadStaffAttachment(ctx, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.InternalAttachmentUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Version: item.Version, Ready: item.Ready}, nil
}
func (c *ControllerV1) InternalTicketAttachmentUploadContentUpdate(ctx context.Context, req *v1.InternalTicketAttachmentUploadContentUpdateReq) (*v1.InternalTicketAttachmentUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(supportdesk.CodeAttachmentRejected)
	}
	item, err := c.service.UploadStaffAttachment(ctx, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.InternalTicketAttachmentUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes, Version: item.Version, Ready: item.Ready}, nil
}
func (c *ControllerV1) AttachmentOpen(ctx context.Context, req *v1.AttachmentOpenReq) (*v1.AttachmentOpenRes, error) {
	file, err := c.service.OpenStaffAttachment(ctx, req.AttachmentNo)
	if err != nil {
		return nil, err
	}
	if err = supportstream.Write(ctx, file); err != nil {
		return nil, err
	}
	return &v1.AttachmentOpenRes{}, nil
}
func (c *ControllerV1) DepartmentList(ctx context.Context, _ *v1.DepartmentListReq) (*v1.DepartmentListRes, error) {
	items, err := c.service.Departments(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.DepartmentItem, 0, len(items))
	for _, item := range items {
		result = append(result, v1.DepartmentItem{DepartmentNo: item.DepartmentNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, ParentDepartmentNo: item.ParentDepartmentNo, AssignmentMode: item.AssignmentMode, Status: item.Status, Version: item.Version})
	}
	return &v1.DepartmentListRes{Departments: result}, nil
}
func (c *ControllerV1) IssueCategoryList(ctx context.Context, _ *v1.IssueCategoryListReq) (*v1.IssueCategoryListRes, error) {
	items, err := c.service.IssueCategories(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.IssueCategoryItem, 0, len(items))
	for _, item := range items {
		result = append(result, v1.IssueCategoryItem{CategoryNo: item.CategoryNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, Priority: item.Priority, SortOrder: item.SortOrder})
	}
	return &v1.IssueCategoryListRes{IssueCategories: result}, nil
}
func (c *ControllerV1) StatusList(ctx context.Context, _ *v1.StatusListReq) (*v1.StatusListRes, error) {
	items, err := c.service.Statuses(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.StatusItem, 0, len(items))
	for _, item := range items {
		result = append(result, v1.StatusItem{StatusNo: item.StatusNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, BaseStage: item.BaseStage, Status: item.Status, Version: item.Version})
	}
	return &v1.StatusListRes{Statuses: result}, nil
}
func (c *ControllerV1) ReplyPresetList(ctx context.Context, req *v1.ReplyPresetListReq) (*v1.ReplyPresetListRes, error) {
	items, err := c.service.ReplyPresets(ctx, req.Locale)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ReplyPresetItem, 0, len(items))
	for _, item := range items {
		result = append(result, v1.ReplyPresetItem{PresetNo: item.PresetNo, DepartmentNo: item.DepartmentNo, Locale: item.Locale, Title: item.Title, Body: item.Body, Status: item.Status, Version: item.Version})
	}
	return &v1.ReplyPresetListRes{ReplyPresets: result}, nil
}
func (c *ControllerV1) AutomationPolicyGet(ctx context.Context, _ *v1.AutomationPolicyGetReq) (*v1.AutomationPolicyGetRes, error) {
	item, err := c.service.AutomationPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AutomationPolicyGetRes{Policy: &v1.AutomationPolicyItem{AutoCloseDays: item.AutoCloseDays, ReopenDays: item.ReopenDays, Version: item.Version}}, nil
}
func (c *ControllerV1) AutomationPolicyUpdate(ctx context.Context, req *v1.AutomationPolicyUpdateReq) (*v1.AutomationPolicyUpdateRes, error) {
	item, err := c.service.UpdateAutomationPolicy(ctx, supportdesk.AutomationPolicyInput{AutoCloseDays: req.AutoCloseDays, ReopenDays: req.ReopenDays, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.AutomationPolicyUpdateRes{Policy: &v1.AutomationPolicyItem{AutoCloseDays: item.AutoCloseDays, ReopenDays: item.ReopenDays, Version: item.Version}}, nil
}

func ToTicket(item *supportdesk.Ticket) *v1.TicketItem {
	if item == nil {
		return nil
	}
	queue := "assigned"
	if item.AssigneeUserID == 0 && item.DepartmentNo == "" {
		queue = "public"
	} else if item.AssigneeUserID == 0 {
		queue = "department"
	}
	result := &v1.TicketItem{TicketNo: item.TicketNo, Type: item.TicketType, Visibility: item.Visibility, CustomerNo: item.CustomerNo, CustomerDisplayLabel: item.CustomerDisplayLabel, CategoryNo: item.CategoryNo, CategoryNameZhCN: item.CategoryNameZhCN, CategoryNameEnUS: item.CategoryNameEnUS, DepartmentNo: item.DepartmentNo, Queue: queue, AssigneeUserID: item.AssigneeUserID, Subject: item.Subject, BaseStage: item.BaseStage, CustomerStage: item.CustomerStage, CustomStatusNo: item.CustomStatusNo, Priority: item.Priority, CurrentRound: item.CurrentRound, FollowUp: item.FollowUpRequired, SLARisk: item.SLARisk, SLAConfigured: item.SLAConfigured, ClosedReason: item.ClosedReason, LastActivityAt: item.LastActivityAt.UnixMilli(), CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), Version: item.Version, RelatedEntitlement: toAdminRelatedEntitlement(item)}
	if item.ResolvedAt != nil {
		result.ResolvedAt = item.ResolvedAt.UnixMilli()
	}
	if item.ClosedAt != nil {
		result.ClosedAt = item.ClosedAt.UnixMilli()
	}
	return result
}
func ToDetail(item *supportdesk.Detail) *v1.TicketDetail {
	if item == nil {
		return nil
	}
	result := &v1.TicketDetail{Ticket: ToTicket(item.Ticket), PublicMessages: toMessages(item.PublicMessages), InternalNotes: toMessages(item.InternalNotes), Attachments: toAttachments(item.Attachments), AvailableActions: item.AvailableActions, AvailableStatusTransitions: toStatusTransitionOptions(item.AvailableStatusTransitions), CustomFields: toAdminCustomFieldValues(item.CustomFields)}
	result.AssignmentHistory = make([]v1.AssignmentItem, 0, len(item.AssignmentHistory))
	for _, history := range item.AssignmentHistory {
		result.AssignmentHistory = append(result.AssignmentHistory, v1.AssignmentItem{HistoryNo: history.HistoryNo, Action: history.Action, FromDepartmentNo: history.FromDepartmentNo, ToDepartmentNo: history.ToDepartmentNo, FromUserID: history.FromUserID, ToUserID: history.ToUserID, ActorUserID: history.ActorUserID, ReasonCode: history.ReasonCode, Reason: history.Reason, OccurredAt: history.OccurredAt.UnixMilli()})
	}
	result.Relations = make([]v1.RelationItem, 0, len(item.Relations))
	for _, relation := range item.Relations {
		result.Relations = append(result.Relations, v1.RelationItem{RelationNo: relation.RelationNo, Type: relation.Type, SourceTicketNo: relation.SourceTicketNo, TargetTicketNo: relation.TargetTicketNo, ActorUserID: relation.ActorUserID, Reason: relation.Reason, CreatedAt: relation.CreatedAt.UnixMilli()})
	}
	result.Escalations = make([]v1.EscalationItem, 0, len(item.Escalations))
	for _, escalation := range item.Escalations {
		result.Escalations = append(result.Escalations, v1.EscalationItem{EscalationNo: escalation.EscalationNo, FromUserID: escalation.FromUserID, ToUserID: escalation.ToUserID, Reason: escalation.Reason, CreatedAt: escalation.CreatedAt.UnixMilli()})
	}
	result.SLARounds = make([]v1.SLARoundItem, 0, len(item.SLARounds))
	for _, round := range item.SLARounds {
		projection := v1.SLARoundItem{RoundNo: round.RoundNo, RuleNo: round.RuleNo, CalendarNo: round.CalendarNo, CalendarMode: round.CalendarMode, PausedSeconds: round.PausedSeconds, FirstResponseBreached: round.FirstResponseBreached, ResolutionBreached: round.ResolutionBreached}
		projection.FirstResponseDueAt = unixMillis(round.FirstResponseDueAt)
		projection.ResolutionDueAt = unixMillis(round.ResolutionDueAt)
		projection.FirstRespondedAt = unixMillis(round.FirstRespondedAt)
		projection.ResolvedAt = unixMillis(round.ResolvedAt)
		projection.StoppedAt = unixMillis(round.StoppedAt)
		result.SLARounds = append(result.SLARounds, projection)
	}
	result.StatusHistory = make([]v1.StatusHistoryItem, 0, len(item.StatusHistory))
	for _, history := range item.StatusHistory {
		result.StatusHistory = append(result.StatusHistory, v1.StatusHistoryItem{HistoryNo: history.HistoryNo, FromStage: history.FromStage, ToStage: history.ToStage, CustomStatusNo: history.CustomStatusNo, Source: history.Source, ReasonCode: history.ReasonCode, Reason: history.Reason, ActorUserID: history.ActorUserID, RuleVersion: history.RuleVersion, OccurredAt: history.OccurredAt.UnixMilli()})
	}
	return result
}

func toStatusTransitionOptions(items []supportdesk.StatusTransitionOption) []v1.TicketStatusTransitionOption {
	result := make([]v1.TicketStatusTransitionOption, 0, len(items))
	for _, item := range items {
		result = append(result, v1.TicketStatusTransitionOption{
			StatusNo: item.StatusNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS,
			BaseStage: item.BaseStage, Version: item.Version, RequiresReason: item.RequiresReason,
		})
	}
	return result
}

func toMessages(items []supportdesk.Message) []v1.MessageItem {
	result := make([]v1.MessageItem, 0, len(items))
	for _, message := range items {
		result = append(result, v1.MessageItem{MessageNo: message.MessageNo, Type: message.Type, Visibility: message.Visibility, AuthorType: message.AuthorType, AuthorUserID: message.AuthorUserID, Body: message.Body, Automated: message.Automated, Hidden: message.Hidden, CreatedAt: message.CreatedAt.UnixMilli(), Attachments: toAttachments(message.Attachments)})
	}
	return result
}

func toAttachments(items []supportdesk.Attachment) []v1.AttachmentItem {
	result := make([]v1.AttachmentItem, 0, len(items))
	for _, attachment := range items {
		result = append(result, v1.AttachmentItem{AttachmentNo: attachment.AttachmentNo, MessageNo: attachment.MessageNo, Visibility: attachment.Visibility, FileName: attachment.FileName, MediaType: attachment.MediaType, SizeBytes: attachment.SizeBytes, Hidden: attachment.Hidden, CreatedAt: attachment.CreatedAt.UnixMilli()})
	}
	return result
}

func unixMillis(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}
