// Package backend registers the IDC downstream-channel source plugin with the host.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/pluginhost"
	idcchannel "lina-plugin-idc-channel"
	accountcontroller "lina-plugin-idc-channel/backend/internal/controller/account"
	admincontroller "lina-plugin-idc-channel/backend/internal/controller/admin"
	channelcontroller "lina-plugin-idc-channel/backend/internal/controller/channel"
	channelservice "lina-plugin-idc-channel/backend/internal/service/channel"
	channelcommercespi "lina-plugin-idc-core/backend/cap/channelcommercecap/spi"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-channel"

// init registers the embedded plugin whose owner dependency is declared in its manifest.
func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(idcchannel.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister, pluginhost.CallbackExecutionModeBlocking, registerRoutes); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	services := registrar.Services()
	if services == nil || services.BizCtx() == nil {
		return gerror.New("channel routes require host business context")
	}
	snapshot, err := channelcommercespi.Freeze()
	if err != nil {
		return err
	}
	coreRuntime, err := snapshot.Factory()(ctx, services, pluginID)
	if err != nil {
		return err
	}
	channelService := channelservice.New(g.DB(), coreRuntime, services.BizCtx())
	channelCtrl := channelcontroller.NewV1(channelService)
	adminCtrl := admincontroller.NewV1(channelService)
	accountCtrl := accountcontroller.NewV1(channelService)
	tokenGuard := channelservice.NewTokenGuard(channelService)
	routes := registrar.Routes()
	mw := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(root pluginhost.RouteGroup) {
		root.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(mw.NeverDoneCtx(), mw.HandlerResponse(), mw.CORS(), mw.RequestBodyLimit(), mw.Ctx(), tokenGuard.Middleware)
				group.Bind(channelCtrl)
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(mw.NeverDoneCtx(), mw.HandlerResponse(), mw.CORS(), mw.RequestBodyLimit(), mw.Ctx())
				group.Middleware(mw.Auth(), mw.Tenancy(), mw.Permission())
				group.Bind(adminCtrl)
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(mw.NeverDoneCtx(), mw.HandlerResponse(), mw.CORS(), mw.RequestBodyLimit(), mw.Ctx())
				group.Middleware(mw.Auth(), mw.Tenancy())
				group.Bind(accountCtrl)
			})
		})
	})
	return routes.Err()
}
