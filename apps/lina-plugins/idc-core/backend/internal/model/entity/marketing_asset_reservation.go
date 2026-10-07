// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingAssetReservation is the golang structure for table marketing_asset_reservation.
type MarketingAssetReservation struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	ReservationNo       string     `json:"reservationNo"       orm:"reservation_no"        description:""`
	AssetNo             string     `json:"assetNo"             orm:"asset_no"              description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	QuoteReference      string     `json:"quoteReference"      orm:"quote_reference"       description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	ReservedUses        int        `json:"reservedUses"        orm:"reserved_uses"         description:""`
	DiscountAmountMinor int64      `json:"discountAmountMinor" orm:"discount_amount_minor" description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"            description:""`
	ConsumedAt          *time.Time `json:"consumedAt"          orm:"consumed_at"           description:""`
	ReleasedAt          *time.Time `json:"releasedAt"          orm:"released_at"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
	ReversedAt          *time.Time `json:"reversedAt"          orm:"reversed_at"           description:""`
}
