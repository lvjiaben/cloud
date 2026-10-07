// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceFulfillmentUnit is the golang structure for table commerce_fulfillment_unit.
type CommerceFulfillmentUnit struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	FulfillmentNo       string     `json:"fulfillmentNo"       orm:"fulfillment_no"        description:""`
	OrderNo             string     `json:"orderNo"             orm:"order_no"              description:""`
	OrderItemNo         string     `json:"orderItemNo"         orm:"order_item_no"         description:""`
	UnitSequence        int        `json:"unitSequence"        orm:"unit_sequence"         description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	ReservationNo       *string    `json:"reservationNo"       orm:"reservation_no"        description:""`
	BusinessOperationNo *string    `json:"businessOperationNo" orm:"business_operation_no" description:""`
	ServiceNo           *string    `json:"serviceNo"           orm:"service_no"            description:""`
	LastEvidenceDigest  *string    `json:"lastEvidenceDigest"  orm:"last_evidence_digest"  description:""`
	StartedAt           *time.Time `json:"startedAt"           orm:"started_at"            description:""`
	CompletedAt         *time.Time `json:"completedAt"         orm:"completed_at"          description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
