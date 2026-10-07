// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingAdjustmentApproval is the golang structure for table billing_adjustment_approval.
type BillingAdjustmentApproval struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	ApprovalNo          string     `json:"approvalNo"          orm:"approval_no"           description:""`
	AdjustmentNo        string     `json:"adjustmentNo"        orm:"adjustment_no"         description:""`
	RequesterIdSnapshot int        `json:"requesterIdSnapshot" orm:"requester_id_snapshot" description:""`
	ReviewerId          int        `json:"reviewerId"          orm:"reviewer_id"           description:""`
	DecisionCode        string     `json:"decisionCode"        orm:"decision_code"         description:""`
	CommentDigest       string     `json:"commentDigest"       orm:"comment_digest"        description:""`
	EvidenceDigest      string     `json:"evidenceDigest"      orm:"evidence_digest"       description:""`
	DecidedAt           *time.Time `json:"decidedAt"           orm:"decided_at"            description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
