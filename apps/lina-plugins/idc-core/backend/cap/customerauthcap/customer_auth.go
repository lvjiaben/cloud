// Package customerauthcap defines the smallest source-only boundary that lets
// a business owner prove the current authenticated customer. It accepts no
// customer number from HTTP and exposes no legal-identity or compliance data.
package customerauthcap

import "context"

const (
	OwnerPluginID           = "idc-core"
	AllowedCallerPluginID   = "idc-resource"
	AllowedAICallerPluginID = "idc-ai"
)

// IsAllowedCallerPlugin reports whether one declared source-plugin consumer
// may resolve the bounded current-customer reader.
func IsAllowedCallerPlugin(pluginID string) bool {
	return pluginID == AllowedCallerPluginID || pluginID == AllowedAICallerPluginID
}

// Subject is the bounded customer owner needed by customer-facing resources.
type Subject struct {
	CustomerNo         string
	CustomerTypeCode   string
	PrincipalReference string
	ActorUserID        int
}

// Reader proves the current request's active customer binding.
type Reader interface {
	CurrentCustomer(context.Context) (*Subject, error)
}
