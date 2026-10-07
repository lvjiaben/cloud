// This file defines stable client-safe errors for governed purchase files.

package purchaseconfigurationfile

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid       = bizerr.MustDefine("IDC_PURCHASE_CONFIGURATION_FILE_INVALID", "The purchase configuration file is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible = bizerr.MustDefine("IDC_PURCHASE_CONFIGURATION_FILE_NOT_ACCESSIBLE", "The purchase configuration file is not accessible", gcode.CodeNotAuthorized)
	CodeConflict      = bizerr.MustDefine("IDC_PURCHASE_CONFIGURATION_FILE_CONFLICT", "The purchase configuration file changed; reload and try again", gcode.CodeInvalidOperation)
	CodeRejected      = bizerr.MustDefine("IDC_PURCHASE_CONFIGURATION_FILE_REJECTED", "The file type, size, or safety result is not allowed", gcode.CodeInvalidOperation)
	CodeUnavailable   = bizerr.MustDefine("IDC_PURCHASE_CONFIGURATION_FILE_UNAVAILABLE", "Purchase configuration file upload is temporarily unavailable", gcode.CodeInternalError)
)
