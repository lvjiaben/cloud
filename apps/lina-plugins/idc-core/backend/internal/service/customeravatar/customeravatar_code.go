// This file defines localized errors for customer avatar validation and persistence.

package customeravatar

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid covers missing, oversized, malformed or unsupported avatar images.
	CodeInvalid = bizerr.MustDefine("IDC_CUSTOMER_AVATAR_INVALID", "Choose a valid PNG, JPEG or WebP image under 5 MB", gcode.CodeInvalidParameter)
	// CodeUnavailable covers file reads and governed storage failures.
	CodeUnavailable = bizerr.MustDefine("IDC_CUSTOMER_AVATAR_UNAVAILABLE", "The avatar could not be saved. Please retry", gcode.CodeInternalError)
)
