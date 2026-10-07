// Customer privacy overview API returns notices, current consent decisions, and bounded request history. Ownership comes only from the authenticated session.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerPrivacyGetReq loads only the authenticated customer's privacy center.
type CustomerPrivacyGetReq struct {
	g.Meta `path:"/account/compliance/privacy" method:"get" tags:"IDC Compliance Account" summary:"Get my privacy and authorization overview" dc:"Return published notices, the authenticated customer's latest consent decisions, and at most one hundred of that customer's privacy requests."`
}

// CustomerPrivacyGetRes is the bounded ownership-only privacy projection.
type CustomerPrivacyGetRes struct {
	Notices  []CustomerPrivacyNotice  `json:"notices" dc:"Published notices relevant to customer processing" eg:"[]"`
	Consents []CustomerPrivacyConsent `json:"consents" dc:"Latest current-customer consent decisions" eg:"[]"`
	Requests []CustomerPrivacyRequest `json:"requests" dc:"Current customer's bounded request history" eg:"[]"`
}
