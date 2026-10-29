// Package backend wires the idc-core source plugin into the host registry.
//
//nolint:cyclop // Source-plugin startup is an explicit fail-closed assembly boundary for independent feature services.
package backend

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	idccore "lina-plugin-idc-core"
	"lina-plugin-idc-core/backend/cap/billingcap"
	billingspi "lina-plugin-idc-core/backend/cap/billingcap/spi"
	channelcommercespi "lina-plugin-idc-core/backend/cap/channelcommercecap/spi"

	customer360spi "lina-plugin-idc-core/backend/cap/customer360cap/spi"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	customerauthspi "lina-plugin-idc-core/backend/cap/customerauthcap/spi"
	closureprecheckspi "lina-plugin-idc-core/backend/cap/customerclosurecap/spi"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	entitlementcreatedspi "lina-plugin-idc-core/backend/cap/entitlementcreatedcap/spi"
	"lina-plugin-idc-core/backend/cap/idccap"
	idcspi "lina-plugin-idc-core/backend/cap/idccap/spi"
	"lina-plugin-idc-core/backend/cap/identitycap"
	identityspi "lina-plugin-idc-core/backend/cap/identitycap/spi"
	lifecycleresourcespi "lina-plugin-idc-core/backend/cap/lifecycleresourcecap/spi"
	"lina-plugin-idc-core/backend/cap/managementcap"
	managementspi "lina-plugin-idc-core/backend/cap/managementcap/spi"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	notificationspi "lina-plugin-idc-core/backend/cap/notificationcap/spi"
	"lina-plugin-idc-core/backend/cap/onboardingcap"
	onboardingspi "lina-plugin-idc-core/backend/cap/onboardingcap/spi"
	"lina-plugin-idc-core/backend/cap/productdeliverycap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	provisionresourcespi "lina-plugin-idc-core/backend/cap/provisionresourcecap/spi"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	renewalownerspi "lina-plugin-idc-core/backend/cap/renewalownercap/spi"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	resourceinterfaceprojectionspi "lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap/spi"
	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	serveroperationspi "lina-plugin-idc-core/backend/cap/serveroperationresourcecap/spi"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	supplierinterfacespi "lina-plugin-idc-core/backend/cap/supplierinterfacecap/spi"
	"lina-plugin-idc-core/backend/cap/supportcap"
	supportspi "lina-plugin-idc-core/backend/cap/supportcap/spi"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
	taxinvoicespi "lina-plugin-idc-core/backend/cap/taxinvoicecap/spi"
	"lina-plugin-idc-core/backend/internal/composition"
	accountcontroller "lina-plugin-idc-core/backend/internal/controller/account"
	activationaccountcontroller "lina-plugin-idc-core/backend/internal/controller/activationaccount"
	activationadmincontroller "lina-plugin-idc-core/backend/internal/controller/activationadmin"
	admincontroller "lina-plugin-idc-core/backend/internal/controller/admin"
	assistanceadmincontroller "lina-plugin-idc-core/backend/internal/controller/assistanceadmin"
	assistancepubliccontroller "lina-plugin-idc-core/backend/internal/controller/assistancepublic"
	contentaccountcontroller "lina-plugin-idc-core/backend/internal/controller/contentaccount"
	contentadmincontroller "lina-plugin-idc-core/backend/internal/controller/contentadmin"
	contentpubliccontroller "lina-plugin-idc-core/backend/internal/controller/contentpublic"
	customeraccountcontroller "lina-plugin-idc-core/backend/internal/controller/customeraccount"
	customeradmincontroller "lina-plugin-idc-core/backend/internal/controller/customeradmin"
	directoryaccountcontroller "lina-plugin-idc-core/backend/internal/controller/directoryaccount"
	directoryadmincontroller "lina-plugin-idc-core/backend/internal/controller/directoryadmin"
	onboardingaccountcontroller "lina-plugin-idc-core/backend/internal/controller/onboardingaccount"
	publiccontroller "lina-plugin-idc-core/backend/internal/controller/public"
	renewalstaffcontroller "lina-plugin-idc-core/backend/internal/controller/renewalstaff"
	serviceaccountcontroller "lina-plugin-idc-core/backend/internal/controller/serviceaccount"
	serviceadmincontroller "lina-plugin-idc-core/backend/internal/controller/serviceadmin"
	supportaccountcontroller "lina-plugin-idc-core/backend/internal/controller/supportaccount"
	supportadmincontroller "lina-plugin-idc-core/backend/internal/controller/supportadmin"
	supportpubliccontroller "lina-plugin-idc-core/backend/internal/controller/supportpublic"
	upgradescontroller "lina-plugin-idc-core/backend/internal/controller/upgrades"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/businessimpact"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/catalogdraft"
	"lina-plugin-idc-core/backend/internal/service/closureprecheck"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/commission"
	"lina-plugin-idc-core/backend/internal/service/contenthub"

	customerloginspi "lina-plugin-idc-core/backend/cap/customerlogincap/spi"
	profilechangespi "lina-plugin-idc-core/backend/cap/profilechangecap/spi"
	loginadmincontroller "lina-plugin-idc-core/backend/internal/controller/loginadmin"
	"lina-plugin-idc-core/backend/internal/service/currency"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customer360summary"
	"lina-plugin-idc-core/backend/internal/service/customeractivation"
	"lina-plugin-idc-core/backend/internal/service/customeraddress"
	"lina-plugin-idc-core/backend/internal/service/customerassistance"
	"lina-plugin-idc-core/backend/internal/service/customeravatar"
	"lina-plugin-idc-core/backend/internal/service/customerclosure"
	"lina-plugin-idc-core/backend/internal/service/customercontact"
	"lina-plugin-idc-core/backend/internal/service/customerexport"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
	"lina-plugin-idc-core/backend/internal/service/entitlementaccess"
	"lina-plugin-idc-core/backend/internal/service/entitlementupgrade"
	"lina-plugin-idc-core/backend/internal/service/financeops"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/identityprojection"
	"lina-plugin-idc-core/backend/internal/service/identityworkflow"
	"lina-plugin-idc-core/backend/internal/service/invoiceprofile"
	"lina-plugin-idc-core/backend/internal/service/lifecycle"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/merchantcontext"
	"lina-plugin-idc-core/backend/internal/service/notificationpolicy"
	"lina-plugin-idc-core/backend/internal/service/onboarding"
	operationsservice "lina-plugin-idc-core/backend/internal/service/operations"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/peerrenewal"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
	"lina-plugin-idc-core/backend/internal/service/productautomationtoken"
	"lina-plugin-idc-core/backend/internal/service/productdeliverydirectory"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-core/backend/internal/service/profileguard"
	"lina-plugin-idc-core/backend/internal/service/providerexecution"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"lina-plugin-idc-core/backend/internal/service/purchaseserverlogin"
	"lina-plugin-idc-core/backend/internal/service/reauth"
	"lina-plugin-idc-core/backend/internal/service/renewal"
	"lina-plugin-idc-core/backend/internal/service/resourceinterfaceprojection"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"lina-plugin-idc-core/backend/internal/service/secret"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
	"lina-plugin-idc-core/backend/internal/service/serviceaction"
	"lina-plugin-idc-core/backend/internal/service/sshkey"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
	"lina-plugin-idc-core/backend/internal/service/taxinvoicesource"
	"lina-plugin-idc-core/backend/internal/service/triallifecycle"
)

// pluginID is the immutable source-plugin identifier.
const pluginID = "idc-core"

const publicBillingResourcePrefix = "plugin.idc-core.catalog.detail.code."

const productFulfillmentSystemReasonResourcePrefix = "plugin.idc-core.fulfillment.reason."

var supplierRuntimeCapabilityInvoker = productautomation.NewRuntimeCapabilityInvoker()

func loadPublicBillingLabelResources() (catalog.PublicBillingLabelResources, error) {
	locales := []struct {
		path string
		set  func(*catalog.PublicLocalizedText, string)
	}{
		{path: "manifest/i18n/zh-CN/plugin.json", set: func(value *catalog.PublicLocalizedText, text string) { value.ZhCN = text }},
		{path: "manifest/i18n/en-US/plugin.json", set: func(value *catalog.PublicLocalizedText, text string) { value.EnUS = text }},
	}
	keys := []string{"charge.one_time", "period.hour", "period.day", "period.month", "period.year", "boolean.true", "boolean.false"}
	result := make(catalog.PublicBillingLabelResources, len(keys))
	for _, locale := range locales {
		content, err := idccore.EmbeddedFiles.ReadFile(locale.path)
		if err != nil {
			return nil, fmt.Errorf("read packed public billing resources: %w", err)
		}
		var bundle map[string]string
		if err = json.Unmarshal(content, &bundle); err != nil {
			return nil, fmt.Errorf("decode packed public billing resources: %w", err)
		}
		for _, key := range keys {
			value := result[key]
			locale.set(&value, bundle[publicBillingResourcePrefix+key])
			result[key] = value
		}
	}
	return result, nil
}

func loadProductFulfillmentSystemReasonCatalog() (productfulfillment.SystemTransitionReasonCatalog, error) {
	locales := []struct {
		path string
		set  func(*productfulfillment.SystemTransitionReason, string)
	}{
		{path: "manifest/i18n/zh-CN/plugin.json", set: func(value *productfulfillment.SystemTransitionReason, text string) { value.LabelZhCN = text }},
		{path: "manifest/i18n/en-US/plugin.json", set: func(value *productfulfillment.SystemTransitionReason, text string) { value.LabelEnUS = text }},
	}
	codes := productfulfillment.RequiredSystemTransitionReasonCodes()
	resources := make(map[string]productfulfillment.SystemTransitionReason, len(codes))
	for _, locale := range locales {
		content, err := idccore.EmbeddedFiles.ReadFile(locale.path)
		if err != nil {
			return nil, gerror.Wrap(err, "read packed product fulfillment system reasons")
		}
		var bundle map[string]string
		if err = json.Unmarshal(content, &bundle); err != nil {
			return nil, gerror.Wrap(err, "decode packed product fulfillment system reasons")
		}
		for _, code := range codes {
			value := resources[code]
			locale.set(&value, bundle[productFulfillmentSystemReasonResourcePrefix+code])
			resources[code] = value
		}
	}
	return productfulfillment.NewSystemTransitionReasonCatalog(resources)
}

