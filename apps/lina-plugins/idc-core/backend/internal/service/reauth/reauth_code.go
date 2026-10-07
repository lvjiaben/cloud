// This file defines stable recent-authentication business error codes.

package reauth

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports malformed purpose, target binding, or proof encoding.
	CodeInvalid = bizerr.MustDefine(
		"IDC_REAUTH_INVALID",
		"The recent authentication request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeRecentLoginRequired asks the caller to complete the host login flow again.
	CodeRecentLoginRequired = bizerr.MustDefine(
		"IDC_REAUTH_RECENT_LOGIN_REQUIRED",
		"Please sign in again before performing this operation",
		gcode.CodeNotAuthorized,
	)
	// CodeProofRejected deliberately collapses expired, tampered, replayed, and
	// mismatched proofs so callers cannot use the response as an oracle.
	CodeProofRejected = bizerr.MustDefine(
		"IDC_REAUTH_PROOF_REJECTED",
		"The recent authentication proof is no longer valid",
		gcode.CodeNotAuthorized,
	)
	// CodeUnavailable fails dangerous operations closed when a runtime dependency fails.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_REAUTH_UNAVAILABLE",
		"Recent authentication is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
