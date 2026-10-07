// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCustomerAssignment is the golang structure for table sales_customer_assignment.
type SalesCustomerAssignment struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	AssignmentNo        string     `json:"assignmentNo"        orm:"assignment_no"         description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	MembershipNo        string     `json:"membershipNo"        orm:"membership_no"         description:""`
	ProtectionExpiresAt *time.Time `json:"protectionExpiresAt" orm:"protection_expires_at" description:""`
	EffectiveAt         *time.Time `json:"effectiveAt"         orm:"effective_at"          description:""`
	EndedAt             *time.Time `json:"endedAt"             orm:"ended_at"              description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	SourceCode          string     `json:"sourceCode"          orm:"source_code"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
