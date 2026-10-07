// Package backend registers the staff-access source plugin.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/pluginhost"
	staffaccessplugin "lina-plugin-staff-access"
	admincontroller "lina-plugin-staff-access/backend/internal/controller/admin"
	"lina-plugin-staff-access/backend/internal/service/staffaccess"
)

const pluginID = "staff-access"

func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(staffaccessplugin.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister, pluginhost.CallbackExecutionModeBlocking, registerRoutes); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("staff access routes require host services")
	}
	services := registrar.Services()
	if services.BizCtx() == nil || services.Users() == nil || services.Auth() == nil || services.Auth().Authz() == nil || services.Sessions() == nil {
		return gerror.New("staff access routes require business context, users, authorization, and sessions")
	}
	service, err := staffaccess.New(g.DB(), services.BizCtx(), services.Users(), services.Auth().Authz(), services.Sessions())
	if err != nil {
		return err
	}
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(), middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy(), middlewares.Permission())
			group.Bind(admincontroller.NewV1(service))
		})
	})
	return routes.Err()
}
