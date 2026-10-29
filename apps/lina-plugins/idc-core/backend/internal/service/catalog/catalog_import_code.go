// This file defines actionable errors for upstream product assignment.

package catalog

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// CodeUpstreamDefinitionIncomplete rejects partial contracts before any local write.
var CodeUpstreamDefinitionIncomplete = bizerr.MustDefine("IDC_CATALOG_UPSTREAM_DEFINITION_INCOMPLETE", "The upstream connector has not supplied a complete product configuration and price schedule; no changes were saved", gcode.CodeInvalidOperation)

// CodeUpstreamConfigurationLocked keeps source evidence immutable while local prices remain editable.
var CodeUpstreamConfigurationLocked = bizerr.MustDefine("IDC_CATALOG_UPSTREAM_CONFIGURATION_LOCKED", "This upstream product configuration is fixed; edit its local sale prices under Package price", gcode.CodeInvalidOperation)
