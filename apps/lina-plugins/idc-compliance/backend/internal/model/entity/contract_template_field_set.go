// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractTemplateFieldSet is the golang structure for table contract_template_field_set.
type ContractTemplateFieldSet struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	FieldSetNo               string     `json:"fieldSetNo"               orm:"field_set_no"               description:""`
	TemplateNo               string     `json:"templateNo"               orm:"template_no"                description:""`
	VersionNo                int64      `json:"versionNo"                orm:"version_no"                 description:""`
	ContentRevisionNo        string     `json:"contentRevisionNo"        orm:"content_revision_no"        description:""`
	CommandIdempotencyDigest string     `json:"commandIdempotencyDigest" orm:"command_idempotency_digest" description:""`
	CommandFingerprint       string     `json:"commandFingerprint"       orm:"command_fingerprint"        description:""`
	CreatedByUserId          int        `json:"createdByUserId"          orm:"created_by_user_id"         description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
}
