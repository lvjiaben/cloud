// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingCampaignReservation is the golang structure for table marketing_campaign_reservation.
type MarketingCampaignReservation struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	ReservationNo         string     `json:"reservationNo"         orm:"reservation_no"          description:""`
	CampaignNo            string     `json:"campaignNo"            orm:"campaign_no"             description:""`
	BenefitNo             string     `json:"benefitNo"             orm:"benefit_no"              description:""`
	CustomerNo            string     `json:"customerNo"            orm:"customer_no"             description:""`
	QuoteReference        string     `json:"quoteReference"        orm:"quote_reference"         description:""`
	RequestSnapshotDigest string     `json:"requestSnapshotDigest" orm:"request_snapshot_digest" description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"           description:""`
	ReservedUnits         int        `json:"reservedUnits"         orm:"reserved_units"          description:""`
	QuotedValueMinor      int64      `json:"quotedValueMinor"      orm:"quoted_value_minor"      description:""`
	BenefitSnapshotDigest string     `json:"benefitSnapshotDigest" orm:"benefit_snapshot_digest" description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	ExpiresAt             *time.Time `json:"expiresAt"             orm:"expires_at"              description:""`
	ConsumedAt            *time.Time `json:"consumedAt"            orm:"consumed_at"             description:""`
	ReleasedAt            *time.Time `json:"releasedAt"            orm:"released_at"             description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"              description:""`
	ReversedAt            *time.Time `json:"reversedAt"            orm:"reversed_at"             description:""`
}
