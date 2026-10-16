// This file implements auth-first administration of interface and frozen
// product-version configuration without exposing secret handles or plaintext.

package productautomation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const preservedSecretSentinel = "configuration-secret-preserved-by-owner"

type interfaceSecretState struct {
	fieldKey string
	handle   secretcap.Handle
	metadata secretcap.Metadata
}

func (s *serviceImpl) GetInterfaceConfiguration(ctx context.Context, interfaceNo string) (*InterfaceConfiguration, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceConfiguration(ctx, interfaceNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	return s.getInterfaceConfigurationAuthorized(ctx, interfaceNo)
}

func (s *serviceImpl) getInterfaceConfigurationAuthorized(ctx context.Context, interfaceNo string) (*InterfaceConfiguration, error) {
	if validateBusinessNo(interfaceNo, "pfi") != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, interfaceNo)
	if err != nil || resourceOwned {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	instance, exact, err := s.loadExactInterfaceEvidence(ctx, interfaceNo)
	if err != nil {
		return nil, err
	}
	revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, interfaceNo, false)
	if err != nil || revision == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	declaration, declarationErr := configurationDeclarationJSON(exact.InterfaceConfiguration)
	if declarationErr != nil || configurationSchemaVersion(exact.InterfaceConfiguration) != revision.SchemaVersion ||
		configurationDigest(declaration) != revision.DeclarationDigest {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	values, err := configurationValuesFromJSON(exact.InterfaceConfiguration, []byte(revision.PublicValueSnapshot))
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	states, err := s.loadInterfaceSecretStates(ctx, revision.Id)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	normalized, _, validationErr := normalizeInterfaceSecretInputs(exact.InterfaceConfiguration, values, states)
	if validationErr != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	resolved, validationErr := s.resolveDynamicCandidates(ctx, exact, exact.InterfaceConfiguration, normalized, configurationSurfaceInterface,
		dynamicCandidateScope{interfaceInstanceNo: interfaceNo, configurationRevision: revision.Revision})
	if validationErr != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	if _, validationErr = compileConfigurationWithResolved(exact, exact.InterfaceConfiguration, normalized, configurationSurfaceInterface, resolved); validationErr != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	presentation, validationErr := configurationcontract.MaterializeResolvedSchema(
		exact.InterfaceConfiguration, configurationSurfaceInterface, exact.AuthorizedMethods, resolved,
	)
	if validationErr != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	statuses := make([]ConfigurationSecretStatus, 0, len(states))
	for _, state := range states {
		statuses = append(statuses, ConfigurationSecretStatus{FieldKey: state.fieldKey, Configured: state.metadata.Configured, MaskedTail: state.metadata.MaskedTail, UpdatedAt: state.metadata.UpdatedAt})
	}
	_ = instance
	return &InterfaceConfiguration{
		Declaration: presentation, Values: values, Secrets: statuses,
		ResolvedCandidates: resolved, Revision: revision.Revision,
	}, nil
}

func (s *serviceImpl) UpdateInterfaceConfiguration(ctx context.Context, input UpdateInterfaceConfigurationInput) (*InterfaceConfiguration, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceConfiguration(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if validateBusinessNo(input.InterfaceInstanceNo, "pfi") != nil || input.ExpectedRevision <= 0 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, input.InterfaceInstanceNo)
	if err != nil || resourceOwned {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	_, exact, err := s.loadExactInterfaceEvidence(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	current, err := s.loadLatestInterfaceConfigurationRevision(ctx, input.InterfaceInstanceNo, false)
	if err != nil || current == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	if current.Revision != input.ExpectedRevision {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	existing, err := s.loadInterfaceSecretStates(ctx, current.Id)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	normalized, replacementValues, err := normalizeInterfaceSecretInputs(exact.InterfaceConfiguration, input.Values, existing)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	compiled, err := s.compileConfigurationResolved(ctx, exact, exact.InterfaceConfiguration, normalized, configurationSurfaceInterface,
		dynamicCandidateScope{interfaceInstanceNo: input.InterfaceInstanceNo, configurationRevision: current.Revision})
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	for key, value := range compiled.SecretValues {
		if value == preservedSecretSentinel {
			delete(compiled.SecretValues, key)
		}
	}
	for key, value := range replacementValues {
		compiled.SecretValues[key] = value
	}
	nextRevision := current.Revision + 1
	secretStates, secretOperation, err := s.applyInterfaceSecretChanges(ctx, input.InterfaceInstanceNo, nextRevision, exact.InterfaceConfiguration, existing, compiled.SecretValues)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		locked, loadErr := s.loadLatestInterfaceConfigurationRevision(txCtx, input.InterfaceInstanceNo, true)
		if loadErr != nil || locked == nil || locked.Id != current.Id || locked.Revision != input.ExpectedRevision {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		if bindErr := s.bindManagementConfigurationFiles(txCtx, configurationSurfaceInterface, input.InterfaceInstanceNo, current.Revision, nextRevision, authorization.ActorUserID, exact.InterfaceConfiguration, normalized); bindErr != nil {
			return bindErr
		}
		if persistErr := s.persistInterfaceConfigurationRevision(txCtx, input.InterfaceInstanceNo, authorization.ActorUserID, nextRevision, compiled, configurationSchemaVersion(exact.InterfaceConfiguration), secretStates); persistErr != nil {
			return persistErr
		}
		if commitErr := s.commitInterfaceSecretOperation(txCtx, secretOperation); commitErr != nil {
			return commitErr
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-automation.interface.configuration", IdempotencyKey: interfaceConfigurationAuditKey(input.InterfaceInstanceNo, nextRevision, current.DeclarationDigest, compiledValueDigest(compiled)),
			OperationCode: "product-automation.interface.configuration.update", TargetType: "product_automation_interface", TargetNo: input.InterfaceInstanceNo,
			ResultCode: "succeeded", ReasonCode: "manual_update", RiskCode: "high", RequiredPermission: authorization.RequiredPermission,
			BeforeSnapshot: map[string]any{"revision": current.Revision, "declaration_digest": current.DeclarationDigest, "public_value_digest": current.ValueDigest, "secret_status_digest": existingInterfaceSecretStatusDigest(existing)},
			AfterSnapshot:  map[string]any{"revision": nextRevision, "declaration_digest": configurationDigest(compiled.DeclarationJSON), "public_value_digest": compiledValueDigest(compiled), "secret_status_digest": interfaceSecretStatusDigest(secretStates), "replaced_secret_fields": sortedStringKeys(replacementValues)},
		})
	})
	if err != nil {
		return nil, normalizeManagementWriteError(errors.Join(err, s.markInterfaceSecretOperationPending(ctx, secretOperation, err), s.compensateInterfaceSecretOperation(ctx, secretOperation)))
	}
	return s.getInterfaceConfigurationAuthorized(ctx, input.InterfaceInstanceNo)
}

func (s *serviceImpl) GetVersionConfiguration(ctx context.Context, versionNo string) (*VersionConfiguration, error) {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if validateBusinessNo(versionNo, "ver") != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if binding == nil {
		return nil, nil
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: binding.PluginId, ReleaseVersion: binding.PluginReleaseVersion, Point: binding.ExtensionPointCode,
		ContractVersion: binding.ContractVersion, ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest,
	})
	if err != nil || !sameExactEvidence(bindingEvidence(binding), exact) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	var row *entity.CatalogProductVersionConfiguration
	if err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Scan(&row); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	var interfaceRevision int64
	if row != nil {
		interfaceRevision = row.InterfaceConfigurationRevision
	} else {
		latest, loadErr := s.loadLatestInterfaceConfigurationRevision(ctx, binding.InterfaceInstanceNo, false)
		if loadErr != nil || latest == nil {
			return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
		}
		interfaceRevision = latest.Revision
	}
	scope := dynamicCandidateScope{versionNo: versionNo, interfaceInstanceNo: binding.InterfaceInstanceNo, configurationRevision: interfaceRevision}
	if row == nil {
		defaultCustomer, schemaErr := selectCustomerSchema(exact, nil)
		if schemaErr != nil {
			return nil, bizerr.NewCode(CodeEvidenceUnavailable)
		}
		product, productCandidates, presentationErr := s.materializeConfiguration(
			ctx, exact, exact.ProductConfiguration, nil, configurationSurfaceProduct, scope,
		)
		if presentationErr != nil {
			return nil, bizerr.NewCode(CodeEvidenceUnavailable)
		}
		customer, customerCandidates, presentationErr := s.materializeConfiguration(
			ctx, exact, defaultCustomer, nil, configurationSurfaceCustomer, scope,
		)
		if presentationErr != nil {
			return nil, bizerr.NewCode(CodeEvidenceUnavailable)
		}
		return &VersionConfiguration{
			CustomerConfigurableFields: configurableCustomerFields(exact),
			ProductDeclaration:         product, CustomerDeclaration: customer,
			MerchantValues: []ConfigurationValue{}, ProductResolvedCandidates: productCandidates,
			CustomerResolvedCandidates: customerCandidates, Version: 0,
		}, nil
	}
	product, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	customer, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	customerSchema, schemaErr := s.importedCustomerSchema(ctx, binding, row, exact)
	if schemaErr != nil {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	imported := false
	if exact.PluginID == portableImportProviderID {
		imported, err = s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: versionNo, InterfaceInstanceNo: binding.InterfaceInstanceNo}).Exist()
		if err != nil {
			return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
		}
	}
	if !sameExactConfiguration(product, exact.ProductConfiguration) ||
		!sameExactConfiguration(customer, customerSchema) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	values, err := configurationValuesFromJSON(product, []byte(row.MerchantValueSnapshot))
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	productPresentation, productCandidates, err := s.materializeConfiguration(
		ctx, exact, product, values, configurationSurfaceProduct, scope,
	)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	customerPresentation, customerCandidates, err := s.materializeConfiguration(
		ctx, exact, customer, nil, configurationSurfaceCustomer, scope,
	)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	selections, selectionErr := frozenCustomerSelections(exact, customer)
	if selectionErr != nil && !imported {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	return &VersionConfiguration{
		CustomerConfigurableFields: configurableCustomerFields(exact), CustomerFieldSelections: selections,
		ImportedConfiguration: imported,
		ProductDeclaration:    productPresentation, CustomerDeclaration: customerPresentation,
		MerchantValues: values, ProductResolvedCandidates: productCandidates,
		CustomerResolvedCandidates: customerCandidates, Version: row.Version,
	}, nil
}

func (s *serviceImpl) materializeConfiguration(
	ctx context.Context,
	exact *capmodel.BusinessExtensionProviderEvidence,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	scope dynamicCandidateScope,
) (*capmodel.ConfigurationSchemaDeclaration, configurationcontract.ResolvedCandidates, error) {
	resolved, err := s.resolveDynamicCandidates(ctx, exact, schema, values, surface, scope)
	if err != nil {
		return nil, nil, err
	}
	if _, err = configurationcontract.EvaluateResolved(schema, values, surface, exact.AuthorizedMethods, resolved); err != nil {
		return nil, nil, err
	}
	presentation, err := configurationcontract.MaterializeResolvedSchema(schema, surface, exact.AuthorizedMethods, resolved)
	if err != nil {
		return nil, nil, err
	}
	return presentation, resolved, nil
}

func (s *serviceImpl) loadExactInterfaceEvidence(ctx context.Context, interfaceNo string) (*entity.ProductAutomationInterfaceInstance, *capmodel.BusinessExtensionProviderEvidence, error) {
	instance, err := s.loadInterface(ctx, interfaceNo, false)
	if err != nil {
		return nil, nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if instance == nil {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: instance.PluginId, ReleaseVersion: instance.PluginReleaseVersion, Point: instance.ExtensionPointCode,
		ContractVersion: instance.ContractVersion, ReleaseDigest: instance.ReleaseDigest, DeclarationDigest: instance.DeclarationDigest,
	})
	if err != nil || !sameExactEvidence(interfaceEvidence(instance), exact) {
		return nil, nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	return instance, exact, nil
}

func (s *serviceImpl) resolveCurrentInterfaceCandidates(
	ctx context.Context,
	interfaceNo string,
	exact *capmodel.BusinessExtensionProviderEvidence,
) (configurationcontract.ResolvedCandidates, error) {
	revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, interfaceNo, false)
	if err != nil || revision == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	values, err := configurationValuesFromJSON(exact.InterfaceConfiguration, []byte(revision.PublicValueSnapshot))
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	states, err := s.loadInterfaceSecretStates(ctx, revision.Id)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	normalized, _, err := normalizeInterfaceSecretInputs(exact.InterfaceConfiguration, values, states)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return s.resolveDynamicCandidates(ctx, exact, exact.InterfaceConfiguration, normalized, configurationSurfaceInterface,
		dynamicCandidateScope{interfaceInstanceNo: interfaceNo, configurationRevision: revision.Revision})
}

func (s *serviceImpl) validateCurrentInterfaceConfigurationWithCandidates(
	ctx context.Context,
	interfaceNo string,
	exact *capmodel.BusinessExtensionProviderEvidence,
	resolved configurationcontract.ResolvedCandidates,
) error {
	revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, interfaceNo, false)
	if err != nil || revision == nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return s.validateInterfaceConfigurationRevision(ctx, interfaceNo, exact, revision, resolved)
}

