// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractStatusEvent is the golang structure of table plugin_idc_compliance_contract_status_event for DAO operations like Where/Data.
type ContractStatusEvent struct {
	g.Meta              `orm:"table:plugin_idc_compliance_contract_status_event, do:true"`
	Id                  any        //
	TenantId            any        //
	ContractNo          any        //
	FlowNo              any        //
	GatewayDeliveryNo   *string    //
	GatewayRouteNo      any        //
	GatewayRouteVersion any        //
	SourceKind          any        //
	ProviderStatus      any        //
	ProjectedStatus     any        //
	DecisionCode        any        //
	ReasonCode          any        //
	SupplierObservedAt  *time.Time //
	ObservedAt          *time.Time //
	CreatedAt           *time.Time //
}