// init registers the embedded core source plugin with the host.
func init() {
	customer360Contributor, err := customer360summary.NewLazyCoreContributor(func() gdb.DB { return g.DB() })
	if err != nil {
		panic(err)
	}
	if err = customer360spi.Declare(customer360Contributor); err != nil {
		panic(err)
	}
	closureContributor, err := closureprecheck.NewLazyCoreContributor(func() gdb.DB { return g.DB() })
	if err != nil {
		panic(err)
	}
	if err = closureprecheckspi.Declare(closureContributor); err != nil {
		panic(err)
	}
	if err := idcspi.Declare(func(context.Context) (idccap.Service, error) {
		return merchantcontext.New(g.DB())
	}); err != nil {
		panic(err)
	}
	if err := managementspi.Declare(newManagementRuntime); err != nil {
		panic(err)
	}
	if err := channelcommercespi.Declare(newChannelCommerceRuntime); err != nil {
		panic(err)
	}
	if err := entitlementcreatedspi.Register(renewalownercap.PeerOwnerID, func(context.Context) (entitlementcreatedcap.Sink, error) {
		return productfulfillment.NewPeerEntitlementCreatedSink(g.DB())
	}); err != nil {
		panic(err)
	}
	if err := resourceinterfaceprojectionspi.DeclareSink(func(context.Context) (resourceinterfaceprojectioncap.Sink, error) {
		return resourceinterfaceprojection.New(g.DB())
	}); err != nil {
		panic(err)
	}
	if err := resourceinterfaceprojectionspi.DeclareUsageReader(func(_ context.Context, authorizer managementcap.Authorizer) (resourceinterfaceprojectioncap.UsageReader, error) {
		return resourceinterfaceprojection.NewUsageReader(g.DB(), authorizer)
	}); err != nil {
		panic(err)
	}
	if err := secretspi.Declare(newSourceSecretService); err != nil {
		panic(err)
	}
	if err := supportspi.Declare(newSourceSupportService); err != nil {
		panic(err)
	}
	if err := onboardingspi.Declare(newSourceOnboardingService); err != nil {
		panic(err)
	}
	if err := identityspi.Declare(newSourceIdentityRuntime); err != nil {
		panic(err)
	}
	if err := customerauthspi.Declare(newSourceCurrentCustomerReader); err != nil {
		panic(err)
	}
	if err := taxinvoicespi.Declare(newSourceTaxInvoiceRuntime); err != nil {
		panic(err)
	}
	if err := notificationspi.Declare(newSourceNotificationRuntime); err != nil {
		panic(err)
	}
	if err := notificationspi.DeclareEventSink(newSourceNotificationEventSink); err != nil {
		panic(err)
	}
	declarations := pluginhost.NewDeclarations(pluginID)
	if err := customerloginspi.Declare(newSourceCustomerLoginDirectory); err != nil {
		panic(err)
	}
	if err := declarations.Providers().ProvideExternalIdentityProvider(newSourceCustomerIdentityEngine); err != nil {
		panic(err)
	}
	impactHandler, err := businessimpact.NewLazy(func() gdb.DB { return g.DB() })
	if err != nil {
		panic(err)
	}
	if err := declarations.Lifecycle().RegisterBusinessImpactHandler(impactHandler); err != nil {
		panic(err)
	}
	declarations.Assets().UseEmbeddedFiles(idccore.EmbeddedFiles)
	if err := declarations.BusinessExtensions().RegisterProviderValidator(
		capmodel.BusinessExtensionProductFulfillment,
		"v1",
		productfulfillmentcap.ValidateProviderDeclarations,
	); err != nil {
		panic(err)
	}
	idcInvoker := merchantcontext.NewCapabilityInvoker()
	billingInvoker := billing.NewChannelPaymentCapabilityInvoker()
	productDeliveryInvoker := productdeliverydirectory.NewCapabilityInvoker()
	supplierRuntimeInvoker := supplierRuntimeCapabilityInvoker
	if err := supplierinterfacespi.Declare(func(_ context.Context) (supplierinterfacecap.Service, error) {
		return supplierinterfacecap.NewSourceClient(supplierRuntimeInvoker, supplierRuntimeInvoker)
	}); err != nil {
		panic(err)
	}
	idcDescriptor, err := idccap.NewDescriptor(idcInvoker)
	if err != nil {
		panic(err)
	}
	if err = declarations.Providers().ProvideCapability(idcDescriptor); err != nil {
		panic(err)
	}
	if err := billingspi.Declare(func(context.Context) (billingcap.Service, error) {
		return billing.NewChannelPaymentSourceClient(billingInvoker)
	}); err != nil {
		panic(err)
	}
	billingDescriptor, err := billingcap.NewDescriptor(billingInvoker)
	if err != nil {
		panic(err)
	}
	if err = declarations.Providers().ProvideCapability(billingDescriptor); err != nil {
		panic(err)
	}
	productDeliveryDescriptor, err := productdeliverycap.NewDescriptor(productDeliveryInvoker)
	if err != nil {
		panic(err)
	}
	if err = declarations.Providers().ProvideCapability(productDeliveryDescriptor); err != nil {
		panic(err)
	}
	supplierRuntimeDescriptor, err := supplierinterfacecap.NewDescriptor(supplierRuntimeInvoker)
	if err != nil {
		panic(err)
	}
	if err = declarations.Providers().ProvideCapability(supplierRuntimeDescriptor); err != nil {
		panic(err)
	}
	if err := declarations.HTTP().RegisterRoutes(pluginhost.ExtensionPointHTTPRouteRegister,
		pluginhost.CallbackExecutionModeBlocking,
		func(ctx context.Context, registrar pluginhost.HTTPRegistrar) error {
			return registerRoutes(ctx, registrar, idcInvoker, billingInvoker, productDeliveryInvoker, supplierRuntimeInvoker)
		}); err != nil {
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

// newSourceTaxInvoiceRuntime constructs the compliance-only source graph from
// the host-authorized principal reader and the installation database.
func newSourceTaxInvoiceRuntime(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (taxinvoicecap.Runtime, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || services.BizCtx() == nil ||
		services.Auth() == nil || services.Auth().Authz() == nil ||
		callerPluginID != taxinvoicecap.AllowedCallerPluginID {
		return taxinvoicecap.Runtime{}, gerror.New("tax invoice source caller is not allowed")
	}
	principalReader, err := principal.NewAuthorizedReader(services.BizCtx(), services.Auth().Authz())
	if err != nil {
		return taxinvoicecap.Runtime{}, err
	}
	lookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return taxinvoicecap.Runtime{}, err
	}
	defer clear(lookupKey)
	identityService, err := identityprojection.New(g.DB(), principalReader, lookupKey)
	if err != nil {
		return taxinvoicecap.Runtime{}, err
	}
	customers, ok := identityService.(identitycap.CurrentCustomerReader)
	if !ok {
		return taxinvoicecap.Runtime{}, gerror.New("current customer tax invoice source is unavailable")
	}
	source, err := taxinvoicesource.New(g.DB(), customers)
	if err != nil {
		return taxinvoicecap.Runtime{}, err
	}
	refundFeed, ok := source.(taxinvoicecap.PostedRefundFeed)
	if !ok {
		return taxinvoicecap.Runtime{}, gerror.New("posted refund feed is unavailable")
	}
	runtime := taxinvoicecap.Runtime{Source: source, RefundFeed: refundFeed}
	if err = taxinvoicecap.ValidateRuntime(runtime); err != nil {
		return taxinvoicecap.Runtime{}, err
	}
	return runtime, nil
}

func registerJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("idc-core jobs require a valid host registrar")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if registrar.Services() == nil || registrar.Services().Storage() == nil {
		return gerror.New("idc-core jobs require plugin storage")
	}
	secretSource, err := newSourceSecretService(ctx)
	if err != nil {
		return err
	}
	secretManager, err := secretSource.Manager(secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: pluginID,
		CallerScope: secretcap.CoreProductAutomationInterfaceScope,
	})
	if err != nil {
		return err
	}
	secretRecovery, err := productautomation.NewSecretOperationRecoveryWorker(g.DB(), secretManager, rand.Reader)
	if err != nil {
		return err
	}
	if err = composition.RegisterProductAutomationSecretRecoveryJob(ctx, registrar, secretRecovery); err != nil {
		return err
	}
	businessJobRegistrar, ok := registrar.(interface {
		BusinessExtensions() pluginhost.BusinessExtensionRegistrar
	})
	if !ok || businessJobRegistrar.BusinessExtensions() == nil {
		return gerror.New("product fulfillment jobs require the governed business-extension registrar")
	}
	businessExtensions, err := providerexecution.New(businessJobRegistrar.BusinessExtensions())
	if err != nil {
		return err
	}
	evidenceKeyID, evidenceKey, err := runtimeconfig.LoadProductFulfillmentEvidenceKey()
	if err != nil {
		return err
	}
	fulfillmentEvidence, err := productfulfillment.NewProviderEvidenceSealer(evidenceKeyID, evidenceKey)
	clear(evidenceKey)
	if err != nil {
		return err
	}
	fulfillmentSystemReasons, err := loadProductFulfillmentSystemReasonCatalog()
	if err != nil {
		return err
	}
	var resourceProvisioningSource provisionresourcecap.Source
	if resourceFactory := provisionresourcespi.Freeze().Factory(); resourceFactory != nil {
		resourceProvisioningSource, err = resourceFactory(ctx)
		if err != nil {
			return gerror.Wrap(err, "construct resource-owner product fulfillment source")
		}
	}
	var peerOutbound resourceinterfaceprojectioncap.ExistingBusinessOutboundSource
	if factory := resourceinterfaceprojectionspi.Freeze().ExistingBusinessOutboundFactory(); factory != nil {
		peerOutbound, err = factory(ctx)
		if err != nil {
			return gerror.Wrap(err, "construct peer business outbound source")
		}
	}
	fulfillmentFiles, err := purchaseconfigurationfile.NewConsumedFileReader(g.DB(), registrar.Services().Storage())
	if err != nil {
		return err
	}
	fulfillmentWorker, err := productfulfillment.NewWorker(g.DB(), businessExtensions, supplierRuntimeCapabilityInvoker, fulfillmentEvidence, fulfillmentSystemReasons, resourceProvisioningSource, peerOutbound, fulfillmentFiles)
	if err != nil {
		return err
	}
	if err = composition.RegisterProductFulfillmentJob(ctx, registrar, fulfillmentWorker); err != nil {
		return err
	}
	serviceActionWorker, err := serviceaction.NewWorker(g.DB(), businessExtensions, supplierRuntimeCapabilityInvoker)
	if err != nil {
		return err
	}
	if err = composition.RegisterManagedServiceActionJob(ctx, registrar, serviceActionWorker); err != nil {
		return err
	}
	if resourceProvisioningSource != nil {
		resourceOutcomeWorker, outcomeErr := productfulfillment.NewResourceOutcomeWorker(g.DB(), resourceProvisioningSource)
		if outcomeErr != nil {
			return outcomeErr
		}
		if outcomeErr = composition.RegisterProductFulfillmentResourceOutcomeJob(ctx, registrar, resourceOutcomeWorker); outcomeErr != nil {
			return outcomeErr
		}
	}
	entitlementOwners := entitlementcreatedspi.Freeze()
	entitlementRouter, sinkErr := entitlementOwners.Build(ctx)
	if sinkErr != nil {
		return gerror.Wrap(sinkErr, "construct entitlement-created owner router")
	}
	entitlementWorker, workerErr := productfulfillment.NewEntitlementCreatedWorker(g.DB(), entitlementRouter)
	if workerErr != nil {
		return workerErr
	}
	if workerErr = composition.RegisterEntitlementCreatedJob(ctx, registrar, entitlementWorker); workerErr != nil {
		return workerErr
	}
	cancellationDispatcher, err := outbox.NewService(g.DB(), outbox.Config{
		LeaseDuration: 10 * time.Minute, MaximumClaimBatch: 20,
	})
	if err != nil {
		return gerror.Wrap(err, "construct product fulfillment cancellation outbox")
	}
	cancellationConsumer, err := productfulfillment.NewOrderCancellationConsumer(g.DB(), fulfillmentSystemReasons)
	if err != nil {
		return err
	}
	cancellationEvidence, err := commerce.NewCancellationEvidenceReader(g.DB())
	if err != nil {
		return err
	}
	var serverOperationSource serveroperationresourcecap.Source
	var serverIPCapacity serveroperationresourcecap.IPCapacity
	if sourceFactory := serveroperationspi.Freeze().Factory(); sourceFactory != nil {
		serverOperationSource, err = sourceFactory(ctx)
		if err != nil {
			return gerror.Wrap(err, "construct resource server operation source")
		}
		serverIPCapacity, _ = serverOperationSource.(serveroperationresourcecap.IPCapacity)
	}
	accessoryCancellation, err := serveroperation.NewServerOperationCancellationConsumer(g.DB(), cancellationEvidence, serverIPCapacity)
	if err != nil {
		return err
	}
	cancellationConsumer, err = productfulfillment.NewRoutedCancellationConsumer(cancellationConsumer, accessoryCancellation)
	if err != nil {
		return err
	}
	cancellationWorker, err := productfulfillment.NewCancellationWorker(cancellationDispatcher, cancellationConsumer)
	if err != nil {
		return err
	}
	if err = composition.RegisterProductFulfillmentCancellationJob(ctx, registrar, cancellationWorker); err != nil {
		return err
	}
	assistanceService, err := customerassistance.NewExpiryWorker(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterCustomerAssistanceExpiryJob(ctx, registrar, assistanceService); err != nil {
		return err
	}
	activationExpiry, err := customeractivation.NewExpiryWorker(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterCustomerActivationExpiryJob(ctx, registrar, activationExpiry); err != nil {
		return err
	}
	exportTasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		return err
	}
	exportWorker, err := customerexport.NewWorker(g.DB(), exportTasks, registrar.Services().Storage())
	if err != nil {
		return err
	}
	if err = composition.RegisterCustomerExportJob(ctx, registrar, exportWorker); err != nil {
		return err
	}
	orderLifecycle, err := commerce.NewOrderLifecycleService(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterOrderExpiryJob(ctx, registrar, orderLifecycle); err != nil {
		return err
	}
	lifecycleScheduler, err := lifecycle.New(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterServiceLifecycleJob(ctx, registrar, lifecycleScheduler); err != nil {
		return err
	}
	creditStatements, err := billing.NewCreditUsageService(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterCreditStatementJob(ctx, registrar, creditStatements); err != nil {
		return err
	}
	refundEvents, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return err
	}
	refundTasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		return err
	}
	localRefunds, err := billing.NewLocalRefundWorker(g.DB(), refundEvents, refundTasks)
	if err != nil {
		return err
	}
	if err = composition.RegisterLocalRefundJob(ctx, registrar, localRefunds); err != nil {
		return err
	}
	refundEntitlements, err := billing.NewRefundEntitlementWorker(g.DB(), refundTasks)
	if err != nil {
		return err
	}
	if err = composition.RegisterRefundEntitlementJob(ctx, registrar, refundEntitlements); err != nil {
		return err
	}
	resizeRefunds, err := billing.NewResizeRefundWorker(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterResizeRefundJob(ctx, registrar, resizeRefunds); err != nil {
		return err
	}
	commissionDispatcher, err := outbox.NewService(g.DB(), outbox.Config{
		LeaseDuration: 10 * time.Minute, MaximumClaimBatch: 20,
	})
	if err != nil {
		return gerror.Wrap(err, "construct commission refund outbox")
	}
	commissionRules, err := saleschannel.NewWithDatabase(g.DB())
	if err != nil {
		return err
	}
	commissionConsumer, err := commission.NewConsumer(g.DB(), commissionRules)
	if err != nil {
		return err
	}
	commissionWorker, err := commission.NewWorker(commissionDispatcher, commissionConsumer)
	if err != nil {
		return err
	}
	if err = composition.RegisterCommissionRefundJob(ctx, registrar, commissionWorker); err != nil {
		return err
	}
	marketingEffects, err := marketing.NewEffectReconciler(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterMarketingEffectJob(ctx, registrar, marketingEffects); err != nil {
		return err
	}
	jobRuntimeServices := registrar.Services()
	if jobRuntimeServices == nil || jobRuntimeServices.BizCtx() == nil {
		return gerror.New("idc-core operations job requires business context")
	}
	operationsWorker, err := operationsservice.New(g.DB(), jobRuntimeServices.BizCtx(), jobRuntimeServices.Sessions(), jobRuntimeServices.Auth().Authz())
	if err != nil {
		return err
	}
	if err = composition.RegisterOperationsSnapshotJob(ctx, registrar, operationsWorker); err != nil {
		return err
	}
	if err = registerSupportAndContentJobs(ctx, registrar); err != nil {
		return err
	}
	financialDispatcher, err := outbox.NewService(g.DB(), outbox.Config{
		LeaseDuration: 10 * time.Minute, MaximumClaimBatch: 20,
	})
	if err != nil {
		return gerror.Wrap(err, "construct financial settlement delivery outbox")
	}
	financialFileEvidence, err := purchaseconfigurationfile.NewOrderEvidenceVerifier(g.DB())
	if err != nil {
		return gerror.Wrap(err, "construct financial settlement file evidence verifier")
	}
	jobServices := registrar.Services()
	if jobServices == nil || jobServices.Plugins() == nil || jobServices.Plugins().Registry() == nil {
		return gerror.New("financial settlement structured configuration requires plugin registry")
	}
	financialConfigurationValidator, err := productautomation.NewFrozenCustomerConfigurationValidator(
		g.DB(), businessExtensions, financialFileEvidence,
	)
	if err != nil {
		return gerror.Wrap(err, "construct financial settlement structured configuration validator")
	}
	financialOwnerGates, err := productfulfillment.NewOwnerGateValidator(g.DB())
	if err != nil {
		return gerror.Wrap(err, "construct product fulfillment owner gates")
	}
	financialGenericConsumer, err := productfulfillment.NewSettlementConsumer(g.DB(), financialConfigurationValidator, financialOwnerGates)
	if err != nil {
		return gerror.Wrap(err, "construct generic financial settlement consumer")
	}
	financialGateReleaser, err := productfulfillment.NewOwnerGateReleaser(g.DB(), financialConfigurationValidator, financialOwnerGates, fulfillmentSystemReasons)
	if err != nil {
		return err
	}
	financialGateRecovery, err := productfulfillment.NewSettlementGateRecoveryWorker(g.DB(), financialGateReleaser, financialGenericConsumer)
	if err != nil {
		return err
	}
	if err = composition.RegisterProductFulfillmentGateRecoveryJob(ctx, registrar, financialGateRecovery); err != nil {
		return err
	}
	financialEvidenceReader, err := billing.NewSettlementEvidenceReader(g.DB())
	if err != nil {
		return gerror.Wrap(err, "construct read-only financial settlement evidence")
	}
	financialAddonConsumer, err := serveroperation.NewServerOperationSettlementConsumer(g.DB(), financialEvidenceReader)
	if err != nil {
		return gerror.Wrap(err, "construct accessory settlement consumer")
	}
	financialRoutedConsumer, err := fulfillment.NewRoutedConsumer(financialGenericConsumer, financialAddonConsumer)
	if err != nil {
		return gerror.Wrap(err, "construct routed financial settlement consumer")
	}
	financialWorker, err := fulfillment.NewWorker(financialDispatcher, financialRoutedConsumer)
	if err != nil {
		return gerror.Wrap(err, "construct financial settlement delivery worker")
	}
	if err = composition.RegisterFinancialSettlementDeliveryJob(
		ctx, registrar, financialWorker,
	); err != nil {
		return err
	}
	lifecycleResourceFactory := lifecycleresourcespi.Freeze().Factory()
	if lifecycleResourceFactory != nil {
		lifecycleResource, lifecycleErr := lifecycleResourceFactory(ctx)
		if lifecycleErr != nil {
			return gerror.Wrap(lifecycleErr, "construct resource lifecycle source")
		}
		lifecycleOutbox, lifecycleErr := outbox.NewEnqueuer(g.DB(), outbox.Config{})
		if lifecycleErr != nil {
			return gerror.Wrap(lifecycleErr, "construct service lifecycle outbox")
		}
		lifecycleProcessor, lifecycleErr := lifecycle.NewProcessor(
			g.DB(), lifecycleResource, lifecycleOutbox,
		)
		if lifecycleErr != nil {
			return lifecycleErr
		}
		if lifecycleErr = composition.RegisterServiceLifecycleProcessorJob(
			ctx, registrar, lifecycleProcessor,
		); lifecycleErr != nil {
			return lifecycleErr
		}
		// Trial expiry uses the same startup-owned async task boundary and the
		// resource lifecycle source. It is intentionally skipped when the
		// resource capability is unavailable, allowing manual-delivery products
		// to keep running without pretending they support pause/resume.
		trialNotifications, trialErr := notificationpolicy.NewEventSink(pluginID, lifecycleOutbox)
		if trialErr != nil {
			return gerror.Wrap(trialErr, "construct trial lifecycle notifications")
		}
		trialWorker, trialErr := triallifecycle.New(g.DB(), exportTasks, lifecycleResource, trialNotifications)
		if trialErr != nil {
			return trialErr
		}
		if trialErr = composition.RegisterTrialLifecycleJob(ctx, registrar, trialWorker); trialErr != nil {
			return trialErr
		}
	}
	if serverOperationSource != nil {
		accessoryInvoices, operationErr := billing.NewInvoiceService(g.DB())
		if operationErr != nil {
			return operationErr
		}
		upgradeWorker, upgradeErr := entitlementupgrade.NewWorker(g.DB(), accessoryInvoices, serverOperationSource)
		if upgradeErr != nil {
			return upgradeErr
		}
		if upgradeErr = composition.RegisterEntitlementUpgradeJob(ctx, registrar, upgradeWorker); upgradeErr != nil {
			return upgradeErr
		}
		accessoryDirectory, ok := serverOperationSource.(serveroperationresourcecap.AccessoryDirectory)
		if !ok {
			return gerror.New("resource server accessory directory is unavailable")
		}
		accessoryOutbox, operationErr := outbox.NewEnqueuer(g.DB(), outbox.Config{})
		if operationErr != nil {
			return operationErr
		}
		accessoryNotifications, operationErr := notificationpolicy.NewEventSink(pluginID, accessoryOutbox)
		if operationErr != nil {
			return operationErr
		}
		serverOperationWorker, operationErr := serveroperation.NewWorker(
			g.DB(), serverOperationSource, accessoryDirectory, accessoryInvoices, accessoryNotifications,
		)
		if operationErr != nil {
			return operationErr
		}
		if operationErr = composition.RegisterServerOperationJob(
			ctx, registrar, serverOperationWorker,
		); operationErr != nil {
			return operationErr
		}
	}
	owners, err := renewalownerspi.Freeze().Build(ctx)
	if err != nil {
		return gerror.Wrap(err, "construct renewal owner router")
	}
	owners, err = peerrenewal.NewRouter(owners, g.DB(), businessExtensions, peerOutbound)
	if err != nil {
		return gerror.Wrap(err, "construct same-system renewal owner")
	}
	cursorKey, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return err
	}
	catalogReader, err := catalog.NewPublicCatalogReader(g.DB(), cursorKey)
	clear(cursorKey)
	if err != nil {
		return err
	}
	currencyReader, err := currency.NewReader(g.DB())
	if err != nil {
		return err
	}
	if err = catalog.BindCurrencies(catalogReader, currencyReader); err != nil {
		return err
	}
	renewalAdjustments, err := newChannelQuoteAdjustments(ctx, registrar.Services())
	if err != nil {
		return err
	}
	automaticWorker, providerWorker, err := renewal.NewWorkersWithQuoteAdjustments(
		ctx, g.DB(), catalogReader, owners, renewalAdjustments,
	)
	if err != nil {
		return err
	}
	if err = composition.RegisterAutomaticRenewalJob(ctx, registrar, automaticWorker); err != nil {
		return err
	}
	return composition.RegisterRenewalProviderJob(ctx, registrar, providerWorker)
}

func registerSupportAndContentJobs(ctx context.Context, registrar pluginhost.JobsRegistrar) error {
	supportSLAWorker, err := supportdesk.NewSLAWorker(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterSupportSLAJob(ctx, registrar, supportSLAWorker); err != nil {
		return err
	}
	supportAutomationWorker, err := supportdesk.NewAutomationWorker(g.DB())
	if err != nil {
		return err
	}
	if err = composition.RegisterSupportAutomationJob(ctx, registrar, supportAutomationWorker); err != nil {
		return err
	}
	services := registrar.Services()
	if services == nil || services.Storage() == nil {
		return gerror.New("idc-core content scan job requires plugin storage")
	}
	contentScanner, err := newContentScanner(ctx)
	if err != nil {
		return err
	}
	contentScanWorker, err := contenthub.NewScanWorker(g.DB(), services.Storage(), contentScanner)
	if err != nil {
		return err
	}
	return composition.RegisterContentScanJob(ctx, registrar, contentScanWorker)
}

func newSourceSecretService(_ context.Context) (secretcap.Source, error) {
	keys, err := runtimeconfig.LoadSecretKeyProvider()
	if err != nil {
		return nil, err
	}
	service, err := secret.New(secret.Config{}, keys, secret.SourceAuthorizer{})
	if err != nil {
		return nil, err
	}
	return service, nil
}

func newSourceSupportService(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (supportcap.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil ||
		callerPluginID != supportcap.AllowedCallerPluginID {
		return nil, gerror.New("anonymous support source caller is not allowed")
	}
	secretSource, err := newSourceSecretService(ctx)
	if err != nil {
		return nil, err
	}
	scope := secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: supportcap.OwnerPluginID,
		CallerScope: supportcap.AnonymousAccessSecretScope,
	}
	manager, err := secretSource.Manager(scope)
	if err != nil {
		return nil, err
	}
	generated, ok := manager.(secretcap.GeneratedManager)
	if !ok {
		return nil, gerror.New("anonymous support token generator is unavailable")
	}
	accessor, err := secretSource.Accessor(scope)
	if err != nil {
		return nil, err
	}
	return supportdesk.NewVerifiedAnonymousCreator(g.DB(), generated, accessor)
}

func newSourceOnboardingService(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (onboardingcap.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || callerPluginID != onboardingcap.AllowedCallerPluginID {
		return nil, gerror.New("customer onboarding source caller is not allowed")
	}
	key, err := runtimeconfig.LoadCustomerOnboardingTokenKey()
	if err != nil {
		return nil, err
	}
	defer clear(key)
	return onboarding.New(g.DB(), key)
}

type currentCustomerReaderAdapter struct {
	reader identitycap.CurrentCustomerReader
}

func (adapter currentCustomerReaderAdapter) CurrentCustomer(
	ctx context.Context,
) (*customerauthcap.Subject, error) {
	if adapter.reader == nil {
		return nil, gerror.New("current customer source is unavailable")
	}
	subject, err := adapter.reader.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, gerror.New("current customer source is unavailable")
	}
	return &customerauthcap.Subject{
		CustomerNo: subject.CustomerNo, CustomerTypeCode: subject.CustomerTypeCode,
		PrincipalReference: subject.PrincipalReference, ActorUserID: subject.ActorUserID,
	}, nil
}

func newSourceCurrentCustomerReader(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (customerauthcap.Reader, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || services.BizCtx() == nil ||
		services.Auth() == nil || services.Auth().Authz() == nil ||
		!customerauthcap.IsAllowedCallerPlugin(callerPluginID) {
		return nil, gerror.New("current customer source caller is not allowed")
	}
	principalReader, err := principal.NewAuthorizedReader(services.BizCtx(), services.Auth().Authz())
	if err != nil {
		return nil, err
	}
	lookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, err
	}
	defer clear(lookupKey)
	identityService, err := identityprojection.New(g.DB(), principalReader, lookupKey)
	if err != nil {
		return nil, err
	}
	reader, ok := identityService.(identitycap.CurrentCustomerReader)
	if !ok {
		return nil, gerror.New("current customer source is unavailable")
	}
	adapter := currentCustomerReaderAdapter{reader: reader}
	if callerPluginID == customerauthcap.AllowedCallerPluginID {
		sessionProofs, proofErr := newRecentAuthService(services)
		if proofErr != nil {
			return nil, proofErr
		}
		confirmation, ok := sessionProofs.(reauth.SessionConfirmationService)
		if !ok {
			return nil, gerror.New("customer confirmation source is unavailable")
		}
		return entitlementaccess.NewWithSessionConfirmation(g.DB(), adapter, confirmation)
	}
	return adapter, nil
}

func newSourceIdentityRuntime(
	_ context.Context,
	services capability.Services,
	callerPluginID string,
) (identitycap.Runtime, error) {
	if services == nil || services.BizCtx() == nil || services.Auth() == nil ||
		services.Auth().Authz() == nil || callerPluginID != identitycap.AllowedCallerPluginID {
		return identitycap.Runtime{}, gerror.New("identity projection source caller is not allowed")
	}
	principalReader, err := principal.NewAuthorizedReader(services.BizCtx(), services.Auth().Authz())
	if err != nil {
		return identitycap.Runtime{}, err
	}
	lookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return identitycap.Runtime{}, err
	}
	defer clear(lookupKey)
	auditConfig, err := runtimeconfig.LoadAuditConfig()
	if err != nil {
		return identitycap.Runtime{}, err
	}
	customerAuditService, err := audit.New(
		services.BizCtx(), principalReader, services.Auth().Authz(), auditConfig,
	)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	customerAudit, err := audit.NewBusinessWriterAdapter(customerAuditService)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	identityService, err := identityprojection.New(g.DB(), principalReader, lookupKey)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	access, err := customer.NewAccessGuard(services.BizCtx(), principalReader)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	legalChanges, err := customer.NewOwnLegalChangeService(access, customerAuditService)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	enterpriseAccount, err := identityworkflow.New(legalChanges)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	systemAuditService, err := audit.NewWithTrustedActorResolver(
		personalVerificationRecoveryActorResolver{}, services.Auth().Authz(), auditConfig,
	)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	systemAudit, err := audit.NewBusinessWriterAdapter(systemAuditService)
	if err != nil {
		return identitycap.Runtime{}, err
	}
	runtime := identitycap.Runtime{
		Identity: identityService, EnterpriseAccount: enterpriseAccount,
		CustomerAudit: customerAudit, SystemAudit: systemAudit,
	}
	if err = identitycap.ValidateRuntime(runtime); err != nil {
		return identitycap.Runtime{}, err
	}
	return runtime, nil
}

func newSourceNotificationRuntime(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (notificationcap.Runtime, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || services.BizCtx() == nil ||
		services.Auth() == nil || services.Auth().Authz() == nil ||
		services.Storage() == nil || services.Sessions() == nil ||
		callerPluginID != notificationcap.AllowedCallerPluginID {
		return notificationcap.Runtime{}, gerror.New("notification source caller is not allowed")
	}
	principalReader, err := principal.NewAuthorizedReader(
		services.BizCtx(), services.Auth().Authz(),
	)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	marketingAuthorizer, err := marketing.NewManagementOverviewAuthorizer(
		services.BizCtx(), principalReader, services.Auth().Authz(),
	)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	cursorKey, err := runtimeconfig.LoadMarketingCursorKey()
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	marketingReader, err := marketing.NewManagementDirectoryReader(
		g.DB(), marketingAuthorizer, cursorKey,
	)
	clear(cursorKey)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	contacts, err := notificationpolicy.NewContactResolver(g.DB())
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	audience, err := notificationpolicy.NewCurrentAudience(g.DB(), services.BizCtx(), principalReader)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	eventDispatcher, err := outbox.NewService(g.DB(), outbox.Config{MaximumClaimBatch: 100})
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	events, err := notificationpolicy.NewEventFeed(eventDispatcher)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	engagement, err := marketing.NewEngagementService(g.DB())
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	marketingGate, err := notificationpolicy.NewMarketingGate(engagement)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	supportReader, err := supportdesk.NewReader(g.DB(), services.BizCtx(), principalReader, services.Auth().Authz(), services.Storage(), services.Org())
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	contentReader, err := contenthub.New(g.DB(), services.BizCtx(), principalReader, services.Auth().Authz(), services.Storage(), services.Sessions(), services.I18n())
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	businessObjects, err := notificationpolicy.NewBusinessObjectAccess(g.DB(), marketingReader, supportReader, contentReader)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	preferences, err := notificationpolicy.NewPreferenceCommander(g.DB(), services.BizCtx(), principalReader)
	if err != nil {
		return notificationcap.Runtime{}, err
	}
	runtime := notificationcap.Runtime{
		Audience: audience, Events: events, Contacts: contacts, BusinessObjects: businessObjects, Marketing: marketingGate,
		Preferences: preferences,
	}
	if err = notificationcap.ValidateRuntime(runtime); err != nil {
		return notificationcap.Runtime{}, err
	}
	return runtime, nil
}

func newSourceNotificationEventSink(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (notificationcap.EventSink, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || strings.TrimSpace(callerPluginID) == "" {
		return nil, gerror.New("notification event source caller is not allowed")
	}
	writer, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return nil, err
	}
	return notificationpolicy.NewEventSink(callerPluginID, writer)
}

type personalVerificationRecoveryActorResolver struct{}

// ResolveTrustedActor fixes recovery provenance in core-owned composition.
func (personalVerificationRecoveryActorResolver) ResolveTrustedActor(
	context.Context,
) (audit.TrustedActor, error) {
	return audit.TrustedActor{
		Type:      audit.ActorSystem,
		Reference: "plugin:idc-compliance:personal-verification-recovery",
	}, nil
}

type catalogRouteServices struct {
	management         catalog.ProductManagementReader
	public             catalog.CatalogReader
	publicCampaigns    marketing.PublicCampaignService
	writer             catalog.ManagementService
	publication        catalog.PublicationService
	publicationPreview catalog.PublicationPreviewService
	purchaseFiles      purchaseconfigurationfile.Service
}

type financialRouteServices struct {
	settlements              billing.FinancialSettlementService
	channelPayments          billingcap.Service
	accountReader            billing.AccountFinanceReader
	accountCommands          billing.AccountFinanceCommandService
	accountRefunds           billing.AccountRefundService
	accountCredit            billing.AccountCreditReader
	accountCreditCommands    billing.AccountCreditCommandService
	accountRenewals          renewal.Service
	managementReader         billing.ManagementFinanceReader
	managementCommands       billing.ManagementFinanceCommandService
	managementCredit         billing.ManagementCreditReader
	managementCreditCommands billing.ManagementCreditCommandService
	managementOperations     financeops.Service
	managementRefunds        billing.ManagementRefundService
}

type customerRouteServices struct {
	principal        principal.Reader
	onboarding       principal.Service
	management       customermanagement.Service
	merchant         idccap.Service
	access           *customer.AccessGuard
	account          customer.Service
	accountLifecycle customer.AccountLifecycleService
	closureReview    customerclosure.Service
	contacts         customercontact.Service
	addresses        customeraddress.Service
	exportDelivery   customerexport.Delivery
}

func completeCustomerRouteServices(
	services capability.Services,
	base *customerRouteServices,
	auditService audit.Writer,
	auditWriter idccap.BusinessAuditWriter,
) error {
	if services == nil || base == nil || auditService == nil || auditWriter == nil {
		return gerror.New("customer workflow route services require complete dependencies")
	}
	principalService, err := principal.New(
		services.BizCtx(), services.Auth().Authz(), auditWriter,
	)
	if err != nil {
		return err
	}
	access, err := customer.NewAccessGuard(services.BizCtx(), principalService)
	if err != nil {
		return err
	}
	base.access = access
	base.onboarding = principalService
	contributors, err := closureprecheckspi.Freeze()
	if err != nil {
		return gerror.Wrap(err, "freeze customer closure contributors")
	}
	precheck, err := closureprecheck.New(g.DB(), contributors)
	if err != nil {
		return err
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		return err
	}
	recentAuth, err := newRecentAuthService(services)
	if err != nil {
		return err
	}
	base.account, err = customer.New(access, auditService, tasks, recentAuth, precheck)
	if err != nil {
		return err
	}
	authorizer, err := customer.NewAccountStateAuthorizer(
		services.BizCtx(), services.Auth().Authz(),
	)
	if err != nil {
		return err
	}
	base.accountLifecycle, err = customer.NewAccountLifecycleService(
		services.BizCtx(), principalService, auditService, authorizer,
	)
	if err != nil {
		return err
	}
	base.closureReview, err = customerclosure.New(
		g.DB(), services.BizCtx(), principalService, authorizer, precheck, auditService,
	)
	if err != nil {
		return err
	}
	contactLookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return err
	}
	base.contacts, err = customercontact.New(g.DB(), access, auditService, contactLookupKey)
	if err != nil {
		clear(contactLookupKey)
		return err
	}
	base.addresses, err = customeraddress.New(g.DB(), access, auditService, contactLookupKey)
	clear(contactLookupKey)
	if err != nil {
		return err
	}
	base.exportDelivery, err = customerexport.NewDelivery(
		g.DB(), access, services.Sessions(), services.Storage(),
	)
	return err
}

func newCatalogRouteServices(
	ctx context.Context,
	managementAuthorizer catalog.ManagementAuthorizer,
	publicationAuthorizer catalog.PublicationAuthorizer,
	auditWriter idccap.BusinessAuditWriter,
	automationAdmission catalog.PublicationAutomationAdmission,
	automationReadiness catalog.PublicationAutomationReadiness,
	publicPurchaseRuntime catalog.PublicPurchaseConfigurationRuntime,
	purchaseFiles purchaseconfigurationfile.Service,
) (*catalogRouteServices, error) {
	cursorKey, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return nil, err
	}
	management, err := catalog.NewProductManagementReader(managementAuthorizer, cursorKey)
	if err != nil {
		clear(cursorKey)
		return nil, err
	}
	publicReader, err := catalog.NewPublicCatalogReaderConfigured(
		g.DB(), cursorKey, publicPurchaseRuntime, purchaseFiles,
	)
	clear(cursorKey)
	if err != nil {
		return nil, err
	}
	billingLabels, err := loadPublicBillingLabelResources()
	if err != nil {
		return nil, err
	}
	if err = catalog.BindPublicBillingLabelResources(publicReader, billingLabels); err != nil {
		return nil, err
	}
	publicCampaignKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, err
	}
	publicCampaigns, err := marketing.NewPublicCampaignService(g.DB(), publicCampaignKey)
	clear(publicCampaignKey)
	if err != nil {
		return nil, err
	}
	writer, err := catalog.NewManagement(
		auditWriter, managementAuthorizer,
	)
	if err != nil {
		return nil, err
	}
	publication, err := catalog.NewPublication(
		auditWriter, publicationAuthorizer, automationAdmission,
	)
	if err != nil {
		return nil, err
	}
	publicationPreview, err := catalog.NewPublicationPreview(
		publicationAuthorizer, automationReadiness,
	)
	if err != nil {
		return nil, err
	}
	return &catalogRouteServices{
		management: management, public: publicReader, publicCampaigns: publicCampaigns, writer: writer,
		publication: publication, publicationPreview: publicationPreview,
		purchaseFiles: purchaseFiles,
	}, nil
}

func newCustomerRouteServices(services capability.Services) (*customerRouteServices, error) {
	merchant, err := merchantcontext.New(g.DB())
	if err != nil {
		return nil, err
	}
	// Customer-facing routes must always use the authorized reader. The plain
	// startup reader intentionally fails closed for bound customers because it
	// cannot prove that the identity has not acquired backoffice authority.
	principalSvc, err := principal.NewAuthorizedReader(
		services.BizCtx(), services.Auth().Authz(),
	)
	if err != nil {
		return nil, err
	}
	contributors, err := customer360spi.Freeze()
	if err != nil {
		return nil, err
	}
	customer360, err := customer360summary.New(contributors)
	if err != nil {
		return nil, err
	}
	management, err := customermanagement.NewWithBusinessSummary(
		services.BizCtx(), principalSvc, services.Auth().Authz(), customer360,
		customermanagement.Config{},
	)
	if err != nil {
		return nil, err
	}
	return &customerRouteServices{
		principal: principalSvc, management: management, merchant: merchant,
	}, nil
}

func newInvoiceProfileRouteService(
	ctx context.Context,
	principalSvc principal.Reader,
) (invoiceprofile.Service, error) {
	lookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, err
	}
	defer clear(lookupKey)
	identityService, err := identityprojection.New(g.DB(), principalSvc, lookupKey)
	if err != nil {
		return nil, err
	}
	customers, ok := identityService.(identitycap.CurrentCustomerReader)
	if !ok {
		return nil, gerror.New("current customer invoice profile reader is unavailable")
	}
	secretSource, err := newSourceSecretService(ctx)
	if err != nil {
		return nil, err
	}
	manager, err := secretSource.Manager(secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: secretcap.OwnerPluginID,
		CallerScope: secretcap.CoreInvoiceProfileBankAccountScope,
	})
	if err != nil {
		return nil, err
	}
	cursorKey, err := runtimeconfig.LoadBillingDirectoryCursorKey()
	if err != nil {
		return nil, err
	}
	defer clear(cursorKey)
	return invoiceprofile.New(g.DB(), customers, manager, cursorKey)
}