func (s *serviceImpl) validateInterfaceConfigurationRevision(
	ctx context.Context,
	interfaceNo string,
	exact *capmodel.BusinessExtensionProviderEvidence,
	revision *entity.ProductAutomationInterfaceConfigurationRevision,
	resolved configurationcontract.ResolvedCandidates,
) error {
	if revision == nil || revision.InterfaceInstanceNo != interfaceNo ||
		!configurationRuntimeAvailable(exact.InterfaceConfiguration, configurationSurfaceInterface) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	// Resource-owned interfaces keep their authoritative configuration in the
	// owner plugin. The core projection deliberately stores only an opaque
	// revision link, so validating it against the connector schema here would
	// reject every valid resource interface (and would leak owner details into
	// the core contract). The resource owner has already validated the actual
	// configuration before projecting this immutable revision.
	resourceOwned, ownerErr := s.resourceInterfaceOwned(ctx, interfaceNo)
	if ownerErr != nil {
		return bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	if resourceOwned {
		if revision.SchemaVersion != "resource-owner-v1" || revision.ResourceOwnerConfigurationRevision == "" {
			return bizerr.NewCode(CodeEvidenceUnavailable)
		}
		return nil
	}
	declaration, err := configurationDeclarationJSON(exact.InterfaceConfiguration)
	if err != nil || configurationDigest(declaration) != revision.DeclarationDigest ||
		configurationSchemaVersion(exact.InterfaceConfiguration) != revision.SchemaVersion {
		return bizerr.NewCode(CodeEvidenceUnavailable)
	}
	storedDeclaration, err := decodeConfigurationDeclaration(revision.DeclarationSnapshot)
	if err != nil || !sameExactConfiguration(storedDeclaration, exact.InterfaceConfiguration) {
		return bizerr.NewCode(CodeEvidenceUnavailable)
	}
	values, err := configurationValuesFromJSON(exact.InterfaceConfiguration, []byte(revision.PublicValueSnapshot))
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	states, err := s.loadInterfaceSecretStates(ctx, revision.Id)
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	normalized, _, err := normalizeInterfaceSecretInputs(exact.InterfaceConfiguration, values, states)
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	compiled, err := compileConfigurationWithResolved(exact, exact.InterfaceConfiguration, normalized, configurationSurfaceInterface, resolved)
	if err != nil || configurationDigest(compiled.ValuesJSON) != revision.ValueDigest {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if revision.SecretFieldCount != len(states) {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}

func sameExactConfiguration(left, right *capmodel.ConfigurationSchemaDeclaration) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftJSON, leftErr := configurationDeclarationJSON(left)
	rightJSON, rightErr := configurationDeclarationJSON(right)
	return leftErr == nil && rightErr == nil && configurationDigest(leftJSON) == configurationDigest(rightJSON)
}

func configurationRuntimeAvailable(schema *capmodel.ConfigurationSchemaDeclaration, _ ConfigurationSurface) bool {
	if schema == nil {
		return true
	}
	for _, field := range schema.Fields {
		if field == nil {
			return false
		}
	}
	return true
}

func (s *serviceImpl) loadInterfaceSecretStates(ctx context.Context, revisionID int64) (map[string]interfaceSecretState, error) {
	var rows []*entity.ProductAutomationInterfaceSecretReference
	if err := s.database.Model(do.ProductAutomationInterfaceSecretReference{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceSecretReference{TenantId: idccap.DefaultTenantID, ConfigurationRevisionId: revisionID}).OrderAsc("field_key").Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "load interface secret status")
	}
	states := make(map[string]interfaceSecretState, len(rows))
	for _, row := range rows {
		handle, err := secretcap.NewHandle(row.SecretRef)
		if err != nil || s.secrets == nil {
			return nil, gerror.New("interface secret status is unavailable")
		}
		metadata, err := s.secrets.Metadata(ctx, handle)
		if err != nil || !metadata.Configured {
			return nil, gerror.Wrap(err, "read interface secret status")
		}
		states[row.FieldKey] = interfaceSecretState{fieldKey: row.FieldKey, handle: handle, metadata: metadata}
	}
	return states, nil
}

func normalizeInterfaceSecretInputs(schema *capmodel.ConfigurationSchemaDeclaration, supplied []ConfigurationValue, existing map[string]interfaceSecretState) ([]ConfigurationValue, map[string]string, error) {
	secretFields := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil && field.Type == capmodel.ConfigurationFieldSecret {
				secretFields[field.Key] = field
			}
		}
	}
	result := make([]ConfigurationValue, 0, len(supplied)+len(existing))
	replacements := make(map[string]string)
	seen := make(map[string]bool)
	for _, value := range supplied {
		field := secretFields[value.FieldKey]
		if field == nil {
			result = append(result, value)
			continue
		}
		if seen[value.FieldKey] || value.TextValue == nil {
			return nil, nil, gerror.New("secret input is ambiguous")
		}
		seen[value.FieldKey] = true
		if *value.TextValue != "" {
			replacements[value.FieldKey] = *value.TextValue
			result = append(result, value)
			continue
		}
		if _, configured := existing[value.FieldKey]; !configured {
			return nil, nil, gerror.New("empty secret cannot create a credential")
		}
		placeholder := preservedSecretSentinel
		result = append(result, ConfigurationValue{FieldKey: value.FieldKey, TextValue: &placeholder})
	}
	for key, field := range secretFields {
		if seen[key] {
			continue
		}
		if _, configured := existing[key]; configured {
			placeholder := preservedSecretSentinel
			result = append(result, ConfigurationValue{FieldKey: key, TextValue: &placeholder})
		} else if field.Required {
			return nil, nil, gerror.New("required secret is not configured")
		}
	}
	return result, replacements, nil
}

