// This file defines the current enterprise verification account projection.
package v1

import "github.com/gogf/gf/v2/frame/g"

// EnterpriseVerificationGetReq reads the current authenticated enterprise state.
type EnterpriseVerificationGetReq struct {
	g.Meta `path:"/account/compliance/enterprise-verification" method:"get" tags:"IDC Compliance Account" summary:"Get my enterprise verification" dc:"Return the current enterprise customer's core legal projection, latest compliance case, active category-only legal-change request, and notice version without accepting a customer number or exposing private storage metadata."`
}

// EnterpriseVerificationGetRes wraps the current enterprise verification state.
type EnterpriseVerificationGetRes struct {
	Item *EnterpriseVerificationAccountStateItem `json:"item" dc:"Current customer-safe enterprise verification state" eg:"{}"`
}
