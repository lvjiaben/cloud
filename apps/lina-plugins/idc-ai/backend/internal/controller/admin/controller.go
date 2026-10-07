package admin

import (
	"context"
	v1 "lina-plugin-idc-ai/backend/api/admin/v1"
	"lina-plugin-idc-ai/backend/internal/service/copilot"
)

type ControllerV1 struct{ service *copilot.Service }

func NewV1(s *copilot.Service) *ControllerV1 { return &ControllerV1{service: s} }
func (c *ControllerV1) AIConfigGet(ctx context.Context, _ *v1.AIConfigGetReq) (*v1.AIConfigGetRes, error) {
	x, e := c.service.GetConfig(ctx)
	if e != nil {
		return nil, e
	}
	return &v1.AIConfigGetRes{Enabled: x.Enabled, BaseURL: x.BaseURL, Model: x.Model, KeyConfigured: x.KeyConfigured, BudgetMinor: x.BudgetMinor, RateLimitPerMinute: x.RateLimitPerMinute, RetentionDays: x.RetentionDays, Revision: x.Revision}, nil
}
func (c *ControllerV1) AIConfigUpdate(ctx context.Context, r *v1.AIConfigUpdateReq) (*v1.AIConfigUpdateRes, error) {
	x, e := c.service.UpdateConfig(ctx, r.BaseURL, r.APIKey, r.Model, r.Enabled, r.BudgetMinor, r.RateLimitPerMinute, r.RetentionDays)
	if e != nil {
		return nil, e
	}
	return &v1.AIConfigUpdateRes{AIConfigGetRes: v1.AIConfigGetRes{Enabled: x.Enabled, BaseURL: x.BaseURL, Model: x.Model, KeyConfigured: x.KeyConfigured, BudgetMinor: x.BudgetMinor, RateLimitPerMinute: x.RateLimitPerMinute, RetentionDays: x.RetentionDays, Revision: x.Revision}}, nil
}
func (c *ControllerV1) AIConfigTest(ctx context.Context, r *v1.AIConfigTestReq) (*v1.AIConfigTestRes, error) {
	ok, msg := c.service.TestConfig(ctx, r.BaseURL, r.APIKey, r.Model)
	st := "unavailable"
	if ok {
		st = "reachable"
	}
	return &v1.AIConfigTestRes{Reachable: ok, Status: st, Message: msg}, nil
}
func (c *ControllerV1) AIChat(ctx context.Context, r *v1.AIChatReq) (*v1.AIChatRes, error) {
	a, e, d, u, s, m, id, er := c.service.Chat(ctx, r.Prompt, "staff", "admin")
	if er != nil {
		return nil, er
	}
	return &v1.AIChatRes{ConversationID: id, Answer: a, Evidence: e, DataAsOf: d, Uncertainty: u, Safety: s, Model: m}, nil
}
func (c *ControllerV1) AIFeedback(ctx context.Context, r *v1.AIFeedbackReq) (*v1.AIFeedbackRes, error) {
	if e := c.service.Feedback(ctx, r.ConversationID, "admin", r.Rating, r.Comment); e != nil {
		return nil, e
	}
	return &v1.AIFeedbackRes{Accepted: true}, nil
}
func (c *ControllerV1) AIAuditList(ctx context.Context, r *v1.AIAuditListReq) (*v1.AIAuditListRes, error) {
	items, e := c.service.Audits(ctx, r.Limit)
	if e != nil {
		return nil, e
	}
	out := make([]v1.AIAuditItem, 0, len(items))
	for _, i := range items {
		tool, _ := i["tool"].(string)
		decision, _ := i["decision"].(string)
		reason, _ := i["reason"].(string)
		createdAt, _ := i["createdAt"].(string)
		out = append(out, v1.AIAuditItem{Tool: tool, Decision: decision, Reason: reason, CreatedAt: createdAt})
	}
	return &v1.AIAuditListRes{Items: out}, nil
}
