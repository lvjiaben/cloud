package contractautosign

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid                      = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_AUTO_SIGN_INVALID", "The enterprise auto-sign authorization request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible                = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_AUTO_SIGN_NOT_ACCESSIBLE", "The enterprise auto-sign authorization is not accessible", gcode.CodeNotAuthorized)
	CodeRecentAuthenticationRequired = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_AUTO_SIGN_RECENT_AUTH_REQUIRED", "Please sign in again before enabling enterprise auto-sign", gcode.CodeNotAuthorized)
	CodeCapabilityUnavailable        = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_AUTO_SIGN_CAPABILITY_UNAVAILABLE", "Tencent enterprise auto-sign is not available for this installation", gcode.CodeNotSupported)
	CodeConflict                     = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_AUTO_SIGN_CONFLICT", "The enterprise auto-sign authorization conflicts with its current state", gcode.CodeInvalidOperation)
	CodeUnavailable                  = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_AUTO_SIGN_UNAVAILABLE", "The enterprise auto-sign authorization service is temporarily unavailable", gcode.CodeInternalError)
)
