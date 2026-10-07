// This file defines stable localized business errors for personal verification.

package personalverification

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_INVALID",
		"The personal verification request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeUnsupportedDocument = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_DOCUMENT_UNSUPPORTED",
		"The submitted identity document is not supported",
		gcode.CodeInvalidParameter,
	)
	CodeAgeUnsupported = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_AGE_UNSUPPORTED",
		"The customer does not meet the supported verification age",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_NOT_ACCESSIBLE",
		"The personal verification case is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_CONFLICT",
		"Another personal verification attempt is already active",
		gcode.CodeInvalidOperation,
	)
	CodeProviderUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_PROVIDER_UNAVAILABLE",
		"Personal verification is temporarily unavailable",
		gcode.CodeInternalError,
	)
	CodeReviewRequired = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_REVIEW_REQUIRED",
		"The personal verification case requires merchant review",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_PERSONAL_VERIFICATION_UNAVAILABLE",
		"The personal verification service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
