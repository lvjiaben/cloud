// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BusinessNumber is the golang structure of table plugin_idc_core_business_number for DAO operations like Where/Data.
type BusinessNumber struct {
	g.Meta                 `orm:"table:plugin_idc_core_business_number, do:true"`
	Id                     any        //
	TenantId               any        //
	NumberType             any        // Stable business number namespace
	BusinessNo             any        // Cryptographically random opaque business number
	ReservationScopeDigest any        // Keyed digest of the normalized caller or command scope; audit reservations use the stable installation lookup key
	IdempotencyKeyDigest   any        // Keyed digest of the reservation idempotency key; audit reservations use the stable installation lookup key
	OwnerType              any        // Owning aggregate type after assignment
	OwnerInternalId        any        // Internal owner ID that is never exposed across plugins
	AssignedAt             *time.Time //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
}
