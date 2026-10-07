package finance

import "github.com/gogf/gf/v2/errors/gerror"

var (
	errInvalidConfig   = gerror.New("zjmf finance: provider configuration is invalid")
	errInvalidResponse = gerror.New("zjmf finance: upstream response is invalid")
	errUnsupported     = gerror.New("zjmf finance: capability is unsupported")
)
