// Package profileguard prevents unverified customer login-contact changes through
// the host's generic self-profile route while IDC account ownership is enabled.
// It also keeps newly written usernames distinct from phone and email aliases.
package profileguard

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/i18ncap"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// Register composes published authentication middleware before the customer
// audience check. It does not replace host routes or write host user storage.
// Staff profile updates and existing password flows retain their host behavior.
func Register(registrar pluginhost.HTTPRegistrar, translations i18ncap.Service, audiences principal.Reader, bizctx bizctxcap.Service, plugins plugincap.StateService, verifier func() profilechangecap.Provider, directory customerlogincap.Directory, users usercap.Service) error {
	if registrar == nil || registrar.GlobalMiddlewares() == nil || registrar.Routes() == nil ||
		registrar.Routes().Middlewares() == nil || translations == nil || audiences == nil || bizctx == nil || plugins == nil || verifier == nil || directory == nil || users == nil {
		return gerror.New("profile contact guard requires HTTP, identity and translation services")
	}
	var (
		m          = registrar.Routes().Middlewares()
		protection = &guard{translations: translations, audiences: audiences, bizctx: bizctx, plugins: plugins, verifier: verifier, directory: directory, users: users}
		handlers   = []pluginhost.MiddlewareHandler{m.HandlerResponse(), m.CORS(), m.RequestBodyLimit(), m.Ctx(), m.Auth(), m.Tenancy(), protection.handle}
	)
	for _, handler := range handlers {
		if handler == nil {
			return gerror.New("profile contact guard authentication middleware is unavailable")
		}
	}
	// Creation only validates the public username policy. The original route
	// still performs authentication, permissions and the actual write.
	for _, handler := range []pluginhost.MiddlewareHandler{m.HandlerResponse(), m.CORS(), m.RequestBodyLimit(), m.Ctx(), protection.handleUsernameCreate} {
		if err := registrar.GlobalMiddlewares().Bind("/*", onlyUsernameCreate(handler)); err != nil {
			return err
		}
	}
	for _, handler := range handlers {
		if err := registrar.GlobalMiddlewares().Bind("/*", onlyProfileUpdate(handler)); err != nil {
			return err
		}
	}
	return nil
}

type guard struct {
	users        usercap.Service
	directory    customerlogincap.Directory
	plugins      plugincap.StateService
	verifier     func() profilechangecap.Provider
	translations i18ncap.Service
	audiences    principal.Reader
	bizctx       bizctxcap.Service
}
