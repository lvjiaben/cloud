// This file defines the reviewed granted-value reversal request API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// MarketingValueGrantReverseRequestReq requests a partial unused-value reversal.
type MarketingValueGrantReverseRequestReq struct {
	g.Meta      `path:"/admin/marketing/grants/{grantNo}/reversal-requests" method:"post" tags:"IDC Marketing" summary:"Request a granted-value ledger reversal" dc:"Reverse the exact unused grant amount through the gift ledger with atomic entitlement update and audit after exact action authorization." permission:"idc:admin:marketing:grant:reverse"`
	GrantNo     string `json:"grantNo" v:"required|length:1,40" dc:"Stable marketing value grant number"`
	RequestNo   string `json:"requestNo" v:"required|length:1,128" dc:"Stable idempotent staff request number"`
	AmountMinor string `json:"amountMinor" v:"required|regex:^\\d+$" dc:"Positive integer amount in the grant currency minor unit"`
	ReasonCode  string `json:"reasonCode" v:"max-length:64" dc:"Stable business reason code"`
}

// MarketingValueGrantReverseRequestRes returns the pending or reused request.
type MarketingValueGrantReverseRequestRes struct {
	Mutation *MarketingValueGrantFinancialMutation `json:"mutation" dc:"Reviewed financial action projection"`
}
