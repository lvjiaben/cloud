// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryRenewalWorkItem is the golang structure of table plugin_idc_core_server_accessory_renewal_work_item for DAO operations like Where/Data.
type ServerAccessoryRenewalWorkItem struct {
	g.Meta             `orm:"table:plugin_idc_core_server_accessory_renewal_work_item, do:true"`
	Id                 any        //
	TenantId           any        //
	WorkItemNo         any        //
	EntitlementNo      any        //
	CustomerNo         any        //
	ServiceNo          any        //
	InvoiceNo          any        //
	RequiredPermission any        //
	StatusCode         any        //
	ReasonCode         any        //
	CreatedAt          *time.Time //
	ResolvedAt         *time.Time //
	UpdatedAt          *time.Time //
	SubjectTypeCode    any        //
	SubjectNo          any        //
}
