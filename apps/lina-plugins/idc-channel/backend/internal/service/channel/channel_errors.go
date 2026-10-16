package channel

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	codeCatalogUnavailable = bizerr.MustDefine("IDC_CHANNEL_CATALOG_UNAVAILABLE", "The product configuration is unavailable", gcode.CodeInternalError)
	// CodePurchaseInvalid rejects malformed customer purchase configuration.
	CodePurchaseInvalid = bizerr.MustDefine("IDC_CHANNEL_PURCHASE_INVALID", "The purchase configuration is invalid", gcode.CodeInvalidParameter)
	codePageInvalid     = bizerr.MustDefine(
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
