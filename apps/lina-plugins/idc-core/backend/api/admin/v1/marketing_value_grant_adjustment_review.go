// This file defines second-person decisions for granted-value financial actions.

package v1

import "github.com/gogf/gf/v2/frame/g"

// MarketingValueGrantAdjustmentReviewReq records an independent staff decision.
type MarketingValueGrantAdjustmentReviewReq struct {
	g.Meta       `path:"/admin/marketing/grant-adjustments/{adjustmentNo}/reviews" method:"post" tags:"IDC Marketing" summary:"Review a granted-value financial action" dc:"Approve or reject one pending granted-value posting or reversal. The requester cannot review their own action." permission:"idc:admin:marketing:grant:review"`
	AdjustmentNo string `json:"adjustmentNo" v:"required|length:1,40" dc:"Stable financial review request number"`
	Decision     string `json:"decision" v:"required|in:approve,reject" dc:"Approve or reject decision"`
	ReasonCode   string `json:"reasonCode" v:"required|length:1,64" dc:"Stable decision reason code"`
}

// MarketingValueGrantAdjustmentReviewRes returns the final or reused decision.
type MarketingValueGrantAdjustmentReviewRes struct {
	Mutation *MarketingValueGrantFinancialMutation `json:"mutation" dc:"Reviewed financial action projection"`
}
