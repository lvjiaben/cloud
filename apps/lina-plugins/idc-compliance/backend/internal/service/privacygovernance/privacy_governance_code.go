// Privacy governance code helpers generate opaque identifiers and stable digests. Raw retry keys and sensitive values must never be persisted.

package privacygovernance

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_INVALID",
		"The material retention request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_NOT_ACCESSIBLE",
		"The material retention record is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_CONFLICT",
		"The material retention record changed; reload and try again",
		gcode.CodeInvalidOperation,
	)
	CodePolicyRequired = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_POLICY_REQUIRED",
		"A current retention policy is required before collecting sensitive material",
		gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_PRIVACY_UNAVAILABLE",
		"The material retention service is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
