// This file defines the versioned JSON invocation envelope shared by governed
// business-extension providers and their source or dynamic runtime adapters.

package capmodel

import "encoding/json"

const (
	// BusinessExtensionInvocationProtocolV1 is the first governed provider-call envelope version.
	BusinessExtensionInvocationProtocolV1 = "v1"
	// BusinessExtensionBridgeRequestType is the fixed guest dispatcher request DTO name.
	BusinessExtensionBridgeRequestType = "BusinessExtensionBridgeRequest"
)

// BusinessExtensionInvocationPurpose distinguishes new-business interactions
// from calls required to operate an already frozen service.
type BusinessExtensionInvocationPurpose string

const (
	// BusinessExtensionInvocationPurposeNewBusiness covers candidate lookup and other new-sale interactions.
	BusinessExtensionInvocationPurposeNewBusiness BusinessExtensionInvocationPurpose = "new-business"
	// BusinessExtensionInvocationPurposeExistingBusiness covers queries and actions for already frozen services.
	BusinessExtensionInvocationPurposeExistingBusiness BusinessExtensionInvocationPurpose = "existing-business"
)

// BusinessExtensionRuntimeContext carries one host-governed opaque lookup key.
// The key is resolved by an owner-aware host service; it never embeds an
// interface instance, revision, configuration, credential, secret, or token.
type BusinessExtensionRuntimeContext struct {
	ContextID string `json:"contextId"`
}

// BusinessExtensionMethodInvocation is the provider-facing, versioned JSON request.
// It intentionally carries no host token, identity snapshot, secret handle, or private manifest state.
type BusinessExtensionMethodInvocation struct {
	Version           string                             `json:"version"`
	OwnerPluginID     string                             `json:"ownerPluginId"`
	ProviderPluginID  string                             `json:"providerPluginId"`
	Point             string                             `json:"point"`
	ContractVersion   string                             `json:"contractVersion"`
	Method            string                             `json:"method"`
	ReleaseVersion    string                             `json:"releaseVersion"`
	ReleaseDigest     string                             `json:"releaseDigest"`
	DeclarationDigest string                             `json:"declarationDigest"`
	Purpose           BusinessExtensionInvocationPurpose `json:"purpose"`
	RuntimeContext    *BusinessExtensionRuntimeContext   `json:"runtimeContext,omitempty"`
	Payload           json.RawMessage                    `json:"payload"`
}

// BusinessExtensionMethodResult is the provider-facing, versioned JSON response.
type BusinessExtensionMethodResult struct {
	Version string          `json:"version"`
	Payload json.RawMessage `json:"payload"`
}

// BusinessExtensionBridgeRequest is the fixed DTO name used by the existing
// dynamic guest controller dispatcher for internal provider calls.
type BusinessExtensionBridgeRequest struct {
	Version           string                             `json:"version"`
	OwnerPluginID     string                             `json:"ownerPluginId"`
	ProviderPluginID  string                             `json:"providerPluginId"`
	Point             string                             `json:"point"`
	ContractVersion   string                             `json:"contractVersion"`
	Method            string                             `json:"method"`
	ReleaseVersion    string                             `json:"releaseVersion"`
	ReleaseDigest     string                             `json:"releaseDigest"`
	DeclarationDigest string                             `json:"declarationDigest"`
	Purpose           BusinessExtensionInvocationPurpose `json:"purpose"`
	RuntimeContext    *BusinessExtensionRuntimeContext   `json:"runtimeContext,omitempty"`
	Payload           json.RawMessage                    `json:"payload"`
}

// BusinessExtensionBridgeResponse is the fixed dynamic guest response DTO.
type BusinessExtensionBridgeResponse struct {
	Version string          `json:"version"`
	Payload json.RawMessage `json:"payload"`
}
