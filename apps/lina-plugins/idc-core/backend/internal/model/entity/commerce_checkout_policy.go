// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceCheckoutPolicy is the golang structure for table commerce_checkout_policy.
type CommerceCheckoutPolicy struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	PolicyNo               string     `json:"policyNo"               orm:"policy_no"                description:""`
	PaymentValiditySeconds int        `json:"paymentValiditySeconds" orm:"payment_validity_seconds" description:""`
	Version                int64      `json:"version"                orm:"version"                  description:""`
	UpdatedBy              int        `json:"updatedBy"              orm:"updated_by"               description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
}
