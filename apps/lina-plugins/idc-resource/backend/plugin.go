// Package backend wires the idc-resource source plugin into the host registry.
package backend

import (
	"context"
	"sync/atomic"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	customer360spi "lina-plugin-idc-core/backend/cap/customer360cap/spi"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	customerauthspi "lina-plugin-idc-core/backend/cap/customerauthcap/spi"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	entitlementcreatedspi "lina-plugin-idc-core/backend/cap/entitlementcreatedcap/spi"
	lifecycleresource "lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	lifecyclespi "lina-plugin-idc-core/backend/cap/lifecycleresourcecap/spi"
	"lina-plugin-idc-core/backend/cap/managementcap"
	managementspi "lina-plugin-idc-core/backend/cap/managementcap/spi"
	poolsource "lina-plugin-idc-core/backend/cap/poolsourcecap"
	poolspi "lina-plugin-idc-core/backend/cap/poolsourcecap/spi"
	provisionresource "lina-plugin-idc-core/backend/cap/provisionresourcecap"
	provisionspi "lina-plugin-idc-core/backend/cap/provisionresourcecap/spi"
	renewalowner "lina-plugin-idc-core/backend/cap/renewalownercap"
	renewalspi "lina-plugin-idc-core/backend/cap/renewalownercap/spi"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	resourceinterfaceprojectionspi "lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap/spi"
	"lina-plugin-idc-core/backend/cap/secretcap"
	serveroperationresource "lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	serveroperationspi "lina-plugin-idc-core/backend/cap/serveroperationresourcecap/spi"
	idcresource "lina-plugin-idc-resource"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/composition"
	accountcontroller "lina-plugin-idc-resource/backend/internal/controller/account"
	admincontroller "lina-plugin-idc-resource/backend/internal/controller/admin"
	serverpagecontroller "lina-plugin-idc-resource/backend/internal/controller/serverpage"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"lina-plugin-idc-resource/backend/internal/service/batchrenewal"
	"lina-plugin-idc-resource/backend/internal/service/channelinvocation"
	"lina-plugin-idc-resource/backend/internal/service/channelsecret"
	"lina-plugin-idc-resource/backend/internal/service/customer360summary"
	"lina-plugin-idc-resource/backend/internal/service/interfacecandidate"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
	"lina-plugin-idc-resource/backend/internal/service/lifecyclesource"
	"lina-plugin-idc-resource/backend/internal/service/managedparameters"
	"lina-plugin-idc-resource/backend/internal/service/poolcandidate"
	"lina-plugin-idc-resource/backend/internal/service/principalprovision"
	"lina-plugin-idc-resource/backend/internal/service/providercapacity"
	"lina-plugin-idc-resource/backend/internal/service/providerconfiguration"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
	"lina-plugin-idc-resource/backend/internal/service/providerverification"
	"lina-plugin-idc-resource/backend/internal/service/provisionsource"
	"lina-plugin-idc-resource/backend/internal/service/renewalsource"
	"lina-plugin-idc-resource/backend/internal/service/resourceoverview"
	"lina-plugin-idc-resource/backend/internal/service/resourceprojection"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"lina-plugin-idc-resource/backend/internal/service/serverdirectory"
	"lina-plugin-idc-resource/backend/internal/service/serveroperationsource"
	"lina-plugin-idc-resource/backend/internal/service/serverownership"
	"lina-plugin-idc-resource/backend/internal/service/serverprojection"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-resource"

var resourceInterfaceCandidates = interfacecandidate.NewLazy()
var managedProvisionParameters = managedparameters.NewLazy()
var configuredResourcePublicOrigin atomic.Value

// init registers the embedded resource source plugin with the host.
func init() {
	if err := resourceinterfaceprojectionspi.DeclareCatalogSource(
		func(_ context.Context, authorizer managementcap.Authorizer) (resourceinterfaceprojectioncap.CatalogSource, error) {
			return interfacecandidate.NewCatalogSource(resourceInterfaceCandidates, authorizer)
		},
	); err != nil {
		panic(err)
	}
	if err := resourceinterfaceprojectionspi.DeclareCandidateSource(
		func(context.Context) (resourceinterfaceprojectioncap.CandidateSource, error) {
			return resourceInterfaceCandidates, nil
		},
	); err != nil {
		panic(err)
	}
	if err := entitlementcreatedspi.Register(
		serverownership.EntitlementOwnerID,
		func(context.Context) (entitlementcreatedcap.Sink, error) {
			return serverownership.NewEntitlementCreatedSink(dao.Task.DB())
		},
	); err != nil {
		panic(err)
	}
	customer360Contributor, err := customer360summary.NewLazy(func() gdb.DB { return dao.Mapping.DB() })
	if err != nil {
		panic(err)
	}
	if err = customer360spi.Declare(customer360Contributor); err != nil {
		panic(err)
	}
	if err := poolspi.Declare(func(context.Context) (poolsource.Source, error) {
		return poolcandidate.New(dao.Pool.DB())
	}); err != nil {
		panic(err)
	}
	if err := renewalspi.Register(renewalsource.OwnerID, func(context.Context) (renewalowner.Source, error) {
		return renewalsource.New(dao.Task.DB())
	}); err != nil {
		panic(err)
	}
	if err := provisionspi.Declare(func(ctx context.Context) (provisionresource.Source, error) {
		providerSnapshot, sourceErr := providerspi.Freeze()
		if sourceErr != nil {
			return nil, sourceErr
		}
		secrets, sourceErr := composition.LoadSecretSource(ctx)
		if sourceErr != nil {
			return nil, sourceErr
		}
		principal, sourceErr := principalprovision.New(
			dao.Task.DB(),
			func(channelNo string) (secretcap.Manager, error) {
				return composition.BindProviderSecretManager(secrets, channelNo)
			},
		)
		if sourceErr != nil {
			return nil, sourceErr
		}
		source, sourceErr := provisionsource.New(
			dao.Task.DB(), secrets, principal.EnsureForPool, providerSnapshot.Declarations(),
			managedProvisionParameters,
		)
		if sourceErr != nil {
			return nil, sourceErr
		}
		source.SetPublicOriginSource(configuredPublicOrigin)
		return source, nil
	}); err != nil {
		panic(err)
	}
	if err := lifecyclespi.Declare(func(context.Context) (lifecycleresource.Source, error) {
		return lifecyclesource.New(dao.Task.DB())
	}); err != nil {
		panic(err)
	}
	if err := serveroperationspi.Declare(func(ctx context.Context) (serveroperationresource.Source, error) {
		secrets, secretErr := composition.LoadSecretSource(ctx)
		if secretErr != nil {
			return nil, secretErr
		}
		return serveroperationsource.NewWithSecrets(dao.Task.DB(), secrets)
	}); err != nil {
		panic(err)
	}
	declarations := pluginhost.NewDeclarations(pluginID)
	declarations.Assets().UseEmbeddedFiles(idcresource.EmbeddedFiles)
	if err := declarations.BusinessExtensions().RegisterProviderValidator(
		resourcecap.DynamicProviderPoint,
		resourcecap.DynamicProviderContractVersion,
		resourcecap.ValidateDynamicProviderDeclarations,
	); err != nil {
		panic(err)
	}
	resourceInvoker := resourceprojection.NewCapabilityInvoker()
	resourceDescriptor, err := resourcecap.NewReadDescriptor(resourceInvoker)
	if err != nil {
		panic(err)
	}
	if err = declarations.Providers().ProvideCapability(resourceDescriptor); err != nil {
		panic(err)
	}
	if err := declarations.HTTP().RegisterRoutes(
		pluginhost.ExtensionPointHTTPRouteRegister,
		pluginhost.CallbackExecutionModeBlocking,
		func(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
			return registerRoutes(ctx, registrar, resourceInvoker)
		},
	); err != nil {
		panic(err)
	}
	if err := declarations.Jobs().RegisterJobs(
		pluginhost.ExtensionPointJobsRegister,
		pluginhost.CallbackExecutionModeBlocking,
		registerJobs,
	); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(declarations); err != nil {
		panic(err)
	}
}

func configuredPublicOrigin() string {
	value := configuredResourcePublicOrigin.Load()
	if origin, ok := value.(string); ok {
		return origin
	}
	return ""
}

func registerJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	if registrar == nil || registrar.BusinessExtensions() == nil {
		return gerror.New("resource background jobs require the owner-bound business extension facade")
	}
	declarations, err := providerspi.Freeze()
	if err != nil {
		return gerror.Wrap(err, "freeze resource Provider declarations")
	}
	dynamicSecrets, err := composition.LoadSecretSource(ctx)
	if err != nil {
		return err
	}
	if err = managedProvisionParameters.Bind(dao.Channel.DB(), registrar.BusinessExtensions()); err != nil {
		return err
	}
	projectionFactory := resourceinterfaceprojectionspi.Freeze().SinkFactory()
	if projectionFactory == nil {
		return gerror.New("resource interface projection sink is unavailable")
	}
	projectionSink, err := projectionFactory(ctx)
	if err != nil {
		return gerror.Wrap(err, "construct resource interface projection sink")
	}
	projectionWorker, err := interfaceprojection.NewWorker(dao.Channel.DB(), projectionSink)
	if err != nil {
		return err
	}
	invocationLeases, err := channelinvocation.New(dao.Channel.DB())
	if err != nil {
		return err
	}
	runtime, err := composition.NewProductionProviderRuntime(
		ctx,
		declarations.Declarations(),
		composition.WithDynamicBusinessExtensions(
			registrar.BusinessExtensions(),
			composition.NewProductionDynamicOutboundFactory(dynamicSecrets, invocationLeases),
		),
	)
	if err != nil {
		return err
	}
	taskStore, err := resourcetask.NewStore(dao.Task.DB())
	if err != nil {
		return err
	}
	healthService := providerhealth.New()
	worker, err := providercapacity.NewWorker(
		dao.ProviderCapacityObservation.DB(), runtime, healthService,
	)
	if err != nil {
		return err
	}
	accessoryWorker, err := accessorydirectory.NewWorker(dao.Task.DB(), runtime)
	if err != nil {
		return err
	}
	taskExecutor, err := resourcetask.NewExecutor(
		taskStore, runtime.ConstructRevisionSession, taskStore.ResolveResources, healthService,
	)
	if err != nil {
		return err
	}
	principalWorker, err := principalprovision.NewWorker(
		dao.Task.DB(),
		func(channelNo string) (secretcap.Manager, error) {
			return composition.BindProviderSecretManager(dynamicSecrets, channelNo)
		},
		runtime.ConstructSession,
	)
	if err != nil {
		return err
	}
	return composition.RegisterResourceJobs(ctx, registrar, worker, accessoryWorker, taskExecutor, principalWorker, projectionWorker)
}

