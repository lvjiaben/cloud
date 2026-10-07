// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// DeliveryChain is the golang structure for table delivery_chain.
type DeliveryChain struct {
	TenantId           int        `json:"tenantId"           orm:"tenant_id"             description:""`
	ChainNo            string     `json:"chainNo"            orm:"chain_no"              description:""`
	RootTaskNo         string     `json:"rootTaskNo"         orm:"root_task_no"          description:""`
	TailTaskNo         string     `json:"tailTaskNo"         orm:"tail_task_no"          description:""`
	ManualResendCount  int        `json:"manualResendCount"  orm:"manual_resend_count"   description:""`
	LastManualResendAt any        `json:"lastManualResendAt" orm:"last_manual_resend_at" description:""`
	Version            int64      `json:"version"            orm:"version"               description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"            description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"            description:""`
}
