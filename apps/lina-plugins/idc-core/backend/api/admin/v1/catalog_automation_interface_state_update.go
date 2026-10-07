// This file defines optimistic stopping and resuming of new business for one automation interface.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceStateUpdateReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/new-business-state" method:"put" tags:"IDC Catalog Management" summary:"Change interface new-business state" dc:"Stop new sales or resume only after current frozen provider evidence is revalidated." permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	NewBusinessState    string `json:"newBusinessState" v:"required|in:accepting,stopped" dc:"Target state" eg:"stopped"`
	ExpectedVersion     int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic concurrency version" eg:"2"`
}
type CatalogAutomationInterfaceStateUpdateRes struct {
	Interface *CatalogAutomationInterfaceItem `json:"interface" dc:"Updated interface" eg:"{}"`
}
