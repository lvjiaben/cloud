// This file defines stable, localized directory configuration and access errors.
package productdirectory

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var (
	CodeUnavailable   = bizerr.MustDefine("IDC_PRODUCT_DIRECTORY_UNAVAILABLE", "The product directory operation is temporarily unavailable", gcode.CodeInternalError)
	CodeInvalid       = bizerr.MustDefine("IDC_PRODUCT_DIRECTORY_INVALID", "The product directory configuration is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible = bizerr.MustDefine("IDC_PRODUCT_DIRECTORY_NOT_ACCESSIBLE", "The product directory is not accessible", gcode.CodeNotAuthorized)
	CodeConflict      = bizerr.MustDefine("IDC_PRODUCT_DIRECTORY_CONFLICT", "The directory changed or a product belongs to another directory; refresh and review its association", gcode.CodeInvalidOperation)
)

// directoryError keeps internal database and audit causes behind a localized boundary.
func directoryError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}