func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar, resourceInvoker *resourceprojection.CapabilityInvoker) error {
	if ctx == nil || ctx.Err() != nil || registrar.Services() == nil {
		return gerror.New("idc-resource management routes require host services")
	}
	publicOrigin, originErr := registrar.Services().HostConfig().String(ctx, "idc.resource.public-origin", "https://localhost.invalid")
	if originErr != nil {
		return gerror.Wrap(originErr, "read idc-resource public origin")
	}
	configuredResourcePublicOrigin.Store(publicOrigin)
	providerDeclarations, err := validateHTTPProviderRuntime(ctx)
	if err != nil {
		return err
	}
	snapshot, err := managementspi.Freeze()
	if err != nil {
		return err
	}
	factory := snapshot.Factory()
	if factory == nil {
		return gerror.New("IDC management runtime factory is unavailable")
	}
	runtime, err := factory(ctx, registrar.Services())
	if err != nil {
		return err
	}
	management, err := providercapacity.NewManagement(dao.ProviderCapacityObservation.DB(), runtime)
	if err != nil {
		return err
	}
	healthManagement, err := providerhealth.NewManagement(dao.Channel.DB(), runtime)
	if err != nil {
		return err
	}
	taskStore, err := resourcetask.NewStore(dao.Task.DB())
	if err != nil {
		return err
	}
	taskManagement, err := resourcetask.NewManagement(taskStore, runtime)
	if err != nil {
		return err
	}
	secretSource, err := composition.LoadSecretSource(ctx)
	if err != nil {
		return err
	}
	if err = bindResourceInterfaceCandidates(registrar, secretSource); err != nil {
		return err
	}
	verificationLeases, err := channelinvocation.New(dao.Channel.DB())
	if err != nil {
		return err
	}
	verificationProviders, err := composition.NewProductionProviderRuntime(
		ctx,
		providerDeclarations,
		composition.WithDynamicBusinessExtensions(
			registrar.BusinessExtensions(),
			composition.NewProductionDynamicOutboundFactory(secretSource, verificationLeases),
		),
	)
	if err != nil {
		return err
	}
	verification, err := providerverification.New(runtime, verificationProviders, providerhealth.New())
	if err != nil {
		return err
	}
	secretManagement, err := channelsecret.New(
		dao.ChannelSecretBinding.DB(), runtime,
		func(channelNo string) (secretcap.Manager, error) {
			return composition.BindProviderSecretManager(secretSource, channelNo)
		},
	)
	if err != nil {
		return err
	}
	overview, err := resourceoverview.New(dao.Task.DB(), runtime)
	if err != nil {
		return err
	}
	usageFactory := resourceinterfaceprojectionspi.Freeze().UsageReaderFactory()
	if usageFactory == nil {
		return gerror.New("resource interface usage factory is unavailable")
	}
	usageReader, err := usageFactory(ctx, runtime.Authorizer)
	if err != nil {
		return err
	}
	configurations, err := providerconfiguration.New(
		dao.Channel.DB(), runtime, providerDeclarations, usageReader, registrar.BusinessExtensions(),
	)
	if err != nil {
		return err
	}
	catalogSource, err := interfacecandidate.NewCatalogSource(resourceInterfaceCandidates, runtime.Authorizer)
	if err != nil {
		return err
	}
	serverRenewals, err := batchrenewal.New(dao.Task.DB(), runtime)
	if err != nil {
		return err
	}
	customerSnapshot, err := customerauthspi.Freeze()
	if err != nil {
		return err
	}
	customerFactory := customerSnapshot.Factory()
	if customerFactory == nil {
		return gerror.New("current customer source factory is unavailable")
	}
	customers, err := customerFactory(ctx, registrar.Services(), customerauthcap.AllowedCallerPluginID)
	if err != nil {
		return err
	}
	serverDirectory, err := serverdirectory.New(dao.Task.DB())
	if err != nil {
		return err
	}
	currentCustomerServers, err := serverprojection.New(customers, serverDirectory)
	if err != nil {
		return err
	}
	accountController := accountcontroller.NewV1(currentCustomerServers)
	serverPageController, err := serverpagecontroller.NewV1(accountController)
	if err != nil {
		return err
	}
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(
					middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy(),
				)
				group.Bind(accountController)
			})
			bindCustomerServerPageRoutes(group, middlewares, customers, serverPageController)
			group.Middleware(
				middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
				middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(),
				middlewares.Tenancy(), middlewares.Permission(),
			)
			group.Bind(admincontroller.NewV1(
				management, healthManagement, taskManagement, secretManagement, overview, configurations, verification, serverRenewals, catalogSource,
			))
		})
	})
	if err = routes.Err(); err != nil {
		return err
	}
	return bindResourceReadCapability(resourceInvoker, currentCustomerServers)
}

