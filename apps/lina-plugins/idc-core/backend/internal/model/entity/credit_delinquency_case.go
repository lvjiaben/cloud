// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditDelinquencyCase is the golang structure for table credit_delinquency_case.
type CreditDelinquencyCase struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	CaseNo                   string     `json:"caseNo"                   orm:"case_no"                     description:""`
	StatementNo              string     `json:"statementNo"              orm:"statement_no"                description:""`
	CreditAccountNo          string     `json:"creditAccountNo"          orm:"credit_account_no"           description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                 description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"               description:""`
	TermsNo                  string     `json:"termsNo"                  orm:"terms_no"                    description:""`
	TermsVersion             int64      `json:"termsVersion"             orm:"terms_version"               description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                 description:""`
	PrincipalMinor           int64      `json:"principalMinor"           orm:"principal_minor"             description:""`
	LateFeeMinor             int64      `json:"lateFeeMinor"             orm:"late_fee_minor"              description:""`
	OutstandingAmountMinor   int64      `json:"outstandingAmountMinor"   orm:"outstanding_amount_minor"    description:""`
	OpenedAt                 *time.Time `json:"openedAt"                 orm:"opened_at"                   description:""`
	GraceUntil               *time.Time `json:"graceUntil"               orm:"grace_until"                 description:""`
	FrozenAt                 *time.Time `json:"frozenAt"                 orm:"frozen_at"                   description:""`
	ServiceActionRequestedAt *time.Time `json:"serviceActionRequestedAt" orm:"service_action_requested_at" description:""`
	WrittenOffAt             *time.Time `json:"writtenOffAt"             orm:"written_off_at"              description:""`
	ResolvedAt               *time.Time `json:"resolvedAt"               orm:"resolved_at"                 description:""`
	ReasonCode               string     `json:"reasonCode"               orm:"reason_code"                 description:""`
	SourceSnapshotDigest     string     `json:"sourceSnapshotDigest"     orm:"source_snapshot_digest"      description:""`
	Version                  int64      `json:"version"                  orm:"version"                     description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                  description:""`
}
