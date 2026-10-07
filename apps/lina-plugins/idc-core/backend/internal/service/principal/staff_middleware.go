package principal

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
)

// StaffMiddleware establishes the plugin-owned staff audience only after the
// host authentication, tenancy, permission, and data-scope middleware passed.
type StaffMiddleware struct {
	service Service
}

// NewStaffMiddleware constructs the admin-route audience guard.
func NewStaffMiddleware(service Service) *StaffMiddleware {
	return &StaffMiddleware{service: service}
}

// Middleware idempotently establishes the current direct staff audience.
func (m *StaffMiddleware) Middleware(request *ghttp.Request) {
	if request == nil {
		return
	}
	if m == nil || m.service == nil {
		request.SetError(gerror.New("IDC staff audience middleware is unavailable"))
		return
	}
	if _, err := m.service.EnsureCurrentStaff(request.Context()); err != nil {
		request.SetError(err)
		return
	}
	request.Middleware.Next()
}
