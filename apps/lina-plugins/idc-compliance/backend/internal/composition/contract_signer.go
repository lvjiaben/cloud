package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-compliance/backend/internal/service/contractsigner"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	securitycodesource "lina-plugin-idc-notify/backend/cap/securitycodecap/source"
)

func BuildContractSignerService(ctx context.Context, services capability.Services, secretSource secretcap.Source,
	authorizer managementcap.Authorizer) (contractsigner.Service, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || nilInterface(secretSource) || nilInterface(authorizer) {
		return nil, gerror.New("contract signer composition is invalid")
	}
	identityRuntime, err := LoadIdentityRuntime(ctx, services)
	if err != nil {
		return nil, err
	}
	credentialAccessor, credentialManager, err := BindAliyunCredentialSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	configurationSource, err := providerconfig.NewRuntimeSource(credentialManager)
	if err != nil {
		return nil, err
	}
	clientFactory, err := personalverification.NewAliyunClientFactory(credentialAccessor)
	if err != nil {
		return nil, err
	}
	profileAccessor, profileManager, err := BindContractSignerSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	securityCodes, err := securitycodesource.New(ctx, services, complianceOwnerPluginID)
	if err != nil {
		return nil, gerror.Wrap(err, "construct notification security-code source")
	}
	return contractsigner.New(g.DB(), identityRuntime.Identity, configurationSource, clientFactory, profileAccessor,
		profileManager, securityCodes, authorizer, identityRuntime.CustomerAudit)
}
