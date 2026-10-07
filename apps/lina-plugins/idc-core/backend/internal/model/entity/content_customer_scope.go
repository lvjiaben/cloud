// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentCustomerScope is the golang structure for table content_customer_scope.
type ContentCustomerScope struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int64      `json:"tenantId"   orm:"tenant_id"   description:""`
	ContentNo  string     `json:"contentNo"  orm:"content_no"  description:""`
	CustomerNo string     `json:"customerNo" orm:"customer_no" description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
}
