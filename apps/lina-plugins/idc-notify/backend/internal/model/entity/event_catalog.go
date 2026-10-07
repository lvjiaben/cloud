// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EventCatalog is the golang structure for table event_catalog.
type EventCatalog struct {
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	EventCode         string     `json:"eventCode"         orm:"event_code"         description:""`
	TopicCode         string     `json:"topicCode"         orm:"topic_code"         description:""`
	ObjectType        string     `json:"objectType"        orm:"object_type"        description:""`
	RequiredVariables string     `json:"requiredVariables" orm:"required_variables" description:""`
	OptionalVariables string     `json:"optionalVariables" orm:"optional_variables" description:""`
	SecurityRequired  bool       `json:"securityRequired"  orm:"security_required"  description:""`
	Enabled           bool       `json:"enabled"           orm:"enabled"            description:""`
	Version           int64      `json:"version"           orm:"version"            description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
