// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCustomerCollaborator is the golang structure for table sales_customer_collaborator.
type SalesCustomerCollaborator struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:""`
	CollaborationNo string     `json:"collaborationNo" orm:"collaboration_no" description:""`
	AssignmentNo    string     `json:"assignmentNo"    orm:"assignment_no"    description:""`
	MembershipNo    string     `json:"membershipNo"    orm:"membership_no"    description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"      description:""`
	EffectiveAt     *time.Time `json:"effectiveAt"     orm:"effective_at"     description:""`
	EndedAt         *time.Time `json:"endedAt"         orm:"ended_at"         description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
}
