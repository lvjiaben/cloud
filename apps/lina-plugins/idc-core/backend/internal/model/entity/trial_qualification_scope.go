// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TrialQualificationScope is the golang structure for table trial_qualification_scope.
type TrialQualificationScope struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"         description:""`
	ScopeNo          string     `json:"scopeNo"          orm:"scope_no"          description:""`
	CustomerNo       string     `json:"customerNo"       orm:"customer_no"       description:""`
	ProductNo        string     `json:"productNo"        orm:"product_no"        description:""`
	HeldQuantity     int        `json:"heldQuantity"     orm:"held_quantity"     description:""`
	ConsumedQuantity int        `json:"consumedQuantity" orm:"consumed_quantity" description:""`
	Version          int64      `json:"version"          orm:"version"           description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"        description:""`
}
