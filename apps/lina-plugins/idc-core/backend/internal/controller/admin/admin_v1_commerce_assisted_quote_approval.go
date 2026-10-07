package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceAssistedQuoteApproval(ctx context.Context, req *v1.CommerceAssistedQuoteApprovalReq) (res *v1.CommerceAssistedQuoteApprovalRes, err error) {
	request, err := c.commerceAssistedQuotes.DecideAssistedQuote(ctx, commerce.AssistedQuoteDecisionInput{
		RequestNo: req.RequestNo, DecisionCode: commerce.AssistedQuoteDecisionApproved,
		DecisionReason: req.DecisionReason, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceAssistedQuoteApprovalRes{Request: toAPICommerceAssistedQuote(request)}, nil
}
