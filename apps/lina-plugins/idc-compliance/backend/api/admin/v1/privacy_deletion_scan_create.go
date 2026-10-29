// Privacy deletion scan API creates tasks for expired materials without deleting them. Actual removal remains a separate confirmed operation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PrivacyDeletionScanCreateReq struct {
	g.Meta `path:"/admin/compliance/privacy/deletion-tasks/scans" method:"post" tags:"IDC Compliance Materials" summary:"Scan expired privacy materials" dc:"Create bounded pending tasks for locally expired materials without deleting any original. Confirmation remains a separate operation." permission:"idc:admin:compliance:privacy:configure"`
	Limit  int `json:"limit" v:"min:1|max:500" d:"100" dc:"Maximum expired candidates inspected from 1 to 500" eg:"100"`
}
type PrivacyDeletionScanCreateRes struct {
	Created int `json:"created" dc:"Number of new pending deletion tasks created" eg:"3"`
}
