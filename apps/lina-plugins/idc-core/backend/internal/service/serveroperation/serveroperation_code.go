// This file maps internal server-operation sentinels to localized HTTP-safe errors.

package serveroperation

import (
	"errors"

	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

var (
	CodeInvalid = bizerr.MustDefine("IDC_SERVER_OPERATION_INVALID",
		"The server operation request is invalid", gcode.CodeInvalidParameter)
	CodeUnavailable = bizerr.MustDefine("IDC_SERVER_OPERATION_UNAVAILABLE",
		"The server operation service is temporarily unavailable", gcode.CodeInternalError)
	CodeActionUnavailable = bizerr.MustDefine("IDC_SERVER_OPERATION_ACTION_UNAVAILABLE",
		"This server operation is not available in the current state", gcode.CodeInvalidOperation)
	CodeConflict = bizerr.MustDefine("IDC_SERVER_OPERATION_CONFLICT",
		"Another server operation is already in progress or the state changed", gcode.CodeInvalidOperation)
	CodeOutputUnavailable = bizerr.MustDefine("IDC_SERVER_OPERATION_OUTPUT_UNAVAILABLE",
		"The confidential server operation result is unavailable", gcode.CodeNotFound)
	CodeOutputConsumed = bizerr.MustDefine("IDC_SERVER_OPERATION_OUTPUT_CONSUMED",
		"The confidential server operation result has already been viewed", gcode.CodeInvalidOperation)
	CodeResizeDiskShrink = bizerr.MustDefine("IDC_SERVER_RESIZE_DISK_SHRINK_NOT_ALLOWED",
		"Disk capacity cannot be reduced during a configuration change", gcode.CodeInvalidOperation)
)

// PublicError converts internal and resource-owner errors into stable,
// localized HTTP response metadata without exposing low-level details.
func PublicError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrInvalidInput):
		return bizerr.WrapCode(err, CodeInvalid)
	case errors.Is(err, ErrActionUnavailable):
		return bizerr.WrapCode(err, CodeActionUnavailable)
	case errors.Is(err, ErrOperationConflict), errors.Is(err, serveroperationresourcecap.ErrTaskConflict):
		return bizerr.WrapCode(err, CodeConflict)
	case errors.Is(err, serveroperationresourcecap.ErrOutputConsumed):
		return bizerr.WrapCode(err, CodeOutputConsumed)
	case errors.Is(err, serveroperationresourcecap.ErrOutputUnavailable):
		return bizerr.WrapCode(err, CodeOutputUnavailable)
	default:
		return bizerr.WrapCode(err, CodeUnavailable)
	}
}
