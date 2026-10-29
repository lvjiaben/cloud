// This file defines stable public authentication adapter errors.
package loginprofile

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var codeAuthInvalid = bizerr.MustDefine("IDC_CUSTOMER_AUTH_INVALID", "The account information or verification is incorrect", gcode.CodeNotAuthorized)
