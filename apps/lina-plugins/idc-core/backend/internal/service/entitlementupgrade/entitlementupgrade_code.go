// This file declares stable customer-facing upgrade errors for plugin translation.

package entitlementupgrade

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid rejects invalid or expired paid-period upgrade requests.
	CodeInvalid = bizerr.MustDefine("IDC_ENTITLEMENT_UPGRADE_INVALID", "The service configuration cannot be upgraded with these choices", gcode.CodeInvalidParameter)
	// CodeUnavailable refuses upgrades whose current Provider cannot prove support.
	CodeUnavailable = bizerr.MustDefine("IDC_ENTITLEMENT_UPGRADE_UNAVAILABLE", "This service does not currently support automatic configuration upgrades", gcode.CodeInvalidOperation)
	// CodeConflict prevents stale, reduced, or concurrent paid upgrade intents.
	CodeConflict = bizerr.MustDefine("IDC_ENTITLEMENT_UPGRADE_CONFLICT", "The service or quote changed; refresh before upgrading", gcode.CodeInvalidOperation)
)