func newServerOperationRouteServices(
	ctx context.Context,
	services capability.Services,
	principalSvc principal.Reader,
	auditService audit.Writer,
	auditWriter idccap.BusinessAuditWriter,
	catalogReader catalog.CatalogReader,
) (serveroperation.Directory, serveroperation.AccountService, serveroperation.ManagementService, serveroperation.AccessoryPricing, sshkey.Service, error) {
	lookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	defer clear(lookupKey)
	identityService, err := identityprojection.New(g.DB(), principalSvc, lookupKey)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	customers, ok := identityService.(identitycap.CurrentCustomerReader)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("current customer server reader is unavailable")
	}
	factory := serveroperationspi.Freeze().Factory()
	if factory == nil {
		return nil, nil, nil, nil, nil, gerror.New("resource server operation source is unavailable")
	}
	source, err := factory(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, gerror.Wrap(err, "construct resource server operation source")
	}
	capabilities, ok := source.(serveroperationresourcecap.CapabilityDirectory)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server capability directory is unavailable")
	}
	images, ok := source.(serveroperationresourcecap.ReinstallDirectory)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource reinstall image directory is unavailable")
	}
	accessories, ok := source.(serveroperationresourcecap.AccessoryDirectory)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server accessory directory is unavailable")
	}
	ipCapacity, ok := source.(serveroperationresourcecap.IPCapacity)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server IP inventory is unavailable")
	}
	inputSecrets, ok := source.(serveroperationresourcecap.InputSecretSource)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server input secret service is unavailable")
	}
	parameters, ok := source.(serveroperationresourcecap.ParameterValidator)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server parameter validator is unavailable")
	}
	outputs, ok := source.(serveroperationresourcecap.ConfidentialOutputRedeemer)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server confidential output service is unavailable")
	}
	ownership, ok := source.(serveroperationresourcecap.OwnershipTransfer)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("resource server ownership transfer service is unavailable")
	}
	operations, err := serveroperation.NewService(g.DB(), auditWriter)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	recentAuthService, err := newRecentAuthService(services)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	recentAuth, ok := recentAuthService.(reauth.SessionConfirmationService)
	if !ok {
		return nil, nil, nil, nil, nil, gerror.New("server operation session confirmation is unavailable")
	}
	accountOperations, err := serveroperation.NewAccountService(
		g.DB(), customers, operations, inputSecrets, parameters, outputs, ownership, recentAuth,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	directory, err := serveroperation.NewDirectory(g.DB(), customers, capabilities, images, accessories)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	pricing, err := serveroperation.NewAccessoryPricing(
		g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), auditWriter,
		accessories, ipCapacity, parameters, customers, recentAuth,
		catalogReader,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	management, err := serveroperation.NewManagementService(
		services.BizCtx(), principalSvc, services.Auth().Authz(), operations, directory, parameters, pricing,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	sshKeys, err := sshkey.New(g.DB(), customers, auditService)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return directory, accountOperations, management, pricing, sshKeys, nil
}

func newFinancialRouteServices(
	ctx context.Context,
	services capability.Services,
	principalSvc principal.Reader,
	auditWriter idccap.BusinessAuditWriter,
	catalogReader catalog.CatalogReader,
) (*financialRouteServices, error) {
	cursorKey, err := runtimeconfig.LoadBillingDirectoryCursorKey()
	if err != nil {
		return nil, err
	}
	defer clear(cursorKey)
	creditUsage, err := billing.NewCreditUsageService(g.DB())
	if err != nil {
		return nil, err
	}
	accountReader, err := billing.NewAccountFinanceReader(g.DB(), principalSvc, cursorKey)
	if err != nil {
		return nil, err
	}
	accountRefunds, err := billing.NewAccountRefundService(
		g.DB(), principalSvc, auditWriter, creditUsage,
	)
	if err != nil {
		return nil, err
	}
	managementAuthorizer, err := billing.NewManagementFinanceAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return nil, err
	}
	managementReader, err := billing.NewManagementFinanceReader(
		g.DB(), managementAuthorizer, cursorKey,
	)
	if err != nil {
		return nil, err
	}
	accountCredit, err := billing.NewAccountCreditReader(g.DB(), principalSvc, cursorKey)
	if err != nil {
		return nil, err
	}
	managementCreditAuthorizer, err := billing.NewManagementCreditAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return nil, err
	}
	creditEvidence, err := billing.NewCreditEvidenceService(
		g.DB(), principalSvc, managementCreditAuthorizer, services.Storage(),
	)
	if err != nil {
		return nil, err
	}
	managementCredit, err := billing.NewManagementCreditReaderWithEvidence(
		g.DB(), managementCreditAuthorizer, cursorKey, creditEvidence,
	)
	if err != nil {
		return nil, err
	}
	recentAuth, err := newRecentAuthService(services)
	if err != nil {
		return nil, err
	}
	managementRefunds, err := billing.NewManagementRefundService(
		g.DB(), managementAuthorizer, recentAuth, auditWriter, creditUsage,
	)
	if err != nil {
		return nil, err
	}
	settlementEvents, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "construct financial settlement outbox")
	}
	settlements, err := billing.NewFinancialSettlementService(
		g.DB(), settlementEvents, creditUsage,
	)
	if err != nil {
		return nil, err
	}
	refundTasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		return nil, err
	}
	orderLifecycle, err := commerce.NewOrderLifecycleService(g.DB())
	if err != nil {
		return nil, err
	}
	paymentClosures, err := composition.NewPaymentClosureBridge(g.DB(), orderLifecycle)
	if err != nil {
		return nil, err
	}
	channelPayments, err := billing.NewChannelPaymentOwnerService(
		g.DB(), settlements, settlementEvents, refundTasks, paymentClosures, principalSvc,
	)
	if err != nil {
		return nil, err
	}
	managementCommands, err := newManagementFinanceCommandService(
		auditWriter, managementAuthorizer, recentAuth, settlements,
	)
	if err != nil {
		return nil, err
	}
	creditApproval, err := billing.NewCreditApprovalService(g.DB())
	if err != nil {
		return nil, err
	}
	accountCreditCommands, err := billing.NewAccountCreditCommandServiceWithEvidence(
		g.DB(), principalSvc, auditWriter, recentAuth, creditApproval, creditUsage, creditEvidence,
	)
	if err != nil {
		return nil, err
	}
	managementCreditCommands, err := billing.NewManagementCreditCommandService(
		g.DB(), auditWriter, managementCreditAuthorizer, recentAuth, creditApproval,
	)
	if err != nil {
		return nil, err
	}
	renewalAdjustments, err := newChannelQuoteAdjustments(ctx, services)
	if err != nil {
		return nil, err
	}
	trialRecovery, err := triallifecycle.NewRecoveryScheduler(g.DB(), refundTasks)
	if err != nil {
		return nil, err
	}
	accountRenewals, err := renewal.NewWithTrialConversion(ctx, g.DB(), principalSvc, catalogReader, renewalAdjustments, trialRecovery)
	if err != nil {
		return nil, err
	}
	accountCommands, err := billing.NewAccountFinanceCommandServiceWithRenewals(g.DB(), principalSvc, settlements, accountRenewals)
	if err != nil {
		return nil, err
	}
	notificationWriter, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return nil, err
	}
	financialNotifications, err := notificationpolicy.NewEventSink(pluginID, notificationWriter)
	if err != nil {
		return nil, err
	}
	managementOperations, err := financeops.New(
		g.DB(), managementAuthorizer, auditWriter, financialNotifications, services.Storage(),
	)
	if err != nil {
		return nil, err
	}
	return &financialRouteServices{
		settlements: settlements, channelPayments: channelPayments,
		accountReader:   accountReader,
		accountCommands: accountCommands,
		accountRefunds:  accountRefunds,
		accountCredit:   accountCredit, accountCreditCommands: accountCreditCommands,
		accountRenewals: accountRenewals, managementReader: managementReader,
		managementCommands: managementCommands, managementCredit: managementCredit,
		managementCreditCommands: managementCreditCommands,
		managementOperations:     managementOperations,
		managementRefunds:        managementRefunds,
	}, nil
}

