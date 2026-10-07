// Package backend wires the idc-portal source plugin into LinaPro.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	idcportal "lina-plugin-idc-portal"
	admincontroller "lina-plugin-idc-portal/backend/internal/controller/admin"
	publiccontroller "lina-plugin-idc-portal/backend/internal/controller/public"
	"lina-plugin-idc-portal/backend/internal/portalroute"
	"lina-plugin-idc-portal/backend/internal/service/brand"
	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-portal"

// init registers the website-delivery resources and exact customer-entry bridge.
func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(idcportal.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(
		pluginhost.ExtensionPointHTTPRouteRegister,
		pluginhost.CallbackExecutionModeBlocking,
		registerRoutes,
	); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

// registerRoutes binds only the exact customer password-reset bridge.
func registerRoutes(_ context.Context, registrar pluginhost.HTTPRegistrar) error {
	if registrar == nil {
		return gerror.New("portal routes require host registrar")
	}
	brands, err := brand.New("")
	if err != nil {
		return err
	}
	externalMenus := externalmenu.New()
	if err := portalroute.Register(registrar); err != nil {
		return err
	}
	routes := registrar.Routes()
	if routes == nil || routes.Middlewares() == nil {
		return gerror.New("portal route registrar is unavailable")
	}
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(), middlewares.RequestBodyLimit(), middlewares.Ctx())
				group.Bind(publiccontroller.NewV1(brands, externalMenus))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(), middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy(), middlewares.Permission())
				group.Bind(admincontroller.NewV1(brands, externalMenus))
			})
		})
	})
	return routes.Err()
}
