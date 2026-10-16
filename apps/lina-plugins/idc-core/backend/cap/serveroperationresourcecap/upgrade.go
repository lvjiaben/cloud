// This file keeps generic commerce configuration opaque while asking the
// server owner to validate and compile a paid configuration upgrade.

package serveroperationresourcecap

import (
	"context"
	"encoding/json"
)

// UpgradeConfigurationRequest contains trusted catalog compiler output. The
// server owner rechecks customer ownership and current Provider capabilities;
// no HTTP caller may submit Provider parameters or override the route.
type UpgradeConfigurationRequest struct {
	ServiceNo, CustomerNo string
	ProductConfiguration  json.RawMessage
	CurrentConfiguration  json.RawMessage
	TargetConfiguration   json.RawMessage
}

// UpgradeConfiguration is a detached, secret-free Provider command accepted
// by the current capability snapshot for the independently resolved route.
type UpgradeConfiguration struct {
	Route      EntitlementRoute
	Parameters json.RawMessage
}

// UpgradeConfigurationCompiler is an optional server-owner boundary. Missing
// support must refuse automatic upgrades before a quote or invoice is created.
type UpgradeConfigurationCompiler interface {
	// CompileUpgrade verifies ownership, rejects unchanged/unsupported changes
	// and every reduction, then validates the resulting Provider parameters.
	CompileUpgrade(context.Context, UpgradeConfigurationRequest) (*UpgradeConfiguration, error)
}
