package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerRenewalConfirmationCreateReq struct {
	EntitlementNo      string `json:"entitlementNo" v:"max-length:40" dc:"Optional exact product context; every selected server must belong to this entitlement" eg:"ENT_0123456789abcdef"`
	g.Meta             `path:"/admin/resource/server-renewals/confirmations" method:"post" tags:"IDC Server Renewals" summary:"Confirm selected server renewals" dc:"Revalidate the exact preview digest, then confirm every eligible renewal independently. One item failure never rolls back or blocks the remaining items." permission:"idc:admin:resource:server-batch:safe"`
	RequestKey         string   `json:"requestKey" v:"required|length:8,128"`
	RenewalCycles      int      `json:"renewalCycles" v:"required|between:1,120"`
	ServerNos          []string `json:"serverNos" v:"required" dc:"The exact ordered server selection used by the preview"`
	ConfirmationDigest string   `json:"confirmationDigest" v:"required|length:64" dc:"Digest returned by the immediately preceding preview"`
}

type ServerRenewalConfirmationCreateRes struct {
	RequestKey    string                     `json:"requestKey"`
	RenewalCycles int                        `json:"renewalCycles"`
	Items         []*ServerRenewalResultItem `json:"items"`
}

type ServerRenewalResultItem struct {
	ServerNo         string `json:"serverNo"`
	ProductNameZhCN  string `json:"productNameZhCN"`
	ProductNameEnUS  string `json:"productNameEnUS"`
	Succeeded        bool   `json:"succeeded"`
	FailureCode      string `json:"failureCode" dc:"Stable safe failure code; empty after success"`
	Status           string `json:"status" dc:"Renewal operation status after success" eg:"pending_payment"`
	CurrencyCode     string `json:"currencyCode" eg:"CNY"`
	TotalAmountMinor string `json:"totalAmountMinor" dc:"Confirmed total in currency minor units as a decimal integer string" eg:"129900"`
}
