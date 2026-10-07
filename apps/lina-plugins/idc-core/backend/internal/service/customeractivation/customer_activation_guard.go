// This file blocks ordinary customer APIs until a direct-created account replaces its initial password.

package customeractivation

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

// InitialPasswordGuard protects ordinary customer account routes.
type InitialPasswordGuard struct {
	service Service
}

// NewInitialPasswordGuard creates the fail-closed first-login route guard.
func NewInitialPasswordGuard(service Service) (*InitialPasswordGuard, error) {
	if service == nil {
		return nil, gerror.New("initial password guard requires customer activation service")
	}
	return &InitialPasswordGuard{service: service}, nil
}

// Middleware rejects ordinary customer API use while password replacement is required.
func (g *InitialPasswordGuard) Middleware(request *ghttp.Request) {
	if request == nil {
		return
	}
	required, err := g.service.InitialPasswordChangeRequired(request.Context())
	if err != nil {
		request.SetError(err)
		return
	}
	if required {
		request.SetError(bizerr.NewCode(customer.CodeInitialPasswordChangeRequired))
		return
	}
	request.Middleware.Next()
}
