// This file composes the customer enterprise-verification submission service.

package composition

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

// EnterpriseVerificationServices keeps customer submission and staff review as
// separate interfaces while sharing the same private storage and key scope.
type EnterpriseVerificationServices struct {
	Customer        enterpriseverification.Service
	CustomerAccount identitycap.EnterpriseAccountService
	Management      enterpriseverification.ManagementService
}

type EnterprisePrivacyGate interface {
	RequireSensitiveCollection(context.Context, string, string) (time.Time, error)
}

// BuildEnterpriseVerificationServicesWithPrivacyGate binds the incident gate
// to the customer and staff runtimes from one startup-owned dependency graph.
func BuildEnterpriseVerificationServicesWithPrivacyGate(
	ctx context.Context,
	services capability.Services,
	secretSource secretcap.Source,
	management managementcap.Runtime,
	privacyGate EnterprisePrivacyGate,
) (*EnterpriseVerificationServices, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || nilInterface(secretSource) ||
		nilInterface(services.Storage()) {
		return nil, gerror.New("enterprise verification composition is invalid")
	}
	identityRuntime, err := LoadIdentityRuntime(ctx, services)
	if err != nil {
		return nil, err
	}
	accessor, manager, err := BindEnterpriseMaterialSecrets(secretSource)
	if err != nil {
		return nil, err
	}
	var customer enterpriseverification.Service
	if nilInterface(privacyGate) {
		customer, err = enterpriseverification.New(
			g.DB(), identityRuntime.Identity, services.Storage(), accessor, manager,
			identityRuntime.CustomerAudit,
		)
	} else {
		customer, err = enterpriseverification.NewWithCollectionGate(
			g.DB(), identityRuntime.Identity, services.Storage(), accessor, manager,
			identityRuntime.CustomerAudit, privacyGate,
		)
	}
	if err != nil {
		return nil, err
	}
	staff, err := enterpriseverification.NewManagement(
		g.DB(), identityRuntime.Identity, services.Storage(), accessor, manager, management,
	)
	if err != nil {
		return nil, err
	}
	return &EnterpriseVerificationServices{
		Customer: customer, CustomerAccount: identityRuntime.EnterpriseAccount,
		Management: staff,
	}, nil
}
