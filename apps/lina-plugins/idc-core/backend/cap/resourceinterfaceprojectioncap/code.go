// This file defines the stable rejection for historical interface references.
package resourceinterfaceprojectioncap

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var CodeRefreshReferenced = bizerr.MustDefine("IDC_RESOURCE_INTERFACE_REFRESH_REFERENCED", "This interface has historical use; create a new interface for the updated connector", gcode.CodeInvalidOperation)