func newManagementFinanceCommandService(
	auditWriter idccap.BusinessAuditWriter,
	authorizer billing.ManagementFinanceAuthorizer,
	recentAuth reauth.Service,
	settlements billing.FinancialSettlementService,
) (billing.ManagementFinanceCommandService, error) {
	adjustments, err := billing.NewFinancialAdjustmentService(g.DB())
	if err != nil {
		return nil, err
	}
	return billing.NewManagementFinanceCommandService(
		g.DB(), auditWriter, authorizer, recentAuth, adjustments, settlements,
	)
}

func newRecentAuthService(services capability.Services) (reauth.Service, error) {
	keyProvider, err := runtimeconfig.LoadRecentAuthSigningKeyProvider()
	if err != nil {
		return nil, err
	}
	signer, err := reauth.NewHMACSigner(keyProvider)
	if err != nil {
		return nil, err
	}
	return reauth.New(
		reauth.Config{}, services.BizCtx(), services.Sessions(),
		reauth.NewDAOReplayStore(), signer,
	)
}

type marketingRouteServices struct {
	overview       marketing.ManagementOverviewReader
	directory      marketing.ManagementDirectoryReader
	assetDirectory marketing.AssetDirectoryReader
	actions        marketing.ManagementActionService
	insights       marketing.MarketingInsightsService
	couponBatches  marketing.CouponBatchService
}

