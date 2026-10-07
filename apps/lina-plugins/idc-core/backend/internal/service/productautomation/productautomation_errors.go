// This file defines stable business error codes for permanent references and temporarily unavailable host evidence.

package productautomation

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeInterfaceEverReferenced = bizerr.MustDefine(
		"IDC_PRODUCT_AUTOMATION_INTERFACE_EVER_REFERENCED",
		"An automation interface referenced by a product version cannot be deleted",
		gcode.CodeInvalidOperation,
	)
	CodeEvidenceUnavailable = bizerr.MustDefine(
		"IDC_PRODUCT_AUTOMATION_EVIDENCE_UNAVAILABLE",
		"The automation provider evidence cannot be verified right now",
		gcode.CodeInternalError,
	)
)
