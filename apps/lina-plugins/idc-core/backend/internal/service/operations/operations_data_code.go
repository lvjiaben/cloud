// This file keeps operating-data errors localized at the service boundary.

package operations

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	// CodeDataUnavailable hides internal query and configuration failures.
	CodeDataUnavailable = bizerr.MustDefine("IDC_OPERATIONS_DATA_UNAVAILABLE", "Operating data could not be loaded; please retry", gcode.CodeInternalError)
	// CodeDataInvalid rejects invalid metric, currency and paging inputs.
	CodeDataInvalid = bizerr.MustDefine("IDC_OPERATIONS_DATA_INVALID", "The operating data filter is invalid", gcode.CodeInvalidParameter)
	// CodeDataDenied prevents aggregates and records from escaping staff scope.
	CodeDataDenied = bizerr.MustDefine("IDC_OPERATIONS_DATA_DENIED", "You do not have access to these operating records", gcode.CodeNotAuthorized)
)

// dataError preserves governed errors and wraps internal failures.
func dataError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeDataUnavailable)
}
