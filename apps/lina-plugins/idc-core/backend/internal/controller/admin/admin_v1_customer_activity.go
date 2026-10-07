package admin

import (
	"context"
	"time"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
)

func (c *ControllerV1) CustomerNoteCreate(ctx context.Context, req *v1.CustomerNoteCreateReq) (*v1.CustomerNoteCreateRes, error) {
	item, err := c.customerMutation.AddNote(ctx, customermanagement.NoteInput{
		CustomerNo: req.CustomerNo, Category: req.Category, Content: req.Content, SupersedesNo: req.SupersedesNo, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerNoteCreateRes{NoteNo: item.NoteNo, CustomerNo: item.CustomerNo, Category: item.Category,
		Status: item.Status, SupersedesNo: item.SupersedesNo, CreatedAt: item.CreatedAt.UnixMilli()}, nil
}

func (c *ControllerV1) CustomerFollowUpPlan(ctx context.Context, req *v1.CustomerFollowUpPlanReq) (*v1.CustomerFollowUpPlanRes, error) {
	item, err := c.customerMutation.PlanFollowUp(ctx, customermanagement.FollowUpPlanInput{
		CustomerNo: req.CustomerNo, Channel: req.Channel, Subject: req.Subject, Detail: req.Detail,
		AssignedUserID: req.AssignedUserID, PlannedAt: time.UnixMilli(req.PlannedAt), IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return followUpResult(item), nil
}

func (c *ControllerV1) CustomerFollowUpTransition(ctx context.Context, req *v1.CustomerFollowUpTransitionReq) (*v1.CustomerFollowUpTransitionRes, error) {
	item, err := c.customerMutation.TransitionFollowUp(ctx, customermanagement.FollowUpTransitionInput{
		CustomerNo: req.CustomerNo, FollowUpNo: req.FollowUpNo, Action: req.Action, Outcome: req.Outcome, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return followUpResult(item), nil
}

func followUpResult(item *customermanagement.FollowUpResult) *v1.CustomerFollowUpPlanRes {
	return &v1.CustomerFollowUpPlanRes{FollowUpNo: item.FollowUpNo, CustomerNo: item.CustomerNo,
		Status: item.Status, PlannedAt: item.PlannedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli()}
}

func (c *ControllerV1) CustomerBlacklistCreate(ctx context.Context, req *v1.CustomerBlacklistCreateReq) (*v1.CustomerBlacklistCreateRes, error) {
	input := customermanagement.BlacklistCreateInput{CustomerNo: req.CustomerNo, Scope: req.Scope,
		ReasonCode: req.ReasonCode, ReasonDetail: req.ReasonDetail, EvidenceReference: req.EvidenceReference,
		RiskLevel: req.RiskLevel, IdempotencyKey: req.IdempotencyKey}
	if req.StartsAt > 0 {
		input.StartsAt = time.UnixMilli(req.StartsAt)
	}
	if req.ExpiresAt > 0 {
		expires := time.UnixMilli(req.ExpiresAt)
		input.ExpiresAt = &expires
	}
	item, err := c.customerMutation.CreateBlacklist(ctx, input)
	if err != nil {
		return nil, err
	}
	return blacklistResult(item), nil
}

func (c *ControllerV1) CustomerBlacklistDecision(ctx context.Context, req *v1.CustomerBlacklistDecisionReq) (*v1.CustomerBlacklistDecisionRes, error) {
	item, err := c.customerMutation.DecideBlacklist(ctx, customermanagement.BlacklistDecisionInput{
		CustomerNo: req.CustomerNo, EntryNo: req.EntryNo, Action: req.Action, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return blacklistResult(item), nil
}

func blacklistResult(item *customermanagement.BlacklistResult) *v1.CustomerBlacklistCreateRes {
	return &v1.CustomerBlacklistCreateRes{EntryNo: item.EntryNo, CustomerNo: item.CustomerNo,
		Status: item.Status, UpdatedAt: item.UpdatedAt.UnixMilli()}
}
