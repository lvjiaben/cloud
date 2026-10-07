// This file defines explicit provider-authoritative rechecking after browser return.

package v1

import "github.com/gogf/gf/v2/frame/g"

// PersonalVerificationAuthoritativeStateUpdateReq rechecks one owned attempt.
type PersonalVerificationAuthoritativeStateUpdateReq struct {
	g.Meta    `path:"/account/compliance/personal-verifications/{attemptNo}/authoritative-state" method:"put" tags:"IDC Compliance Account" summary:"Refresh authoritative personal verification state" dc:"Actively query Aliyun for an attempt owned by the authenticated personal customer and commit only a verified provider result. Browser return parameters never determine success."`
	AttemptNo string `json:"attemptNo" v:"required|max-length:64" dc:"Owned opaque personal verification attempt number" eg:"pva_0123456789abcdef0123456789abcdef"`
}

// PersonalVerificationAuthoritativeStateUpdateRes returns the refreshed safe projection.
type PersonalVerificationAuthoritativeStateUpdateRes struct {
	Item *PersonalVerificationItem `json:"item" dc:"Authoritatively refreshed customer-safe verification projection" eg:"{}"`
}
