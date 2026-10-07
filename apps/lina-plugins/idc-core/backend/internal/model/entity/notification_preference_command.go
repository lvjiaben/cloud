// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NotificationPreferenceCommand is the golang structure for table notification_preference_command.
type NotificationPreferenceCommand struct {
	Id                 int64      `json:"id"                 orm:"id"                  description:""`
	TenantId           int64      `json:"tenantId"           orm:"tenant_id"           description:""`
	CommandNo          string     `json:"commandNo"          orm:"command_no"          description:""`
	CustomerNo         string     `json:"customerNo"         orm:"customer_no"         description:""`
	IdempotencyDigest  string     `json:"idempotencyDigest"  orm:"idempotency_digest"  description:""`
	RequestFingerprint string     `json:"requestFingerprint" orm:"request_fingerprint" description:""`
	EvidenceDigest     string     `json:"evidenceDigest"     orm:"evidence_digest"     description:""`
	ResultVersion      int64      `json:"resultVersion"      orm:"result_version"      description:""`
	AppliedAt          *time.Time `json:"appliedAt"          orm:"applied_at"          description:""`
}
