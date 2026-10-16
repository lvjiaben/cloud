// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Channel is the golang structure of table plugin_idc_resource_channel for DAO operations like Where/Data.
type Channel struct {
	g.Meta                    `orm:"table:plugin_idc_resource_channel, do:true"`
	Id                        any        //
	TenantId                  any        //
	ChannelNo                 any        // Opaque resource-owner channel reference
	ProviderId                any        // Provider descriptor identifier declared through resourcecap SPI
	ProviderPluginId          any        // Plugin that owns the frozen Provider implementation
	DisplayName               any        //
	Endpoint                  any        // Administrator-approved HTTPS API origin without credentials
	NonSecretSettings         any        // Adapter settings validated to exclude credentials and secret references
	StatusCode                any        //
	Version                   any        //
	CreatedBy                 any        //
	UpdatedBy                 any        //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
	DeletedAt                 *time.Time // Recoverable configuration deletion time; historical records remain referenced
	ProviderRuntimeType       any        // Frozen Provider runtime type: source or dynamic
	ProviderContractVersion   any        // Exact owner contract version frozen when a dynamic Provider channel is created
	ProviderReleaseVersion    any        // Exact managed plugin release version frozen when a dynamic Provider channel is created
	ProviderReleaseDigest     any        // Exact managed plugin release digest frozen when a dynamic Provider channel is created
	ProviderDeclarationDigest any        // Exact resource-provider declaration digest frozen when a dynamic Provider channel is created
	TlsCertificateSha256      any        // Optional administrator-trusted exact leaf certificate SHA-256; empty uses system trust
}
