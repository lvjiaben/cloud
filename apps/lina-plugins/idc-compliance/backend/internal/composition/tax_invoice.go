// This file freezes and loads the core-owned tax-invoice source boundary.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoiceredletter"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicerequest"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
	taxinvoicespi "lina-plugin-idc-core/backend/cap/taxinvoicecap/spi"
)

// BuildTaxInvoiceRedLetterService composes the core refund source, exact
// customer ownership, staff authorization, and closed supplier adapter.
func BuildTaxInvoiceRedLetterService(
	ctx context.Context,
	services capability.Services,
	runtime managementcap.Runtime,
	customers identitycap.CurrentCustomerReader,
) (taxinvoiceredletter.Service, error) {
	taxRuntime, err := LoadTaxInvoiceRuntime(ctx, services)
	if err != nil {
		return nil, err
	}
	identityRuntime, err := LoadIdentityRuntime(ctx, services)
	if err != nil {
		return nil, err
	}
	service, err := taxinvoiceredletter.New(
		g.DB(), taxRuntime.Source, customers, runtime,
		identityRuntime.CustomerAudit, identityRuntime.SystemAudit,
		taxinvoiceredletter.ClosedGateway{},
	)
	if err != nil {
		return nil, gerror.Wrap(err, "construct tax invoice red-letter service")
	}
	return service, nil
}

// LoadTaxInvoiceRuntime constructs the compliance-only source runtime. The
// compliance allocation reader is supplied to individual source calls so it
// can join the caller's transaction and report current reserved usage.
func LoadTaxInvoiceRuntime(
	ctx context.Context,
	services capability.Services,
) (taxinvoicecap.Runtime, error) {
	if ctx == nil || ctx.Err() != nil || services == nil {
		return taxinvoicecap.Runtime{}, gerror.New("compliance tax invoice runtime context is invalid")
	}
	snapshot, err := taxinvoicespi.Freeze()
	if err != nil {
		return taxinvoicecap.Runtime{}, gerror.Wrap(err, "freeze tax invoice source declaration")
	}
	factory := snapshot.Factory()
	if factory == nil {
		return taxinvoicecap.Runtime{}, gerror.New("tax invoice source factory is unavailable")
	}
	runtime, err := factory(ctx, services, taxinvoicecap.AllowedCallerPluginID)
	if err != nil {
		return taxinvoicecap.Runtime{}, gerror.Wrap(err, "construct compliance tax invoice source")
	}
	if err = taxinvoicecap.ValidateRuntime(runtime); err != nil {
		return taxinvoicecap.Runtime{}, gerror.Wrap(err, "validate compliance tax invoice source")
	}
	return runtime, nil
}

// BuildTaxInvoiceManagementService composes the provider-neutral staff
// workflow. Production remains explicitly closed until the NuoNuo evidence
// gate is satisfied; this constructor performs no provider network call.
func BuildTaxInvoiceManagementService(
	runtime managementcap.Runtime,
	secretSource secretcap.Source,
) (taxinvoicemanagement.Service, error) {
	accessor, _, err := BindTaxInvoiceSnapshotSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	service, err := taxinvoicemanagement.New(
		g.DB(), runtime, taxinvoicemanagement.ClosedNuoNuoGate{}, accessor,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "construct tax invoice management service")
	}
	return service, nil
}

// BuildTaxInvoiceRequestService composes the supplier-neutral request owner.
// The returned service is ready for the customer HTTP layer added by task
// 10.12; no provider client or background worker is created here.
func BuildTaxInvoiceRequestService(
	ctx context.Context,
	services capability.Services,
	secretSource secretcap.Source,
	customers identitycap.CurrentCustomerReader,
	privacyGate PersonalPrivacyCollectionGate,
) (taxinvoicerequest.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || nilInterface(secretSource) || nilInterface(customers) || nilInterface(privacyGate) {
		return nil, gerror.New("tax invoice request composition is invalid")
	}
	runtime, err := LoadTaxInvoiceRuntime(ctx, services)
	if err != nil {
		return nil, err
	}
	accessor, manager, err := BindTaxInvoiceSnapshotSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	requestService, err := taxinvoicerequest.NewWithCollectionGate(g.DB(), runtime.Source, accessor, manager, customers, privacyGate)
	if err != nil {
		return nil, gerror.Wrap(err, "construct tax invoice request service")
	}
	return requestService, nil
}
