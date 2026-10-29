// Package backend registers the shared-services source plugin.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	sharedservices "lina-plugin-shared-services"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
	"lina-plugin-shared-services/backend/internal/composition"
	"lina-plugin-shared-services/backend/internal/service/challenge"
	"lina-plugin-shared-services/backend/internal/service/inbox"
)

const pluginID = "shared-services"

var (
	challengeInvoker = challenge.NewInvoker()
	inboxInvoker     = inbox.NewInvoker()
)

func init() {
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(sharedservices.EmbeddedFiles)
	challengeDescriptor, err := challengecap.NewDescriptor(challengeInvoker)
	if err != nil {
		panic(err)
	}
	if err = plugin.Providers().ProvideCapability(challengeDescriptor); err != nil {
		panic(err)
	}
	inboxDescriptor, err := inboxcap.NewDescriptor(inboxInvoker)
	if err != nil {
		panic(err)
	}
	if err = plugin.Providers().ProvideCapability(inboxDescriptor); err != nil {
		panic(err)
	}
	if err = plugin.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister,
		pluginhost.CallbackExecutionModeBlocking, registerRuntime); err != nil {
		panic(err)
	}
	if err = plugin.Jobs().RegisterJobs(pluginhost.ExtensionPointJobsRegister,
		pluginhost.CallbackExecutionModeBlocking, registerJobs); err != nil {
		panic(err)
	}
	if err = pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

func registerRuntime(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("shared-services runtime requires host services")
	}
	runtime, err := composition.Build(registrar.Services().Notifications(), registrar.Services().BizCtx())
	if err != nil {
		return err
	}
	if err = challengeInvoker.Bind(runtime.Challenge); err != nil {
		return err
	}
	return inboxInvoker.Bind(runtime.Inbox)
}

func registerJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("shared-services jobs require host services")
	}
	runtime, err := composition.Build(registrar.Services().Notifications(), registrar.Services().BizCtx())
	if err != nil {
		return err
	}
	return composition.RegisterCleanupJob(ctx, registrar, runtime.Challenge)
}
