// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BusinessAuditCustomerLink is the golang structure for table business_audit_customer_link.
type BusinessAuditCustomerLink struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	AuditNo    string     `json:"auditNo"    orm:"audit_no"    description:""`
	CustomerNo string     `json:"customerNo" orm:"customer_no" description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
}
