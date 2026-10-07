// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingPublicExposureState is the golang structure for table marketing_public_exposure_state.
type MarketingPublicExposureState struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	DeviceDigest    string     `json:"deviceDigest"    orm:"device_digest"     description:""`
	CampaignNo      string     `json:"campaignNo"      orm:"campaign_no"       description:""`
	CurrencyCode    string     `json:"currencyCode"    orm:"currency_code"     description:""`
	WindowStartedAt *time.Time `json:"windowStartedAt" orm:"window_started_at" description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
