// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingChannelConsent is the golang structure for table marketing_channel_consent.
type MarketingChannelConsent struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"         description:""`
	ContactNo         string     `json:"contactNo"         orm:"contact_no"          description:""`
	ChannelCode       string     `json:"channelCode"       orm:"channel_code"        description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"         description:""`
	CurrentEventNo    string     `json:"currentEventNo"    orm:"current_event_no"    description:""`
	ConsentOccurredAt *time.Time `json:"consentOccurredAt" orm:"consent_occurred_at" description:""`
	Version           int64      `json:"version"           orm:"version"             description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"          description:""`
}
