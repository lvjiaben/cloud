// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProviderApiLog is the golang structure for table provider_api_log.
type ProviderApiLog struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int64      `json:"tenantId"         orm:"tenant_id"          description:""`
	ChannelNo        string     `json:"channelNo"        orm:"channel_no"         description:""`
	ProviderPluginId string     `json:"providerPluginId" orm:"provider_plugin_id" description:""`
	MethodCode       string     `json:"methodCode"       orm:"method_code"        description:""`
	Succeeded        bool       `json:"succeeded"        orm:"succeeded"          description:""`
	ErrorCode        string     `json:"errorCode"        orm:"error_code"         description:""`
	DurationMs       int64      `json:"durationMs"       orm:"duration_ms"        description:""`
	StartedAt        *time.Time `json:"startedAt"        orm:"started_at"         description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
}
