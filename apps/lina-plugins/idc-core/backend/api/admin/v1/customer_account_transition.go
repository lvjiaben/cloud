package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerAccountTransitionReq struct {
	g.Meta          `path:"/admin/customers/{customerNo}/account-status" method:"put" tags:"IDC Customer Management" summary:"Update customer account status" dc:"Apply an explicit staff-managed customer account status transition and retain audit evidence." permission:"idc:admin:customer:update"`
	CustomerNo      string `json:"customerNo" v:"required" dc:"Stable external customer number"`
	ToStatus        string `json:"toStatus" v:"required|in:active,restricted,disabled" dc:"Target customer account status"`
	Reason          string `json:"reason" v:"required|in:activation_completed,verification_required,risk_restriction,restriction_resolved,manual_disable,account_reactivated" dc:"Bounded account status transition reason"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Current customer optimistic concurrency version"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:12|max-length:128"`
}

type CustomerAccountTransitionRes struct {
	CustomerNo     string `json:"customerNo"`
	FromStatus     string `json:"fromStatus"`
	ToStatus       string `json:"toStatus"`
	Reason         string `json:"reason"`
	SubjectVersion int64  `json:"subjectVersion"`
	OccurredAt     int64  `json:"occurredAt" dc:"Database occurrence time in Unix milliseconds"`
}
