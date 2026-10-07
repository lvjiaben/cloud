// This file defines stable translated errors for the staff usage projection.

package resourceinterfaceprojection

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// Usage errors remain separate from resource configuration write failures.
var (
	codeUsageNotAccessible = bizerr.MustDefine("IDC_INTERFACE_USAGE_NOT_ACCESSIBLE", "The interface usage is not accessible", gcode.CodeNotAuthorized)
	codeUsageInvalid       = bizerr.MustDefine("IDC_INTERFACE_USAGE_INVALID", "The interface usage request is invalid", gcode.CodeInvalidParameter)
	codeUsageUnavailable   = bizerr.MustDefine("IDC_INTERFACE_USAGE_UNAVAILABLE", "The interface usage is temporarily unavailable", gcode.CodeInternalError)
)
