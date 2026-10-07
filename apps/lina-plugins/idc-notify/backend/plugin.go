// Package backend registers the IDC notification source plugin with the host.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/pluginhost"
	customer360spi "lina-plugin-idc-core/backend/cap/customer360cap/spi"
	idcnotify "lina-plugin-idc-notify"
	"lina-plugin-idc-notify/backend/internal/composition"
	accountcontroller "lina-plugin-idc-notify/backend/internal/controller/account"
	admincontroller "lina-plugin-idc-notify/backend/internal/controller/admin"
	callbackcontroller "lina-plugin-idc-notify/backend/internal/controller/callback"
	publiccontroller "lina-plugin-idc-notify/backend/internal/controller/public"
	staffcontroller "lina-plugin-idc-notify/backend/internal/controller/staff"
	customer360summarysvc "lina-plugin-idc-notify/backend/internal/service/customer360summary"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-notify"

// init registers the embedded plugin whose owner dependency is declared in its manifest.
func init() {
	customer360Contributor, err := customer360summarysvc.NewLazy(func() gdb.DB { return g.DB() })
	if err != nil {
		panic(err)
	}
	if err = customer360spi.Declare(customer360Contributor); err != nil {
		panic(err)
	}
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(idcnotify.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister,
		pluginhost.CallbackExecutionModeBlocking, registerRoutes); err != nil {
		panic(err)
	}
	if err := plugin.Jobs().RegisterJobs(pluginhost.ExtensionPointJobsRegister,
		pluginhost.CallbackExecutionModeBlocking, registerJobs); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("idc-notify routes require host services")
	}
	service, err := composition.BuildNotificationService(ctx, registrar.Services())
	if err != nil {
		return err
	}
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx())
				group.Bind(publiccontroller.NewV1(service))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.CORS(), middlewares.RequestBodyLimit())
				group.Bind(callbackcontroller.NewV1(service))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy())
				group.Bind(accountcontroller.NewV1(service))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy())
				group.Bind(staffcontroller.NewV1(service))
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy(), middlewares.Permission())
				group.Bind(admincontroller.NewV1(service))
			})
		})
	})
	return routes.Err()
}

func registerJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("idc-notify jobs require host services")
	}
	service, err := composition.BuildNotificationService(ctx, registrar.Services())
	if err != nil {
		return err
	}
	return composition.RegisterJobs(ctx, registrar, service)
}
