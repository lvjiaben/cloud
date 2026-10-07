// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServicePriceComponentSnapshot is the golang structure of table plugin_idc_core_service_price_component_snapshot for DAO operations like Where/Data.
type ServicePriceComponentSnapshot struct {
	g.Meta                        `orm:"table:plugin_idc_core_service_price_component_snapshot, do:true"`
	Id                            any        //
	TenantId                      any        //
	ComponentSnapshotNo           any        //
	PriceSnapshotNo               any        //
	ServiceNo                     any        //
	LineNumber                    any        //
	ComponentTypeCode             any        //
	ChargePhaseCode               any        //
	OptionNo                      *string    //
	SourceReference               any        //
	Quantity                      any        //
	UnitAmountMinor               any        //
	AmountMinor                   any        //
	SourceDigest                  any        //
	ContentDigest                 any        //
	CreatedAt                     *time.Time //
	SubjectTypeCode               any        //
	SubjectNo                     any        //
	ConfigurationPriceDimensionNo any        //
	ConfigurationSnapshotNo       any        //
}
