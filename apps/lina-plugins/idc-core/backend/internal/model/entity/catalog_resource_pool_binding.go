// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogResourcePoolBinding is the golang structure for table catalog_resource_pool_binding.
type CatalogResourcePoolBinding struct {
	Id                      int64      `json:"id"                      orm:"id"                         description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                  description:""`
	BindingNo               string     `json:"bindingNo"               orm:"binding_no"                 description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                 description:""`
	PoolNo                  string     `json:"poolNo"                  orm:"pool_no"                    description:"Opaque resource-owner pool business number; no cross-plugin foreign key"`
	RegionCode              string     `json:"regionCode"              orm:"region_code"                description:""`
	LineCode                string     `json:"lineCode"                orm:"line_code"                  description:""`
	Priority                int        `json:"priority"                orm:"priority"                   description:""`
	Weight                  int        `json:"weight"                  orm:"weight"                     description:""`
	NewOrderEnabled         bool       `json:"newOrderEnabled"         orm:"new_order_enabled"          description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"                description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                 description:""`
	MaintenanceModeCode     string     `json:"maintenanceModeCode"     orm:"maintenance_mode_code"      description:"Normal, drain, or maintenance mode for new-order routing only"`
	MinimumSellableQuantity int64      `json:"minimumSellableQuantity" orm:"minimum_sellable_quantity"  description:"Local sellable quantity threshold required before automatic fulfillment is promised"`
	UncertainSalePolicyCode string     `json:"uncertainSalePolicyCode" orm:"uncertain_sale_policy_code" description:"Fail-closed stop-sale or explicit manual-confirmation policy for uncertain inventory"`
}
