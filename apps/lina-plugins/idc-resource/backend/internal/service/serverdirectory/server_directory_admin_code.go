// This file defines stable localized failures for the staff server directory.
package serverdirectory

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeDirectoryAdminUnavailable   = bizerr.MustDefine("IDC_RESOURCE_DIRECTORY_ADMIN_UNAVAILABLE", "The server directory is temporarily unavailable", gcode.CodeInternalError)
	CodeDirectoryAdminInvalid       = bizerr.MustDefine("IDC_RESOURCE_DIRECTORY_ADMIN_INVALID", "The server directory request is invalid", gcode.CodeInvalidParameter)
	CodeDirectoryAdminNotAccessible = bizerr.MustDefine("IDC_RESOURCE_DIRECTORY_ADMIN_NOT_ACCESSIBLE", "The server is not accessible", gcode.CodeNotAuthorized)
)
