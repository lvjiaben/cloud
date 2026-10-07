// This file defines stable customer tax-invoice request business errors.

package taxinvoicerequest

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports malformed or no-longer-eligible customer selections.
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_REQUEST_INVALID",
		"The tax invoice request is invalid or no longer eligible",
		gcode.CodeInvalidParameter,
	)
	// CodeConflict reports an idempotency key reused with different customer input.
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_REQUEST_CONFLICT",
		"The tax invoice request conflicts with an earlier submission",
		gcode.CodeBusinessValidationFailed,
	)
	// CodeUnavailable reports a local persistence or encrypted-snapshot failure.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_TAX_INVOICE_REQUEST_UNAVAILABLE",
		"The tax invoice request service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
