// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ExceptionFund is the golang structure for table exception_fund.
type ExceptionFund struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	ExceptionNo    string     `json:"exceptionNo"    orm:"exception_no"    description:""`
	AttemptNo      string     `json:"attemptNo"      orm:"attempt_no"      description:""`
	ChannelNo      string     `json:"channelNo"      orm:"channel_no"      description:""`
	CurrencyCode   string     `json:"currencyCode"   orm:"currency_code"   description:""`
	AmountMinor    int64      `json:"amountMinor"    orm:"amount_minor"    description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"     description:""`
	StatusCode     string     `json:"statusCode"     orm:"status_code"     description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest" description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
	ResolvedAt     *time.Time `json:"resolvedAt"     orm:"resolved_at"     description:""`
}
