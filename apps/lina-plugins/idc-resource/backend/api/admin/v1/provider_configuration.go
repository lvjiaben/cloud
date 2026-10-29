// This file declares supplier interface and resource-pool administration APIs.
// List pages include permission-scoped local usage, never remote inventory counts.

package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ProviderConfigurationListReq struct {
	g.Meta                `path:"/admin/resource/provider-configurations" method:"get" tags:"IDC Resource Channels" summary:"List Provider channels and resource pools" dc:"Read independently paged local supplier interfaces and resource pools with current staff-visible usage counts" permission:"idc:admin:resource:configuration:list"`
	ChannelPage           int    `json:"channelPage" dc:"Channel page number; defaults to 1" eg:"1"`
	ChannelPageSize       int    `json:"channelPageSize" dc:"Channel page size: 10, 20, 50 or 100; defaults to 20" eg:"20"`
	PoolPage              int    `json:"poolPage" dc:"Pool page number; defaults to 1" eg:"1"`
	PoolPageSize          int    `json:"poolPageSize" dc:"Pool page size: 10, 20, 50 or 100; defaults to 20" eg:"20"`
	ChannelKeyword        string `json:"channelKeyword" dc:"Optional channel-name search, at most 128 characters" eg:"East"`
	AvailableChannelsOnly bool   `json:"availableChannelsOnly" dc:"Exclude deleted channels for the pool selector; defaults to false" eg:"false"`
}

type ProviderConfigurationListRes struct {
	Channels           []*ProviderChannelItem   `json:"channels" dc:"Bounded configured server supplier interfaces"`
	Pools              []*ResourcePoolItem      `json:"pools" dc:"Bounded server resource pools"`
	ProviderCandidates []*ProviderCandidateItem `json:"providerCandidates" dc:"Managed server connectors currently accepting new business"`
	ChannelTotal       int                      `json:"channelTotal" dc:"Exact channel count within the same search and staff scope" eg:"42"`
	PoolTotal          int                      `json:"poolTotal" dc:"Exact pool count within the same staff scope" eg:"108"`
}

// ProviderChannelUsageItem separates current local services from effective products.
type ProviderChannelUsageItem struct {
	ActiveInstances    int `json:"activeInstances" dc:"Currently bound local instances with active entitlement, regardless of power state" eg:"12"`
	TotalInstances     int `json:"totalInstances" dc:"All currently bound local instances, including imported and terminated instances" eg:"18"`
	ConfiguredProducts int `json:"configuredProducts" dc:"Distinct non-deleted products whose current effective version uses this interface" eg:"3"`
}

type ProviderCandidateItem struct {
	PluginID               string          `json:"pluginId" dc:"Stable internal connector selection value"`
	ProviderID             string          `json:"providerId" dc:"Stable server Provider implementation identifier"`
	DisplayName            string          `json:"displayName" dc:"Administrator-visible connector name"`
	ReleaseVersion         string          `json:"releaseVersion" dc:"Installed managed connector release"`
	InterfaceConfiguration json.RawMessage `json:"interfaceConfiguration" dc:"Host-rendered structured channel fields declared by this connector release"`
}

type ProviderChannelItem struct {
	TLSCertificateSHA256      string                       `json:"tlsCertificateSha256" dc:"Historical certificate fingerprint retained for configuration history; not used by the current outbound certificate policy" eg:""`
	ChannelNo                 string                       `json:"channelNo"`
	InterfaceInstanceNo       string                       `json:"interfaceInstanceNo" dc:"Stable product-automation interface number bound to this supplier channel" eg:"pfi_0123456789abcdef0123456789abcdef"`
	ProviderID                string                       `json:"providerId"`
	ProviderPluginID          string                       `json:"providerPluginId"`
	DisplayName               string                       `json:"displayName"`
	Endpoint                  string                       `json:"endpoint"`
	NonSecretSettings         json.RawMessage              `json:"nonSecretSettings"`
	Status                    string                       `json:"status"`
	SecretFields              []*ProviderSecretFieldStatus `json:"secretFields"`
	InterfaceConfiguration    json.RawMessage              `json:"interfaceConfiguration"`
	ProviderContractVersion   string                       `json:"providerContractVersion"`
	ProviderReleaseVersion    string                       `json:"providerReleaseVersion"`
	ProviderReleaseDigest     string                       `json:"providerReleaseDigest"`
	ProviderDeclarationDigest string                       `json:"providerDeclarationDigest"`
	ConnectionStatus          string                       `json:"connectionStatus" dc:"Latest verified connection health: healthy, degraded, or unknown"`
	ConnectionStatusCode      string                       `json:"connectionStatusCode" dc:"Stable non-sensitive latest verification outcome"`
	ConnectionVerifiedAt      *int64                       `json:"connectionVerifiedAt,omitempty" dc:"Latest verification time in Unix milliseconds"`
	Version                   int64                        `json:"version"`
	Usage                     *ProviderChannelUsageItem    `json:"usage,omitempty" dc:"Authoritative local usage populated by the paged directory" eg:"{}"`
	CatalogAvailable          bool                         `json:"catalogAvailable" dc:"Whether this interface's frozen connector declares both upstream product read methods" eg:"true"`
}

type ProviderSecretFieldStatus struct {
	Key       string `json:"key"`
	Required  bool   `json:"required"`
	Ready     bool   `json:"ready"`
	UpdatedAt *int64 `json:"updatedAt,omitempty" dc:"Current secret lifecycle fence as a Unix timestamp in milliseconds" eg:"1700000000000"`
}