func newMarketingRouteServices(
	services capability.Services,
	principalSvc principal.Reader,
	auditWriter idccap.BusinessAuditWriter,
) (*marketingRouteServices, error) {
	authorizer, err := marketing.NewManagementOverviewAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return nil, err
	}
	overview, err := marketing.NewManagementOverviewReader(g.DB(), authorizer)
	if err != nil {
		return nil, err
	}
	cursorKey, err := runtimeconfig.LoadMarketingCursorKey()
	if err != nil {
		return nil, err
	}
	directory, err := marketing.NewManagementDirectoryReader(g.DB(), authorizer, cursorKey)
	if err != nil {
		clear(cursorKey)
		return nil, err
	}
	assetDirectory, err := marketing.NewAssetDirectoryReader(g.DB(), authorizer, cursorKey)
	clear(cursorKey)
	if err != nil {
		return nil, err
	}
	points, err := marketing.NewPointsService(g.DB())
	if err != nil {
		return nil, err
	}
	valueGrants, err := marketing.NewValueGrantService(g.DB())
	if err != nil {
		return nil, err
	}
	financialAdjustments, err := billing.NewFinancialAdjustmentService(g.DB())
	if err != nil {
		return nil, err
	}
	actions, err := marketing.NewManagementActionService(
		g.DB(), auditWriter, authorizer, points, valueGrants, financialAdjustments,
	)
	if err != nil {
		return nil, err
	}
	insights, err := marketing.NewMarketingInsightsService(g.DB(), authorizer, auditWriter)
	if err != nil {
		return nil, err
	}
	couponBatchKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, err
	}
	couponBatches, err := marketing.NewCouponBatchService(g.DB(), auditWriter, authorizer, couponBatchKey)
	clear(couponBatchKey)
	if err != nil {
		return nil, err
	}
	return &marketingRouteServices{
		overview: overview, directory: directory, assetDirectory: assetDirectory, actions: actions,
		insights: insights, couponBatches: couponBatches,
	}, nil
}

