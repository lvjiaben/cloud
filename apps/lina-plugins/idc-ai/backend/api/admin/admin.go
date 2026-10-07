package admin

import (
	"context"
	v1 "lina-plugin-idc-ai/backend/api/admin/v1"
)

type IAdminV1 interface {
	AIConfigGet(context.Context, *v1.AIConfigGetReq) (*v1.AIConfigGetRes, error)
	AIConfigUpdate(context.Context, *v1.AIConfigUpdateReq) (*v1.AIConfigUpdateRes, error)
	AIConfigTest(context.Context, *v1.AIConfigTestReq) (*v1.AIConfigTestRes, error)
	AIChat(context.Context, *v1.AIChatReq) (*v1.AIChatRes, error)
	AIFeedback(context.Context, *v1.AIFeedbackReq) (*v1.AIFeedbackRes, error)
	AIAuditList(context.Context, *v1.AIAuditListReq) (*v1.AIAuditListRes, error)
}
