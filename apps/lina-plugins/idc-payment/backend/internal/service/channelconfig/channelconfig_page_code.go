// This file defines localized errors for bounded administrative page reads.
// These codes do not authorize money movement or change channel state.

package channelconfig

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodePageInvalid rejects malformed numeric pagination before querying.
	CodePageInvalid = bizerr.MustDefine("IDC_PAYMENT_CHANNEL_PAGE_INVALID", "The payment page request is invalid", gcode.CodeInvalidParameter)
	// CodePageUnavailable hides internal query details while allowing retry.
	CodePageUnavailable = bizerr.MustDefine("IDC_PAYMENT_CHANNEL_PAGE_UNAVAILABLE", "The payment list is temporarily unavailable", gcode.CodeInternalError)
)