func (s *serviceImpl) applyInterfaceSecretChanges(ctx context.Context, interfaceNo string, revision int64, schema *capmodel.ConfigurationSchemaDeclaration, existing map[string]interfaceSecretState, replacements map[string]string) ([]interfaceSecretState, *interfaceSecretOperation, error) {
	states := make([]interfaceSecretState, 0, len(existing)+len(replacements))
	if schema == nil {
		return states, nil, nil
	}
	operation, err := s.prepareInterfaceSecretOperation(ctx, interfaceNo, revision-1, sortedStringKeys(replacements))
	if err != nil {
		return nil, nil, err
	}
	for _, field := range schema.Fields {
		if field == nil || field.Type != capmodel.ConfigurationFieldSecret {
			continue
		}
		current, configured := existing[field.Key]
		plaintext, replacing := replacements[field.Key]
		if !replacing {
			if configured {
				states = append(states, current)
			}
			continue
		}
		bytes := []byte(plaintext)
		producer := func(consume secretcap.Consumer) error { return consume(bytes) }
		// The persisted operation identity is the stable create/recover key.
		// Older revisions keep their handles and failed attempts converge from
		// the owner-private operation journal without retaining plaintext.
		idempotencyKey := secretOperationIdempotencyKey(operation.OperationNo, field.Key)
		created, err := s.secrets.Create(ctx, idempotencyKey, producer)
		clear(bytes)
		if err != nil {
			created, err = s.secrets.RecoverCreate(ctx, idempotencyKey)
		}
		if err != nil {
			failure := errors.Join(err, s.markInterfaceSecretOperationPending(ctx, operation, err), s.compensateInterfaceSecretOperation(ctx, operation))
			return nil, operation, failure
		}
		states = append(states, interfaceSecretState{fieldKey: field.Key, handle: created.Handle, metadata: created.Metadata})
		if err = s.recordInterfaceSecretOperationField(ctx, operation.ID, field.Key, created); err != nil {
			return nil, operation, errors.Join(err, s.markInterfaceSecretOperationPending(ctx, operation, err), s.compensateInterfaceSecretOperation(ctx, operation))
		}
	}
	sort.Slice(states, func(i, j int) bool { return states[i].fieldKey < states[j].fieldKey })
	if operation != nil {
		if err = s.markInterfaceSecretOperationReady(ctx, operation.ID); err != nil {
			return nil, operation, errors.Join(err, s.markInterfaceSecretOperationPending(ctx, operation, err), s.compensateInterfaceSecretOperation(ctx, operation))
		}
	}
	return states, operation, nil
}

