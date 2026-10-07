// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EventPolicy is the golang structure for table event_policy.
type EventPolicy struct {
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	EventCode     string     `json:"eventCode"     orm:"event_code"     description:""`
	RecipientKind string     `json:"recipientKind" orm:"recipient_kind" description:""`
	ChannelCode   string     `json:"channelCode"   orm:"channel_code"   description:""`
	Enabled       bool       `json:"enabled"       orm:"enabled"        description:""`
	Version       int64      `json:"version"       orm:"version"        description:""`
	UpdatedBy     int64      `json:"updatedBy"     orm:"updated_by"     description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
	UpdatedAt     *time.Time `json:"updatedAt"     orm:"updated_at"     description:""`
}
