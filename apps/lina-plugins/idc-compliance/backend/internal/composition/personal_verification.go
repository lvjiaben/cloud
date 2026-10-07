// This file composes personal-verification services without runtime globals.

package composition

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

// PersonalVerificationServices separates requester and recovery audit identities.
type PersonalVerificationServices struct {
	Identity      identitycap.Service
	Customer      personalverification.Service
	System        personalverification.Service
	CustomerAudit idccap.BusinessAuditWriter
}

type PersonalPrivacyCollectionGate interface {
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
}

// BuildPersonalVerificationServices creates requester and trusted-system runtimes.
func BuildPersonalVerificationServices(
	ctx context.Context,
	services capability.Services,
	secretSource secretcap.Source,
) (PersonalVerificationServices, error) {
	return BuildPersonalVerificationServicesWithPrivacyGate(ctx, services, secretSource, nil)
}

// BuildPersonalVerificationServicesWithPrivacyGate binds the production collection gate.
func BuildPersonalVerificationServicesWithPrivacyGate(
	ctx context.Context,
	services capability.Services,
	secretSource secretcap.Source,
	privacyGate PersonalPrivacyCollectionGate,
) (PersonalVerificationServices, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || nilInterface(secretSource) {
		return PersonalVerificationServices{}, gerror.New("personal verification composition is invalid")
	}
	identityRuntime, err := LoadIdentityRuntime(ctx, services)
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	credentialAccessor, credentialManager, err := BindAliyunCredentialSecrets(secretSource)
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	configurationSource, err := providerconfig.NewRuntimeSource(credentialManager)
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	clientFactory, err := personalverification.NewAliyunClientFactory(credentialAccessor)
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	snapshotAccessor, snapshotManager, err := BindPersonalIdentitySecrets(secretSource)
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	var customer personalverification.Service
	if nilInterface(privacyGate) {
		customer, err = personalverification.New(
			g.DB(), identityRuntime.Identity, configurationSource, clientFactory,
			snapshotAccessor, snapshotManager, identityRuntime.CustomerAudit,
		)
	} else {
		customer, err = personalverification.NewWithCollectionGate(
			g.DB(), identityRuntime.Identity, configurationSource, clientFactory,
			snapshotAccessor, snapshotManager, identityRuntime.CustomerAudit, privacyGate,
		)
	}
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	system, err := personalverification.New(
		g.DB(), identityRuntime.Identity, configurationSource, clientFactory,
		snapshotAccessor, snapshotManager, identityRuntime.SystemAudit,
	)
	if err != nil {
		return PersonalVerificationServices{}, err
	}
	return PersonalVerificationServices{
		Identity: identityRuntime.Identity, Customer: customer, System: system,
		CustomerAudit: identityRuntime.CustomerAudit,
	}, nil
}
