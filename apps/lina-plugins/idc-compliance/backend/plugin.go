// Package backend registers the IDC compliance source plugin with the host.
package backend

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/pluginhost"
	idccompliance "lina-plugin-idc-compliance"
	accountapi "lina-plugin-idc-compliance/backend/api/account"
	adminapi "lina-plugin-idc-compliance/backend/api/admin"
	"lina-plugin-idc-compliance/backend/internal/composition"
	accountcontroller "lina-plugin-idc-compliance/backend/internal/controller/account"
	admincontroller "lina-plugin-idc-compliance/backend/internal/controller/admin"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-compliance/backend/internal/runtimeconfig"
	closureprechecksvc "lina-plugin-idc-compliance/backend/internal/service/closureprecheck"
	compliancecron "lina-plugin-idc-compliance/backend/internal/service/cron"
	customer360summarysvc "lina-plugin-idc-compliance/backend/internal/service/customer360summary"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoiceredletter"
	customer360spi "lina-plugin-idc-core/backend/cap/customer360cap/spi"
	closureprecheckspi "lina-plugin-idc-core/backend/cap/customerclosurecap/spi"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	managementspi "lina-plugin-idc-core/backend/cap/managementcap/spi"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-compliance"

// init registers the embedded plugin whose owner dependency is declared in its manifest.
func init() {
	customer360Contributor, err := customer360summarysvc.NewLazy(func() gdb.DB { return g.DB() })
	if err != nil {
		panic(err)
	}
	if err = customer360spi.Declare(customer360Contributor); err != nil {
		panic(err)
	}
	closureContributor, err := closureprechecksvc.NewLazy(func() gdb.DB { return g.DB() })
	if err != nil {
		panic(err)
	}
	if err = closureprecheckspi.Declare(closureContributor); err != nil {
		panic(err)
	}
	plugin := pluginhost.NewDeclarations(pluginID)
	plugin.Assets().UseEmbeddedFiles(idccompliance.EmbeddedFiles)
	if err := plugin.HTTP().RegisterRoutes(
		pluginhost.ExtensionPointHTTPRouteRegister,
		pluginhost.CallbackExecutionModeBlocking,
		registerRoutes,
	); err != nil {
		panic(err)
	}
	if err := plugin.Jobs().RegisterJobs(
		pluginhost.ExtensionPointJobsRegister,
		pluginhost.CallbackExecutionModeBlocking,
		registerJobs,
	); err != nil {
		panic(err)
	}
	if err := pluginhost.RegisterSourcePlugin(plugin); err != nil {
		panic(err)
	}
}

// registerJobs composes the fixed-system-audit recovery runtime on the primary node.
func registerJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || registrar.Services() == nil {
		return gerror.New("idc-compliance recovery jobs require host services")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	secretSource, err := composition.LoadSecretSource(ctx)
	if err != nil {
		return err
	}
	personalServices, err := composition.BuildPersonalVerificationServices(
		ctx, registrar.Services(), secretSource,
	)
	if err != nil {
		return err
	}
	if err = compliancecron.RegisterPersonalVerificationRecovery(
		ctx, registrar, personalServices.System,
	); err != nil {
		return err
	}
	identityRuntime, err := composition.LoadIdentityRuntime(ctx, registrar.Services())
	if err != nil {
		return err
	}
	managementSnapshot, err := managementspi.Freeze()
	if err != nil {
		return err
	}
	managementFactory := managementSnapshot.Factory()
	if managementFactory == nil {
		return gerror.New("IDC management runtime factory is unavailable for privacy deletion")
	}
	managementRuntime, err := managementFactory(ctx, registrar.Services())
	if err != nil {
		return err
	}
	customerReader, ok := identityRuntime.Identity.(identitycap.CurrentCustomerReader)
	if !ok {
		return gerror.New("current customer reader is unavailable for privacy deletion")
	}
	privacyConsents, ok := identityRuntime.Identity.(identitycap.PrivacyConsentService)
	if !ok {
		return gerror.New("privacy consent service is unavailable for privacy deletion")
	}
	_, snapshotManager, err := composition.BindPersonalIdentitySecrets(secretSource)
	if err != nil {
		return err
	}
	privacyService, err := privacygovernance.NewComplete(g.DB(), managementRuntime, customerReader, privacyConsents, identityRuntime.CustomerAudit, registrar.Services().Storage(), snapshotManager)
	if err != nil {
		return err
	}
	if err = compliancecron.RegisterPrivacyDeletion(ctx, registrar, privacyService); err != nil {
		return err
	}
	if err = registerTaxInvoiceRedLetterJob(ctx, registrar, identityRuntime); err != nil {
		return err
	}
	return nil
}

