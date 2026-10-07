// This file defines the reviewed granted-value posting request API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// MarketingValueGrantPostRequestReq requests a second-person-reviewed ledger posting.
type MarketingValueGrantPostRequestReq struct {
	g.Meta     `path:"/admin/marketing/grants/{grantNo}/posting-requests" method:"post" tags:"IDC Marketing" summary:"Request a granted-value ledger posting" dc:"Create a mandatory second-person review request. This call never credits the customer gift account directly." permission:"idc:admin:marketing:grant:post"`
	GrantNo    string `json:"grantNo" v:"required|length:1,40" dc:"Stable marketing value grant number"`
	RequestNo  string `json:"requestNo" v:"required|length:1,128" dc:"Stable idempotent staff request number"`
	ReasonCode string `json:"reasonCode" v:"required|length:1,64" dc:"Stable business reason code"`
}

// MarketingValueGrantPostRequestRes returns the pending or reused request.
type MarketingValueGrantPostRequestRes struct {
	Mutation *MarketingValueGrantFinancialMutation `json:"mutation" dc:"Reviewed financial action projection"`
}
