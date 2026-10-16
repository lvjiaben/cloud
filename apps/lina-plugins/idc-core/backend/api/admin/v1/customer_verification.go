// Staff verification commands update only the core-owned current identity projection.
package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerVerificationUpdateReq requires a current customer version and a stable retry key.
type CustomerVerificationUpdateReq struct {
	g.Meta          `path:"/admin/customers/{customerNo}/verification" method:"put" tags:"IDC Customer Management" summary:"Update customer verification" dc:"Manually approve or clear current identity data without deleting historical cases, orders, contracts or audit records." permission:"idc:admin:customer:update"`
	CustomerNo      string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Visible customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	Action          string `json:"action" v:"required|in:approve,clear" dc:"Action: approve grants manual verification; clear removes current identity data and verification" eg:"approve"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Current customer version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Stable retry key" eg:"550e8400-e29b-41d4-a716-446655440000"`
}

// CustomerVerificationUpdateRes acknowledges the committed command; callers refresh current details.
type CustomerVerificationUpdateRes struct{}
