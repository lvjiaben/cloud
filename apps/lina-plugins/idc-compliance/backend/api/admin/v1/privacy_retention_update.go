// Privacy retention update API replaces one data-type policy using a replay-safe request. It requires a positive merchant-selected duration.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyRetentionUpdateReq struct {
	g.Meta         `path:"/admin/compliance/privacy/retention-policies" method:"put" tags:"IDC Compliance Privacy" summary:"Set privacy retention policy" dc:"Replace one data-type policy with a positive merchant-configured number of days. No duration is hardcoded." permission:"idc:admin:compliance:privacy:configure"`
	DataTypeCode   string `json:"dataTypeCode" v:"required|max-length:40" dc:"Governed data type" eg:"personal_verification_material"`
	RetentionDays  int    `json:"retentionDays" v:"required|min:1" dc:"Positive number of days" eg:"365"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:128" dc:"Replay-safe browser request key" eg:"privacy-retention-0123456789abcdef"`
}

type PrivacyRetentionUpdateRes struct {
	Item PrivacyRetentionPolicyItem `json:"item" dc:"Current retention policy" eg:"{}"`
}
