// This file defines stable localized currency failures; storage errors never reach clients.
package currency

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	codeDenied      = bizerr.MustDefine("IDC_CURRENCY_DENIED", "You do not have permission to configure merchant currencies", gcode.CodeNotAuthorized)
	codeInvalid     = bizerr.MustDefine("IDC_CURRENCY_INVALID", "Select a valid ISO currency and refresh the record before editing", gcode.CodeInvalidParameter)
	codeConflict    = bizerr.MustDefine("IDC_CURRENCY_CONFLICT", "This currency already exists or changed; refresh and try again", gcode.CodeInvalidOperation)
	codeDefault     = bizerr.MustDefine("IDC_CURRENCY_DEFAULT_REQUIRED", "Set another enabled currency as default before disabling or deleting this one", gcode.CodeInvalidOperation)
	codeUnavailable = bizerr.MustDefine("IDC_CURRENCY_UNAVAILABLE", "Currency configuration is unavailable; please retry", gcode.CodeInternalError)
)

func safeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, codeUnavailable)
}
