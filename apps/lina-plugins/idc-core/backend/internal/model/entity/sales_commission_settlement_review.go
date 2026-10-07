// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionSettlementReview is the golang structure for table sales_commission_settlement_review.
type SalesCommissionSettlementReview struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	ReviewNo       string     `json:"reviewNo"       orm:"review_no"        description:""`
	SettlementNo   string     `json:"settlementNo"   orm:"settlement_no"    description:""`
	DecisionCode   string     `json:"decisionCode"   orm:"decision_code"    description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"      description:""`
	ReviewerUserId int        `json:"reviewerUserId" orm:"reviewer_user_id" description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest"  description:""`
	ReviewedAt     *time.Time `json:"reviewedAt"     orm:"reviewed_at"      description:""`
}
