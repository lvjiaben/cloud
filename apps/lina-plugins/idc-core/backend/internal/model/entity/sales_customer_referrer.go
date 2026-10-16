// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCustomerReferrer is the golang structure for table sales_customer_referrer.
type SalesCustomerReferrer struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	CustomerNo         string     `json:"customerNo"         orm:"customer_no"          description:""`
	ReferrerCustomerNo string     `json:"referrerCustomerNo" orm:"referrer_customer_no" description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
	EffectiveAt        *time.Time `json:"effectiveAt"        orm:"effective_at"         description:""`
	CreatedBy          int        `json:"createdBy"          orm:"created_by"           description:""`
}
