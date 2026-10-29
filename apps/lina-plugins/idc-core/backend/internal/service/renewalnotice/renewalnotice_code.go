// This file defines stable errors for the current-staff renewal notice boundary.

package renewalnotice

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid     = bizerr.MustDefine("IDC_RENEWAL_NOTICE_INVALID", "The renewal notification request is invalid", gcode.CodeInvalidParameter)
	CodeDenied      = bizerr.MustDefine("IDC_RENEWAL_NOTICE_DENIED", "Renewal work items are not accessible", gcode.CodeNotAuthorized)
	CodeUnavailable = bizerr.MustDefine("IDC_RENEWAL_NOTICE_UNAVAILABLE", "Renewal notifications are temporarily unavailable", gcode.CodeInternalError)
)
