// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Channel is the golang structure for table channel.
type Channel struct {
	Id                        int64      `json:"id"                        orm:"id"                          description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                   description:""`
	ChannelNo                 string     `json:"channelNo"                 orm:"channel_no"                  description:"Opaque resource-owner channel reference"`
	ProviderId                string     `json:"providerId"                orm:"provider_id"                 description:"Provider descriptor identifier declared through resourcecap SPI"`
	ProviderPluginId          string     `json:"providerPluginId"          orm:"provider_plugin_id"          description:"Plugin that owns the frozen Provider implementation"`
	DisplayName               string     `json:"displayName"               orm:"display_name"                description:""`
	Endpoint                  string     `json:"endpoint"                  orm:"endpoint"                    description:"Administrator-approved HTTPS API origin without credentials"`
	NonSecretSettings         string     `json:"nonSecretSettings"         orm:"non_secret_settings"         description:"Adapter settings validated to exclude credentials and secret references"`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                 description:""`
	Version                   int64      `json:"version"                   orm:"version"                     description:""`
	CreatedBy                 int        `json:"createdBy"                 orm:"created_by"                  description:""`
	UpdatedBy                 int        `json:"updatedBy"                 orm:"updated_by"                  description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                  description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                  description:""`
	DeletedAt                 *time.Time `json:"deletedAt"                 orm:"deleted_at"                  description:"Recoverable configuration deletion time; historical records remain referenced"`
	ProviderRuntimeType       string     `json:"providerRuntimeType"       orm:"provider_runtime_type"       description:"Frozen Provider runtime type: source or dynamic"`
	ProviderContractVersion   string     `json:"providerContractVersion"   orm:"provider_contract_version"   description:"Exact owner contract version frozen when a dynamic Provider channel is created"`
	ProviderReleaseVersion    string     `json:"providerReleaseVersion"    orm:"provider_release_version"    description:"Exact managed plugin release version frozen when a dynamic Provider channel is created"`
	ProviderReleaseDigest     string     `json:"providerReleaseDigest"     orm:"provider_release_digest"     description:"Exact managed plugin release digest frozen when a dynamic Provider channel is created"`
	ProviderDeclarationDigest string     `json:"providerDeclarationDigest" orm:"provider_declaration_digest" description:"Exact resource-provider declaration digest frozen when a dynamic Provider channel is created"`
}
