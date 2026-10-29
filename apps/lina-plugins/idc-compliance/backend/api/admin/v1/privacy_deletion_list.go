// Privacy deletion list API returns a bounded task directory. It exposes safe identifiers and states rather than deleted private content.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyDeletionListReq struct {
	g.Meta   `path:"/admin/compliance/privacy/deletion-tasks" method:"get" tags:"IDC Compliance Materials" summary:"List material deletion tasks" dc:"Return one bounded page and exact total of retained authentication and tax material deletion tasks." permission:"idc:admin:compliance:privacy:query"`
	Page     int `json:"page" v:"min:1|max:100000" d:"1" dc:"One-based deletion task page" eg:"1"`
	PageSize int `json:"pageSize" v:"in:10,20,50,100" d:"20" dc:"Tasks per page: 10, 20, 50, or 100" eg:"20"`
}
type PrivacyDeletionListRes struct {
	Total int                   `json:"total" dc:"Exact total deletion tasks under the same authorization and target scope" eg:"1"`
	Items []PrivacyDeletionItem `json:"items" dc:"Bounded privacy deletion task directory" eg:"[]"`
}
