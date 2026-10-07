// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingPointsLot is the golang structure for table marketing_points_lot.
type MarketingPointsLot struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	LotNo                   string     `json:"lotNo"                   orm:"lot_no"                    description:""`
	CustomerNo              string     `json:"customerNo"              orm:"customer_no"               description:""`
	CampaignNo              *string    `json:"campaignNo"              orm:"campaign_no"               description:""`
	CampaignTypeCode        *string    `json:"campaignTypeCode"        orm:"campaign_type_code"        description:""`
	SourceTypeCode          string     `json:"sourceTypeCode"          orm:"source_type_code"          description:""`
	SourceReference         string     `json:"sourceReference"         orm:"source_reference"          description:""`
	OriginalPoints          int64      `json:"originalPoints"          orm:"original_points"           description:""`
	AvailablePoints         int64      `json:"availablePoints"         orm:"available_points"          description:""`
	FrozenPoints            int64      `json:"frozenPoints"            orm:"frozen_points"             description:""`
	ConsumedPoints          int64      `json:"consumedPoints"          orm:"consumed_points"           description:""`
	ExpiredPoints           int64      `json:"expiredPoints"           orm:"expired_points"            description:""`
	ReversedPoints          int64      `json:"reversedPoints"          orm:"reversed_points"           description:""`
	ReversalShortfallPoints int64      `json:"reversalShortfallPoints" orm:"reversal_shortfall_points" description:"Cumulative refunded award points already consumed and therefore represented as account debt"`
	ExpiresAt               *time.Time `json:"expiresAt"               orm:"expires_at"                description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
