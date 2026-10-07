// This file defines the category-only enterprise legal-change entry.
package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseLegalChangeCreateReq opens a re-verification workflow without legal values.
type EnterpriseLegalChangeCreateReq struct {
	g.Meta                 `path:"/account/compliance/enterprise-legal-change-requests" method:"post" tags:"IDC Compliance Account" summary:"Request enterprise legal-profile change" dc:"Open one core-owned category-only legal-change request for the authenticated verified enterprise. Replacement legal values and documents are submitted only to a later compliance case."`
	FieldCodes             []string `json:"fieldCodes" v:"required" dc:"One or two changed categories: legal_name and/or legal_identifier; collection bounds are enforced by the legal-change service" eg:"[\"legal_name\",\"legal_identifier\"]"`
	ReasonCode             string   `json:"reasonCode" v:"required|in:legal_name_change,identifier_correction,enterprise_registration_change,other" dc:"Closed legal-change reason code" eg:"enterprise_registration_change"`
	ExpectedProfileVersion int64    `json:"expectedProfileVersion" v:"required|min:1" dc:"Current enterprise legal-profile version shown to the customer" eg:"7"`
	IdempotencyKey         string   `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Random retry key; the raw value is never persisted" eg:"enterprise-legal-change-01J5M6S2R9Z8"`
}

// EnterpriseLegalChangeCreateRes wraps the new or replayed change request.
type EnterpriseLegalChangeCreateRes struct {
	Item *EnterpriseLegalChangeItem `json:"item" dc:"Category-only enterprise legal-change request" eg:"{}"`
}
