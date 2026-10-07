package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceAssistedQuoteRejection(ctx context.Context, req *v1.CommerceAssistedQuoteRejectionReq) (res *v1.CommerceAssistedQuoteRejectionRes, err error) {
	request, err := c.commerceAssistedQuotes.DecideAssistedQuote(ctx, commerce.AssistedQuoteDecisionInput{
		RequestNo: req.RequestNo, DecisionCode: commerce.AssistedQuoteDecisionRejected,
		DecisionReason: req.DecisionReason, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceAssistedQuoteRejectionRes{Request: toAPICommerceAssistedQuote(request)}, nil
}
