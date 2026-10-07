// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingChannelConsentEvent is the golang structure for table marketing_channel_consent_event.
type MarketingChannelConsentEvent struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	EventNo              string     `json:"eventNo"              orm:"event_no"               description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:""`
	ContactNo            string     `json:"contactNo"            orm:"contact_no"             description:""`
	ChannelCode          string     `json:"channelCode"          orm:"channel_code"           description:""`
	ActionCode           string     `json:"actionCode"           orm:"action_code"            description:""`
	PolicyReference      string     `json:"policyReference"      orm:"policy_reference"       description:""`
	PolicyVersion        string     `json:"policyVersion"        orm:"policy_version"         description:""`
	Locale               string     `json:"locale"               orm:"locale"                 description:""`
	SourceCode           string     `json:"sourceCode"           orm:"source_code"            description:""`
	EvidenceDigest       string     `json:"evidenceDigest"       orm:"evidence_digest"        description:""`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:""`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:""`
	OccurredAt           *time.Time `json:"occurredAt"           orm:"occurred_at"            description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
}
