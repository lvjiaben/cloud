// This file defines stable business errors for supplier configuration operations.

package supplierconfig

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_INVALID",
		"The local supplier configuration request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_NOT_ACCESSIBLE",
		"The local supplier configuration is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeRecentAuthenticationRequired = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_RECENT_AUTH_REQUIRED",
		"A recent full sign-in is required before changing a local supplier credential or subject binding",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_CONFLICT",
		"The local supplier configuration conflicts with its current state",
		gcode.CodeInvalidOperation,
	)
	CodeCredentialUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_CREDENTIAL_UNAVAILABLE",
		"The local supplier credential is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeExternalEvidenceRequired = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_EXTERNAL_EVIDENCE_REQUIRED",
		"Independent supplier capability confirmation is required before this adapter can be enabled",
		gcode.CodeInvalidOperation,
	)
	CodeGatewayUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_GATEWAY_UNAVAILABLE",
		"The installation-local compliance gateway could not complete the supplier subject operation",
		gcode.CodeInternalError,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_SUPPLIER_UNAVAILABLE",
		"The local supplier configuration is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
