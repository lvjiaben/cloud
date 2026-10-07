// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentQualification is the golang structure for table sales_agent_qualification.
type SalesAgentQualification struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	QualificationNo     string     `json:"qualificationNo"     orm:"qualification_no"      description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	LevelNo             string     `json:"levelNo"             orm:"level_no"              description:""`
	SourceApplicationNo *string    `json:"sourceApplicationNo" orm:"source_application_no" description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	EffectiveAt         *time.Time `json:"effectiveAt"         orm:"effective_at"          description:""`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"            description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
