// Package backend wires the org-core source plugin into the host plugin registry.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/orgcap/orgspi"
	"lina-core/pkg/plugin/pluginhost"
	orgcenter "lina-plugin-org-core"
	deptcontroller "lina-plugin-org-core/backend/internal/controller/dept"
	postcontroller "lina-plugin-org-core/backend/internal/controller/post"
	"lina-plugin-org-core/backend/internal/provider/orgcapadapter"
	deptsvc "lina-plugin-org-core/backend/internal/service/dept"
	postsvc "lina-plugin-org-core/backend/internal/service/post"
)

// org-core plugin constants.
const (
	// pluginID is the immutable identifier published by the embedded source plugin.
	pluginID = "org-core"
)

// init registers the org-core source plugin and its host callbacks.
func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(orgcenter.EmbeddedFiles)
	if err := plugin.Providers().ProvideOrg(provideOrg); err != nil {
		panic(err)
	}
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

// provideOrg creates the org-core organization capability adapter from
// host-published services during framework capability activation.
func provideOrg(_ context.Context, env orgspi.ProviderEnv) (orgspi.Provider, error) {
	if env.Tenant == nil || env.Tenant.Filter() == nil {
		return nil, gerror.New("org-core provider requires host tenant-filter service")
	}
	if env.Users == nil {
		return nil, gerror.New("org-core provider requires host user capability service")
	}
	return orgcapadapter.New(env.Tenant, env.Users), nil
}

// registerRoutes binds department and post management routes through the published host middleware set.
func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	var (
		routes      = registrar.Routes()
		middlewares = routes.Middlewares()
		services    = registrar.Services()
	)
	if services == nil {
		return gerror.New("org-core routes require host i18n, tenant-filter, and user capability services")
	}
	tenantSvc := services.Tenant()
	if services.I18n() == nil || tenantSvc == nil || tenantSvc.Filter() == nil || services.Users() == nil {
		return gerror.New("org-core routes require host i18n, tenant-filter, and user capability services")
	}
	deptSvc := deptsvc.New(tenantSvc, services.Users())
	postSvc := postsvc.New(services.I18n(), tenantSvc)
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Middleware(
				middlewares.NeverDoneCtx(),
				middlewares.HandlerResponse(),
				middlewares.CORS(),
				middlewares.RequestBodyLimit(),
				middlewares.Ctx(),
			)
			group.Group("/", func(group pluginhost.RouteGroup) {
				group.Middleware(
					middlewares.Auth(),
					middlewares.Tenancy(),
					middlewares.Permission(),
				)
				group.Bind(
					deptcontroller.NewV1(deptSvc),
					postcontroller.NewV1(postSvc),
				)
			})
		})
	})
	return nil
}
