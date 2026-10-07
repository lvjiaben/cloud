// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Attempt is the golang structure for table attempt.
type Attempt struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	AttemptNo              string     `json:"attemptNo"              orm:"attempt_no"               description:""`
	ChannelNo              string     `json:"channelNo"              orm:"channel_no"               description:""`
	InvoiceNo              string     `json:"invoiceNo"              orm:"invoice_no"               description:""`
	OrderNo                string     `json:"orderNo"                orm:"order_no"                 description:""`
	PurposeCode            string     `json:"purposeCode"            orm:"purpose_code"             description:""`
	PurposeReference       string     `json:"purposeReference"       orm:"purpose_reference"        description:""`
	CustomerRefDigest      string     `json:"customerRefDigest"      orm:"customer_ref_digest"      description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	AmountMinor            int64      `json:"amountMinor"            orm:"amount_minor"             description:""`
	BalanceLockReference   string     `json:"balanceLockReference"   orm:"balance_lock_reference"   description:""`
	BalanceLockedMinor     int64      `json:"balanceLockedMinor"     orm:"balance_locked_minor"     description:""`
	ModeCode               string     `json:"modeCode"               orm:"mode_code"                description:""`
	RequestDigest          string     `json:"requestDigest"          orm:"request_digest"           description:""`
	ExternalTradeReference string     `json:"externalTradeReference" orm:"external_trade_reference" description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"              description:""`
	ExpiresAt              *time.Time `json:"expiresAt"              orm:"expires_at"               description:""`
	SubmittedAt            *time.Time `json:"submittedAt"            orm:"submitted_at"             description:""`
	ConfirmedAt            *time.Time `json:"confirmedAt"            orm:"confirmed_at"             description:""`
	ClosedAt               *time.Time `json:"closedAt"               orm:"closed_at"                description:""`
	ResultUnknown          bool       `json:"resultUnknown"          orm:"result_unknown"           description:""`
	FundingResultDigest    string     `json:"fundingResultDigest"    orm:"funding_result_digest"    description:""`
	Version                int64      `json:"version"                orm:"version"                  description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
}
