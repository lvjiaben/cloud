// This source-only startup seam constructs native commerce from the existing
// resource source without importing resource internals or building a cycle.
package backend

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	servercommercialspi "lina-plugin-idc-core/backend/cap/servercommercialcap/spi"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/currency"
	"lina-plugin-idc-core/backend/internal/service/identityprojection"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func init() {
	if err := servercommercialspi.Declare(newSourceServerCommercial); err != nil {
		panic(err)
	}
}

func newSourceServerCommercial(ctx context.Context, services capability.Services, caller string, source serveroperationresourcecap.Source) (servercommercialcap.Service, error) {
	if ctx == nil || ctx.Err() != nil || caller != servercommercialcap.AllowedCallerPluginID || services == nil || services.BizCtx() == nil || services.Auth() == nil || services.Auth().Authz() == nil || services.Storage() == nil || source == nil {
		return nil, gerror.New("server commercial source caller is not allowed")
	}
	parameters, ok := source.(serveroperationresourcecap.ParameterValidator)
	if !ok {
		return nil, gerror.New("native resource parameter validation is unavailable")
	}
	reader, err := principal.NewAuthorizedReader(services.BizCtx(), services.Auth().Authz())
	if err != nil {
		return nil, err
	}
	key, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, err
	}
	defer clear(key)
	identities, err := identityprojection.New(g.DB(), reader, key)
	if err != nil {
		return nil, err
	}
	customers, ok := identities.(identitycap.CurrentCustomerReader)
	if !ok {
		return nil, gerror.New("native commercial customer reader is unavailable")
	}
	auditConfig, err := runtimeconfig.LoadAuditConfig()
	if err != nil {
		return nil, err
	}
	auditor, err := audit.New(services.BizCtx(), reader, services.Auth().Authz(), auditConfig)
	if err != nil {
		return nil, err
	}
	writer, err := audit.NewBusinessWriterAdapter(auditor)
	if err != nil {
		return nil, err
	}
	public, files, err := newNativeCommercialCatalog(ctx, services, reader)
	if err != nil {
		return nil, err
	}
	prices, err := serveroperation.NewNativePriceBuilder(g.DB(), public, source)
	if err != nil {
		return nil, err
	}
	evidence, err := billing.NewSettlementEvidenceReader(g.DB())
	if err != nil {
		return nil, err
	}
	invoices, err := billing.NewInvoiceService(g.DB())
	if err != nil {
		return nil, err
	}
	refunds, err := billing.NewCompletedRefundEvidenceReader(g.DB())
	if err != nil {
		return nil, err
	}
	targetQuotes, err := newNativeTargetQuoteSource(ctx, services, reader, public, files)
	if err != nil {
		return nil, err
	}
	return serveroperation.NewNativeCommercialService(g.DB(), services.BizCtx(), reader, services.Auth().Authz(), writer, customers, parameters, prices, evidence, invoices, refunds, targetQuotes)
}

func newNativeCommercialCatalog(ctx context.Context, services capability.Services, reader principal.Reader) (catalog.CatalogReader, purchaseconfigurationfile.Service, error) {
	key, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return nil, nil, err
	}
	defer clear(key)
	targets, err := catalog.NewPublicPurchaseFileTargetResolver(g.DB(), key, supplierRuntimeCapabilityInvoker)
	if err != nil {
		return nil, nil, err
	}
	scanner, err := newContentScanner(ctx)
	if err != nil {
		return nil, nil, err
	}
	files, err := purchaseconfigurationfile.New(g.DB(), reader, services.Storage(), targets, scanner)
	if err != nil {
		return nil, nil, err
	}
	public, err := catalog.NewPublicCatalogReaderConfigured(g.DB(), key, supplierRuntimeCapabilityInvoker, files)
	if err != nil {
		return nil, nil, err
	}
	currencies, err := currency.NewReader(g.DB())
	if err != nil {
		return nil, nil, err
	}
	if err = catalog.BindCurrencies(public, currencies); err != nil {
		return nil, nil, err
	}
	labels, err := loadPublicBillingLabelResources()
	if err != nil {
		return nil, nil, err
	}
	if err = catalog.BindPublicBillingLabelResources(public, labels); err != nil {
		return nil, nil, err
	}
	return public, files, nil
}

// newNativeTargetQuoteSource binds real customer commerce and its shared catalog
// freezer at startup. It has no request-time service construction or fake actor.
func newNativeTargetQuoteSource(ctx context.Context, services capability.Services, reader principal.Reader, public catalog.CatalogReader, files purchaseconfigurationfile.Service) (serveroperation.NativeTargetQuoteSource, error) {
	resolver, err := catalog.PurchaseResolverFromPublicReader(public)
	if err != nil {
		return nil, err
	}
	adjustments, err := newChannelQuoteAdjustments(ctx, services)
	if err != nil {
		return nil, err
	}
	events, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return nil, err
	}
	credit, err := billing.NewCreditUsageService(g.DB())
	if err != nil {
		return nil, err
	}
	settlements, err := billing.NewFinancialSettlementService(g.DB(), events, credit)
	if err != nil {
		return nil, err
	}
	owner, err := commerce.NewWithQuoteAdjustments(g.DB(), resolver, reader, adjustments, settlements, nil)
	if err != nil {
		return nil, err
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(public)
	if err != nil {
		return nil, err
	}
	if err = bindChannelCheckoutCapabilities(owner, g.DB(), freezer, files); err != nil {
		return nil, err
	}
	authorizer, err := commerce.NewTargetQuoteAuthorizer(services.BizCtx(), reader, services.Auth().Authz())
	if err != nil {
		return nil, err
	}
	if err = commerce.BindTargetQuoteAuthorizer(owner, authorizer); err != nil {
		return nil, err
	}
	return owner, nil
}
