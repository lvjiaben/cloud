// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAffiliateAttribution is the golang structure for table sales_affiliate_attribution.
type SalesAffiliateAttribution struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	AttributionNo      string     `json:"attributionNo"      orm:"attribution_no"       description:""`
	LinkNo             string     `json:"linkNo"             orm:"link_no"              description:""`
	TouchNo            string     `json:"touchNo"            orm:"touch_no"             description:""`
	ReferrerCustomerNo string     `json:"referrerCustomerNo" orm:"referrer_customer_no" description:""`
	ReferredCustomerNo string     `json:"referredCustomerNo" orm:"referred_customer_no" description:""`
	PolicyCode         string     `json:"policyCode"         orm:"policy_code"          description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	RiskReasonCode     *string    `json:"riskReasonCode"     orm:"risk_reason_code"     description:""`
	EvidenceDigest     string     `json:"evidenceDigest"     orm:"evidence_digest"      description:""`
	AttributedAt       *time.Time `json:"attributedAt"       orm:"attributed_at"        description:""`
	ExpiresAt          *time.Time `json:"expiresAt"          orm:"expires_at"           description:""`
}
