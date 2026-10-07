// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingFundHold is the golang structure for table billing_fund_hold.
type BillingFundHold struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	HoldNo              string     `json:"holdNo"              orm:"hold_no"               description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	AccountNo           string     `json:"accountNo"           orm:"account_no"            description:""`
	AccountTypeCode     string     `json:"accountTypeCode"     orm:"account_type_code"     description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	SourceTypeCode      string     `json:"sourceTypeCode"      orm:"source_type_code"      description:""`
	SourceReference     string     `json:"sourceReference"     orm:"source_reference"      description:""`
	AmountMinor         int64      `json:"amountMinor"         orm:"amount_minor"          description:""`
	ConsumedAmountMinor int64      `json:"consumedAmountMinor" orm:"consumed_amount_minor" description:""`
	ReleasedAmountMinor int64      `json:"releasedAmountMinor" orm:"released_amount_minor" description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	ContentDigest       string     `json:"contentDigest"       orm:"content_digest"        description:""`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"            description:""`
	ConsumedAt          *time.Time `json:"consumedAt"          orm:"consumed_at"           description:""`
	ReleasedAt          *time.Time `json:"releasedAt"          orm:"released_at"           description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
