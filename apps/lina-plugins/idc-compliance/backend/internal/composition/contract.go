// This file binds the core-owned contract snapshot and compliance-owned encrypted snapshot scope.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-compliance/backend/internal/service/contractlifecycle"
	"lina-plugin-idc-compliance/backend/internal/service/contractsigner"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontracttemplate"
	"lina-plugin-idc-core/backend/cap/contractcap"
	contractspi "lina-plugin-idc-core/backend/cap/contractcap/spi"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

// LoadContractRuntime freezes the owner declaration and constructs the exact
// compliance-only order, identity, signer-matching, and audit boundaries.
func LoadContractRuntime(
	ctx context.Context,
	services capability.Services,
) (contractcap.Runtime, error) {
	if ctx == nil || ctx.Err() != nil || services == nil {
		return contractcap.Runtime{}, gerror.New("compliance contract runtime context is invalid")
	}
	snapshot, err := contractspi.Freeze()
	if err != nil {
		return contractcap.Runtime{}, gerror.Wrap(err, "freeze contract runtime source declaration")
	}
	factory := snapshot.Factory()
	if factory == nil {
		return contractcap.Runtime{}, gerror.New("contract runtime source factory is unavailable")
	}
	runtime, err := factory(ctx, services, contractcap.AllowedCallerPluginID)
	if err != nil {
		return contractcap.Runtime{}, gerror.Wrap(err, "construct compliance contract runtime source")
	}
	if err = contractcap.ValidateRuntime(runtime); err != nil {
		return contractcap.Runtime{}, gerror.Wrap(err, "validate compliance contract runtime source")
	}
	return runtime, nil
}

// BuildElectronicContractPreparationService composes the local-only service
// that seals one immutable preview before any Tencent signing flow is launched.
func BuildElectronicContractPreparationService(
	ctx context.Context,
	services capability.Services,
	secretSource secretcap.Source,
	authorizer managementcap.Authorizer,
	signers contractsigner.Service,
	gateway *localgatewayclient.Client,
	customers identitycap.CurrentCustomerReader,
	signingReturnURL string,
	privacyGate PersonalPrivacyCollectionGate,
) (electroniccontractpreparation.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || nilInterface(secretSource) ||
		nilInterface(authorizer) || nilInterface(signers) || nilInterface(gateway) ||
		nilInterface(customers) || nilInterface(privacyGate) {
		return nil, gerror.New("electronic contract preparation composition is invalid")
	}
	runtime, err := LoadContractRuntime(ctx, services)
	if err != nil {
		return nil, err
	}
	accessor, manager, err := BindContractSnapshotSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	templates, err := electroniccontracttemplate.NewDirectorySource(g.DB())
	if err != nil {
		return nil, gerror.Wrap(err, "construct contract template directory")
	}
	artifactTexts, err := electroniccontractpreparation.LoadArtifactTextCatalog(ctx, services.Manifest())
	if err != nil {
		return nil, gerror.Wrap(err, "load electronic contract artifact translations")
	}
	lifecycle, err := contractlifecycle.New(g.DB(), gateway, runtime.Audit)
	if err != nil {
		return nil, gerror.Wrap(err, "construct electronic contract lifecycle service")
	}
	service, err := electroniccontractpreparation.NewWithGatewayAndPrivacyGate(
		g.DB(), runtime, accessor, manager, templates, authorizer, artifactTexts, gateway, lifecycle,
		customers, signingReturnURL, privacyGate, signers,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "construct electronic contract preparation service")
	}
	return service, nil
}
