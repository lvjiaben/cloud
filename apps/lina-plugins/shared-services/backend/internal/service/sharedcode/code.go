// Package sharedcode owns stable business errors shared by the plugin services.
package sharedcode

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeSourceNotAuthorized  = bizerr.MustDefine("SHARED_SOURCE_NOT_AUTHORIZED", "The calling plugin is not authorized", gcode.CodeNotAuthorized)
	CodeChallengeInvalid     = bizerr.MustDefine("SHARED_CHALLENGE_INVALID", "The verification request is invalid", gcode.CodeInvalidParameter)
	CodeChallengeRejected    = bizerr.MustDefine("SHARED_CHALLENGE_REJECTED", "Verification failed", gcode.CodeNotAuthorized)
	CodeChallengeConflict    = bizerr.MustDefine("SHARED_CHALLENGE_CONFLICT", "The verification challenge has changed or was already used", gcode.CodeInvalidOperation)
	CodeChallengeUnavailable = bizerr.MustDefine("SHARED_CHALLENGE_UNAVAILABLE", "Human verification is temporarily unavailable", gcode.CodeInternalError)
	CodeInboxInvalid         = bizerr.MustDefine("SHARED_INBOX_INVALID", "The in-app message request is invalid", gcode.CodeInvalidParameter)
	CodeInboxConflict        = bizerr.MustDefine("SHARED_INBOX_CONFLICT", "The same message key was already used with different content", gcode.CodeInvalidOperation)
	CodeInboxUnavailable     = bizerr.MustDefine("SHARED_INBOX_UNAVAILABLE", "In-app messaging is temporarily unavailable", gcode.CodeInternalError)
)
