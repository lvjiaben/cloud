// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SecurityCodePurpose is the golang structure for table security_code_purpose.
type SecurityCodePurpose struct {
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	CallerPluginId string     `json:"callerPluginId" orm:"caller_plugin_id" description:""`
	PurposeCode    string     `json:"purposeCode"    orm:"purpose_code"     description:""`
	EventCode      string     `json:"eventCode"      orm:"event_code"       description:""`
	Enabled        bool       `json:"enabled"        orm:"enabled"          description:""`
	Version        int64      `json:"version"        orm:"version"          description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"       description:""`
}
