// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractStatusEvent is the golang structure for table contract_status_event.
type ContractStatusEvent struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	ContractNo          string     `json:"contractNo"          orm:"contract_no"           description:""`
	FlowNo              string     `json:"flowNo"              orm:"flow_no"               description:""`
	GatewayDeliveryNo   *string    `json:"gatewayDeliveryNo"   orm:"gateway_delivery_no"   description:""`
	GatewayRouteNo      string     `json:"gatewayRouteNo"      orm:"gateway_route_no"      description:""`
	GatewayRouteVersion int64      `json:"gatewayRouteVersion" orm:"gateway_route_version" description:""`
	SourceKind          string     `json:"sourceKind"          orm:"source_kind"           description:""`
	ProviderStatus      string     `json:"providerStatus"      orm:"provider_status"       description:""`
	ProjectedStatus     string     `json:"projectedStatus"     orm:"projected_status"      description:""`
	DecisionCode        string     `json:"decisionCode"        orm:"decision_code"         description:""`
	ReasonCode          string     `json:"reasonCode"          orm:"reason_code"           description:""`
	SupplierObservedAt  *time.Time `json:"supplierObservedAt"  orm:"supplier_observed_at"  description:""`
	ObservedAt          *time.Time `json:"observedAt"          orm:"observed_at"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
