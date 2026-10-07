// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ReconciliationSupplement is the golang structure for table reconciliation_supplement.
type ReconciliationSupplement struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	RequestNo           string     `json:"requestNo"           orm:"request_no"            description:""`
	LineNo              string     `json:"lineNo"              orm:"line_no"               description:""`
	AttemptNo           string     `json:"attemptNo"           orm:"attempt_no"            description:""`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"            description:""`
	CustomerRefDigest   string     `json:"customerRefDigest"   orm:"customer_ref_digest"   description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	AmountMinor         int64      `json:"amountMinor"         orm:"amount_minor"          description:""`
	ExternalReference   string     `json:"externalReference"   orm:"external_reference"    description:""`
	EvidenceDigest      string     `json:"evidenceDigest"      orm:"evidence_digest"       description:""`
	ContentDigest       string     `json:"contentDigest"       orm:"content_digest"        description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	SubmitterUserId     int        `json:"submitterUserId"     orm:"submitter_user_id"     description:""`
	ReviewerUserId      int        `json:"reviewerUserId"      orm:"reviewer_user_id"      description:""`
	RejectionReasonCode string     `json:"rejectionReasonCode" orm:"rejection_reason_code" description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	SubmittedAt         *time.Time `json:"submittedAt"         orm:"submitted_at"          description:""`
	ReviewedAt          *time.Time `json:"reviewedAt"          orm:"reviewed_at"           description:""`
	AppliedAt           *time.Time `json:"appliedAt"           orm:"applied_at"            description:""`
}
