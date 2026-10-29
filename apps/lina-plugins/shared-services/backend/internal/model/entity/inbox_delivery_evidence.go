// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// InboxDeliveryEvidence is the golang structure for table inbox_delivery_evidence.
type InboxDeliveryEvidence struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int64      `json:"tenantId"       orm:"tenant_id"        description:""`
	EvidenceNo     string     `json:"evidenceNo"     orm:"evidence_no"      description:""`
	MessageNo      string     `json:"messageNo"      orm:"message_no"       description:""`
	CallerPluginId string     `json:"callerPluginId" orm:"caller_plugin_id" description:""`
	EvidenceType   string     `json:"evidenceType"   orm:"evidence_type"    description:""`
	ContentDigest  string     `json:"contentDigest"  orm:"content_digest"   description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
	HostMessageId  string     `json:"hostMessageId"  orm:"host_message_id"  description:""`
}
