// This file installs the plugin-owned old-session denial guard using published
// authentication middleware. It never parses JWTs or modifies host routes.
package backend

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-notify/backend/internal/service/notification"
	"net/http"
	"path"
	"strings"
)

func registerRecoverySessionGuard(registrar pluginhost.HTTPRegistrar, service notification.Service) error {
	routes := registrar.Routes()
	m := routes.Middlewares()
	// Public entry points retain their normal guest behavior. Logout and narrow
	// cleanup remain usable by a denied session only to finish its own revocation.
	applies := func(r *ghttp.Request) bool {
		if r.Header.Get("Authorization") == "" {
			return false
		}
		return protectedRecoveryPath(r.URL.Path)
	}
	wrap := func(handler pluginhost.MiddlewareHandler) pluginhost.MiddlewareHandler {
		return func(r *ghttp.Request) {
			if applies(r) {
				handler(r)
			} else {
				r.Middleware.Next()
			}
		}
	}
	check := func(r *ghttp.Request) {
		if err := service.CheckRecoverySession(r.Context()); err != nil {
			r.Response.Header().Set("Cache-Control", "no-store")
			status := http.StatusServiceUnavailable
			if bizerr.Is(err, notification.CodePasswordChanged) {
				status = http.StatusUnauthorized
			}
			r.Response.WriteHeader(status)
			r.SetError(err)
			return
		}
		r.Middleware.Next()
	}
	for _, handler := range []pluginhost.MiddlewareHandler{m.HandlerResponse(), m.CORS(), m.RequestBodyLimit(), m.Ctx(), m.Auth(), m.Tenancy(), check} {
		if err := registrar.GlobalMiddlewares().Bind("/*", wrap(handler)); err != nil {
			return err
		}
	}
	return nil
}

// protectedRecoveryPath recognizes only canonical public namespaces. A resource
// named public or an encoded traversal cannot bypass old-session denial.
func protectedRecoveryPath(value string) bool {
	value = path.Clean(value)
	if strings.HasPrefix(value, "/api/v1/config/public/") {
		return false
	}
	parts := strings.SplitN(value, "/", 7)
	if len(parts) >= 6 && parts[1] == "x" && parts[3] == "api" && parts[4] == "v1" && parts[5] == "public" {
		return false
	}
	switch value {
	case "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/register", "/api/v1/auth/forget-password", "/api/v1/auth/reset-password", "/x/idc-notify/api/v1/account/password-reset/session-cleanups":
		return false
	default:
		return true
	}
}