// registerTaxInvoiceRedLetterJob composes the shared core runtimes for refund intake.
func registerTaxInvoiceRedLetterJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	identityRuntime identitycap.Runtime,
) error {
	customerReader, ok := identityRuntime.Identity.(identitycap.CurrentCustomerReader)
	if !ok {
		return gerror.New("current customer red-letter identity reader is unavailable")
	}
	managementSnapshot, err := managementspi.Freeze()
	if err != nil {
		return gerror.Wrap(err, "freeze IDC management runtime declaration for red-letter intake")
	}
	managementFactory := managementSnapshot.Factory()
	if managementFactory == nil {
		return gerror.New("IDC management runtime factory is unavailable for red-letter intake")
	}
	managementRuntime, err := managementFactory(ctx, registrar.Services())
	if err != nil {
		return gerror.Wrap(err, "construct compliance management runtime for red-letter intake")
	}
	taxRuntime, err := composition.LoadTaxInvoiceRuntime(ctx, registrar.Services())
	if err != nil {
		return err
	}
	redLetterService, err := taxinvoiceredletter.New(
		g.DB(), taxRuntime.Source, customerReader, managementRuntime,
		identityRuntime.CustomerAudit, identityRuntime.SystemAudit,
		taxinvoiceredletter.ClosedGateway{},
	)
	if err != nil {
		return err
	}
	redLetterIntake, err := taxinvoiceredletter.NewIntakeWorker(
		g.DB(), taxRuntime.RefundFeed, redLetterService,
	)
	if err != nil {
		return err
	}
	if err = compliancecron.RegisterTaxInvoiceRedLetter(ctx, registrar, redLetterIntake); err != nil {
		return err
	}
	return nil
}

