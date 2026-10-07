package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderRiskReviewRejection(ctx context.Context, req *v1.CommerceOrderRiskReviewRejectionReq) (res *v1.CommerceOrderRiskReviewRejectionRes, err error) {
	decision, err := c.commerceRiskReview.DecideRiskReview(ctx, commerce.OrderRiskReviewDecisionInput{
		ReviewNo: req.ReviewNo, RequestNo: req.RequestNo,
		DecisionCode:   commerce.OrderRiskReviewDecisionRejected,
		EvidenceDigest: req.EvidenceDigest,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderRiskReviewRejectionRes{Decision: toAPICommerceOrderRiskReviewDecision(decision)}, nil
}
