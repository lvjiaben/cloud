// This file defines the current customer's local personal-verification state query.

package v1

import "github.com/gogf/gf/v2/frame/g"

// PersonalVerificationGetReq queries local state without provider I/O.
type PersonalVerificationGetReq struct {
	g.Meta `path:"/account/compliance/personal-verification" method:"get" tags:"IDC Compliance Account" summary:"Get personal verification state" dc:"Return only the authenticated personal customer's locally persisted state. This read never calls Aliyun and never accepts a customer or tenant identifier."`
}

// PersonalVerificationGetRes returns the current local projection and consent policy version.
type PersonalVerificationGetRes struct {
	Item          *PersonalVerificationItem `json:"item" dc:"Current customer-safe personal verification projection" eg:"{}"`
	PolicyVersion string                    `json:"policyVersion" dc:"Sensitive-personal-information policy version required for a new consent" eg:"1"`
}
