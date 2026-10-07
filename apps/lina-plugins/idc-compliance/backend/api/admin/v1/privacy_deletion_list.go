// Privacy deletion list API returns a bounded task directory. It exposes safe identifiers and states rather than deleted private content.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyDeletionListReq struct {
	g.Meta `path:"/admin/compliance/privacy/deletion-tasks" method:"get" tags:"IDC Compliance Privacy" summary:"List privacy deletion tasks" permission:"idc:admin:compliance:privacy:query"`
	Limit  int `json:"limit" v:"min:1|max:200" d:"100" dc:"Maximum tasks returned from 1 to 200" eg:"100"`
}
type PrivacyDeletionListRes struct {
	Items []PrivacyDeletionItem `json:"items" dc:"Bounded privacy deletion task directory" eg:"[]"`
}