type supportContentRouteControllers struct {
	supportAccount *supportaccountcontroller.ControllerV1
	supportAdmin   *supportadmincontroller.ControllerV1
	supportPublic  *supportpubliccontroller.ControllerV1
	contentAdmin   *contentadmincontroller.ControllerV1
	contentAccount *contentaccountcontroller.ControllerV1
	contentPublic  *contentpubliccontroller.ControllerV1
}

func newSupportContentRouteControllers(
	ctx context.Context,
	services capability.Services,
	principalSvc principal.Reader,
	auditWriter idccap.BusinessAuditWriter,
	contentScanner contenthub.Scanner,
) (*supportContentRouteControllers, error) {
	notificationWriter, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return nil, err
	}
	supportNotifications, err := notificationpolicy.NewEventSink(pluginID, notificationWriter)
	if err != nil {
		return nil, err
	}
	upstreamSecretSource, err := newSourceSecretService(ctx)
	if err != nil {
		return nil, err
	}
	upstreamSecretScope := secretcap.Scope{TenantID: idccap.DefaultTenantID, OwnerPluginID: pluginID, CallerScope: secretcap.CoreSupportUpstreamChannelScope}
	upstreamSecretManager, err := upstreamSecretSource.Manager(upstreamSecretScope)
	if err != nil {
		return nil, err
	}
	upstreamSecretAccessor, err := upstreamSecretSource.Accessor(upstreamSecretScope)
	if err != nil {
		return nil, err
	}
	supportService, err := supportdesk.NewProduction(
		g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), services.Storage(), supportNotifications,
		auditWriter, services.Org(), upstreamSecretManager, upstreamSecretAccessor, services.Sessions(),
	)
	if err != nil {
		return nil, err
	}
	supportAccount, err := supportaccountcontroller.NewV1(supportService)
	if err != nil {
		return nil, err
	}
	supportAdmin, err := supportadmincontroller.NewV1(supportService)
	if err != nil {
		return nil, err
	}
	supportPublic, err := supportpubliccontroller.NewV1(supportService)
	if err != nil {
		return nil, err
	}
	contentService, err := contenthub.NewWithScanner(
		g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), services.Storage(), services.Sessions(), services.I18n(), contentScanner,
	)
	if err != nil {
		return nil, err
	}
	contentAdmin, err := contentadmincontroller.NewV1(contentService)
	if err != nil {
		return nil, err
	}
	contentAccount, err := contentaccountcontroller.NewV1(contentService)
	if err != nil {
		return nil, err
	}
	contentPublic, err := contentpubliccontroller.NewV1(contentService)
	if err != nil {
		return nil, err
	}
	return &supportContentRouteControllers{
		supportAccount: supportAccount,
		supportAdmin:   supportAdmin,
		supportPublic:  supportPublic,
		contentAdmin:   contentAdmin,
		contentAccount: contentAccount,
		contentPublic:  contentPublic,
	}, nil
}

func newContentScanner(ctx context.Context) (contenthub.Scanner, error) {
	config, err := runtimeconfig.LoadContentScannerConfig()
	if err != nil {
		return nil, err
	}
	scanner, err := contenthub.NewClamdScanner(contenthub.ClamdConfig{
		Network:             config.Network,
		Address:             config.Address,
		Timeout:             config.Timeout,
		DangerousLimitBytes: config.TestedMaxFileSizeBytes,
	})
	if err != nil {
		return nil, err
	}
	if scanner.Configured() && scanner.DangerousLimitBytes() > 0 {
		if err = scanner.SelfTest(ctx); err != nil {
			logger.Warningf(ctx, "IDC dangerous content types remain disabled because local clamd self-test failed: %v", err)
		}
	}
	return scanner, nil
}

