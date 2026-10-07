// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerContactPreference is the golang structure for table customer_contact_preference.
type CustomerContactPreference struct {
	Id          int64      `json:"id"          orm:"id"           description:""`
	TenantId    int        `json:"tenantId"    orm:"tenant_id"    description:""`
	CustomerNo  string     `json:"customerNo"  orm:"customer_no"  description:""`
	ContactNo   string     `json:"contactNo"   orm:"contact_no"   description:""`
	TopicCode   string     `json:"topicCode"   orm:"topic_code"   description:""`
	ChannelCode string     `json:"channelCode" orm:"channel_code" description:""`
	Enabled     bool       `json:"enabled"     orm:"enabled"      description:""`
	Version     int64      `json:"version"     orm:"version"      description:""`
	CreatedAt   *time.Time `json:"createdAt"   orm:"created_at"   description:""`
	UpdatedAt   *time.Time `json:"updatedAt"   orm:"updated_at"   description:""`
}
