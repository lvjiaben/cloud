// This file defines safe errors for merchant payment routing settings.
package channelconfig

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodePublicAddressForbidden    = bizerr.MustDefine("IDC_PAYMENT_PUBLIC_ADDRESS_FORBIDDEN", "Only directly authenticated staff can change payment addresses", gcode.CodeNotAuthorized)
	CodePublicAddressInvalid      = bizerr.MustDefine("IDC_PAYMENT_PUBLIC_ADDRESS_INVALID", "Enter a valid HTTPS notification origin and a valid customer return origin", gcode.CodeInvalidParameter)
	CodePublicAddressConflict     = bizerr.MustDefine("IDC_PAYMENT_PUBLIC_ADDRESS_CONFLICT", "Payment addresses changed; reload before saving", gcode.CodeBusinessValidationFailed)
	CodePublicAddressUnavailable  = bizerr.MustDefine("IDC_PAYMENT_PUBLIC_ADDRESS_UNAVAILABLE", "Payment address settings are temporarily unavailable", gcode.CodeInternalError)
	CodePublicAddressUnconfigured = bizerr.MustDefine("IDC_PAYMENT_PUBLIC_ADDRESS_UNCONFIGURED", "Configure payment notification and customer return addresses before accepting online payments", gcode.CodeBusinessValidationFailed)
)
