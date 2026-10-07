package channel

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	codePageInvalid = bizerr.MustDefine(
		"IDC_CHANNEL_PAGE_INVALID",
		"The channel pagination request is invalid",
		gcode.CodeInvalidParameter,
	)
	codeTokenNotAccepted = bizerr.MustDefine(
		"IDC_CHANNEL_TOKEN_NOT_ACCEPTED",
		"The channel credential is not accepted",
		gcode.CodeNotAuthorized,
	)
	codeLifecycleInvalid = bizerr.MustDefine(
		"IDC_CHANNEL_LIFECYCLE_INVALID",
		"The channel lifecycle request is invalid",
		gcode.CodeInvalidParameter,
	)
	codeWebhookNotFound = bizerr.MustDefine(
		"IDC_CHANNEL_WEBHOOK_NOT_FOUND",
		"The requested channel webhook event was not found",
		gcode.CodeNotFound,
	)
)
