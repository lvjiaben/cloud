package contractsigner

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid              = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_SIGNER_INVALID", "The contract signer request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible        = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_SIGNER_NOT_ACCESSIBLE", "The contract signer is not accessible", gcode.CodeNotAuthorized)
	CodeConflict             = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_SIGNER_CONFLICT", "The contract signer has changed", gcode.CodeInvalidOperation)
	CodeVerificationRequired = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_SIGNER_VERIFICATION_REQUIRED", "The contract signer must complete identity and phone verification", gcode.CodeInvalidOperation)
	CodeProviderUnavailable  = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_SIGNER_PROVIDER_UNAVAILABLE", "Contract signer identity verification is temporarily unavailable", gcode.CodeInternalError)
	CodeUnavailable          = bizerr.MustDefine("IDC_COMPLIANCE_CONTRACT_SIGNER_UNAVAILABLE", "The contract signer service is temporarily unavailable", gcode.CodeInternalError)
)
