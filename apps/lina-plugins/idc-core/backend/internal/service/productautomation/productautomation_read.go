// This file implements bounded provider, interface, and historical binding reads.

package productautomation

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// ListProviderCandidates performs one bounded host discovery call and returns
// only safe display fields plus authenticated selection tokens.
func (s *serviceImpl) ListProviderCandidates(ctx context.Context, page PageInput) (*ProviderCandidatePage, error) {
	authorization, err := s.authorizer.AuthorizeProviderCandidates(ctx)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	page, err = normalizePage(page)
	if err != nil {
		return nil, err
	}
	providers, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point: capmodel.BusinessExtensionProductFulfillment,
		Page:  capmodel.PageRequest{PageNum: page.Page, PageSize: page.PageSize, Limit: page.PageSize},
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeEvidenceUnavailable)
	}
	if providers == nil || providers.Total < len(providers.Items) || len(providers.Items) > page.PageSize {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	result := &ProviderCandidatePage{Items: make([]ProviderCandidate, 0, len(providers.Items)), Total: providers.Total}
	for _, evidence := range providers.Items {
		frozenEvidenceValid := validFrozenEvidence(evidence)
		var declaration *productfulfillmentcap.ProviderDeclaration
		var parseErr error
		if frozenEvidenceValid {
			declaration, parseErr = productfulfillmentcap.ParseProviderDeclaration(evidence.ProviderMetadata, evidence.AuthorizedMethods)
		}
		if !frozenEvidenceValid || parseErr != nil {
			pluginID := ""
			interfaceCardinality := ""
			var authorizedMethods []string
			var interfaceSchemaErr, productSchemaErr, customerSchemaErr error
			if evidence != nil {
				pluginID = evidence.PluginID
				interfaceCardinality = evidence.InterfaceConfigurationCardinality
				authorizedMethods = evidence.AuthorizedMethods
				if evidence.InterfaceConfiguration != nil {
					_, interfaceSchemaErr = validateConfigurationSchema(evidence.InterfaceConfiguration, configurationSurfaceInterface, evidence.AuthorizedMethods)
				}
				if evidence.ProductConfiguration != nil {
					_, productSchemaErr = validateConfigurationSchema(evidence.ProductConfiguration, configurationSurfaceProduct, evidence.AuthorizedMethods)
				}
				if evidence.CustomerConfiguration != nil {
					_, customerSchemaErr = validateConfigurationSchema(evidence.CustomerConfiguration, configurationSurfaceCustomer, evidence.AuthorizedMethods)
				}
			}
			g.Log().Warningf(ctx, "reject product fulfillment provider evidence plugin=%s frozen_valid=%t declaration_error=%v interface_cardinality=%s authorized_methods=%v interface_schema_error=%v product_schema_error=%v customer_schema_error=%v", pluginID, frozenEvidenceValid, parseErr, interfaceCardinality, authorizedMethods, interfaceSchemaErr, productSchemaErr, customerSchemaErr)
			return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
		}
		token, encodeErr := s.codec.EncodeProviderSelection(ctx, evidence)
		if encodeErr != nil {
			return nil, bizerr.WrapCode(encodeErr, catalog.CodeManagementUnavailable)
		}
		if token == "" || len(token) > 4096 {
			return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
		}
		result.Items = append(result.Items, ProviderCandidate{
			SelectionToken: token, DisplayName: evidence.DisplayName,
			ReleaseVersion: evidence.ReleaseVersion, ContractVersion: evidence.ContractVersion,
			FulfillmentCapability:  fulfillmentCapability(declaration),
			InterfaceConfiguration: evidence.InterfaceConfiguration,
		})
	}
	return result, nil
}

func fulfillmentCapability(declaration *productfulfillmentcap.ProviderDeclaration) *capmodel.LocalizedText {
	if declaration == nil {
		return nil
	}
	return &capmodel.LocalizedText{EnUS: declaration.Capability.EnUS, ZhCN: declaration.Capability.ZhCN}
}

