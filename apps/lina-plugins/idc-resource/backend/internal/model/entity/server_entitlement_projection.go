// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerEntitlementProjection is the golang structure for table server_entitlement_projection.
type ServerEntitlementProjection struct {
	Id                         int64      `json:"id"                         orm:"id"                           description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	EntitlementNo              string     `json:"entitlementNo"              orm:"entitlement_no"               description:""`
	SourceIntentNo             string     `json:"sourceIntentNo"             orm:"source_intent_no"             description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                  description:"Frozen customer owner delivered through the owner command boundary; never derived from JSON or another plugin table"`
	ProductNo                  string     `json:"productNo"                  orm:"product_no"                   description:"Opaque generic product reference frozen by the entitlement-created event"`
	ProductVersionNo           string     `json:"productVersionNo"           orm:"product_version_no"           description:"Opaque generic product version reference frozen by the entitlement-created event"`
	ProductNameZhCn            string     `json:"productNameZhCn"            orm:"product_name_zh_cn"           description:"Frozen customer-safe Simplified Chinese product label delivered by the entitlement owner event"`
	ProductNameEnUs            string     `json:"productNameEnUs"            orm:"product_name_en_us"           description:"Frozen customer-safe English product label delivered by the entitlement owner event"`
	LocalResourceNo            string     `json:"localResourceNo"            orm:"local_resource_no"            description:"Deterministic generic entitlement number used as the resource-local identity"`
	MappingNo                  string     `json:"mappingNo"                  orm:"mapping_no"                   description:""`
	ProviderTaskNo             string     `json:"providerTaskNo"             orm:"provider_task_no"             description:""`
	InterfaceInstanceNo        string     `json:"interfaceInstanceNo"        orm:"interface_instance_no"        description:""`
	ProviderPluginId           string     `json:"providerPluginId"           orm:"provider_plugin_id"           description:""`
	PluginReleaseVersion       string     `json:"pluginReleaseVersion"       orm:"plugin_release_version"       description:""`
	FulfillmentContractVersion string     `json:"fulfillmentContractVersion" orm:"fulfillment_contract_version" description:""`
	ServerContractVersion      string     `json:"serverContractVersion"      orm:"server_contract_version"      description:""`
	ReleaseDigest              string     `json:"releaseDigest"              orm:"release_digest"               description:""`
	DeclarationDigest          string     `json:"declarationDigest"          orm:"declaration_digest"           description:""`
	Version                    int64      `json:"version"                    orm:"version"                      description:""`
	BoundAt                    *time.Time `json:"boundAt"                    orm:"bound_at"                     description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
	UpdatedAt                  *time.Time `json:"updatedAt"                  orm:"updated_at"                   description:""`
	ActionReference            string     `json:"actionReference"            orm:"action_reference"             description:"Stable opaque resource-owner action reference; safe for routing but never rendered as customer-visible text"`
	ServerNo                   string     `json:"serverNo"                   orm:"server_no"                    description:"Stable customer-visible server business number; distinct from the hidden action reference"`
}
