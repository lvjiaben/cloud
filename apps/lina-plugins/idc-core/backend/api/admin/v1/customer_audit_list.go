// Customer audit lists use the audit owner's scoped reader and return no sensitive snapshots.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerAuditListReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/audit-records" method:"get" tags:"IDC Customer Management" summary:"Read recent customer operation records" dc:"Return at most fifty recent audit summaries for a visible customer. Secret snapshots and technical request metadata are excluded." permission:"idc:admin:audit:list"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Visible customer number" eg:"CUS_0123456789abcdef0123456789abcdef"`
}
type CustomerAuditItem struct {
	AuditNo       string `json:"auditNo" dc:"Audit business number" eg:"AUD_0123456789abcdef0123456789abcdef"`
	OccurredAt    int64  `json:"occurredAt" dc:"Unix timestamp in milliseconds" eg:"1780000000000"`
	OperationCode string `json:"operationCode" dc:"Stable operation code for translated display" eg:"customer.password.reset"`
	Result        string `json:"result" dc:"Result: succeeded, denied or failed" eg:"succeeded"`
}
type CustomerAuditListRes struct {
	Items []*CustomerAuditItem `json:"items" dc:"Up to fifty recent operation records" eg:"[]"`
}
