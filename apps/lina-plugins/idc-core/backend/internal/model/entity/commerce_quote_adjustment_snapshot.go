// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceQuoteAdjustmentSnapshot is the golang structure for table commerce_quote_adjustment_snapshot.
type CommerceQuoteAdjustmentSnapshot struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	QuoteAdjustmentNo         string     `json:"quoteAdjustmentNo"         orm:"quote_adjustment_no"          description:""`
	QuoteItemNo               string     `json:"quoteItemNo"               orm:"quote_item_no"                description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"                description:""`
	ApplicationOrder          int        `json:"applicationOrder"          orm:"application_order"            description:""`
	AdjustmentTypeCode        string     `json:"adjustmentTypeCode"        orm:"adjustment_type_code"         description:""`
	SourceReference           string     `json:"sourceReference"           orm:"source_reference"             description:""`
	SourceVersionDigest       string     `json:"sourceVersionDigest"       orm:"source_version_digest"        description:""`
	ReservationReference      *string    `json:"reservationReference"      orm:"reservation_reference"        description:""`
	AmountMinor               int64      `json:"amountMinor"               orm:"amount_minor"                 description:""`
	ContentDigest             string     `json:"contentDigest"             orm:"content_digest"               description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
	OriginalPeriodAmountMinor int64      `json:"originalPeriodAmountMinor" orm:"original_period_amount_minor" description:""`
	TargetPeriodAmountMinor   int64      `json:"targetPeriodAmountMinor"   orm:"target_period_amount_minor"   description:""`
}
