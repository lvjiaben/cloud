// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BusinessAuditCustomerLink is the golang structure of table plugin_idc_core_business_audit_customer_link for DAO operations like Where/Data.
type BusinessAuditCustomerLink struct {
	g.Meta     `orm:"table:plugin_idc_core_business_audit_customer_link, do:true"`
	Id         any        //
	TenantId   any        //
	AuditNo    any        //
	CustomerNo any        //
	CreatedAt  *time.Time //
}
