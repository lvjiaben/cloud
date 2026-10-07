// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// UnsubscribeEvidence is the golang structure for table unsubscribe_evidence.
type UnsubscribeEvidence struct {
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	EvidenceNo     string     `json:"evidenceNo"     orm:"evidence_no"     description:""`
	OwnerUserId    int64      `json:"ownerUserId"    orm:"owner_user_id"   description:""`
	CustomerNo     string     `json:"customerNo"     orm:"customer_no"     description:""`
	ContactNo      string     `json:"contactNo"      orm:"contact_no"      description:""`
	ChannelCode    string     `json:"channelCode"    orm:"channel_code"    description:""`
	TopicCode      string     `json:"topicCode"      orm:"topic_code"      description:""`
	SourceCode     string     `json:"sourceCode"     orm:"source_code"     description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest" description:""`
	OccurredAt     *time.Time `json:"occurredAt"     orm:"occurred_at"     description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
