// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAffiliateTouch is the golang structure for table sales_affiliate_touch.
type SalesAffiliateTouch struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	TouchNo        string     `json:"touchNo"        orm:"touch_no"        description:""`
	LinkNo         string     `json:"linkNo"         orm:"link_no"         description:""`
	VisitorDigest  string     `json:"visitorDigest"  orm:"visitor_digest"  description:""`
	DeviceDigest   string     `json:"deviceDigest"   orm:"device_digest"   description:""`
	IpDigest       string     `json:"ipDigest"       orm:"ip_digest"       description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest" description:""`
	OccurredAt     *time.Time `json:"occurredAt"     orm:"occurred_at"     description:""`
	ExpiresAt      *time.Time `json:"expiresAt"      orm:"expires_at"      description:""`
}
