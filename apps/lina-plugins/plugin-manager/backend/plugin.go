// Package backend registers the built-in workspace through published plugin seams.
package backend

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
	pluginmanager "lina-plugin-plugin-manager"
	"lina-plugin-plugin-manager/backend/internal/controller/management"
	"lina-plugin-plugin-manager/backend/internal/hostcatalog"
	"lina-plugin-plugin-manager/backend/internal/service/catalog"
)

// pluginID is the stable owner of the replacement workspace.
const pluginID = "plugin-manager"

// init is the top-level static registration boundary for the built-in plugin.
func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(pluginmanager.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister, pluginhost.CallbackExecutionModeBlocking, registerRoutes); err != nil {
		panic(err)
	}
	if err := plugin.Access().RegisterMenuFilter(pluginhost.ExtensionPointMenuFilter, pluginhost.CallbackExecutionModeBlocking, filterMenu); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

// filterMenu hides only the original navigation entry; permissions remain unchanged.
func filterMenu(_ context.Context, menu pluginhost.MenuDescriptor) (bool, error) {
	return menu.MenuKey() != "extension:plugin:list", nil
}

// registerRoutes owns the immutable category config and one stateless public HTTP reader.
func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if registrar == nil || registrar.Services() == nil || registrar.Routes() == nil || registrar.Routes().Middlewares() == nil {
		return gerror.New("plugin manager requires host registrar and published services")
	}
	var config catalog.Config
	if err := registrar.Services().Manifest().Scan(ctx, "config/config.yaml", "catalog", &config); err != nil {
		return err
	}
	if err := registrar.Services().Plugins().Config().Scan(ctx, "catalog", &config); err != nil {
		return err
	}
	service, err := catalog.New(config, &hostcatalog.Reader{})
	if err != nil {
		return err
	}
	routes := registrar.Routes()
	middleware := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Middleware(middleware.NeverDoneCtx(), middleware.HandlerResponse(), middleware.CORS(), middleware.RequestBodyLimit(), middleware.Ctx(), middleware.Auth(), middleware.Tenancy(), middleware.Permission())
			group.Bind(management.NewV1(service))
		})
	})
	return routes.Err()
}
