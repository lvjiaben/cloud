// This file implements host-admitted interface creation and draft binding writes.

package productautomation

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provideradmissioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// CreateInterface decodes a trusted selection, then commits the immutable row
// and audit while the host admission guard retains its lifecycle lock.
func (s *serviceImpl) CreateInterface(ctx context.Context, input CreateInterfaceInput) (*InterfaceInstance, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceCreate(ctx)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if err = validateDisplayName(input.DisplayName); err != nil || input.SelectionToken == "" || len(input.SelectionToken) > 4096 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	selected, err := s.codec.DecodeProviderSelection(ctx, input.SelectionToken)
	if errors.Is(err, ErrInvalidSelectionToken) {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if !validFrozenEvidence(selected) {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	// The bounded session token carries immutable identity, not executable
	// schemas. Reload the exact public declaration before compiling any input.
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: selected.PluginID, ReleaseVersion: selected.ReleaseVersion, Point: selected.Point,
		ContractVersion: selected.ContractVersion, ReleaseDigest: selected.ReleaseDigest,
		DeclarationDigest: selected.DeclarationDigest,
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if !sameEvidence(selected, exact) || !validProviderEvidence(exact) {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	selected = exact
	interfaceNo, err := newInterfaceNo(s.random)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	compiled, err := s.compileConfigurationResolved(
		ctx, selected, selected.InterfaceConfiguration, input.Configuration,
		configurationSurfaceInterface, dynamicCandidateScope{},
	)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	storedSecrets, secretOperation, err := s.createConfigurationSecrets(ctx, interfaceNo, compiled.SecretValues)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var result *InterfaceInstance
	err = s.extensions.WithNewBusinessAdmission(ctx, admissionInput(selected), func(guardCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
		if !sameProviderContract(selected, evidence) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return s.database.Transaction(guardCtx, func(txCtx context.Context, _ gdb.TX) error {
			methods := append([]string{}, evidence.AuthorizedMethods...)
			insert, insertErr := s.database.Model(do.ProductAutomationInterfaceInstance{}).Ctx(txCtx).Data(do.ProductAutomationInterfaceInstance{
				TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: interfaceNo, DisplayName: input.DisplayName,
				PluginId: evidence.PluginID, ExtensionPointCode: evidence.Point,
				PluginReleaseVersion: evidence.ReleaseVersion, ContractVersion: evidence.ContractVersion,
				AuthorizedMethods: methods, ReleaseDigest: evidence.ReleaseDigest,
				DeclarationDigest: evidence.DeclarationDigest, NewBusinessState: InterfaceStateAccepting,
				Version: 1, CreatedBy: authorization.ActorUserID, ChangedBy: authorization.ActorUserID,
			}).Insert()
			if insertErr != nil {
				return gerror.Wrap(insertErr, "insert product automation interface")
			}
			if affected, affectedErr := insert.RowsAffected(); affectedErr != nil {
				return gerror.Wrap(affectedErr, "confirm product automation interface insert")
			} else if affected != 1 {
				return gerror.New("product automation interface insert affected an unexpected row count")
			}
			if persistErr := s.persistInterfaceConfiguration(
				txCtx, interfaceNo, authorization.ActorUserID, compiled,
				configurationSchemaVersion(selected.InterfaceConfiguration), storedSecrets,
			); persistErr != nil {
				return persistErr
			}
			if commitErr := s.commitInterfaceSecretOperation(txCtx, secretOperation); commitErr != nil {
				return commitErr
			}
			if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
				IdempotencyScope: "product-automation.interface.create", IdempotencyKey: interfaceNo,
				OperationCode: "product-automation.interface.create", TargetType: "product_automation_interface", TargetNo: interfaceNo,
				ResultCode: "succeeded", ReasonCode: "manual_create", RiskCode: "medium",
				RequiredPermission: authorization.RequiredPermission,
				BeforeSnapshot:     map[string]any{}, AfterSnapshot: map[string]any{"display_name": input.DisplayName},
			}); auditErr != nil {
				return gerror.Wrap(auditErr, "append product automation interface audit")
			}
			row, loadErr := s.loadInterface(txCtx, interfaceNo, false)
			if loadErr != nil {
				return loadErr
			}
			if row == nil {
				return gerror.New("product automation interface disappeared after insert")
			}
			projection := safeInterface(row)
			result = &projection
			return nil
		})
	})
	if err != nil {
		pendingErr := s.markInterfaceSecretOperationPending(ctx, secretOperation, err)
		if cleanupErr := s.compensateInterfaceSecretOperation(ctx, secretOperation); cleanupErr != nil || pendingErr != nil {
			return nil, bizerr.WrapCode(
				errors.Join(err, pendingErr, cleanupErr),
				catalog.CodeManagementUnavailable,
			)
		}
		if bizerr.Is(err, catalog.CodeManagementConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return result, nil
}

// RenameInterface changes only the merchant-facing name. Existing bindings
// retain their frozen name snapshot.
func (s *serviceImpl) RenameInterface(ctx context.Context, input RenameInterfaceInput) (*InterfaceInstance, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceRename(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if validateDisplayName(input.DisplayName) != nil || input.ExpectedVersion <= 0 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, input.InterfaceInstanceNo)
	if err != nil || resourceOwned {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	var output *InterfaceInstance
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		row, loadErr := s.loadInterface(txCtx, input.InterfaceInstanceNo, true)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		if row.Version != input.ExpectedVersion {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if row.DisplayName == input.DisplayName {
			value := safeInterface(row)
			output = &value
			return nil
		}
		result, updateErr := s.database.Model(do.ProductAutomationInterfaceInstance{}).Ctx(txCtx).
			Where(do.ProductAutomationInterfaceInstance{TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: row.InterfaceInstanceNo, Version: row.Version}).
			Data(do.ProductAutomationInterfaceInstance{DisplayName: input.DisplayName, Version: row.Version + 1, ChangedBy: authorization.ActorUserID}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "rename product automation interface")
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "product-automation.interface.rename", IdempotencyKey: fmt.Sprintf("%s:%d", row.InterfaceInstanceNo, row.Version+1), OperationCode: "product-automation.interface.rename", TargetType: "product_automation_interface", TargetNo: row.InterfaceInstanceNo, ResultCode: "succeeded", ReasonCode: "manual_rename", RiskCode: "medium", RequiredPermission: authorization.RequiredPermission, BeforeSnapshot: map[string]any{"display_name": row.DisplayName, "version": row.Version}, AfterSnapshot: map[string]any{"display_name": input.DisplayName, "version": row.Version + 1}}); auditErr != nil {
			return auditErr
		}
		updated, loadErr := s.loadInterface(txCtx, row.InterfaceInstanceNo, false)
		if loadErr != nil {
			return loadErr
		}
		value := safeInterface(updated)
		output = &value
		return nil
	})
	if err != nil {
		return nil, normalizeManagementWriteError(err)
	}
	return output, nil
}

// DeleteInterface deletes only an interface that has never been referenced.
func (s *serviceImpl) DeleteInterface(ctx context.Context, input DeleteInterfaceInput) error {
	authorization, err := s.authorizer.AuthorizeInterfaceDelete(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if input.ExpectedVersion <= 0 {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, input.InterfaceInstanceNo)
	if err != nil || resourceOwned {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		row, loadErr := s.loadInterface(txCtx, input.InterfaceInstanceNo, true)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		if row.Version != input.ExpectedVersion || row.EverReferenced {
			if row.EverReferenced {
				return bizerr.NewCode(CodeInterfaceEverReferenced)
			}
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if revokeErr := s.revokeUnusedInterfaceSecrets(txCtx, row.InterfaceInstanceNo); revokeErr != nil {
			return revokeErr
		}
		result, deleteErr := s.database.Model(do.ProductAutomationInterfaceInstance{}).Ctx(txCtx).Where(do.ProductAutomationInterfaceInstance{TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: row.InterfaceInstanceNo, Version: row.Version, EverReferenced: false}).Delete()
		if deleteErr != nil {
			return gerror.Wrap(deleteErr, "delete product automation interface")
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "product-automation.interface.delete", IdempotencyKey: row.InterfaceInstanceNo, OperationCode: "product-automation.interface.delete", TargetType: "product_automation_interface", TargetNo: row.InterfaceInstanceNo, ResultCode: "succeeded", ReasonCode: "mistaken_create", RiskCode: "high", RequiredPermission: authorization.RequiredPermission, BeforeSnapshot: map[string]any{"display_name": row.DisplayName, "version": row.Version}, AfterSnapshot: map[string]any{}})
	})
	return normalizeManagementWriteError(err)
}

// UpdateNewBusinessState applies the API's optimistic-lock contract.
func (s *serviceImpl) UpdateNewBusinessState(ctx context.Context, input InterfaceStateInput) (*InterfaceInstance, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceRename(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 || (input.NewBusinessState != InterfaceStateAccepting && input.NewBusinessState != InterfaceStateStopped) {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, input.InterfaceInstanceNo)
	if err != nil || resourceOwned {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	apply := func(applyCtx context.Context) error {
		return s.database.Transaction(applyCtx, func(txCtx context.Context, _ gdb.TX) error {
			row, loadErr := s.loadInterface(txCtx, input.InterfaceInstanceNo, true)
			if loadErr != nil {
				return loadErr
			}
			if row == nil {
				return bizerr.NewCode(catalog.CodeManagementNotAccessible)
			}
			if row.Version != input.ExpectedVersion {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if row.NewBusinessState == input.NewBusinessState {
				return nil
			}
			return s.transitionInterfaceState(txCtx, authorization, row, input.NewBusinessState, "manual_state_change")
		})
	}
	if input.NewBusinessState == InterfaceStateAccepting {
		row, loadErr := s.loadInterface(ctx, input.InterfaceInstanceNo, false)
		if loadErr != nil {
			return nil, bizerr.WrapCode(loadErr, catalog.CodeManagementUnavailable)
		}
		if row == nil {
			return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		selected := interfaceEvidence(row)
		_, exact, resolveErr := s.loadExactInterfaceEvidence(ctx, input.InterfaceInstanceNo)
		if resolveErr != nil || !sameProviderContract(selected, exact) {
			return nil, bizerr.NewCode(CodeEvidenceUnavailable)
		}
		resolved, resolveErr := s.resolveCurrentInterfaceCandidates(ctx, input.InterfaceInstanceNo, exact)
		if resolveErr != nil {
			return nil, bizerr.WrapCode(resolveErr, CodeEvidenceUnavailable)
		}
		err = s.extensions.WithNewBusinessAdmission(ctx, admissionInput(selected), func(guardCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
			if !sameProviderContract(selected, evidence) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if validationErr := s.validateCurrentInterfaceConfigurationWithCandidates(guardCtx, input.InterfaceInstanceNo, evidence, resolved); validationErr != nil {
				return validationErr
			}
			return apply(guardCtx)
		})
	} else {
		err = apply(ctx)
	}
	if err != nil {
		if input.NewBusinessState == InterfaceStateAccepting && !bizerr.Is(err, catalog.CodeManagementNotAccessible) && !bizerr.Is(err, catalog.CodeManagementConflict) && !bizerr.Is(err, catalog.CodeManagementInvalid) {
			return nil, bizerr.WrapCode(err, CodeEvidenceUnavailable)
		}
		return nil, normalizeManagementWriteError(err)
	}
	row, err := s.loadInterface(ctx, input.InterfaceInstanceNo, false)
	if err != nil || row == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	value := safeInterface(row)
	return &value, nil
}

// Bind retains an unchanged draft binding locally. New or different bindings
// require host admission and the product-version row lock through audit commit.
func (s *serviceImpl) Bind(ctx context.Context, input BindInput) (*Binding, error) {
	authorization, err := s.authorizer.AuthorizeBindingWrite(ctx, input.VersionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if err = validateBusinessNo(input.VersionNo, "ver"); err != nil {
		return nil, err
	}
	if err = validateBusinessNo(input.InterfaceInstanceNo, "pfi"); err != nil {
		return nil, err
	}
	unchanged, err := s.retainDraftBinding(ctx, input)
	if err != nil {
		return nil, normalizeManagementWriteError(err)
	}
	if unchanged != nil {
		return unchanged, nil
	}
	detached, err := s.prepareDetachedOffer(ctx, input.VersionNo, authorization.ActorUserID)
	if err != nil {
		return nil, err
	}
	instance, err := s.loadInterface(ctx, input.InterfaceInstanceNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if instance == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	selected := interfaceEvidence(instance)
	_, exact, err := s.loadExactInterfaceEvidence(ctx, input.InterfaceInstanceNo)
	if err != nil || !sameProviderContract(selected, exact) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	resolved, err := s.resolveCurrentInterfaceCandidates(ctx, input.InterfaceInstanceNo, exact)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeEvidenceUnavailable)
	}
	var result *Binding
	err = s.extensions.WithNewBusinessAdmission(ctx, admissionInput(selected), func(guardCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
		if !sameProviderContract(selected, evidence) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if validationErr := s.validateCurrentInterfaceConfigurationWithCandidates(guardCtx, input.InterfaceInstanceNo, evidence, resolved); validationErr != nil {
			return validationErr
		}
		return s.database.Transaction(guardCtx, func(txCtx context.Context, _ gdb.TX) error {
			if lockErr := s.lockDraftVersion(txCtx, input.VersionNo); lockErr != nil {
				return lockErr
			}
			lockedInstance, loadErr := s.loadInterface(txCtx, input.InterfaceInstanceNo, true)
			if loadErr != nil {
				return loadErr
			}
			if lockedInstance == nil || lockedInstance.NewBusinessState != InterfaceStateAccepting || !sameEvidence(selected, interfaceEvidence(lockedInstance)) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			before, loadErr := s.loadBinding(txCtx, input.VersionNo, true)
			if loadErr != nil {
				return loadErr
			}
			if before != nil && before.InterfaceInstanceNo == input.InterfaceInstanceNo {
				projection := safeBinding(before)
				projection.InterfaceNewBusinessState = lockedInstance.NewBusinessState
				projection.EvidenceVerified = true
				result = &projection
				return nil
			}
			if clearErr := s.clearDraftFulfillmentPolicy(txCtx, input.VersionNo); clearErr != nil {
				return clearErr
			}
			if before != nil {
				if detachErr := s.detachDraftCommercialConfiguration(txCtx, input.VersionNo, authorization.ActorUserID, detached); detachErr != nil {
					return detachErr
				}
				deleteResult, deleteErr := s.database.Model(do.CatalogProductVersionFulfillmentBinding{}).Ctx(txCtx).
					Where(do.CatalogProductVersionFulfillmentBinding{TenantId: idccap.DefaultTenantID, VersionNo: input.VersionNo}).Delete()
				if deleteErr != nil {
					return gerror.Wrap(deleteErr, "replace product automation binding")
				}
				if affected, affectedErr := deleteResult.RowsAffected(); affectedErr != nil {
					return gerror.Wrap(affectedErr, "confirm replaced product automation binding delete")
				} else if affected != 1 {
					return gerror.New("replaced product automation binding delete affected an unexpected row count")
				}
			}
			// Keep the selected local interface snapshot. Host admission above
			// uses the current executable; an upgrade must not invent a new
			// interface tuple and violate the database binding invariant.
			methods := append([]string{}, lockedInstance.AuthorizedMethods...)
			_, writeErr := s.database.Model(do.CatalogProductVersionFulfillmentBinding{}).Ctx(txCtx).
				Data(do.CatalogProductVersionFulfillmentBinding{
					TenantId: idccap.DefaultTenantID, VersionNo: input.VersionNo,
					InterfaceInstanceNo: input.InterfaceInstanceNo, InterfaceDisplayNameSnapshot: lockedInstance.DisplayName,
					PluginId: lockedInstance.PluginId, ExtensionPointCode: lockedInstance.ExtensionPointCode,
					PluginReleaseVersion: lockedInstance.PluginReleaseVersion, ContractVersion: lockedInstance.ContractVersion,
					AuthorizedMethods: methods, ReleaseDigest: lockedInstance.ReleaseDigest,
					DeclarationDigest: lockedInstance.DeclarationDigest, BoundBy: authorization.ActorUserID,
					BoundAt: gdb.Raw("CURRENT_TIMESTAMP"),
				}).Insert()
			if writeErr != nil {
				return gerror.Wrap(writeErr, "insert product automation binding")
			}
			bound, loadErr := s.loadBinding(txCtx, input.VersionNo, false)
			if loadErr != nil {
				return loadErr
			}
			if bound == nil {
				return gerror.New("product automation binding disappeared after insert")
			}
			if auditErr := s.audit.AppendBusinessTransition(txCtx, bindingAudit(authorization, input, before, lockedInstance, bound.Id)); auditErr != nil {
				return gerror.Wrap(auditErr, "append product automation binding audit")
			}
			projection := safeBinding(bound)
			projection.InterfaceNewBusinessState = lockedInstance.NewBusinessState
			projection.EvidenceVerified = true
			result = &projection
			return nil
		})
	})
	if err != nil {
		if bizerr.Is(err, catalog.CodeManagementConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return result, nil
}

// retainDraftBinding recognizes an unchanged selection under the normal lock
// order. It does not admit a new binding, rewrite configuration, clear policy or
// attest historical evidence. Actual interface changes still use Bind's gates.
func (s *serviceImpl) retainDraftBinding(ctx context.Context, input BindInput) (*Binding, error) {
	var result *Binding
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if err := s.lockDraftVersion(txCtx, input.VersionNo); err != nil {
			return err
		}
		instance, err := s.loadInterface(txCtx, input.InterfaceInstanceNo, true)
		if err != nil {
			return err
		}
		if instance == nil {
			return bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		binding, err := s.loadBinding(txCtx, input.VersionNo, true)
		if err != nil {
			return err
		}
		if binding == nil || binding.InterfaceInstanceNo != input.InterfaceInstanceNo {
			return nil
		}
		projection := safeBinding(binding)
		projection.InterfaceNewBusinessState = instance.NewBusinessState
		result = &projection
		return nil
	})
	return result, err
}

// Unbind removes only a draft-version binding and appends audit in the same transaction.
func (s *serviceImpl) Unbind(ctx context.Context, versionNo string) error {
	authorization, err := s.authorizer.AuthorizeBindingWrite(ctx, versionNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if err = validateBusinessNo(versionNo, "ver"); err != nil {
		return err
	}
	detached, err := s.prepareDetachedOffer(ctx, versionNo, authorization.ActorUserID)
	if err != nil {
		return err
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if lockErr := s.lockDraftVersion(txCtx, versionNo); lockErr != nil {
			return lockErr
		}
		before, loadErr := s.loadBinding(txCtx, versionNo, true)
		if loadErr != nil {
			return loadErr
		}
		if before == nil {
			// The same DELETE may be retried after the first transaction committed
			// but before its response reached the caller. Authorization and the
			// draft-version lock above still hide inaccessible targets; an already
			// absent binding is therefore a successful no-op and must not append a
			// second audit transition.
			return nil
		}
		if clearErr := s.clearDraftFulfillmentPolicy(txCtx, versionNo); clearErr != nil {
			return clearErr
		}
		if detachErr := s.detachDraftCommercialConfiguration(txCtx, versionNo, authorization.ActorUserID, detached); detachErr != nil {
			return detachErr
		}
		result, deleteErr := s.database.Model(do.CatalogProductVersionFulfillmentBinding{}).Ctx(txCtx).
			Where(do.CatalogProductVersionFulfillmentBinding{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Delete()
		if deleteErr != nil {
			return gerror.Wrap(deleteErr, "delete draft product automation binding")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil {
			return gerror.Wrap(affectedErr, "confirm product automation binding delete")
		} else if affected != 1 {
			return gerror.New("product automation binding delete affected an unexpected row count")
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-automation.binding.unbind", IdempotencyKey: bindingAuditIdentity(versionNo, before.Id),
			OperationCode: "product-automation.binding.unbind", TargetType: "catalog_product_version", TargetNo: versionNo,
			ResultCode: "succeeded", ReasonCode: "manual_unbind", RiskCode: "medium",
			RequiredPermission: authorization.RequiredPermission,
			BeforeSnapshot:     map[string]any{"interface_instance_no": before.InterfaceInstanceNo}, AfterSnapshot: map[string]any{},
		})
	})
	if err != nil {
		if bizerr.Is(err, catalog.CodeManagementNotAccessible) || bizerr.Is(err, catalog.CodeManagementConflict) {
			return err
		}
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return nil
}

// StopNewBusiness idempotently stops future bindings and publications. The
// interface identity and all historical bindings remain intact.
func (s *serviceImpl) StopNewBusiness(ctx context.Context, interfaceNo string) error {
	authorization, err := s.authorizer.AuthorizeInterfaceStop(ctx, interfaceNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if err = validateBusinessNo(interfaceNo, "pfi"); err != nil {
		return err
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		instance, loadErr := s.loadInterface(txCtx, interfaceNo, true)
		if loadErr != nil {
			return loadErr
		}
		if instance == nil {
			return bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		if instance.NewBusinessState == InterfaceStateStopped {
			return nil
		}
		return s.transitionInterfaceState(txCtx, authorization, instance, InterfaceStateStopped, "manual_stop")
	})
	return normalizeManagementWriteError(err)
}

// ResumeNewBusiness idempotently resumes only after the host has revalidated
// the exact frozen plugin release and declaration evidence.
func (s *serviceImpl) ResumeNewBusiness(ctx context.Context, interfaceNo string) error {
	authorization, err := s.authorizer.AuthorizeInterfaceResume(ctx, interfaceNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if err = validateBusinessNo(interfaceNo, "pfi"); err != nil {
		return err
	}
	instance, err := s.loadInterface(ctx, interfaceNo, false)
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if instance == nil {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	selected := interfaceEvidence(instance)
	_, exact, err := s.loadExactInterfaceEvidence(ctx, interfaceNo)
	if err != nil || !sameProviderContract(selected, exact) {
		return bizerr.NewCode(CodeEvidenceUnavailable)
	}
	resolved, err := s.resolveCurrentInterfaceCandidates(ctx, interfaceNo, exact)
	if err != nil {
		return bizerr.WrapCode(err, CodeEvidenceUnavailable)
	}
	err = s.extensions.WithNewBusinessAdmission(ctx, admissionInput(selected), func(guardCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
		if !sameProviderContract(selected, evidence) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if validationErr := s.validateCurrentInterfaceConfigurationWithCandidates(guardCtx, interfaceNo, evidence, resolved); validationErr != nil {
			return validationErr
		}
		return s.database.Transaction(guardCtx, func(txCtx context.Context, _ gdb.TX) error {
			locked, loadErr := s.loadInterface(txCtx, interfaceNo, true)
			if loadErr != nil {
				return loadErr
			}
			if locked == nil || !sameEvidence(selected, interfaceEvidence(locked)) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if locked.NewBusinessState == InterfaceStateAccepting {
				return nil
			}
			return s.transitionInterfaceState(txCtx, authorization, locked, InterfaceStateAccepting, "manual_resume")
		})
	})
	return normalizeManagementWriteError(err)
}

// WithPublicationAdmission serializes publication with binding and interface
// state changes, then runs action using the transaction context holding locks.
func (s *serviceImpl) WithPublicationAdmission(ctx context.Context, versionNo string, action func(context.Context) error) error {
	authorization, err := s.authorizer.AuthorizePublicationAdmission(ctx, versionNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if err = validateBusinessNo(versionNo, "ver"); err != nil {
		return err
	}
	if action == nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return normalizeManagementWriteError(s.withPublicationBindingAdmission(ctx, versionNo, action))
}

// CheckPublicationReadiness performs the same frozen-evidence, current-host,
// and accepting-interface checks as final publication, but commits no state.
func (s *serviceImpl) CheckPublicationReadiness(ctx context.Context, versionNo string) (catalog.PublicationReadinessCheck, error) {
	check := catalog.PublicationReadinessCheck{Code: catalog.PublicationCheckAutomation}
	authorization, err := s.authorizer.AuthorizePublicationAdmission(ctx, versionNo)
	if err != nil {
		return check, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return check, err
	}
	if err = validateBusinessNo(versionNo, "ver"); err != nil {
		return check, err
	}
	err = normalizeManagementWriteError(s.withPublicationBindingAdmission(
		ctx, versionNo, func(context.Context) error { return nil },
	))
	if err == nil {
		check.Passed = true
		return check, nil
	}
	check.ReasonCode = publicationReadinessReason(err)
	if check.ReasonCode != "" {
		return check, nil
	}
	return check, err
}

// publicationReadinessReason preserves business causes instead of reporting
// every failed automation check as a missing purchase-option price.
func publicationReadinessReason(err error) catalog.PublicationReadinessReason {
	switch {
	case bizerr.Is(err, CodeEvidenceUnavailable):
		return catalog.PublicationReasonProviderUnavailable
	case bizerr.Is(err, codeCustomerOptionPricesRequired):
		return catalog.PublicationReasonConfigurationPricesMissing
	case bizerr.Is(err, codeSupplierCapabilitiesUnconfirmed):
		return catalog.PublicationReasonSupplierUnconfirmed
	case bizerr.Is(err, catalog.CodeManagementConflict), bizerr.Is(err, catalog.CodeManagementNotAccessible):
		return catalog.PublicationReasonConfigurationIncomplete
	default:
		return ""
	}
}

// publicPurchaseEvidence belongs to one read only; it is never cached across
// requests, so provider disablement and interface changes remain authoritative.
type publicPurchaseEvidence struct {
	binding       *entity.CatalogProductVersionFulfillmentBinding
	configuration *entity.CatalogProductVersionConfiguration
	provider      *capmodel.BusinessExtensionProviderEvidence
}

// CheckPublicPurchaseAvailability reads active host discovery and immutable local
// facts without claiming a lifecycle mutation lock. Concurrent catalog reads must
// not reject each other. Actual provider calls and final order admission retain
// their host lifecycle guards; no enabled-state or contract result is cached.
func (s *serviceImpl) CheckPublicPurchaseAvailability(ctx context.Context, versionNo string) (bool, error) {
	evidence, err := s.loadPublicPurchaseEvidence(ctx, versionNo)
	return evidence != nil, err
}

// loadPublicPurchaseEvidence validates current availability and retains the
// exact active evidence for candidate resolution within the same read.
func (s *serviceImpl) loadPublicPurchaseEvidence(ctx context.Context, versionNo string) (*publicPurchaseEvidence, error) {
	if err := validateBusinessNo(versionNo, "ver"); err != nil {
		return nil, err
	}
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return &publicPurchaseEvidence{}, nil
	}
	page, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point: binding.ExtensionPointCode, ContractVersion: binding.ContractVersion,
		Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100},
	})
	if err != nil || page == nil {
		return nil, bizerr.WrapCode(err, CodeEvidenceUnavailable)
	}
	selected := bindingEvidence(binding)
	var active *capmodel.BusinessExtensionProviderEvidence
	for _, evidence := range page.Items {
		if evidence != nil && evidence.PluginID == binding.PluginId && sameProviderContract(selected, evidence) {
			active = evidence
			break
		}
	}
	if active == nil {
		return nil, nil
	}
	var version struct {
		StatusCode string `orm:"status_code"`
	}
	if err = s.database.Model(do.CatalogProductVersion{}).Ctx(ctx).Fields("status_code").Where(do.CatalogProductVersion{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Scan(&version); err != nil {
		return nil, gerror.Wrap(err, "load public purchase product version")
	}
	if version.StatusCode != "published" {
		return nil, nil
	}
	instance, err := s.loadInterface(ctx, binding.InterfaceInstanceNo, false)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.NewBusinessState != InterfaceStateAccepting || !sameEvidence(selected, interfaceEvidence(instance)) {
		return nil, nil
	}
	configuration, err := s.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil || configuration == nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementConflict)
	}
	revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, binding.InterfaceInstanceNo, false)
	if err != nil {
		return nil, err
	}
	err = s.validateConfigurationInterfaceRevision(ctx, binding.InterfaceInstanceNo, configuration, revision)
	if err == nil {
		return &publicPurchaseEvidence{binding: binding, configuration: configuration, provider: active}, nil
	}
	if bizerr.Is(err, catalog.CodeManagementConflict) || bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		return nil, nil
	}
	return nil, err
}

// WithSaleStateAdmission permits local stopping even when a provider is offline.
// Resuming uses the same exact provider evidence as publication; the catalog
// callback locks and rechecks the current product pointer before changing state.
func (s *serviceImpl) WithSaleStateAdmission(ctx context.Context, versionNo string, active bool, action func(context.Context) error) error {
	authorization, err := s.authorizer.AuthorizePublicationAdmission(ctx, versionNo)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if validateBusinessNo(versionNo, "ver") != nil || action == nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	if !active {
		return normalizeManagementWriteError(s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return action(ctx) }))
	}
	return normalizeManagementWriteError(s.withPublicationBindingAdmissionState(ctx, versionNo, action, true))
}

func (s *serviceImpl) withPublicationBindingAdmission(
	ctx context.Context,
	versionNo string,
	action func(context.Context) error,
) error {
	return s.withPublicationBindingAdmissionState(ctx, versionNo, action, false)
}

func (s *serviceImpl) withPublicationBindingAdmissionState(ctx context.Context, versionNo string, action func(context.Context) error, published bool) error {
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return err
	}
	if binding == nil {
		err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			if lockErr := s.checkPublicationVersionState(txCtx, versionNo, published); lockErr != nil {
				return lockErr
			}
			current, loadErr := s.loadBinding(txCtx, versionNo, true)
			if loadErr != nil {
				return loadErr
			}
			if current != nil {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if policyErr := s.validateFrozenFulfillmentPolicy(txCtx, versionNo, nil); policyErr != nil {
				return policyErr
			}
			configuration, configurationErr := s.readManualConfiguration(txCtx, versionNo)
			if configurationErr != nil {
				return configurationErr
			}
			if configuration.Version > 0 {
				presentation, presentationErr := compileCustomerPresentations(configuration.CustomerDeclaration, configuration.CustomerFieldSelections)
				if presentationErr != nil {
					return presentationErr
				}
				if priceErr := s.validateCustomerSelectionPrices(txCtx, versionNo, nil, configuration.CustomerDeclaration, presentation); priceErr != nil {
					return priceErr
				}
			}
			return action(txCtx)
		})
		return err
	}
	selected := bindingEvidence(binding)
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: binding.PluginId, ReleaseVersion: binding.PluginReleaseVersion,
		Point: binding.ExtensionPointCode, ContractVersion: binding.ContractVersion,
		ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest,
	})
	if err != nil || !sameProviderContract(selected, exact) {
		return bizerr.NewCode(CodeEvidenceUnavailable)
	}
	resolved, err := s.resolveFrozenConfigurationCandidates(ctx, binding, exact)
	if err != nil {
		return err
	}
	err = s.extensions.WithNewBusinessAdmission(ctx, admissionInput(selected), func(guardCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
		if !sameProviderContract(selected, evidence) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return s.database.Transaction(guardCtx, func(txCtx context.Context, _ gdb.TX) error {
			if lockErr := s.checkPublicationVersionState(txCtx, versionNo, published); lockErr != nil {
				return lockErr
			}
			lockedBinding, loadErr := s.loadBinding(txCtx, versionNo, true)
			if loadErr != nil {
				return loadErr
			}
			if lockedBinding == nil || !sameEvidence(selected, bindingEvidence(lockedBinding)) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			instance, loadErr := s.loadInterface(txCtx, lockedBinding.InterfaceInstanceNo, true)
			if loadErr != nil {
				return loadErr
			}
			if instance == nil || instance.NewBusinessState != InterfaceStateAccepting ||
				!sameEvidence(bindingEvidence(lockedBinding), interfaceEvidence(instance)) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if validationErr := s.validateFrozenVersionConfiguration(
				txCtx, lockedBinding, evidence, true, resolved,
			); validationErr != nil {
				return validationErr
			}
			if policyErr := s.validateFrozenFulfillmentPolicy(txCtx, versionNo, evidence); policyErr != nil {
				return policyErr
			}
			return action(txCtx)
		})
	})
	return err
}

func (s *serviceImpl) transitionInterfaceState(
	ctx context.Context,
	authorization Authorization,
	instance *entity.ProductAutomationInterfaceInstance,
	targetState string,
	reasonCode string,
) error {
	result, err := s.database.Model(do.ProductAutomationInterfaceInstance{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceInstance{TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: instance.InterfaceInstanceNo, Version: instance.Version}).
		Data(do.ProductAutomationInterfaceInstance{NewBusinessState: targetState, Version: instance.Version + 1,
			ChangedBy: authorization.ActorUserID}).Update()
	if err != nil {
		return gerror.Wrap(err, "update product automation interface state")
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil {
		return gerror.Wrap(affectedErr, "confirm product automation interface state update")
	} else if affected != 1 {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "product-automation.interface.state", IdempotencyKey: fmt.Sprintf("%s:%d", instance.InterfaceInstanceNo, instance.Version+1),
		OperationCode: "product-automation.interface." + targetState, TargetType: "product_automation_interface", TargetNo: instance.InterfaceInstanceNo,
		ResultCode: "succeeded", ReasonCode: reasonCode, RiskCode: "medium", RequiredPermission: authorization.RequiredPermission,
		BeforeSnapshot: map[string]any{"new_business_state": instance.NewBusinessState, "version": instance.Version},
		AfterSnapshot:  map[string]any{"new_business_state": targetState, "version": instance.Version + 1},
	})
}

func normalizeManagementWriteError(err error) error {
	if provideradmissioncap.IsBusy(err) {
		return bizerr.NewCode(codeProviderBusy)
	}
	if err == nil || bizerr.Is(err, catalog.CodePublicationState) || bizerr.Is(err, catalog.CodePublicationInvalid) || bizerr.Is(err, catalog.CodePublicationNotAccessible) || bizerr.Is(err, catalog.CodePublicationIncomplete) || bizerr.Is(err, catalog.CodeManagementNotAccessible) || bizerr.Is(err, catalog.CodeManagementConflict) ||
		bizerr.Is(err, catalog.CodeSaleCopyRequired) || bizerr.Is(err, catalog.CodeSalePricingRequired) || bizerr.Is(err, catalog.CodeSaleTaxRequired) ||
		bizerr.Is(err, catalog.CodeManagementInvalid) || bizerr.Is(err, catalog.CodeFulfillmentPolicyBindingRequired) ||
		bizerr.Is(err, catalog.CodeFulfillmentPolicyContractChanged) || bizerr.Is(err, catalog.CodeFulfillmentPolicyImmutable) ||
		bizerr.Is(err, catalog.CodeTrialFulfillmentUnavailable) || bizerr.Is(err, codeSupplierCapabilitiesUnconfirmed) || bizerr.Is(err, codeProviderBusy) || bizerr.Is(err, codeProviderValidationUnavailable) || bizerr.Is(err, codeProviderValidationResponseInvalid) ||
		bizerr.Is(err, codeOfferDetachUnsupported) || bizerr.Is(err, codeOfferMappingRequired) || bizerr.Is(err, codeOfferOptionsRequired) || bizerr.Is(err, codeChoiceRelationsRequired) ||
		bizerr.Is(err, codeCustomerOptionPricesRequired) || bizerr.Is(err, CodeInterfaceEverReferenced) || bizerr.Is(err, CodeEvidenceUnavailable) {
		return err
	}
	return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
}

func (s *serviceImpl) loadInterface(ctx context.Context, interfaceNo string, lock bool) (*entity.ProductAutomationInterfaceInstance, error) {
	model := s.database.Model(do.ProductAutomationInterfaceInstance{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceInstance{TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: interfaceNo})
	if lock {
		model = model.LockUpdate()
	}
	var row *entity.ProductAutomationInterfaceInstance
	err := model.Scan(&row)
	return row, gerror.Wrap(err, "load product automation interface")
}

func (s *serviceImpl) lockDraftVersion(ctx context.Context, versionNo string) error {
	var row struct {
		StatusCode string `orm:"status_code"`
	}
	err := s.database.Model(do.CatalogProductVersion{}).Ctx(ctx).
		Fields("status_code").Where(do.CatalogProductVersion{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).
		LockUpdate().Scan(&row)
	if err != nil {
		return gerror.Wrap(err, "lock product automation draft version")
	}
	if row.StatusCode == "" {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	if row.StatusCode != "draft" {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}

func admissionInput(evidence *capmodel.BusinessExtensionProviderEvidence) pluginhost.BusinessExtensionAdmissionInput {
	return pluginhost.BusinessExtensionAdmissionInput{
		PluginID: evidence.PluginID, Point: evidence.Point, ContractVersion: evidence.ContractVersion,
		Method: "", ReleaseVersion: evidence.ReleaseVersion, ReleaseDigest: evidence.ReleaseDigest,
		DeclarationDigest: evidence.DeclarationDigest,
	}
}

func interfaceEvidence(row *entity.ProductAutomationInterfaceInstance) *capmodel.BusinessExtensionProviderEvidence {
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: row.PluginId, DisplayName: row.DisplayName, ReleaseVersion: row.PluginReleaseVersion,
		Point: row.ExtensionPointCode, ContractVersion: row.ContractVersion,
		AuthorizedMethods: row.AuthorizedMethods, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest,
	}
}

func bindingAudit(authorization Authorization, input BindInput, before *entity.CatalogProductVersionFulfillmentBinding, instance *entity.ProductAutomationInterfaceInstance, bindingID int64) idccap.BusinessAuditTransition {
	beforeSnapshot := map[string]any{}
	operation := "product-automation.binding.bind"
	if before != nil {
		beforeSnapshot["interface_instance_no"] = before.InterfaceInstanceNo
		operation = "product-automation.binding.replace"
	}
	return idccap.BusinessAuditTransition{
		IdempotencyScope: operation, IdempotencyKey: bindingAuditIdentity(input.VersionNo, bindingID),
		OperationCode: operation, TargetType: "catalog_product_version", TargetNo: input.VersionNo,
		ResultCode: "succeeded", ReasonCode: "manual_binding", RiskCode: "medium",
		RequiredPermission: authorization.RequiredPermission, BeforeSnapshot: beforeSnapshot,
		AfterSnapshot: map[string]any{"interface_instance_no": instance.InterfaceInstanceNo, "interface_display_name": instance.DisplayName},
	}
}

func bindingAuditIdentity(versionNo string, bindingID int64) string {
	return fmt.Sprintf("%s:%d", versionNo, bindingID)
}

// checkPublicationVersionState preserves the draft lock for publishing. For
// immutable published versions, the catalog callback supplies product-first
// locking and current-pointer fencing, avoiding reverse lock order with copies.
func (s *serviceImpl) checkPublicationVersionState(ctx context.Context, versionNo string, published bool) error {
	if !published {
		return s.lockDraftVersion(ctx, versionNo)
	}
	var row struct {
		StatusCode string `orm:"status_code"`
	}
	if err := s.database.Model(do.CatalogProductVersion{}).Ctx(ctx).Fields("status_code").Where(do.CatalogProductVersion{TenantId: 0, VersionNo: versionNo}).Scan(&row); err != nil {
		return err
	}
	if row.StatusCode != "published" {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}
