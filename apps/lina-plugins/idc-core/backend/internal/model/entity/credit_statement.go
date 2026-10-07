// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditStatement is the golang structure for table credit_statement.
type CreditStatement struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	StatementNo            string     `json:"statementNo"            orm:"statement_no"             description:""`
	CreditAccountNo        string     `json:"creditAccountNo"        orm:"credit_account_no"        description:""`
	CustomerNo             string     `json:"customerNo"             orm:"customer_no"              description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	TermsNo                string     `json:"termsNo"                orm:"terms_no"                 description:""`
	TermsVersion           int64      `json:"termsVersion"           orm:"terms_version"            description:""`
	PeriodStartedAt        *time.Time `json:"periodStartedAt"        orm:"period_started_at"        description:""`
	PeriodEndedAt          *time.Time `json:"periodEndedAt"          orm:"period_ended_at"          description:""`
	LineCount              int        `json:"lineCount"              orm:"line_count"               description:""`
	PrincipalMinor         int64      `json:"principalMinor"         orm:"principal_minor"          description:""`
	FeeMinor               int64      `json:"feeMinor"               orm:"fee_minor"                description:""`
	AdjustmentMinor        int64      `json:"adjustmentMinor"        orm:"adjustment_minor"         description:""`
	TotalAmountMinor       int64      `json:"totalAmountMinor"       orm:"total_amount_minor"       description:""`
	PaidAmountMinor        int64      `json:"paidAmountMinor"        orm:"paid_amount_minor"        description:""`
	WrittenOffAmountMinor  int64      `json:"writtenOffAmountMinor"  orm:"written_off_amount_minor" description:""`
	OutstandingAmountMinor int64      `json:"outstandingAmountMinor" orm:"outstanding_amount_minor" description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"              description:""`
	ContentDigest          string     `json:"contentDigest"          orm:"content_digest"           description:""`
	IssuedAt               *time.Time `json:"issuedAt"               orm:"issued_at"                description:""`
	PaymentDueAt           *time.Time `json:"paymentDueAt"           orm:"payment_due_at"           description:""`
	OverdueAt              *time.Time `json:"overdueAt"              orm:"overdue_at"               description:""`
	SettledAt              *time.Time `json:"settledAt"              orm:"settled_at"               description:""`
	CancelledAt            *time.Time `json:"cancelledAt"            orm:"cancelled_at"             description:""`
	WrittenOffAt           *time.Time `json:"writtenOffAt"           orm:"written_off_at"           description:""`
	Version                int64      `json:"version"                orm:"version"                  description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
}
