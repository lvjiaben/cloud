// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BusinessNumber is the golang structure for table business_number.
type BusinessNumber struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	NumberType             string     `json:"numberType"             orm:"number_type"              description:"Stable business number namespace"`
	BusinessNo             string     `json:"businessNo"             orm:"business_no"              description:"Cryptographically random opaque business number"`
	ReservationScopeDigest string     `json:"reservationScopeDigest" orm:"reservation_scope_digest" description:"Keyed digest of the normalized caller or command scope; audit reservations use the stable installation lookup key"`
	IdempotencyKeyDigest   string     `json:"idempotencyKeyDigest"   orm:"idempotency_key_digest"   description:"Keyed digest of the reservation idempotency key; audit reservations use the stable installation lookup key"`
	OwnerType              string     `json:"ownerType"              orm:"owner_type"               description:"Owning aggregate type after assignment"`
	OwnerInternalId        int64      `json:"ownerInternalId"        orm:"owner_internal_id"        description:"Internal owner ID that is never exposed across plugins"`
	AssignedAt             *time.Time `json:"assignedAt"             orm:"assigned_at"              description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
	UpdatedAt              *time.Time `json:"updatedAt"              orm:"updated_at"               description:""`
}
