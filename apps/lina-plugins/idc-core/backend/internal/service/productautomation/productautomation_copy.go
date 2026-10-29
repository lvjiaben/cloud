// This file copies frozen automation facts under exact staff authorization and
// new-business admission for new products. Same-product revisions preserve local
// frozen facts without provider admission; publication still revalidates them.

package productautomation

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// ProductCopyService owns copy admission and version-specific file references.
type ProductCopyService interface {
	// WithProductCopy creates automation facts only after the catalog callback
	// returns a new draft version. revision is only for a same-product editing
	// draft, whose identity is enforced by the catalog writer. It preserves frozen
	// facts without admitting new business; publication retains its own gates.
	// Missing permissions, source changes and audit errors roll back all writes.
	WithProductCopy(context.Context, string, bool, func(context.Context) (string, error)) error
}

// ProductCopyServiceFromService reuses the existing startup service instance.
func ProductCopyServiceFromService(service Service) (ProductCopyService, error) {
	result, ok := service.(ProductCopyService)
	if !ok || result == nil {
		return nil, gerror.New("product automation copy service is unavailable")
	}
	return result, nil
}

func (s *serviceImpl) WithProductCopy(ctx context.Context, source string, revision bool, action func(context.Context) (string, error)) error {
	if action == nil || validateBusinessNo(source, "ver") != nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	authorization, binding, err := s.authorizeProductCopy(ctx, source)
	if err != nil {
		return err
	}
	copyAction := func(copyCtx context.Context) error {
		return s.database.Transaction(copyCtx, func(txCtx context.Context, _ gdb.TX) error {
			// Catalog locks product/version before automation rows, matching edits.
			target, writeErr := action(txCtx)
			if writeErr != nil {
				return writeErr
			}
			if target == "" {
				return bizerr.NewCode(catalog.CodeManagementInvalid)
			}

			if checkErr := s.checkCopyAdmission(txCtx, source, binding, authorization.ActorUserID, revision); checkErr != nil {
				return checkErr
			}

			current, loadErr := s.loadBinding(txCtx, source, true)
			if loadErr != nil {
				return loadErr
			}
			if (current == nil) != (binding == nil) || (current != nil && (current.Id != binding.Id || !sameEvidence(bindingEvidence(current), bindingEvidence(binding)))) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			var configuration *entity.CatalogProductVersionConfiguration
			if loadErr = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).Where(do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: source}).LockUpdate().Scan(&configuration); loadErr != nil {
				return loadErr
			}
			if configuration != nil && current == nil {
				if _, readErr := s.readManualConfiguration(txCtx, source); readErr != nil {
					return readErr
				}
			}
			if writeErr = s.lockDraftVersion(txCtx, target); writeErr != nil {
				return writeErr
			}
			if current != nil {
				result, insertErr := s.database.Model(do.CatalogProductVersionFulfillmentBinding{}).Ctx(txCtx).Data(do.CatalogProductVersionFulfillmentBinding{TenantId: 0, VersionNo: target, InterfaceInstanceNo: current.InterfaceInstanceNo, InterfaceDisplayNameSnapshot: current.InterfaceDisplayNameSnapshot, PluginId: current.PluginId, ExtensionPointCode: current.ExtensionPointCode, PluginReleaseVersion: current.PluginReleaseVersion, ContractVersion: current.ContractVersion, AuthorizedMethods: current.AuthorizedMethods, ReleaseDigest: current.ReleaseDigest, DeclarationDigest: current.DeclarationDigest, BoundBy: authorization.ActorUserID, BoundAt: gdb.Raw("CURRENT_TIMESTAMP")}).Insert()
				if insertErr != nil {
					return insertErr
				}
				count, countErr := result.RowsAffected()
				if countErr != nil || count != 1 {
					return bizerr.NewCode(catalog.CodeManagementConflict)
				}
			}
			if configuration != nil {
				if writeErr = s.copyVersionConfiguration(txCtx, configuration, target, authorization.ActorUserID); writeErr != nil {
					return writeErr
				}
			}
			if current == nil {
				return nil
			}
			return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
				IdempotencyScope: "product-automation.product.copy", IdempotencyKey: target,
				OperationCode: "product-automation.product.copy", TargetType: "catalog_product_version", TargetNo: target,
				ResultCode: "succeeded", ReasonCode: "manual_copy", RiskCode: "medium", RequiredPermission: authorization.RequiredPermission,
				BeforeSnapshot: map[string]any{"source_version_no": source},
				AfterSnapshot:  map[string]any{"interface_instance_no": current.InterfaceInstanceNo, "configuration_copied": configuration != nil, "editing_revision": revision},
			})
		})
	}
	if binding == nil || revision {
		return normalizeManagementWriteError(copyAction(ctx))
	}
	return normalizeManagementWriteError(s.extensions.WithNewBusinessAdmission(ctx, admissionInput(bindingEvidence(binding)), func(guardCtx context.Context, exact *capmodel.BusinessExtensionProviderEvidence) error {
		if !sameProviderContract(bindingEvidence(binding), exact) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		instance, loadErr := s.loadInterface(guardCtx, binding.InterfaceInstanceNo, false)
		if loadErr != nil {
			return loadErr
		}
		if instance == nil || instance.NewBusinessState != InterfaceStateAccepting || !sameProviderContract(interfaceEvidence(instance), exact) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return copyAction(guardCtx)
	}))
}

