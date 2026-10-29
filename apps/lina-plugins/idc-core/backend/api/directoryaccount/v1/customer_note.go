// This file exposes only the authenticated customer's non-sensitive entitlement note.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerNote struct {
	EntitlementNo string `json:"entitlementNo" dc:"Exact customer-owned service entitlement" eg:"ent_0123456789abcdef0123456789abcdef"`
	Remark        string `json:"remark" dc:"Customer non-sensitive note, at most 1000 characters; empty clears it" eg:"Production website"`
	Revision      int64  `json:"revision" dc:"Independent note revision, unrelated to paid terms or lifecycle" eg:"1"`
}
type CustomerNoteGetReq struct {
	g.Meta        `path:"/account/service-entitlements/{entitlementNo}/customer-note" method:"get" tags:"IDC Product Directories" summary:"Get my service note" dc:"Read the exact current customer's service note; foreign and missing services are indistinguishable."`
	EntitlementNo string `json:"entitlementNo" in:"path" v:"required|max-length:40" dc:"Exact current-customer entitlement reference" eg:"ent_0123456789abcdef0123456789abcdef"`
}
type CustomerNoteGetRes struct {
	SupportAssociationReference string       `json:"supportAssociationReference" dc:"Opaque association for opening a support ticket about this owned service; empty when unavailable" eg:"sar_0123456789abcdef0123456789abcdef"`
	Note                        CustomerNote `json:"note" dc:"Current note and its independent revision" eg:"{}"`
}
type CustomerNoteUpdateReq struct {
	g.Meta           `path:"/account/service-entitlements/{entitlementNo}/customer-note" method:"put" tags:"IDC Product Directories" summary:"Update my service note" dc:"Atomically save a non-sensitive note after current ownership and independent optimistic revision checks without changing paid terms, fulfillment or lifecycle."`
	EntitlementNo    string `json:"entitlementNo" in:"path" v:"required|max-length:40" dc:"Exact current-customer entitlement reference" eg:"ent_0123456789abcdef0123456789abcdef"`
	Remark           string `json:"remark" v:"max-length:1000" dc:"Non-sensitive note, empty clears it; never include passwords or keys" eg:"Production website"`
	ExpectedRevision int64  `json:"expectedRevision" v:"required|min:1" dc:"Independent note revision returned by the current read" eg:"1"`
}
type CustomerNoteUpdateRes struct {
	Note CustomerNote `json:"note" dc:"Saved note and new independent revision" eg:"{}"`
}
