// This file centralizes internal connector failures that never expose raw supplier details.

package dcim

import "github.com/gogf/gf/v2/errors/gerror"

var (
	errInvalidConfiguration = gerror.New("DCIM connector configuration is invalid")
	errInvalidResponse      = gerror.New("DCIM supplier response is invalid")
	errUnsupported          = gerror.New("DCIM supplier operation is unsupported")
)
