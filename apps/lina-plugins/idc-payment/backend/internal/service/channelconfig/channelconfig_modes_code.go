// This file defines localized errors for payment mode editing.
package channelconfig

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeChannelModesInvalid     = bizerr.MustDefine("IDC_PAYMENT_CHANNEL_MODES_INVALID", "Choose supported payment modes for this channel", gcode.CodeInvalidParameter)
	CodeChannelModesConflict    = bizerr.MustDefine("IDC_PAYMENT_CHANNEL_MODES_CONFLICT", "Payment channel changed; reload before saving", gcode.CodeBusinessValidationFailed)
	CodeChannelModesUnavailable = bizerr.MustDefine("IDC_PAYMENT_CHANNEL_MODES_UNAVAILABLE", "Payment channel settings are temporarily unavailable", gcode.CodeInternalError)
	CodeChannelModesForbidden   = bizerr.MustDefine("IDC_PAYMENT_CHANNEL_MODES_FORBIDDEN", "Only directly authenticated staff can change payment modes", gcode.CodeNotAuthorized)
)
