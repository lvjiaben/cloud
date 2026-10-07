// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryPricePolicyVersion is the golang structure of table plugin_idc_core_server_accessory_price_policy_version for DAO operations like Where/Data.
type ServerAccessoryPricePolicyVersion struct {
	g.Meta             `orm:"table:plugin_idc_core_server_accessory_price_policy_version, do:true"`
	Id                 any        //
	TenantId           any        //
	PolicyNo           any        //
	PolicyVersion      any        //
	AccessoryKind      any        //
	CurrencyCode       any        //
	FreeCountPerServer any        //
	UnitAmountMinor    any        //
	PeriodUnitCode     any        //
	PeriodCount        any        //
	ContentDigest      any        //
	PublishedBy        any        //
	PublishedAt        *time.Time //
	TrafficGb          any        //
	NameZhCn           any        //
	NameEnUs           any        //
}