// authorizeProductCopy keeps commercial-definition authority independent of
// supplier administration; a bound source retains both original checks.
func (s *serviceImpl) authorizeProductCopy(ctx context.Context, source string) (Authorization, *entity.CatalogProductVersionFulfillmentBinding, error) {
	configurationAuth, err := s.authorizer.AuthorizeVersionConfiguration(ctx, source)
	if err != nil || requireAuthorization(configurationAuth) != nil {
		return Authorization{}, nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	binding, err := s.loadBinding(ctx, source, false)
	if err != nil {
		return Authorization{}, nil, err
	}
	if binding == nil {
		return configurationAuth, nil, nil
	}
	authorization, err := s.authorizer.AuthorizeBindingWrite(ctx, source)
	if err != nil || requireAuthorization(authorization) != nil || configurationAuth.ActorUserID != authorization.ActorUserID {
		return Authorization{}, nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return authorization, binding, nil
}

func (s *serviceImpl) copyVersionConfiguration(ctx context.Context, source *entity.CatalogProductVersionConfiguration, target string, actor int) error {
	values, digest, err := s.copyProductConfigurationFiles(ctx, source, target, actor)
	if err != nil {
		return err
	}
	data := do.CatalogProductVersionConfiguration{TenantId: 0, VersionNo: target, InterfaceInstanceNo: source.InterfaceInstanceNo, InterfaceConfigurationRevision: source.InterfaceConfigurationRevision, ProductSchemaVersion: source.ProductSchemaVersion, ProductDeclarationSnapshot: source.ProductDeclarationSnapshot, MerchantValueSnapshot: values, CustomerSchemaVersion: source.CustomerSchemaVersion, CustomerDeclarationSnapshot: source.CustomerDeclarationSnapshot, CustomerPresentationSnapshot: source.CustomerPresentationSnapshot, DeclarationDigest: source.DeclarationDigest, MerchantValueDigest: digest, Version: source.Version, ChangedBy: actor}
	if source.InterfaceInstanceNo == "" {
		if source.InterfaceConfigurationRevision != 0 {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		data.InterfaceInstanceNo, data.InterfaceConfigurationRevision = gdb.Raw("NULL"), gdb.Raw("NULL")
	}
	result, err := s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).Data(data).Insert()
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}

// checkCopyAdmission repeats staff checks and locks the interface before its
// binding, following the same lock order used by normal binding changes.
func (s *serviceImpl) checkCopyAdmission(ctx context.Context, source string, binding *entity.CatalogProductVersionFulfillmentBinding, actor int, revision bool) error {
	configuration, err := s.authorizer.AuthorizeVersionConfiguration(ctx, source)
	if err != nil || requireAuthorization(configuration) != nil || configuration.ActorUserID != actor {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	if binding == nil {
		return nil
	}
	authorization, err := s.authorizer.AuthorizeBindingWrite(ctx, source)
	if err != nil || requireAuthorization(authorization) != nil || authorization.ActorUserID != actor {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	instance, err := s.loadInterface(ctx, binding.InterfaceInstanceNo, true)
	if err != nil {
		return err
	}
	if instance == nil || instance.PluginId != binding.PluginId || instance.ExtensionPointCode != binding.ExtensionPointCode {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	// A stopped or upgraded interface may be repaired in a local editing draft.
	// Keep the historical binding unchanged; do not attest it as newly admitted.
	if !revision && (instance.NewBusinessState != InterfaceStateAccepting || !sameEvidence(interfaceEvidence(instance), bindingEvidence(binding))) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}