// ListInterfaces performs one count and one bounded data query without provider
// lookups, so result size cannot create N+1 host or database access. A provider
// selection requests product-binding candidates from both managed interfaces and
// resource-owned projections; the management directory remains managed-only.
func (s *serviceImpl) ListInterfaces(ctx context.Context, page PageInput) (*InterfacePage, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceList(ctx)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	page, err = normalizePage(page)
	if err != nil {
		return nil, err
	}
	model := s.database.Model(do.ProductAutomationInterfaceInstance{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceInstance{
			TenantId: idccap.DefaultTenantID,
		})
	if page.ProviderSelectionToken != "" {
		selected, decodeErr := s.codec.DecodeProviderSelection(ctx, page.ProviderSelectionToken)
		if decodeErr != nil || !validFrozenEvidence(selected) {
			return nil, bizerr.NewCode(CodeEvidenceUnavailable)
		}
		model = model.Where(do.ProductAutomationInterfaceInstance{
			PluginId:           selected.PluginID,
			ExtensionPointCode: selected.Point, ContractVersion: selected.ContractVersion,
		})
	} else if !page.ForProductBinding {
		model = model.Where(do.ProductAutomationInterfaceInstance{InterfaceOriginCode: "managed"})
	}
	if page.NamePrefix != "" {
		model = model.Where("display_name LIKE ? ESCAPE '\\'", escapeLikePrefix(page.NamePrefix))
	}
	if page.NewBusinessState != "" {
		model = model.Where(do.ProductAutomationInterfaceInstance{NewBusinessState: page.NewBusinessState})
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var rows []*entity.ProductAutomationInterfaceInstance
	if err = model.Fields(
		"interface_instance_no", "display_name", "plugin_release_version", "contract_version",
		"new_business_state", "ever_referenced", "version", "created_at", "changed_at", "interface_origin_code", "authorized_methods",
	).OrderDesc("id").Page(page.Page, page.PageSize).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	availability, err := s.interfaceCatalogAvailability(ctx, rows)
	if err != nil {
		return nil, err
	}
	result := &InterfacePage{Items: make([]InterfaceInstance, 0, len(rows)), Total: total}
	for _, row := range rows {
		item := safeInterface(row)
		item.CatalogAvailable = availability[row.InterfaceInstanceNo]
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// GetBinding authorizes before storage access, then validates immutable evidence
// through the host historical exact-read path without applying current-sale state.
// Unavailable evidence is shown as unverified in this read-only summary; all
// configuration, publication and execution gates retain their strict checks.
func (s *serviceImpl) GetBinding(ctx context.Context, versionNo string) (*Binding, error) {
	authorization, err := s.authorizer.AuthorizeBindingRead(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if err = validateBusinessNo(versionNo, "ver"); err != nil {
		return nil, err
	}
	row, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		return nil, nil
	}
	result := safeBinding(row)
	instance, err := s.loadInterface(ctx, row.InterfaceInstanceNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if instance == nil {
		return &result, nil
	}
	result.InterfaceNewBusinessState = instance.NewBusinessState
	availability, availabilityErr := s.interfaceCatalogAvailability(ctx, []*entity.ProductAutomationInterfaceInstance{instance})
	if availabilityErr != nil {
		return nil, availabilityErr
	}
	result.CatalogAvailable = availability[instance.InterfaceInstanceNo]
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: row.PluginId, ReleaseVersion: row.PluginReleaseVersion,
		Point: row.ExtensionPointCode, ContractVersion: row.ContractVersion,
		ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest,
	})
	if err != nil || !sameProviderContract(bindingEvidence(row), exact) || !sameEvidence(bindingEvidence(row), interfaceEvidence(instance)) {
		return &result, nil
	}
	result.EvidenceVerified = true
	return &result, nil
}

// AuthorizeFulfillmentPolicyWrite verifies the exact historical provider
// contract after the binding-write permission has been granted.
func (s *serviceImpl) AuthorizeFulfillmentPolicyWrite(ctx context.Context, versionNo string) (catalog.FulfillmentPolicyWriteAuthorization, error) {
	authorization, err := s.authorizer.AuthorizeBindingWrite(ctx, versionNo)
	if err != nil {
		return catalog.FulfillmentPolicyWriteAuthorization{}, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return catalog.FulfillmentPolicyWriteAuthorization{}, err
	}
	contract, err := s.loadFrozenFulfillmentContract(ctx, versionNo)
	if err != nil {
		return catalog.FulfillmentPolicyWriteAuthorization{}, err
	}
	return catalog.FulfillmentPolicyWriteAuthorization{ActorUserID: authorization.ActorUserID, Contract: contract}, nil
}

// GetFrozenFulfillmentContract returns only release-frozen semantics and never
// accepts granularity or method names from a merchant request.
func (s *serviceImpl) GetFrozenFulfillmentContract(ctx context.Context, versionNo string) (*catalog.FrozenFulfillmentContract, error) {
	authorization, err := s.authorizer.AuthorizeBindingRead(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	return s.loadFrozenFulfillmentContract(ctx, versionNo)
}

func (s *serviceImpl) loadFrozenFulfillmentContract(ctx context.Context, versionNo string) (*catalog.FrozenFulfillmentContract, error) {
	if err := validateBusinessNo(versionNo, "ver"); err != nil {
		return nil, err
	}
	row, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		return nil, nil
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: row.PluginId, ReleaseVersion: row.PluginReleaseVersion,
		Point: row.ExtensionPointCode, ContractVersion: row.ContractVersion,
		ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest,
	})
	if err != nil || !sameProviderContract(bindingEvidence(row), exact) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	instance, err := s.loadInterface(ctx, row.InterfaceInstanceNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if instance == nil || !sameEvidence(bindingEvidence(row), interfaceEvidence(instance)) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	contract, err := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
	if err != nil {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	renewalMode, err := productfulfillmentcap.RenewalMode(contract)
	if err != nil {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	result := &catalog.FrozenFulfillmentContract{
		TrialPauseResume: contract.TrialPauseResume,
		Capability:       fulfillmentCapability(contract), Granularity: contract.Granularity,
		OngoingManagement: contract.OngoingManagement,
		RenewalMode:       renewalMode, RenewalMethod: contract.MethodRoles.Renewal,
		EntitlementOwnerID: contract.EntitlementOwnerID,
		ProvisionMethod:    contract.MethodRoles.Provision, QueryMethod: contract.MethodRoles.Query,
		DeclarationDigest: row.DeclarationDigest,
	}
	if err = s.applyImportedFulfillmentContract(ctx, versionNo, exact, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serviceImpl) loadBinding(ctx context.Context, versionNo string, lock bool) (*entity.CatalogProductVersionFulfillmentBinding, error) {
	model := s.database.Model(do.CatalogProductVersionFulfillmentBinding{}).Ctx(ctx).
		Where(do.CatalogProductVersionFulfillmentBinding{TenantId: idccap.DefaultTenantID, VersionNo: versionNo})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.CatalogProductVersionFulfillmentBinding
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load product automation binding")
	}
	return row, nil
}

func safeInterface(row *entity.ProductAutomationInterfaceInstance) InterfaceInstance {
	return InterfaceInstance{
		InterfaceInstanceNo: row.InterfaceInstanceNo, DisplayName: row.DisplayName,
		ReleaseVersion: row.PluginReleaseVersion, ContractVersion: row.ContractVersion,
		NewBusinessState: row.NewBusinessState, Version: row.Version, CreatedAt: automationTime(row.CreatedAt), ChangedAt: automationTime(row.ChangedAt),
		CanDelete: !row.EverReferenced,
	}
}

func automationTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func safeBinding(row *entity.CatalogProductVersionFulfillmentBinding) Binding {
	return Binding{
		VersionNo: row.VersionNo, InterfaceInstanceNo: row.InterfaceInstanceNo,
		InterfaceDisplayName: row.InterfaceDisplayNameSnapshot,
		ReleaseVersion:       row.PluginReleaseVersion, ContractVersion: row.ContractVersion, BoundAt: row.BoundAt,
	}
}

func bindingEvidence(row *entity.CatalogProductVersionFulfillmentBinding) *capmodel.BusinessExtensionProviderEvidence {
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: row.PluginId, DisplayName: row.InterfaceDisplayNameSnapshot,
		ReleaseVersion: row.PluginReleaseVersion, Point: row.ExtensionPointCode,
		ContractVersion: row.ContractVersion, AuthorizedMethods: row.AuthorizedMethods,
		ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest,
	}
}