//nolint:cyclop // Startup assembly keeps each explicitly injected service/controller and fail-closed binding result visible in one ordered boundary.
func registerRoutes(
	ctx context.Context,
	registrar pluginhost.HTTPRegistrar,
	idcInvoker *merchantcontext.CapabilityInvoker,
	billingInvoker *billing.ChannelPaymentCapabilityInvoker,
	productDeliveryInvoker *productdeliverydirectory.CapabilityInvoker,
	supplierRuntimeInvoker *productautomation.RuntimeCapabilityInvoker,
) error {
	services := registrar.Services()
	if services == nil || services.Auth() == nil || services.Auth().Authz() == nil || services.BizCtx() == nil ||
		services.Storage() == nil || services.Sessions() == nil || services.Users() == nil || registrar.BusinessExtensions() == nil {
		return gerror.New("idc-core management routes require host authz and bizctx services")
	}
	businessExtensions, err := providerexecution.New(registrar.BusinessExtensions())
	if err != nil {
		return err
	}
	customerServices, err := newCustomerRouteServices(services)
	if err != nil {
		return err
	}
	principalSvc := customerServices.principal
	loginDirectory, err := buildCustomerLoginDirectory(services.BizCtx(), services.Users())
	if err != nil {
		return err
	}
	loginAdmin, err := loginadmincontroller.NewV1(loginDirectory)
	if err != nil {
		return err
	}
	loginRoutes := registrar.Routes()
	lm := loginRoutes.Middlewares()
	loginRoutes.Group(loginRoutes.APIPrefix()+"/api/v1", func(group pluginhost.RouteGroup) {
		group.Middleware(lm.HandlerResponse(), lm.CORS(), lm.RequestBodyLimit(), lm.Ctx(), lm.Auth(), lm.Tenancy(), lm.Permission())
		group.Bind(loginAdmin)
	})
	if err := profileguard.Register(registrar, services.I18n(), principalSvc, services.BizCtx(), services.Plugins().State(), profilechangespi.Current, loginDirectory, services.Users()); err != nil {
		return err
	}
	productDeliveryDirectory, err := productdeliverydirectory.New(g.DB(), principalSvc)
	if err != nil {
		return err
	}
	if err = productDeliveryInvoker.Bind(productDeliveryDirectory); err != nil {
		return err
	}
	catalogAuthorizer, err := catalog.NewManagementAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return err
	}
	auditConfig, err := runtimeconfig.LoadAuditConfig()
	if err != nil {
		return err
	}
	auditSvc, err := audit.New(
		services.BizCtx(),
		principalSvc,
		services.Auth().Authz(),
		auditConfig,
	)
	if err != nil {
		return err
	}
	auditWriter, err := audit.NewBusinessWriterAdapter(auditSvc)
	if err != nil {
		return err
	}
	if err = completeCustomerRouteServices(
		services, customerServices, auditSvc, auditWriter,
	); err != nil {
		return err
	}
	catalogPublicationAuthorizer, err := catalog.NewProductPublicationAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return err
	}
	productAutomationAuthorizer, err := productautomation.NewManagementAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(), catalogPublicationAuthorizer,
	)
	if err != nil {
		return err
	}
	audienceKey, err := runtimeconfig.LoadProductAutomationSelectionAudienceKey()
	if err != nil {
		return err
	}
	audienceReader, err := productautomationtoken.NewProductionAudienceReader(
		audienceKey, services.BizCtx(), principalSvc, services.Sessions(),
	)
	clear(audienceKey)
	if err != nil {
		return err
	}
	selectionKey, err := runtimeconfig.LoadProductAutomationSelectionKey()
	if err != nil {
		audienceReader.Close()
		return err
	}
	selectionCodec, err := productautomationtoken.New(selectionKey, audienceReader, rand.Reader)
	clear(selectionKey)
	if err != nil {
		audienceReader.Close()
		return err
	}
	automationRuntimeOwned := false
	defer closeProductAutomationRuntimeUnlessOwned(&automationRuntimeOwned, selectionCodec, audienceReader)
	productAutomationSecretSource, err := newSourceSecretService(ctx)
	if err != nil {
		return err
	}
	productAutomationSecrets, err := productAutomationSecretSource.Manager(secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: pluginID,
		CallerScope: secretcap.CoreProductAutomationInterfaceScope,
	})
	if err != nil {
		return err
	}
	productAutomationSecretAccessor, err := productAutomationSecretSource.Accessor(secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: pluginID,
		CallerScope: secretcap.CoreProductAutomationInterfaceScope,
	})
	if err != nil {
		return err
	}
	contentScanner, err := newContentScanner(ctx)
	if err != nil {
		return err
	}
	var resourceCandidateSource resourceinterfaceprojectioncap.CandidateSource
	if candidateFactory := resourceinterfaceprojectionspi.Freeze().CandidateSourceFactory(); candidateFactory != nil {
		resourceCandidateSource, err = candidateFactory(ctx)
		if err != nil {
			return gerror.Wrap(err, "construct resource interface candidate source")
		}
	}
	booleanLabels, err := loadPublicBillingLabelResources()
	if err != nil {
		return err
	}
	falseLabel, trueLabel := booleanLabels["boolean.false"], booleanLabels["boolean.true"]
	automationSvc, err := productautomation.NewConfigured(
		g.DB(), businessExtensions, productAutomationAuthorizer, auditWriter, selectionCodec,
		productAutomationSecrets, productAutomationSecretAccessor, services.Storage(), contentScanner, resourceCandidateSource,
		capmodel.LocalizedText{ZhCN: falseLabel.ZhCN, EnUS: falseLabel.EnUS},
		capmodel.LocalizedText{ZhCN: trueLabel.ZhCN, EnUS: trueLabel.EnUS},
	)
	if err != nil {
		return err
	}
	if err = supplierRuntimeInvoker.Bind(automationSvc); err != nil {
		return err
	}
	purchaseFileCursorKey, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return err
	}
	purchaseFileTargets, err := catalog.NewPublicPurchaseFileTargetResolver(
		g.DB(), purchaseFileCursorKey, automationSvc,
	)
	clear(purchaseFileCursorKey)
	if err != nil {
		return err
	}
	purchaseFiles, err := purchaseconfigurationfile.New(
		g.DB(), principalSvc, services.Storage(), purchaseFileTargets, contentScanner,
	)
	if err != nil {
		return err
	}
	productFulfillmentFrozen, err := productautomation.NewFrozenCustomerConfigurationValidator(
		g.DB(), businessExtensions, purchaseFiles,
	)
	if err != nil {
		return err
	}
	productFulfillmentGates, err := productfulfillment.NewOwnerGateValidator(g.DB())
	if err != nil {
		return err
	}
	productFulfillmentAuthorizer, err := productfulfillment.NewManagementAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return err
	}
	productFulfillmentActors, err := productfulfillment.NewActorDirectory(services.Users())
	if err != nil {
		return err
	}
	productFulfillmentCursorKey, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return err
	}
	productFulfillmentSystemReasons, err := loadProductFulfillmentSystemReasonCatalog()
	if err != nil {
		clear(productFulfillmentCursorKey)
		return err
	}
	var managementResourceSource provisionresourcecap.Source
	if factory := provisionresourcespi.Freeze().Factory(); factory != nil {
		managementResourceSource, err = factory(ctx)
		if err != nil {
			return err
		}
	}
	productFulfillmentService, err := productfulfillment.NewFullConfigured(
		g.DB(), productFulfillmentFrozen, productFulfillmentGates, productFulfillmentSystemReasons, productFulfillmentAuthorizer,
		auditWriter, productFulfillmentActors, productFulfillmentCursorKey, managementResourceSource,
	)
	clear(productFulfillmentCursorKey)
	if err != nil {
		return err
	}
	currencyService, err := currency.New(g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), auditWriter)
	if err != nil {
		return err
	}
	catalogServices, err := newCatalogRouteServices(
		ctx, catalogAuthorizer, catalogPublicationAuthorizer, auditWriter,
		automationSvc, automationSvc, automationSvc, purchaseFiles,
	)
	if err != nil {
		return err
	}
	serverLoginTargets, ok := catalogServices.public.(purchaseserverlogin.TargetResolver)
	if !ok {
		return gerror.New("public catalog server login resolver is unavailable")
	}
	serverLogins, err := purchaseserverlogin.New(g.DB(), principalSvc, productAutomationSecretSource, serverLoginTargets)
	if err != nil {
		return err
	}
	if err = catalog.BindCurrencies(catalogServices.public, currencyService); err != nil {
		return err
	}
	importAuthorizer, err := catalog.NewImportAuthorizer(services.BizCtx(), principalSvc, services.Auth().Authz())
	if err != nil {
		return err
	}
	var catalogSource resourceinterfaceprojectioncap.CatalogSource
	if factory := resourceinterfaceprojectionspi.Freeze().CatalogSourceFactory(); factory != nil {
		catalogSource, err = factory(ctx, importAuthorizer)
		if err != nil {
			return gerror.Wrap(err, "construct upstream catalog read source")
		}
	}
	if err = catalog.BindEditorImportedBillingSource(catalogServices.management, catalogSource); err != nil {
		return err
	}
	copyWriter, err := catalog.ProductCopyWriterFromManagement(catalogServices.writer)
	if err != nil {
		return err
	}
	copyAutomation, err := productautomation.ProductCopyServiceFromService(automationSvc)
	if err != nil {
		return err
	}
	catalogImportService, err := catalogdraft.New(importAuthorizer, catalogSource, catalogServices.writer, automationSvc)
	if err != nil {
		return err
	}
	mutationSvc, err := customermanagement.NewMutation(
		services.BizCtx(),
		principalSvc,
		services.Auth().Authz(),
		auditWriter,
	)
	if err != nil {
		return err
	}
	marketingServices, err := newMarketingRouteServices(services, principalSvc, auditWriter)
	if err != nil {
		return err
	}
	orderRiskReviewAuthorizer, err := commerce.NewOrderRiskReviewAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return err
	}
	orderRiskReviewSvc, err := commerce.NewOrderRiskReviewService(
		g.DB(), auditWriter, orderRiskReviewAuthorizer,
	)
	if err != nil {
		return err
	}
	purchaseResolver, err := catalog.PurchaseResolverFromPublicReader(catalogServices.public)
	if err != nil {
		return err
	}
	financialServices, err := newFinancialRouteServices(
		ctx, services, principalSvc, auditWriter, catalogServices.public,
	)
	if err != nil {
		return err
	}
	invoiceProfiles, err := newInvoiceProfileRouteService(ctx, principalSvc)
	if err != nil {
		return err
	}
	serverDirectory, serverOperations, serverAccessoryManagement, accessoryPricing, sshKeys, err := newServerOperationRouteServices(
		ctx, services, principalSvc, auditSvc, auditWriter, catalogServices.public,
	)
	if err != nil {
		return err
	}
	serverBatchReauth, err := newRecentAuthService(services)
	if err != nil {
		return err
	}
	serverBatchNotifications, err := newSourceNotificationEventSink(ctx, services, pluginID)
	if err != nil {
		return err
	}
	serverBatches, err := serveroperation.NewBatchManagement(
		serverAccessoryManagement, accessoryPricing, serverBatchReauth, auditWriter, serverBatchNotifications,
	)
	if err != nil {
		return err
	}
	quoteAdjustments, err := newChannelQuoteAdjustments(ctx, services)
	if err != nil {
		return err
	}
	commerceSvc, err := commerce.NewWithQuoteAdjustments(
		g.DB(), purchaseResolver, principalSvc, quoteAdjustments, financialServices.settlements, serverLogins,
	)
	if err != nil {
		return err
	}
	purchaseFreezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(catalogServices.public)
	if err != nil {
		return err
	}
	if err = commerce.BindCheckoutCurrencies(commerceSvc, currencyService); err != nil {
		return err
	}
	if err = commerce.BindStructuredConfigurationSnapshots(commerceSvc, purchaseFreezer, purchaseFiles); err != nil {
		return err
	}
	paymentPreparationSvc, err := commerce.NewPaymentPreparationService(g.DB(), principalSvc)
	if err != nil {
		return err
	}
	orderDirectoryAuthorizer, err := commerce.NewManagementOrderAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		return err
	}
	orderCursorKey, err := runtimeconfig.LoadCommerceOrderCursorKey()
	if err != nil {
		return err
	}
	accountOrderReader, err := commerce.NewAccountOrderReader(g.DB(), principalSvc, orderCursorKey)
	if err != nil {
		clear(orderCursorKey)
		return err
	}
	managementOrderReader, err := commerce.NewManagementOrderReader(
		g.DB(), orderDirectoryAuthorizer, orderCursorKey,
	)
	if err != nil {
		clear(orderCursorKey)
		return err
	}
	assistedQuoteAuthorizer, err := commerce.NewAssistedQuoteAuthorizer(
		services.BizCtx(), principalSvc, services.Auth().Authz(),
	)
	if err != nil {
		clear(orderCursorKey)
		return err
	}
	assistedQuoteSvc, err := commerce.NewAssistedQuoteService(
		g.DB(), purchaseResolver, commerceSvc, auditWriter, assistedQuoteAuthorizer, orderCursorKey,
	)
	clear(orderCursorKey)
	if err != nil {
		return err
	}
	if err = commerce.BindAssistedCurrencies(assistedQuoteSvc, currencyService); err != nil {
		return err
	}
	serverOperationPolicy, err := serveroperation.NewPolicyManagement(
		g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), auditWriter,
	)
	if err != nil {
		return err
	}
	salesChannelService, err := saleschannel.NewWithDatabase(g.DB(), services.BizCtx())
	if err != nil {
		return err
	}
	salesManagementService, err := salesmanagement.New(g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), services.Users(), salesChannelService, auditWriter)
	if err != nil {
		return err
	}
	priceCopyService, err := salesmanagement.ProductPriceCopyServiceFromService(salesManagementService)
	if err != nil {
		return err
	}
	catalogCopyService, err := catalogdraft.NewCopy(copyWriter, copyAutomation, priceCopyService)
	if err != nil {
		return err
	}
	operationsService, err := operationsservice.New(g.DB(), services.BizCtx(), services.Sessions(), services.Auth().Authz())
	if err != nil {
		return err
	}
	accountController, err := accountcontroller.NewV1(
		commerceSvc, paymentPreparationSvc, accountOrderReader, financialServices.accountReader,
		financialServices.accountCommands, financialServices.accountRefunds, financialServices.accountCredit,
		financialServices.accountCreditCommands, financialServices.accountRenewals, invoiceProfiles,
		serverDirectory, serverOperations, accessoryPricing, sshKeys,
	)
	if err != nil {
		return err
	}
	couponKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return err
	}
	couponClaims, err := marketing.NewCouponClaimService(g.DB(), principalSvc, couponKey)
	clear(couponKey)
	if err != nil {
		return err
	}
	if err = accountcontroller.BindCouponClaims(accountController, couponClaims); err != nil {
		return err
	}
	accountPurchaseConfiguration, err := catalog.StructuredConfigurationReaderFromPublicReader(catalogServices.public)
	if err != nil {
		return err
	}
	if err = accountcontroller.BindPurchaseConfigurationFiles(accountController, catalogServices.purchaseFiles, accountPurchaseConfiguration); err != nil {
		return err
	}
	if err = accountcontroller.BindPurchaseServerLogins(accountController, serverLogins); err != nil {
		return err
	}
	customerAvatarService, err := customeravatar.New(customerServices.access, services.Storage())
	if err != nil {
		return err
	}
	customerAccountController, err := customeraccountcontroller.NewV1(
		customerServices.account, customerServices.contacts, customerServices.addresses, customerServices.exportDelivery, customerAvatarService,
	)
	if err != nil {
		return err
	}
	customerSalesChannelService, ok := salesChannelService.(saleschannel.CustomerService)
	if !ok {
		return gerror.New("sales channel service does not expose customer commission operations")
	}
	if err = customeraccountcontroller.BindSalesChannel(customerAccountController, customerSalesChannelService); err != nil {
		return err
	}
	adminController, err := admincontroller.NewV1(
		customerServices.management, mutationSvc, catalogServices.management, catalogServices.writer, catalogServices.publication,
		catalogServices.publicationPreview, marketingServices.overview, marketingServices.directory, marketingServices.assetDirectory,
		marketingServices.actions, orderRiskReviewSvc, managementOrderReader, assistedQuoteSvc,
		financialServices.managementReader,
		financialServices.managementCommands,
		financialServices.managementCredit,
		financialServices.managementCreditCommands,
	)
	if err != nil {
		return err
	}
	if err = admincontroller.BindCustomerAudit(adminController, auditSvc); err != nil {
		return err
	}
	if err = admincontroller.BindCustomerAccountLifecycle(adminController, customerServices.accountLifecycle); err != nil {
		return err
	}
	if err = admincontroller.BindFinancialOperations(adminController, financialServices.managementOperations); err != nil {
		return err
	}
	if err = admincontroller.BindRefunds(adminController, financialServices.managementRefunds); err != nil {
		return err
	}
	if err = admincontroller.BindMarketingInsights(adminController, marketingServices.insights); err != nil {
		return err
	}
	if err = admincontroller.BindMarketingCouponBatches(adminController, marketingServices.couponBatches); err != nil {
		return err
	}
	if err = admincontroller.BindServerOperationPolicy(adminController, serverOperationPolicy); err != nil {
		return err
	}
	if err = admincontroller.BindAccessoryPricing(adminController, accessoryPricing); err != nil {
		return err
	}
	if err = admincontroller.BindServerAccessoryManagement(adminController, serverAccessoryManagement); err != nil {
		return err
	}
	if err = admincontroller.BindServerBatches(adminController, serverBatches); err != nil {
		return err
	}
	if err = admincontroller.BindProductAutomation(adminController, automationSvc); err != nil {
		return err
	}
	if err = admincontroller.BindCatalogCopy(adminController, catalogCopyService); err != nil {
		return err
	}
	if err = admincontroller.BindCatalogImport(adminController, catalogImportService); err != nil {
		return err
	}
	if err = admincontroller.BindProductFulfillment(adminController, productFulfillmentService); err != nil {
		return err
	}
	if err = admincontroller.BindSalesManagement(adminController, salesManagementService); err != nil {
		return err
	}
	if err = admincontroller.BindSalesChannel(adminController, salesChannelService); err != nil {
		return err
	}
	if err = admincontroller.BindCurrencies(adminController, currencyService); err != nil {
		return err
	}
	if err = admincontroller.BindOperations(adminController, operationsService); err != nil {
		return err
	}
	serviceIdentityKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return err
	}
	serviceCustomers, err := identityprojection.New(g.DB(), principalSvc, serviceIdentityKey)
	clear(serviceIdentityKey)
	if err != nil {
		return err
	}
	serviceCustomerReader, ok := serviceCustomers.(identitycap.CurrentCustomerReader)
	if !ok {
		return gerror.New("managed service customer identity is unavailable")
	}
	serviceActionAccess, err := serviceaction.NewAccess(services.BizCtx(), principalSvc, services.Auth().Authz(), serviceCustomerReader)
	if err != nil {
		return err
	}
	directoryAccess, err := productdirectory.NewAccess(services.BizCtx(), principalSvc, services.Auth().Authz(), serviceCustomerReader)
	if err != nil {
		return err
	}
	directoryService, err := productdirectory.New(g.DB(), directoryAccess, auditWriter)
	if err != nil {
		return err
	}
	directoryAdminController, err := directoryadmincontroller.NewV1(directoryService)
	if err != nil {
		return err
	}
	directoryAccountController, err := directoryaccountcontroller.NewV1(directoryService)
	if err != nil {
		return err
	}
	serviceActions, err := serviceaction.NewConfigured(g.DB(), businessExtensions, supplierRuntimeInvoker, supplierRuntimeInvoker, services.Storage())
	if err != nil {
		return err
	}
	serviceAccountController, err := serviceaccountcontroller.NewV1(serviceActions, serviceActionAccess)
	if err != nil {
		return err
	}
	serviceAdminController, err := serviceadmincontroller.NewV1(serviceActions, serviceActionAccess)
	if err != nil {
		return err
	}
	customerAdminController, err := customeradmincontroller.NewV1(customerServices.closureReview)
	if err != nil {
		return err
	}
	assistanceTokenKey, err := runtimeconfig.LoadCustomerAssistanceTokenKey()
	if err != nil {
		return err
	}
	assistanceService, err := customerassistance.New(
		g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), auditWriter,
		services.Auth().Token(), assistanceTokenKey,
	)
	clear(assistanceTokenKey)
	if err != nil {
		return err
	}
	assistanceAdminController, err := assistanceadmincontroller.NewV1(assistanceService)
	if err != nil {
		return err
	}
	assistancePublicController, err := assistancepubliccontroller.NewV1(assistanceService)
	if err != nil {
		return err
	}
	activationWriter, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return err
	}
	activationNotifications, err := notificationpolicy.NewEventSink(pluginID, activationWriter)
	if err != nil {
		return err
	}
	activationLookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return err
	}
	activationService, err := customeractivation.New(g.DB(), services.BizCtx(), principalSvc, services.Auth().Authz(), services.Users(), loginDirectory, auditWriter, activationNotifications, activationLookupKey, runtimeconfig.CustomerPublicURL)
	clear(activationLookupKey)
	if err != nil {
		return err
	}
	initialPasswordGuard, err := customeractivation.NewInitialPasswordGuard(activationService)
	if err != nil {
		return err
	}
	activationAdminController, err := activationadmincontroller.NewV1(activationService)
	if err != nil {
		return err
	}
	activationAccountController, err := activationaccountcontroller.NewV1(activationService, loginDirectory)
	if err != nil {
		return err
	}
	registrationSignalKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return err
	}
	registrationSignals, err := marketing.NewRegistrationSignalRecorder(g.DB(), registrationSignalKey)
	clear(registrationSignalKey)
	if err != nil {
		return err
	}
	onboardingAccountController, err := onboardingaccountcontroller.NewV1(customerServices.onboarding, registrationSignals, loginDirectory)
	if err != nil {
		return err
	}
	supportContentControllers, err := newSupportContentRouteControllers(ctx, services, principalSvc, auditWriter, contentScanner)
	if err != nil {
		return err
	}
	upgradeFactory := serveroperationspi.Freeze().Factory()
	if upgradeFactory == nil {
		return gerror.New("upgrade resource source unavailable")
	}
	upgradeSource, err := upgradeFactory(context.Background())
	if err != nil {
		return err
	}
	upgradeInvoices, err := billing.NewInvoiceService(g.DB())
	if err != nil {
		return err
	}
	upgradeRuntime, err := entitlementupgrade.New(g.DB(), principalSvc, upgradeInvoices, upgradeSource)
	if err != nil {
		return err
	}
	upgradeController, err := upgradescontroller.NewV1(upgradeRuntime)
	if err != nil {
		return err
	}
	renewalStaffController, err := newRenewalStaffController(ctx, g.DB(), services, principalSvc)
	if err != nil {
		return err
	}
	err = bindRouteControllers(
		registrar, idcInvoker, customerServices.merchant, billingInvoker, financialServices.channelPayments,
		catalogServices, commerceSvc, accountController, adminController, customerAvatarService,
		customerServices.access, customerAccountController, customerAdminController,
		assistanceAdminController, assistancePublicController,
		activationAdminController, activationAccountController,
		initialPasswordGuard,
		onboardingAccountController,
		principal.NewStaffMiddleware(customerServices.onboarding),
		supportContentControllers.supportAccount, supportContentControllers.supportAdmin,
		supportContentControllers.supportPublic, supportContentControllers.contentAdmin,
		supportContentControllers.contentAccount, supportContentControllers.contentPublic,
		serviceAccountController, serviceAdminController, serviceActionAccess,
		directoryAccountController, directoryAdminController, upgradeController, renewalStaffController,
	)
	if err == nil {
		automationRuntimeOwned = true
	}
	return err
}

