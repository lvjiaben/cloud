// Privacy incident list API returns a bounded local incident directory and timeline. It excludes raw leaked content and access credentials.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyIncidentListReq struct {
	g.Meta `path:"/admin/compliance/privacy/incidents" method:"get" tags:"IDC Compliance Privacy" summary:"List privacy incidents" dc:"Return a bounded installation-local incident directory and safe timeline." permission:"idc:admin:compliance:privacy:query"`
	Limit  int `json:"limit" v:"min:1|max:100" d:"50" dc:"Maximum incidents returned from 1 to 100" eg:"50"`
}

type PrivacyIncidentListRes struct {
	Items []PrivacyIncidentItem `json:"items" dc:"Bounded installation-local incident directory" eg:"[]"`
}
