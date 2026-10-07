// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditApplicationReview is the golang structure for table credit_application_review.
type CreditApplicationReview struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	ReviewNo       string     `json:"reviewNo"       orm:"review_no"       description:""`
	ApplicationNo  string     `json:"applicationNo"  orm:"application_no"  description:""`
	CustomerNo     string     `json:"customerNo"     orm:"customer_no"     description:""`
	CurrencyCode   string     `json:"currencyCode"   orm:"currency_code"   description:""`
	ReviewSequence int        `json:"reviewSequence" orm:"review_sequence" description:""`
	DecisionCode   string     `json:"decisionCode"   orm:"decision_code"   description:""`
	ReviewerId     int        `json:"reviewerId"     orm:"reviewer_id"     description:""`
	CommentDigest  string     `json:"commentDigest"  orm:"comment_digest"  description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest" description:""`
	ContentDigest  string     `json:"contentDigest"  orm:"content_digest"  description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
