// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// IpReservation is the golang structure for table ip_reservation.
type IpReservation struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	ReservationNo         string     `json:"reservationNo"         orm:"reservation_no"          description:""`
	ServiceNo             string     `json:"serviceNo"             orm:"service_no"              description:""`
	ChannelNo             string     `json:"channelNo"             orm:"channel_no"              description:""`
	PoolNo                string     `json:"poolNo"                orm:"pool_no"                 description:""`
	CapacityObservationNo string     `json:"capacityObservationNo" orm:"capacity_observation_no" description:""`
	CapacityKind          string     `json:"capacityKind"          orm:"capacity_kind"           description:""`
	Quantity              int        `json:"quantity"              orm:"quantity"                description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	IdempotencyDigest     string     `json:"idempotencyDigest"     orm:"idempotency_digest"      description:""`
	RequestDigest         string     `json:"requestDigest"         orm:"request_digest"          description:""`
	ProviderTaskNo        string     `json:"providerTaskNo"        orm:"provider_task_no"        description:""`
	ReleaseReasonCode     string     `json:"releaseReasonCode"     orm:"release_reason_code"     description:""`
	ExpiresAt             *time.Time `json:"expiresAt"             orm:"expires_at"              description:""`
	ConsumedAt            *time.Time `json:"consumedAt"            orm:"consumed_at"             description:""`
	ReleasedAt            *time.Time `json:"releasedAt"            orm:"released_at"             description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"              description:""`
	Version               int64      `json:"version"               orm:"version"                 description:""`
	CommercialOperationNo string     `json:"commercialOperationNo" orm:"commercial_operation_no" description:""`
}
