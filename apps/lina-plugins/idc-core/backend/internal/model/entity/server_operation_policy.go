// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerOperationPolicy is the golang structure for table server_operation_policy.
type ServerOperationPolicy struct {
	TenantId              int       `json:"tenantId"              orm:"tenant_id"                description:""`
	RequireHighRiskReauth bool      `json:"requireHighRiskReauth" orm:"require_high_risk_reauth" description:""`
	Version               int64     `json:"version"               orm:"version"                  description:""`
	CreatedAt             time.Time `json:"createdAt"             orm:"created_at"               description:""`
	UpdatedAt             time.Time `json:"updatedAt"             orm:"updated_at"               description:""`
	AllowCustomerTransfer bool      `json:"allowCustomerTransfer" orm:"allow_customer_transfer"  description:""`
}
