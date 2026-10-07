// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AutoSignAuthTemplate is the golang structure for table auto_sign_auth_template.
type AutoSignAuthTemplate struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:""`
	AuthorizationNo string     `json:"authorizationNo" orm:"authorization_no" description:""`
	TemplateNo      string     `json:"templateNo"      orm:"template_no"      description:""`
	VersionNo       int64      `json:"versionNo"       orm:"version_no"       description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
}
