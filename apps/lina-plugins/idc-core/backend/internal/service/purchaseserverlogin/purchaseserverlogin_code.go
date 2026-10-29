// This file defines stable, localized and secret-free server-login errors.
package purchaseserverlogin

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid       = bizerr.MustDefine("IDC_PURCHASE_SERVER_LOGIN_INVALID", "Check the hostname and password and try again", gcode.CodeInvalidParameter)
	CodeNotAccessible = bizerr.MustDefine("IDC_PURCHASE_SERVER_LOGIN_NOT_ACCESSIBLE", "The server login choices are not accessible; enter them again", gcode.CodeNotAuthorized)
	CodeUnavailable   = bizerr.MustDefine("IDC_PURCHASE_SERVER_LOGIN_UNAVAILABLE", "Saving server login choices is temporarily unavailable", gcode.CodeInternalError)
)