type ResourcePoolItem struct {
	PoolNo          string `json:"poolNo"`
	ChannelNo       string `json:"channelNo"`
	ChannelName     string `json:"channelName" dc:"Display name of the owning channel, independent of the channel page" eg:"East China"`
	DisplayName     string `json:"displayName"`
	ResourceKind    string `json:"resourceKind"`
	Status          string `json:"status"`
	RoutingPriority int    `json:"routingPriority"`
	RoutingWeight   int    `json:"routingWeight"`
	Version         int64  `json:"version"`
}

type ProviderChannelCreateReq struct {
	TLSCertificateSHA256 *string `json:"tlsCertificateSha256" v:"max-length:64" dc:"Historical certificate fingerprint retained for configuration history; not used by the current outbound certificate policy" eg:""`
	g.Meta               `path:"/admin/resource/provider-configurations/channels" method:"post" tags:"IDC Resource Channels" summary:"Create a Provider channel" permission:"idc:admin:resource:configuration:write"`
	DisplayName          string         `json:"displayName" v:"required|max-length:128"`
	Endpoint             string         `json:"endpoint" v:"required|max-length:2048"`
	ProviderPluginID     string         `json:"providerPluginId" v:"required|max-length:128"`
	NonSecretSettings    map[string]any `json:"nonSecretSettings" dc:"Structured non-confidential connector settings declared by the selected release"`
}

type ProviderChannelCreateRes struct {
	Item *ProviderChannelItem `json:"item"`
}

type ProviderChannelUpdateReq struct {
	RefreshProvider      bool    `json:"refreshProvider" dc:"Explicitly refresh this unused interface to its enabled connector release; retains endpoint, certificate pin, secrets and status; defaults to false" eg:"false"`
	TLSCertificateSHA256 *string `json:"tlsCertificateSha256" v:"max-length:64" dc:"Historical certificate fingerprint; omit to retain the stored value; certificate verification is disabled by the current interface policy" eg:""`
	g.Meta               `path:"/admin/resource/provider-configurations/channels/{channelNo}" method:"put" tags:"IDC Resource Channels" summary:"Update a Provider channel" permission:"idc:admin:resource:configuration:write"`
	ChannelNo            string         `json:"channelNo" in:"path" v:"required|max-length:64"`
	DisplayName          string         `json:"displayName" v:"required|max-length:128"`
	Endpoint             string         `json:"endpoint" v:"required|max-length:2048"`
	Status               string         `json:"status" v:"required|in:disabled,enabled,maintenance,deleted"`
	Version              int64          `json:"version" v:"required|min:1"`
	NonSecretSettings    map[string]any `json:"nonSecretSettings" dc:"Structured non-confidential connector settings declared by the frozen release"`
}

type ProviderChannelUpdateRes struct {
	Item *ProviderChannelItem `json:"item"`
}

type ProviderChannelVerificationCreateReq struct {
	g.Meta    `path:"/admin/resource/provider-configurations/channels/{channelNo}/connection-verifications" method:"post" tags:"IDC Resource Channels" summary:"Verify a Provider channel connection" dc:"Perform one bounded authenticated read against the configured upstream endpoint without enabling the channel or changing supplier data." permission:"idc:admin:resource:configuration:write"`
	ChannelNo string `json:"channelNo" in:"path" v:"required|max-length:64"`
}

type ProviderChannelVerificationCreateRes struct {
	Verified   bool   `json:"verified" dc:"Whether the authenticated upstream check completed successfully"`
	StatusCode string `json:"statusCode" dc:"Verification outcome: verified, configuration_incomplete, verification_unsupported, tls_untrusted, timeout, dns, network, endpoint_not_found, invalid_response, auth_rejected, local_auth_policy, or connection_failed"`
	ObservedAt int64  `json:"observedAt" dc:"Verification observation time in Unix milliseconds"`
}

type ResourcePoolCreateReq struct {
	g.Meta          `path:"/admin/resource/provider-configurations/pools" method:"post" tags:"IDC Resource Pools" summary:"Create a resource pool" permission:"idc:admin:resource:configuration:write"`
	ChannelNo       string `json:"channelNo" v:"required|max-length:64"`
	DisplayName     string `json:"displayName" v:"required|max-length:128"`
	ResourceKind    string `json:"resourceKind" v:"required|in:virtual_server,bare_metal"`
	RoutingPriority int    `json:"routingPriority" v:"min:0|max:100000"`
	RoutingWeight   int    `json:"routingWeight" v:"min:0|max:10000"`
}

type ResourcePoolCreateRes struct {
	Item *ResourcePoolItem `json:"item"`
}

type ResourcePoolUpdateReq struct {
	g.Meta          `path:"/admin/resource/provider-configurations/pools/{poolNo}" method:"put" tags:"IDC Resource Pools" summary:"Update a resource pool" permission:"idc:admin:resource:configuration:write"`
	PoolNo          string `json:"poolNo" in:"path" v:"required|max-length:64"`
	DisplayName     string `json:"displayName" v:"required|max-length:128"`
	ResourceKind    string `json:"resourceKind" v:"required|in:virtual_server,bare_metal"`
	Status          string `json:"status" v:"required|in:disabled,enabled,draining,maintenance,deleted"`
	RoutingPriority int    `json:"routingPriority" v:"min:0|max:100000"`
	RoutingWeight   int    `json:"routingWeight" v:"min:0|max:10000"`
	Version         int64  `json:"version" v:"required|min:1"`
}

type ResourcePoolUpdateRes struct {
	Item *ResourcePoolItem `json:"item"`
}
