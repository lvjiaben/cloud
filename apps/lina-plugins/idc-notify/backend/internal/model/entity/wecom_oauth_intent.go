// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// WecomOauthIntent is the golang structure for table wecom_oauth_intent.
type WecomOauthIntent struct {
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	IntentNo          string     `json:"intentNo"          orm:"intent_no"          description:""`
	StateDigest       string     `json:"stateDigest"       orm:"state_digest"       description:""`
	HostUserId        int64      `json:"hostUserId"        orm:"host_user_id"       description:""`
	StaffVersion      int64      `json:"staffVersion"      orm:"staff_version"      description:""`
	SessionDigest     string     `json:"sessionDigest"     orm:"session_digest"     description:""`
	ChannelGeneration int64      `json:"channelGeneration" orm:"channel_generation" description:""`
	ChannelVersion    int64      `json:"channelVersion"    orm:"channel_version"    description:""`
	CorpId            string     `json:"corpId"            orm:"corp_id"            description:""`
	AgentId           int64      `json:"agentId"           orm:"agent_id"           description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"        description:""`
	FailureCode       string     `json:"failureCode"       orm:"failure_code"       description:""`
	LeaseToken        any        `json:"leaseToken"        orm:"lease_token"        description:""`
	LeaseExpiresAt    any        `json:"leaseExpiresAt"    orm:"lease_expires_at"   description:""`
	ExpiresAt         any        `json:"expiresAt"         orm:"expires_at"         description:""`
	CompletedAt       any        `json:"completedAt"       orm:"completed_at"       description:""`
	Version           int64      `json:"version"           orm:"version"            description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
