// This file defines stable tax-invoice management business errors.
package taxinvoicemanagement

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_MANAGEMENT_INVALID",
		"The tax invoice management request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_MANAGEMENT_NOT_ACCESSIBLE",
		"The tax invoice request is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_MANAGEMENT_CONFLICT",
		"The tax invoice request conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_MANAGEMENT_UNAVAILABLE",
		"The tax invoice management service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
