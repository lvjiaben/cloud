// This file defines the secret-safe installation-wide Aliyun FACE configuration query.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProviderConfigurationGetReq reads the fixed installation-wide provider target.
type ProviderConfigurationGetReq struct {
	g.Meta `path:"/admin/compliance/providers/aliyun-face" method:"get" tags:"IDC Compliance Provider" summary:"Get Aliyun identity verification configuration" dc:"Return the installation-wide Aliyun FACE configuration without secret references, credentials, endpoints, ciphertext, validation payloads, or identity data." permission:"idc:admin:compliance:provider:query"`
}

// ProviderConfigurationGetRes wraps one secret-safe configuration projection.
type ProviderConfigurationGetRes struct {
	Item *ProviderConfigurationItem `json:"item" dc:"Secret-safe installation-wide provider configuration" eg:"{}"`
}

// ProviderConfigurationItem excludes secret routing references and credential material.
type ProviderConfigurationItem struct {
	ProviderCode         string `json:"providerCode" dc:"Fixed provider code aliyun_face" eg:"aliyun_face"`
	Status               string `json:"status" dc:"Configuration state: draft, updating, configured, or disabled" eg:"configured"`
	SceneID              string `json:"sceneId" dc:"Merchant-owned Aliyun verification scene identifier; empty before configuration" eg:"100000001"`
	CredentialConfigured bool   `json:"credentialConfigured" dc:"Whether the configured secret reference resolves to active secret metadata" eg:"true"`
	CredentialMaskedTail string `json:"credentialMaskedTail" dc:"Display-only credential suffix; empty when unavailable" eg:"K001"`
	CredentialGeneration int64  `json:"credentialGeneration" dc:"Committed credential replacement generation" eg:"2"`
	ValidationStatus     string `json:"validationStatus" dc:"Bounded connectivity status: untested, reachable, rejected, or unavailable" eg:"reachable"`
	Version              int64  `json:"version" dc:"Optimistic concurrency version; zero before first configuration" eg:"3"`
	UpdatedAt            *int64 `json:"updatedAt" dc:"Latest configuration update Unix timestamp in milliseconds; null before first configuration" eg:"1786464000000"`
	ValidatedAt          *int64 `json:"validatedAt" dc:"Latest connectivity validation Unix timestamp in milliseconds; null when untested" eg:"1786464060000"`
}
