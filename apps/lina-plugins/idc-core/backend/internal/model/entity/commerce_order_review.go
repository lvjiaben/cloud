// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceOrderReview is the golang structure for table commerce_order_review.
type CommerceOrderReview struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	ReviewNo       string     `json:"reviewNo"       orm:"review_no"        description:""`
	OrderNo        string     `json:"orderNo"        orm:"order_no"         description:""`
	CustomerNo     string     `json:"customerNo"     orm:"customer_no"      description:""`
	ReviewTypeCode string     `json:"reviewTypeCode" orm:"review_type_code" description:""`
	StatusCode     string     `json:"statusCode"     orm:"status_code"      description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"      description:""`
	RequestDigest  *string    `json:"requestDigest"  orm:"request_digest"   description:""`
	EvidenceDigest *string    `json:"evidenceDigest" orm:"evidence_digest"  description:""`
	ReviewedBy     *int       `json:"reviewedBy"     orm:"reviewed_by"      description:""`
	RequestedAt    *time.Time `json:"requestedAt"    orm:"requested_at"     description:""`
	ReviewedAt     *time.Time `json:"reviewedAt"     orm:"reviewed_at"      description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"       description:""`
}
