package admin

import (
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func toAPICommerceOrderRiskReviewDecision(
	decision *commerce.OrderRiskReviewDecision,
) *v1.CommerceOrderRiskReviewDecisionItem {
	if decision == nil {
		return nil
	}
	return &v1.CommerceOrderRiskReviewDecisionItem{
		ReviewNo: decision.ReviewNo, OrderNo: decision.OrderNo,
		CustomerNo: decision.CustomerNo, StatusCode: decision.StatusCode,
		ReasonCode: decision.ReasonCode, ReviewedBy: decision.ReviewedBy,
		ReviewedAt: decision.ReviewedAt.UnixMilli(), OrderStatus: decision.OrderStatus,
		Reused: decision.Reused,
	}
}