func bindResourceInterfaceCandidates(
	registrar pluginhost.HTTPRegistrar,
	secretSource secretcap.Source,
) error {
	if registrar.BusinessExtensions() == nil {
		return gerror.New("resource interface candidates require the owner-bound business extension facade")
	}
	invocationLeases, err := channelinvocation.New(dao.Channel.DB())
	if err != nil {
		return err
	}
	return resourceInterfaceCandidates.Bind(
		dao.Channel.DB(), registrar.BusinessExtensions(),
		composition.NewProductionDynamicOutboundFactory(secretSource, invocationLeases),
	)
}

// validateHTTPProviderRuntime makes the HTTP and Jobs startup paths consume
// the same immutable Provider declaration snapshot. It constructs no channel
// client and performs no upstream call.
func validateHTTPProviderRuntime(ctx context.Context) ([]providerspi.Declaration, error) {
	providerDeclarations, err := providerspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze resource Provider declarations")
	}
	_, err = composition.NewProductionProviderRuntime(ctx, providerDeclarations.Declarations())
	if err != nil {
		return nil, err
	}
	return providerDeclarations.Declarations(), nil
}

func bindResourceReadCapability(resourceInvoker *resourceprojection.CapabilityInvoker, currentCustomerServers resourcecap.CurrentCustomerServerService) error {
	capabilityDirectory, err := serveroperationsource.New(dao.Task.DB())
	if err != nil {
		return err
	}
	readService, err := resourceprojection.New(capabilityDirectory)
	if err != nil {
		return err
	}
	return resourceInvoker.Bind(readService, currentCustomerServers)
}
