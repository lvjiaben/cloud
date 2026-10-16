// This file defines the in-process bridge for a core-authorized existing
// business invocation. Credentials and endpoint configuration remain owned by
// idc-resource; the core registrar retains its own provider invocation scope.

package resourceinterfaceprojectioncap

import (
	"context"

	"lina-core/pkg/plugin/pluginhost"
)

// ExistingBusinessOutboundRequest contains only evidence frozen by the core
// business owner. It must not be constructed from an unauthorized HTTP request.
type ExistingBusinessOutboundRequest struct {
	InterfaceInstanceNo            string
	InterfaceConfigurationRevision string
	ResourceChannelRef             string
	PluginID                       string
	PluginReleaseVersion           string
	ContractVersion                string
	ReleaseDigest                  string
	DeclarationDigest              string
	Method                         string
}

// ExistingBusinessOutboundSource temporarily lends a guarded handler to the
// core owner after exact resource evidence checks. The callback must register
// it through core's own WithConfidentialOutboundContext and must not retain it.
// The resource owner never returns plaintext credentials or endpoint values.
type ExistingBusinessOutboundSource interface {
	WithOutbound(context.Context, ExistingBusinessOutboundRequest, func(pluginhost.ConfidentialOutboundHandler) error) error
}
