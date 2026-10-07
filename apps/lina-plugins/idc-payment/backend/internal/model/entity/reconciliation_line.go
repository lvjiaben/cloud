// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ReconciliationLine is the golang structure for table reconciliation_line.
type ReconciliationLine struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	LineNo             string     `json:"lineNo"             orm:"line_no"              description:""`
	BatchNo            string     `json:"batchNo"            orm:"batch_no"             description:""`
	AttemptNo          string     `json:"attemptNo"          orm:"attempt_no"           description:""`
	RefundAttemptNo    string     `json:"refundAttemptNo"    orm:"refund_attempt_no"    description:""`
	ExternalReference  string     `json:"externalReference"  orm:"external_reference"   description:""`
	DifferenceCode     string     `json:"differenceCode"     orm:"difference_code"      description:""`
	LocalAmountMinor   int64      `json:"localAmountMinor"   orm:"local_amount_minor"   description:""`
	ChannelAmountMinor int64      `json:"channelAmountMinor" orm:"channel_amount_minor" description:""`
	CurrencyCode       string     `json:"currencyCode"       orm:"currency_code"        description:""`
	ResolutionCode     string     `json:"resolutionCode"     orm:"resolution_code"      description:""`
	EvidenceDigest     string     `json:"evidenceDigest"     orm:"evidence_digest"      description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	ResolvedAt         *time.Time `json:"resolvedAt"         orm:"resolved_at"          description:""`
	RecordTypeCode     string     `json:"recordTypeCode"     orm:"record_type_code"     description:""`
	CustomerRefDigest  string     `json:"customerRefDigest"  orm:"customer_ref_digest"  description:""`
	LocalStatusCode    string     `json:"localStatusCode"    orm:"local_status_code"    description:""`
	ChannelStatusCode  string     `json:"channelStatusCode"  orm:"channel_status_code"  description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
}
