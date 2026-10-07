// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// InboxMessage is the golang structure for table inbox_message.
type InboxMessage struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int64      `json:"tenantId"        orm:"tenant_id"         description:""`
	MessageNo       string     `json:"messageNo"       orm:"message_no"        description:""`
	RecipientUserId int64      `json:"recipientUserId" orm:"recipient_user_id" description:""`
	CallerPluginId  string     `json:"callerPluginId"  orm:"caller_plugin_id"  description:""`
	SourceType      string     `json:"sourceType"      orm:"source_type"       description:""`
	SourceNo        string     `json:"sourceNo"        orm:"source_no"         description:""`
	IdempotencyKey  string     `json:"idempotencyKey"  orm:"idempotency_key"   description:""`
	ContentDigest   string     `json:"contentDigest"   orm:"content_digest"    description:""`
	Title           string     `json:"title"           orm:"title"             description:""`
	Body            string     `json:"body"            orm:"body"              description:""`
	Category        string     `json:"category"        orm:"category"          description:""`
	ActionUrl       any        `json:"actionUrl"       orm:"action_url"        description:""`
	Status          string     `json:"status"          orm:"status"            description:""`
	ReadAt          any        `json:"readAt"          orm:"read_at"           description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
