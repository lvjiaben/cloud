// This file defines the localized rejection for malformed or non-EPay GET notifications.
package paymentflow

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// CodeEPayNotificationInvalid prevents GET from widening other provider contracts.
var CodeEPayNotificationInvalid = bizerr.MustDefine("IDC_PAYMENT_EPAY_NOTIFICATION_INVALID", "The EPay notification is invalid", gcode.CodeInvalidParameter)
