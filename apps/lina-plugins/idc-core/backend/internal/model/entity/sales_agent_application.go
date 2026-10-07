// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentApplication is the golang structure for table sales_agent_application.
type SalesAgentApplication struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	ApplicationNo     string     `json:"applicationNo"     orm:"application_no"     description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"        description:""`
	RequestedLevelNo  string     `json:"requestedLevelNo"  orm:"requested_level_no" description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"        description:""`
	ApplicationDigest string     `json:"applicationDigest" orm:"application_digest" description:""`
	SubmittedAt       *time.Time `json:"submittedAt"       orm:"submitted_at"       description:""`
	ReviewedBy        *int       `json:"reviewedBy"        orm:"reviewed_by"        description:""`
	ReviewReasonCode  *string    `json:"reviewReasonCode"  orm:"review_reason_code" description:""`
	ReviewedAt        *time.Time `json:"reviewedAt"        orm:"reviewed_at"        description:""`
	Version           int64      `json:"version"           orm:"version"            description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
