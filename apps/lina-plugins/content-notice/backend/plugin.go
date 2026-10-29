// Package backend wires the content-notice source plugin into the host registry.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	contentnotice "lina-plugin-content-notice"
	noticecontroller "lina-plugin-content-notice/backend/internal/controller/notice"
	publicnoticecontroller "lina-plugin-content-notice/backend/internal/controller/publicnotice"
	noticeservice "lina-plugin-content-notice/backend/internal/service/notice"
)

const pluginID = "content-notice"

func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(contentnotice.EmbeddedFiles)
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

// registerRoutes binds notice management through the same authenticated,
// tenant-aware, permission-checked source-plugin boundary used by other built-ins.
func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("content-notice routes require host services")
	}
	services := registrar.Services()
	tenantService := services.Tenant()
	if services.BizCtx() == nil || services.Users() == nil ||
		tenantService == nil || tenantService.Filter() == nil {
		return gerror.New("content-notice routes require business-context, tenant-filter, and user services")
	}

	service := noticeservice.New(
		services.BizCtx(),
		tenantService,
		services.Users(),
	)
	publicController, err := publicnoticecontroller.NewV1WithService(service)
	if err != nil {
		return err
	}
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(), middlewares.RequestBodyLimit(), middlewares.Ctx())
			group.Bind(publicController)
		})
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Middleware(
				middlewares.NeverDoneCtx(),
				middlewares.HandlerResponse(),
				middlewares.CORS(),
				middlewares.RequestBodyLimit(),
				middlewares.Ctx(),
				middlewares.Auth(),
				middlewares.Tenancy(),
				middlewares.Permission(),
			)
			group.Bind(noticecontroller.NewV1WithService(service))
		})
	})
	return routes.Err()
}
