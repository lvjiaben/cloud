package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerTagBatchReq struct {
	g.Meta         `path:"/admin/customers/tags/batch" method:"post" tags:"IDC Customer Management" summary:"Batch update customer tags" dc:"Assign or revoke at most twenty active tags for at most one hundred visible customers in one idempotent, audited transaction." permission:"idc:admin:customer:tag:batch"`
	Action         string   `json:"action" v:"required|in:assign,revoke" dc:"Relationship action: assign adds active tag links; revoke closes active links" eg:"assign"`
	CustomerNos    []string `json:"customerNos" v:"required" dc:"Unique customer business numbers; one to one hundred entries" eg:"[\"CUS_0123456789abcdef0123456789abcdef\"]"`
	TagNos         []string `json:"tagNos" v:"required" dc:"Unique active tag business numbers; one to twenty entries" eg:"[\"TAG_0123456789abcdef0123456789abcdef\"]"`
	Reason         string   `json:"reason" v:"required|max-length:256" dc:"Required normalized business reason recorded in sealed audit evidence" eg:"Quarterly account classification"`
	IdempotencyKey string   `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Stable client request key used to safely replay the same batch submission" eg:"customer-tag-batch-20260808-0001"`
}

type CustomerTagBatchRes struct {
	AuditNo       string `json:"auditNo" dc:"Sealed business audit number searchable from every affected customer" eg:"AUD_0123456789abcdef0123456789abcdef"`
	Replayed      bool   `json:"replayed" dc:"Whether this response reused an already committed idempotent request" eg:"false"`
	CustomerCount int    `json:"customerCount" dc:"Number of normalized affected customers" eg:"2"`
	TagCount      int    `json:"tagCount" dc:"Number of normalized affected tags" eg:"1"`
}
