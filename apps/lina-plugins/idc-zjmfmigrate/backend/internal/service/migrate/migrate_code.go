package migrate

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var CodeInvalidPage = bizerr.MustDefine("IDC_ZJMFMIGRATE_PAGE_INVALID", "The requested migration page is invalid", gcode.CodeInvalidParameter)