// registerRoutes constructs one shared graph and binds customer and management routes separately.
func registerRoutes(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar.Services() == nil {
		return gerror.New("idc-compliance management routes require host services")
	}
	services := registrar.Services()
	runtime, err := loadManagementRuntime(ctx, registrar)
	if err != nil {
		return err
	}
	secretSource, err := composition.LoadSecretSource(ctx)
	if err != nil {
		return err
	}
	secretAccessor, secretManager, err := composition.BindAliyunCredentialSecrets(secretSource)
	if err != nil {
		return err
	}
	connectivityChecker, err := providerconfig.NewAliyunConnectivityChecker(secretAccessor)
	if err != nil {
		return err
	}
	configurationService, err := providerconfig.New(
		secretManager, runtime, services.Sessions(), connectivityChecker,
	)
	if err != nil {
		return err
	}
	supplierSecretManager, err := composition.BindLocalSupplierCredentialSecrets(secretSource)
	if err != nil {
		return err
	}
	localGateway, err := localgatewayclient.LoadFromEnvironment(nil)
	if err != nil {
		return gerror.Wrap(err, "construct installation-local compliance gateway client")
	}
	localGatewayOwnedByProcess := false
	defer func() {
		if !localGatewayOwnedByProcess {
			localGateway.Close()
		}
	}()
	supplierConfigurationService, err := supplierconfig.New(
		supplierSecretManager, runtime, services.Sessions(), localGateway,
	)
	if err != nil {
		return err
	}
	customerReader, privacyConsents, customerAudit, err := loadPrivacyIdentityRuntime(ctx, registrar)
	if err != nil {
		return err
	}
	_, privacySnapshotManager, err := composition.BindPersonalIdentitySecrets(secretSource)
	if err != nil {
		return err
	}
	privacyGovernanceService, err := privacygovernance.NewComplete(
		g.DB(), runtime, customerReader, privacyConsents, customerAudit,
		services.Storage(), privacySnapshotManager,
	)
	if err != nil {
		return err
	}
	personalServices, err := composition.BuildPersonalVerificationServicesWithPrivacyGate(
		ctx, services, secretSource, privacyGovernanceService,
	)
	if err != nil {
		return err
	}
	taxInvoiceManagementService, err := composition.BuildTaxInvoiceManagementService(runtime, secretSource)
	if err != nil {
		return err
	}
	taxInvoiceRequestService, err := composition.BuildTaxInvoiceRequestService(
		ctx, services, secretSource, customerReader, privacyGovernanceService,
	)
	if err != nil {
		return err
	}
	taxInvoiceRedLetterService, err := composition.BuildTaxInvoiceRedLetterService(
		ctx, services, runtime, customerReader,
	)
	if err != nil {
		return err
	}
	enterpriseServices, err := composition.BuildEnterpriseVerificationServicesWithPrivacyGate(
		ctx, services, secretSource, runtime, privacyGovernanceService,
	)
	if err != nil {
		return err
	}
	returnURL, err := runtimeconfig.LoadPersonalVerificationReturnURL()
	if err != nil {
		return err
	}
	accountController, err := accountcontroller.NewV1WithPrivacy(
		personalServices.Identity, personalServices.Customer, enterpriseServices.Customer,
		enterpriseServices.CustomerAccount, returnURL,
		taxInvoiceRequestService, taxInvoiceRedLetterService, privacyGovernanceService,
	)
	if err != nil {
		return err
	}
	adminController, err := admincontroller.NewV1WithPrivacyGovernance(
		configurationService, enterpriseServices.Management, supplierConfigurationService,
		taxInvoiceManagementService, taxInvoiceRedLetterService, privacyGovernanceService,
	)
	if err != nil {
		return err
	}
	if err = bindComplianceRoutes(registrar, accountController, adminController); err != nil {
		return err
	}
	// Source plugins live for the host process lifetime, so the shared signer
	// remains owned by the registered service graph after route registration.
	localGatewayOwnedByProcess = true
	return nil
}

// loadManagementRuntime freezes and constructs the shared management runtime once.
func loadManagementRuntime(ctx context.Context, registrar pluginhost.HTTPRegistrar) (managementcap.Runtime, error) {
	snapshot, err := managementspi.Freeze()
	if err != nil {
		return managementcap.Runtime{}, gerror.Wrap(err, "freeze IDC management runtime declaration")
	}
	factory := snapshot.Factory()
	if factory == nil {
		return managementcap.Runtime{}, gerror.New("IDC management runtime factory is unavailable")
	}
	runtime, err := factory(ctx, registrar.Services())
	if err != nil {
		return managementcap.Runtime{}, gerror.Wrap(err, "construct compliance management runtime")
	}
	return runtime, nil
}

// loadPrivacyIdentityRuntime narrows the shared identity graph to privacy contracts.
func loadPrivacyIdentityRuntime(
	ctx context.Context,
	registrar pluginhost.HTTPRegistrar,
) (identitycap.CurrentCustomerReader, identitycap.PrivacyConsentService, idccap.BusinessAuditWriter, error) {
	runtime, err := composition.LoadIdentityRuntime(ctx, registrar.Services())
	if err != nil {
		return nil, nil, nil, err
	}
	reader, ok := runtime.Identity.(identitycap.CurrentCustomerReader)
	if !ok {
		return nil, nil, nil, gerror.New("current customer privacy identity reader is unavailable")
	}
	consents, ok := runtime.Identity.(identitycap.PrivacyConsentService)
	if !ok {
		return nil, nil, nil, gerror.New("current customer privacy consent service is unavailable")
	}
	return reader, consents, runtime.CustomerAudit, nil
}

func bindComplianceRoutes(
	registrar pluginhost.HTTPRegistrar,
	accountController accountapi.IAccountV1,
	adminController adminapi.IAdminV1,
) error {
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(
					middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(),
					middlewares.Tenancy(),
				)
				group.Bind(accountController)
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(
					middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(),
					middlewares.Tenancy(), middlewares.Permission(),
				)
				group.Bind(adminController)
			})
		})
	})
	return routes.Err()
}