type productAutomationRuntimeCloser interface {
	Close()
}

func closeProductAutomationRuntimeUnlessOwned(owned *bool, closers ...productAutomationRuntimeCloser) {
	if owned != nil && *owned {
		return
	}
	for _, closer := range closers {
		if closer != nil {
			closer.Close()
		}
	}
}

func bindRouteControllers(
	registrar pluginhost.HTTPRegistrar,
	idcInvoker *merchantcontext.CapabilityInvoker,
	merchantService idccap.Service,
	billingInvoker *billing.ChannelPaymentCapabilityInvoker,
	channelPayments billingcap.Service,
	catalogServices *catalogRouteServices,
	commerceService commerce.Service,
	accountController *accountcontroller.ControllerV1,
	adminController *admincontroller.ControllerV1,
	customerAvatars customeravatar.Service,
	customerAccess *customer.AccessGuard,
	customerAccountController *customeraccountcontroller.ControllerV1,
	customerAdminController *customeradmincontroller.ControllerV1,
	assistanceAdminController *assistanceadmincontroller.ControllerV1,
	assistancePublicController *assistancepubliccontroller.ControllerV1,
	activationAdminController *activationadmincontroller.ControllerV1,
	activationAccountController *activationaccountcontroller.ControllerV1,
	initialPasswordGuard *customeractivation.InitialPasswordGuard,
	onboardingAccountController *onboardingaccountcontroller.ControllerV1,
	staffMiddleware *principal.StaffMiddleware,
	supportAccountController *supportaccountcontroller.ControllerV1,
	supportAdminController *supportadmincontroller.ControllerV1,
	supportPublicController *supportpubliccontroller.ControllerV1,
	contentAdminController *contentadmincontroller.ControllerV1,
	contentAccountController *contentaccountcontroller.ControllerV1,
	contentPublicController *contentpubliccontroller.ControllerV1,
	serviceAccountController *serviceaccountcontroller.ControllerV1,
	serviceAdminController *serviceadmincontroller.ControllerV1,
	serviceActionAccess serviceaction.Access,
	directoryAccountController *directoryaccountcontroller.ControllerV1,
	directoryAdminController *directoryadmincontroller.ControllerV1,
	upgradeController *upgradescontroller.ControllerV1,
	renewalStaffController *renewalstaffcontroller.ControllerV1,
) error {
	if serviceActionAccess == nil {
		return gerror.New("service customer page bridge requires its shared access service")
	}
	publicController, err := publiccontroller.NewV1(catalogServices.public, catalogServices.publicCampaigns, commerceService, customerAvatars)
	if err != nil {
		return err
	}
	currencyReader, err := currency.NewReader(g.DB())
	if err != nil {
		return err
	}
	if err = publiccontroller.BindCurrencies(publicController, currencyReader); err != nil {
		return err
	}
	routes := registrar.Routes()
	middlewares := routes.Middlewares()
	routes.Group(routes.APIPrefix(), func(group pluginhost.RouteGroup) {
		group.Group("/api/v1", func(group pluginhost.RouteGroup) {
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy())
				group.Bind(activationAccountController)
				group.Bind(onboardingAccountController)
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx())
				group.Bind(publicController)
				group.Bind(assistancePublicController)
				group.Bind(supportPublicController)
				group.Bind(contentPublicController)
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(), middlewares.Tenancy(),
					customerAccess.Middleware, initialPasswordGuard.Middleware, middlewares.PageBridge())
				group.Bind(directoryAccountController)
				group.Bind(accountController)
				group.Bind(upgradeController)
				group.Bind(customerAccountController)
				group.Bind(supportAccountController)
				group.Bind(contentAccountController)
				group.BindCustomerPageBridge(func(ctx context.Context) (int, error) {
					actor, accessErr := serviceActionAccess.Customer(ctx)
					return actor.UserID, accessErr
				}, serviceAccountController)
			})
			group.Group("", func(group pluginhost.RouteGroup) {
				group.Middleware(middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
					middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(),
					middlewares.Tenancy(), middlewares.Permission(), staffMiddleware.Middleware, middlewares.PageBridge())
				group.Bind(renewalStaffController)
				group.Bind(directoryAdminController)
				group.Bind(adminController)
				group.Bind(customerAdminController)
				group.Bind(assistanceAdminController)
				group.Bind(activationAdminController)
				group.Bind(supportAdminController)
				group.Bind(contentAdminController)
				group.Bind(serviceAdminController)
			})
		})
	})
	if err = routes.Err(); err != nil {
		return err
	}
	if err := idcInvoker.Bind(merchantService); err != nil {
		return err
	}
	return billingInvoker.Bind(channelPayments)
}
