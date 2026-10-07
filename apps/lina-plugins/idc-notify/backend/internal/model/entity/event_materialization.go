// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EventMaterialization is the golang structure for table event_materialization.
type EventMaterialization struct {
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	MaterializationNo string     `json:"materializationNo" orm:"materialization_no" description:""`
	SourcePlugin      string     `json:"sourcePlugin"      orm:"source_plugin"      description:""`
	SourceEventNo     string     `json:"sourceEventNo"     orm:"source_event_no"    description:""`
	EventCode         string     `json:"eventCode"         orm:"event_code"         description:""`
	RecipientKind     string     `json:"recipientKind"     orm:"recipient_kind"     description:""`
	RecipientDigest   string     `json:"recipientDigest"   orm:"recipient_digest"   description:""`
	ChannelCode       string     `json:"channelCode"       orm:"channel_code"       description:""`
	PolicyVersion     int64      `json:"policyVersion"     orm:"policy_version"     description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"        description:""`
	ReasonCode        string     `json:"reasonCode"        orm:"reason_code"        description:""`
	TaskNo            string     `json:"taskNo"            orm:"task_no"            description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
