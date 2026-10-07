package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderRiskReviewApproval(ctx context.Context, req *v1.CommerceOrderRiskReviewApprovalReq) (res *v1.CommerceOrderRiskReviewApprovalRes, err error) {
	decision, err := c.commerceRiskReview.DecideRiskReview(ctx, commerce.OrderRiskReviewDecisionInput{
		ReviewNo: req.ReviewNo, RequestNo: req.RequestNo,
		DecisionCode:   commerce.OrderRiskReviewDecisionApproved,
		EvidenceDigest: req.EvidenceDigest,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderRiskReviewApprovalRes{Decision: toAPICommerceOrderRiskReviewDecision(decision)}, nil
}
