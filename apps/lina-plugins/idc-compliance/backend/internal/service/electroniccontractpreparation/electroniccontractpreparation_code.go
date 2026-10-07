// This file defines stable contract-preparation business errors.

package electroniccontractpreparation

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_CUSTOMER_CONTRACT_INVALID",
		"The customer contract request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_CUSTOMER_CONTRACT_NOT_ACCESSIBLE",
		"The customer contract is not accessible",
		gcode.CodeNotFound,
	)
	CodeNotSignable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CUSTOMER_CONTRACT_NOT_SIGNABLE",
		"The customer contract cannot be signed or rechecked in its current state",
		gcode.CodeBusinessValidationFailed,
	)
	CodeProviderUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CUSTOMER_CONTRACT_PROVIDER_UNAVAILABLE",
		"The electronic signing service is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_CONTRACT_PREPARATION_UNAVAILABLE",
		"The electronic contract preparation service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
