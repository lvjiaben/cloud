// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// IpReservation is the golang structure of table plugin_idc_resource_ip_reservation for DAO operations like Where/Data.
type IpReservation struct {
	g.Meta                `orm:"table:plugin_idc_resource_ip_reservation, do:true"`
	Id                    any        //
	TenantId              any        //
	ReservationNo         any        //
	ServiceNo             any        //
	ChannelNo             any        //
	PoolNo                any        //
	CapacityObservationNo any        //
	CapacityKind          any        //
	Quantity              any        //
	StatusCode            any        //
	IdempotencyDigest     any        //
	RequestDigest         any        //
	ProviderTaskNo        any        //
	ReleaseReasonCode     any        //
	ExpiresAt             *time.Time //
	ConsumedAt            *time.Time //
	ReleasedAt            *time.Time //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
	Version               any        //
	CommercialOperationNo any        //
}
