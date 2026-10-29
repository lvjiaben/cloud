// Customer consent overview returns only current decisions owned by the authenticated session.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerPrivacyGetReq loads only the authenticated customer's privacy center.
type CustomerPrivacyGetReq struct {
	g.Meta `path:"/account/compliance/privacy" method:"get" tags:"IDC Compliance Account" summary:"Get my privacy and authorization overview" dc:"Return only the authenticated customer's latest consent decisions."`
}

// CustomerPrivacyGetRes is the bounded ownership-only privacy projection.
type CustomerPrivacyGetRes struct {
	Consents []CustomerPrivacyConsent `json:"consents" dc:"Latest current-customer consent decisions" eg:"[]"`
}
