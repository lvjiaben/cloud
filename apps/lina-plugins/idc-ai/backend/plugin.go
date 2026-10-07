// Package backend registers the IDC AI source plugin with the host.
package backend

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/pluginhost"
	idcai "lina-plugin-idc-ai"
	admincontroller "lina-plugin-idc-ai/backend/internal/controller/admin"
	customercontroller "lina-plugin-idc-ai/backend/internal/controller/customer"
	"lina-plugin-idc-ai/backend/internal/service/copilot"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	customerauthspi "lina-plugin-idc-core/backend/cap/customerauthcap/spi"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-ai"

// init registers the embedded plugin whose read-only dependencies are declared in its manifest.
func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(idcai.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister, pluginhost.CallbackExecutionModeBlocking, registerRoutes); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("idc-ai routes require host services")
	}
	service := copilot.New(g.DB(), registrar.Services().I18n())
	customerSnapshot, err := customerauthspi.Freeze()
	if err != nil {
		return err
	}
	customerFactory := customerSnapshot.Factory()
	if customerFactory == nil {
		return gerror.New("idc-ai current customer source factory is unavailable")
	}
	customers, err := customerFactory(
		ctx,
		registrar.Services(),
		customerauthcap.AllowedAICallerPluginID,
	)
	if err != nil {
		return err
	}
	r := registrar.Routes()
	m := r.Middlewares()
	r.Group(r.APIPrefix(), func(g pluginhost.RouteGroup) {
		g.Group("/api/v1", func(g pluginhost.RouteGroup) {
			g.Group("", func(g pluginhost.RouteGroup) {
				g.Middleware(m.NeverDoneCtx(), m.HandlerResponse(), m.CORS(), m.Ctx(), m.Auth(), m.Tenancy(), m.Permission())
				g.Bind(admincontroller.NewV1(service))
			})
			g.Group("", func(g pluginhost.RouteGroup) {
				g.Middleware(m.NeverDoneCtx(), m.HandlerResponse(), m.CORS(), m.Ctx(), m.Auth(), m.Tenancy())
				g.Bind(customercontroller.NewV1(service, customers))
			})
		})
	})
	return r.Err()
}
