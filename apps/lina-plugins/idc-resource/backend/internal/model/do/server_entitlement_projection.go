// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerEntitlementProjection is the golang structure of table plugin_idc_resource_server_entitlement_projection for DAO operations like Where/Data.
type ServerEntitlementProjection struct {
	g.Meta                     `orm:"table:plugin_idc_resource_server_entitlement_projection, do:true"`
	Id                         any        //
	TenantId                   any        //
	EntitlementNo              any        //
	SourceIntentNo             any        //
	CustomerNo                 any        // Frozen customer owner delivered through the owner command boundary; never derived from JSON or another plugin table
	ProductNo                  any        // Opaque generic product reference frozen by the entitlement-created event
	ProductVersionNo           any        // Opaque generic product version reference frozen by the entitlement-created event
	ProductNameZhCn            any        // Frozen customer-safe Simplified Chinese product label delivered by the entitlement owner event
	ProductNameEnUs            any        // Frozen customer-safe English product label delivered by the entitlement owner event
	LocalResourceNo            any        // Deterministic generic entitlement number used as the resource-local identity
	MappingNo                  any        //
	ProviderTaskNo             any        //
	InterfaceInstanceNo        any        //
	ProviderPluginId           any        //
	PluginReleaseVersion       any        //
	FulfillmentContractVersion any        //
	ServerContractVersion      any        //
	ReleaseDigest              any        //
	DeclarationDigest          any        //
	Version                    any        //
	BoundAt                    *time.Time //
	CreatedAt                  *time.Time //
	UpdatedAt                  *time.Time //
	ActionReference            any        // Stable opaque resource-owner action reference; safe for routing but never rendered as customer-visible text
	ServerNo                   any        // Stable customer-visible server business number; distinct from the hidden action reference
}
