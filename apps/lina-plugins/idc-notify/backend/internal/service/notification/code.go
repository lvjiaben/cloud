package notification

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodePasswordChanged              = bizerr.MustDefine("IDC_NOTIFY_PASSWORD_CHANGED", "Your password has changed; please sign in again", gcode.CodeNotAuthorized)
	CodeContactManualVerification    = bizerr.MustDefine("IDC_NOTIFY_CONTACT_MANUAL_VERIFICATION", "No existing contact is available for verification; submit a support ticket", gcode.CodeInvalidOperation)
	CodeInvalid                      = bizerr.MustDefine("IDC_NOTIFY_INVALID", "The notification request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible                = bizerr.MustDefine("IDC_NOTIFY_NOT_ACCESSIBLE", "The notification resource is not accessible", gcode.CodeNotAuthorized)
	CodeRecentAuthenticationRequired = bizerr.MustDefine("IDC_NOTIFY_RECENT_AUTH_REQUIRED", "A recent full sign-in is required", gcode.CodeNotAuthorized)
	CodeConflict                     = bizerr.MustDefine("IDC_NOTIFY_CONFLICT", "The notification resource has changed", gcode.CodeInvalidOperation)
	CodeChannelUnavailable           = bizerr.MustDefine("IDC_NOTIFY_CHANNEL_UNAVAILABLE", "The notification channel is unavailable", gcode.CodeInvalidOperation)
	CodeHumanVerificationUnavailable = bizerr.MustDefine("IDC_NOTIFY_HUMAN_VERIFICATION_UNAVAILABLE", "SMS verification is unavailable until the host human-verification service is configured", gcode.CodeNotSupported)
	CodeUnavailable                  = bizerr.MustDefine("IDC_NOTIFY_UNAVAILABLE", "The notification service is temporarily unavailable", gcode.CodeInternalError)
	CodeVerificationInvalid          = bizerr.MustDefine("IDC_NOTIFY_VERIFICATION_INVALID", "The verification code or proof is invalid", gcode.CodeInvalidParameter)
	CodeVerificationExpired          = bizerr.MustDefine("IDC_NOTIFY_VERIFICATION_EXPIRED", "The verification code has expired", gcode.CodeInvalidOperation)
	CodeVerificationRateLimited      = bizerr.MustDefine("IDC_NOTIFY_VERIFICATION_RATE_LIMITED", "Too many verification attempts; try again later", gcode.CodeOperationFailed)
	CodeVerificationResultUnknown    = bizerr.MustDefine("IDC_NOTIFY_VERIFICATION_RESULT_UNKNOWN", "The provider result is unknown; request a new code after the displayed wait", gcode.CodeOperationFailed)
)
