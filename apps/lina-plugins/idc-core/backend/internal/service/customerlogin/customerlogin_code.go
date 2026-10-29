package customerlogin

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid     = bizerr.MustDefine("IDC_CUSTOMER_LOGIN_INVALID", "The account information or password is incorrect", gcode.CodeNotAuthorized)
	CodeUnavailable = bizerr.MustDefine("IDC_CUSTOMER_LOGIN_UNAVAILABLE", "Account login is temporarily unavailable", gcode.CodeInternalError)
	CodeChanged     = bizerr.MustDefine("IDC_CUSTOMER_LOGIN_CHANGED", "Account information changed; retry the operation", gcode.CodeInvalidOperation)
)