func (s *serviceImpl) persistInterfaceConfigurationRevision(ctx context.Context, interfaceNo string, actor int, revision int64, compiled *canonicalConfiguration, schemaVersion string, secrets []interfaceSecretState) error {
	id, err := s.database.Model(do.ProductAutomationInterfaceConfigurationRevision{}).Ctx(ctx).Data(do.ProductAutomationInterfaceConfigurationRevision{
		TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: interfaceNo, Revision: revision, SchemaVersion: schemaVersion,
		DeclarationSnapshot: string(compiled.DeclarationJSON), PublicValueSnapshot: string(compiled.ValuesJSON),
		DeclarationDigest: configurationDigest(compiled.DeclarationJSON), ValueDigest: compiledValueDigest(compiled), SecretFieldCount: len(secrets), CreatedBy: actor,
	}).InsertAndGetId()
	if err != nil {
		return gerror.Wrap(err, "insert interface configuration revision")
	}
	for _, state := range secrets {
		updatedAt := state.metadata.UpdatedAt
		if _, err = s.database.Model(do.ProductAutomationInterfaceSecretReference{}).Ctx(ctx).Data(do.ProductAutomationInterfaceSecretReference{
			TenantId: idccap.DefaultTenantID, ConfigurationRevisionId: id, FieldKey: state.fieldKey,
			SecretRef: state.handle.OpaqueRef(), Configured: state.metadata.Configured, MaskedTail: state.metadata.MaskedTail, SecretUpdatedAt: &updatedAt,
		}).Insert(); err != nil {
			return gerror.Wrap(err, "insert interface secret status")
		}
	}
	count, err := s.database.Model(do.ProductAutomationInterfaceSecretReference{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceSecretReference{TenantId: idccap.DefaultTenantID, ConfigurationRevisionId: id}).Count()
	if err != nil {
		return gerror.Wrap(err, "confirm interface secret status count")
	}
	if count != len(secrets) {
		return gerror.New("interface secret status count does not match revision")
	}
	return nil
}

func compiledValueDigest(compiled *canonicalConfiguration) string {
	return configurationDigest(compiled.ValuesJSON)
}

func interfaceSecretStatusDigest(states []interfaceSecretState) string {
	status := make(map[string]bool, len(states))
	for _, state := range states {
		status[state.fieldKey] = state.metadata.Configured
	}
	return secretConfiguredStatusDigest(status)
}

func existingInterfaceSecretStatusDigest(states map[string]interfaceSecretState) string {
	status := make(map[string]bool, len(states))
	for key, state := range states {
		status[key] = state.metadata.Configured
	}
	return secretConfiguredStatusDigest(status)
}

func secretConfiguredStatusDigest(status map[string]bool) string {
	type safeSecretStatus struct {
		FieldKey   string `json:"field_key"`
		Configured bool   `json:"configured"`
	}
	keys := sortedStringKeys(status)
	items := make([]safeSecretStatus, 0, len(keys))
	for _, key := range keys {
		items = append(items, safeSecretStatus{FieldKey: key, Configured: status[key]})
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return configurationDigest([]byte("invalid-secret-status"))
	}
	return configurationDigest(payload)
}

func sortedStringKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func interfaceConfigurationAuditKey(interfaceNo string, revision int64, declarationDigest, valueDigest string) string {
	return interfaceNo + ":" + strconv.FormatInt(revision, 10) + ":" + declarationDigest + ":" + valueDigest
}

func decodeConfigurationDeclaration(value string) (*capmodel.ConfigurationSchemaDeclaration, error) {
	return configurationcontract.DecodeDeclaration(value)
}

func configurationValuesFromJSON(schema *capmodel.ConfigurationSchemaDeclaration, payload []byte) ([]ConfigurationValue, error) {
	if schema == nil {
		return []ConfigurationValue{}, nil
	}
	var document map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&document); err != nil {
		return nil, gerror.Wrap(err, "decode configuration values")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, gerror.New("configuration values contain trailing data")
	}
	known := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		if field != nil {
			known[field.Key] = struct{}{}
		}
	}
	for key := range document {
		if _, ok := known[key]; !ok {
			return nil, gerror.New("configuration values contain an unknown field")
		}
	}
	values := make([]ConfigurationValue, 0, len(document))
	for _, field := range schema.Fields {
		if field == nil || field.Type == capmodel.ConfigurationFieldSecret {
			continue
		}
		raw, exists := document[field.Key]
		if !exists {
			continue
		}
		value := ConfigurationValue{FieldKey: field.Key}
		switch field.Type {
		case capmodel.ConfigurationFieldInteger:
			var typed int64
			if err := json.Unmarshal(raw, &typed); err != nil {
				return nil, err
			}
			value.IntegerValue = &typed
		case capmodel.ConfigurationFieldBoolean:
			var typed bool
			if err := json.Unmarshal(raw, &typed); err != nil {
				return nil, err
			}
			value.BooleanValue = &typed
		case capmodel.ConfigurationFieldMultiSelect:
			if err := json.Unmarshal(raw, &value.OptionTokens); err != nil {
				return nil, err
			}
		case capmodel.ConfigurationFieldSingleSelect:
			var typed string
			if err := json.Unmarshal(raw, &typed); err != nil {
				return nil, err
			}
			value.OptionTokens = []string{typed}
		case capmodel.ConfigurationFieldTreeSelect:
			var typed string
			if err := json.Unmarshal(raw, &typed); err != nil {
				return nil, err
			}
			value.TreeOptionTokens = []string{typed}
		case capmodel.ConfigurationFieldAddress:
			if err := json.Unmarshal(raw, &value.AddressOptionTokens); err != nil {
				return nil, err
			}
		case capmodel.ConfigurationFieldFile:
			if err := json.Unmarshal(raw, &value.FileTokens); err != nil {
				return nil, err
			}
		default:
			var typed string
			if err := json.Unmarshal(raw, &typed); err != nil {
				return nil, err
			}
			switch field.Type {
			case capmodel.ConfigurationFieldDecimal:
				value.DecimalValue = &typed
			case capmodel.ConfigurationFieldDate:
				value.DateValue = &typed
			case capmodel.ConfigurationFieldDateTime:
				value.DateTimeValue = &typed
			default:
				value.TextValue = &typed
			}
		}
		values = append(values, value)
	}
	return values, nil
}
