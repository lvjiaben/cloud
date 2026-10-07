// Privacy retention list API returns the active policy for each governed data type. Missing policy remains visible by absence and must not be defaulted.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyRetentionListReq struct {
	g.Meta `path:"/admin/compliance/privacy/retention-policies" method:"get" tags:"IDC Compliance Privacy" summary:"List privacy retention policies" dc:"Return the current installation-local policy for each sensitive data type." permission:"idc:admin:compliance:privacy:query"`
}

type PrivacyRetentionListRes struct {
	Items []PrivacyRetentionPolicyItem `json:"items" dc:"Current retention policies" eg:"[]"`
}
