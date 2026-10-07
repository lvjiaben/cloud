package notificationpolicy

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var codeCurrentStaffNotAccessible = bizerr.MustDefine(
	"IDC_NOTIFICATION_CURRENT_STAFF_NOT_ACCESSIBLE",
	"The current staff identity is not accessible",
	gcode.CodeNotAuthorized,
)
