// Privacy governance code helpers generate opaque identifiers and stable digests. Raw retry keys and sensitive values must never be persisted.

package privacygovernance

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_INVALID",
		"The privacy governance request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_NOT_ACCESSIBLE",
		"The privacy governance record is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_CONFLICT",
		"The privacy governance record changed; reload and try again",
		gcode.CodeInvalidOperation,
	)
	CodePolicyRequired = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_POLICY_REQUIRED",
		"A published privacy assessment and retention policy are required before collection",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_UNAVAILABLE",
		"The privacy governance service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
