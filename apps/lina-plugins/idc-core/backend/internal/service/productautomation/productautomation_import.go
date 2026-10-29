// This file freezes connector-proven partial product configuration for imports.
// Remote candidate reads finish before the owner transaction; ordinary saves
// and publication retain their strict completeness requirements.

package productautomation

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// ImportConfigurationIssue identifies an unfilled declared field, without values.
type ImportConfigurationIssue struct {
	FieldKey string
	NameZhCN string
	NameEnUS string
}

// ImportBinding is an opaque, validated preparation, usable only with its
// originating service and direct staff actor. It contains no supplier secrets.
type ImportBinding struct {
	MissingConfiguration     []ImportConfigurationIssue
	owner                    *serviceImpl
	actor                    int
	interfaceNo              string
	interfaceVersion         int64
	interfaceRevision        int64
	readEvidence             resourceinterfaceprojectioncap.CatalogReadEvidence
	exact                    *capmodel.BusinessExtensionProviderEvidence
	product                  *canonicalConfiguration
	customerJSON             []byte
	customerPresentationJSON []byte
	customerSchema           *capmodel.ConfigurationSchemaDeclaration
	portable                 *channelcommercecap.ImportSnapshot
	interfaceCandidates      configurationcontract.ResolvedCandidates
}

// ImportBindingService is the same-owner atomic draft import admission seam.
type ImportBindingService interface {
	// PrepareImportBinding validates proven values and resolves only supplied
	// dynamic selections outside a transaction; missing required fields survive.
	PrepareImportBinding(context.Context, string, *resourceinterfaceprojectioncap.CatalogReadEvidence, []resourceinterfaceprojectioncap.CatalogConfigurationValue, *channelcommercecap.ImportSnapshot) (*ImportBinding, error)
	// WithImportBinding holds exact release and local revision admission while
	// action creates the draft in the same transaction. An empty returned version
	// means a concurrent idempotent replay and performs no binding write.
	WithImportBinding(context.Context, *ImportBinding, func(context.Context) (string, error)) error
}

func (s *serviceImpl) PrepareImportBinding(ctx context.Context, interfaceNo string, readEvidence *resourceinterfaceprojectioncap.CatalogReadEvidence, supplied []resourceinterfaceprojectioncap.CatalogConfigurationValue, portable *channelcommercecap.ImportSnapshot) (*ImportBinding, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceList(ctx)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if validateBusinessNo(interfaceNo, "pfi") != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	instance, exact, err := s.loadExactInterfaceEvidence(ctx, interfaceNo)
	if err != nil {
		return nil, err
	}
	if instance.NewBusinessState != InterfaceStateAccepting {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, interfaceNo, false)
	if err != nil || revision == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if !matchesImportReadEvidence(readEvidence, instance, revision, exact) {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	values, err := importConfigurationValues(supplied)
	if err != nil {
		return nil, err
	}
	resolutionSchema, emptyCandidates, err := importResolutionSchema(exact.ProductConfiguration, values, exact.AuthorizedMethods)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	resolved, err := s.resolveDynamicCandidates(ctx, exact, resolutionSchema, values, configurationSurfaceProduct,
		dynamicCandidateScope{interfaceInstanceNo: interfaceNo, configurationRevision: revision.Revision, resourceConfigurationRevision: readEvidence.InterfaceConfigurationRevision})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeEvidenceUnavailable)
	}
	for fieldKey, candidates := range emptyCandidates {
		resolved[fieldKey] = candidates
	}
	product, err := configurationcontract.CompileImportedDraftResolved(exact.ProductConfiguration, values, exact.AuthorizedMethods, resolved)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	evaluation, err := configurationcontract.EvaluateResolved(exact.ProductConfiguration, values, configurationSurfaceProduct, exact.AuthorizedMethods, resolved)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	customerSchema, err := selectCustomerSchema(exact, nil)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	if portable != nil {
		if err = validatePortableImportEvidence(portable, readEvidence, exact, supplied); err != nil {
			return nil, err
		}
		customerSchema = portable.Schema
	}
	customerPresentationJSON, err := portableCustomerPresentationJSON(portable)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	customerJSON, err := configurationDeclarationJSON(customerSchema)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if _, err = configurationcontract.ValidateSchema(customerSchema, configurationSurfaceCustomer, exact.AuthorizedMethods); err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	interfaceCandidates, err := s.resolveCurrentInterfaceCandidates(ctx, interfaceNo, exact)
	if err != nil {
		return nil, err
	}
	if err = s.validateCurrentInterfaceConfigurationWithCandidates(ctx, interfaceNo, exact, interfaceCandidates); err != nil {
		return nil, err
	}
	missing := make([]ImportConfigurationIssue, 0, len(evaluation.Issues))
	labels := make(map[string]*capmodel.LocalizedText)
	if exact.ProductConfiguration != nil {
		for _, field := range exact.ProductConfiguration.Fields {
			labels[field.Key] = field.Label
		}
	}
	seen := make(map[string]bool)
	for _, issue := range evaluation.Issues {
		if seen[issue.FieldKey] {
			continue
		}
		seen[issue.FieldKey] = true
		label := labels[issue.FieldKey]
		if label != nil {
			missing = append(missing, ImportConfigurationIssue{FieldKey: issue.FieldKey, NameZhCN: label.ZhCN, NameEnUS: label.EnUS})
		}
	}
	frozenRead := *readEvidence
	frozenRead.AuthorizedMethods = append([]string(nil), readEvidence.AuthorizedMethods...)
	return &ImportBinding{MissingConfiguration: missing, owner: s, actor: authorization.ActorUserID, interfaceNo: interfaceNo, readEvidence: frozenRead,
		interfaceVersion: instance.Version, interfaceRevision: revision.Revision, exact: exact, product: product, customerJSON: customerJSON, customerPresentationJSON: customerPresentationJSON, customerSchema: customerSchema, portable: portable, interfaceCandidates: interfaceCandidates}, nil
}

// matchesImportReadEvidence joins the resource-owned read snapshot to the
// product-owned projection. A delayed projection or changed revision fails closed.
func matchesImportReadEvidence(read *resourceinterfaceprojectioncap.CatalogReadEvidence, instance *entity.ProductAutomationInterfaceInstance, revision *entity.ProductAutomationInterfaceConfigurationRevision, exact *capmodel.BusinessExtensionProviderEvidence) bool {
	if read == nil || read.Validate() != nil || instance == nil || revision == nil || exact == nil {
		return false
	}
	return read.InterfaceInstanceNo == instance.InterfaceInstanceNo && revision.InterfaceInstanceNo == instance.InterfaceInstanceNo &&
		instance.InterfaceOriginCode == "resource" && read.OwnerID == instance.ResourceOwnerId && read.ResourceChannelRef == instance.ResourceChannelRef &&
		read.InterfaceConfigurationRevision == revision.ResourceOwnerConfigurationRevision &&
		read.PluginID == exact.PluginID && read.PluginReleaseVersion == exact.ReleaseVersion && read.ContractVersion == exact.ContractVersion &&
		read.ReleaseDigest == exact.ReleaseDigest && read.DeclarationDigest == exact.DeclarationDigest &&
		len(read.AuthorizedMethods) == len(exact.AuthorizedMethods) && sameImportMethods(read.AuthorizedMethods, exact.AuthorizedMethods)
}

func sameImportMethods(left, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}

// importConfigurationValues accepts only bounded, explicit scalar arms.
func importConfigurationValues(supplied []resourceinterfaceprojectioncap.CatalogConfigurationValue) ([]ConfigurationValue, error) {
	if len(supplied) > 128 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	values := make([]ConfigurationValue, 0, len(supplied))
	for _, suppliedValue := range supplied {
		arms := 0
		if suppliedValue.TextValue != nil {
			arms++
			if len(*suppliedValue.TextValue) > 8192 {
				return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
			}
		}
		if suppliedValue.IntegerValue != nil {
			arms++
		}
		if suppliedValue.DecimalValue != nil {
			arms++
			if len(*suppliedValue.DecimalValue) > 128 {
				return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
			}
		}
		if suppliedValue.BooleanValue != nil {
			arms++
		}
		if suppliedValue.OptionValue != nil {
			arms++
			if len(*suppliedValue.OptionValue) > 256 {
				return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
			}
		}
		if arms != 1 {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		value := ConfigurationValue{FieldKey: suppliedValue.FieldKey, TextValue: suppliedValue.TextValue, IntegerValue: suppliedValue.IntegerValue, DecimalValue: suppliedValue.DecimalValue, BooleanValue: suppliedValue.BooleanValue}
		if suppliedValue.OptionValue != nil {
			value.OptionTokens = []string{*suppliedValue.OptionValue}
		}
		values = append(values, value)
	}
	return values, nil
}

// importResolutionSchema omits remote lookup for unfilled choices. Their empty
// candidate sets exist only for partial evaluation, never publication evidence.
func importResolutionSchema(schema *capmodel.ConfigurationSchemaDeclaration, values []ConfigurationValue, methods []string) (*capmodel.ConfigurationSchemaDeclaration, configurationcontract.ResolvedCandidates, error) {
	if _, err := configurationcontract.ValidateSchema(schema, configurationSurfaceProduct, methods); err != nil {
		return nil, nil, err
	}
	empty := make(configurationcontract.ResolvedCandidates)
	if schema == nil {
		return nil, empty, nil
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "copy import configuration schema")
	}
	var result capmodel.ConfigurationSchemaDeclaration
	if err = json.Unmarshal(encoded, &result); err != nil {
		return nil, nil, gerror.Wrap(err, "copy import configuration schema")
	}
	provided := make(map[string]bool, len(values))
	for _, value := range values {
		provided[value.FieldKey] = true
	}
	for _, field := range result.Fields {
		if field.DynamicCandidates != nil && !provided[field.Key] {
			field.DynamicCandidates = nil
			empty[field.Key] = []*capmodel.ConfigurationCandidate{}
		}
	}
	return &result, empty, nil
}

func (s *serviceImpl) WithImportBinding(ctx context.Context, prepared *ImportBinding, action func(context.Context) (string, error)) error {
	if prepared == nil || prepared.owner != s || action == nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	authorization, err := s.authorizer.AuthorizeInterfaceList(ctx)
	if err != nil {
		return err
	}
	if err = requireAuthorization(authorization); err != nil {
		return err
	}
	if authorization.ActorUserID != prepared.actor {
		return bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return normalizeManagementWriteError(s.extensions.WithNewBusinessAdmission(ctx, admissionInput(prepared.exact), func(guardCtx context.Context, evidence *capmodel.BusinessExtensionProviderEvidence) error {
		if !sameProviderContract(prepared.exact, evidence) {
			return bizerr.NewCode(catalog.CodeManagementConflict)
		}
		return s.database.Transaction(guardCtx, func(txCtx context.Context, _ gdb.TX) error {
			instance, err := s.loadInterface(txCtx, prepared.interfaceNo, true)
			if err != nil {
				return err
			}
			if instance == nil || instance.Version != prepared.interfaceVersion || instance.NewBusinessState != InterfaceStateAccepting || !sameProviderContract(interfaceEvidence(instance), evidence) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			revision, err := s.loadLatestInterfaceConfigurationRevision(txCtx, prepared.interfaceNo, true)
			if err != nil {
				return err
			}
			if revision == nil || revision.Revision != prepared.interfaceRevision {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if !matchesImportReadEvidence(&prepared.readEvidence, instance, revision, evidence) {
				return bizerr.NewCode(catalog.CodeManagementConflict)
			}
			if err = s.validateCurrentInterfaceConfigurationWithCandidates(txCtx, prepared.interfaceNo, evidence, prepared.interfaceCandidates); err != nil {
				return err
			}
			versionNo, err := action(txCtx)
			if err != nil || versionNo == "" {
				return err
			}
			authorization, err := s.authorizer.AuthorizeBindingWrite(txCtx, versionNo)
			if err != nil {
				return err
			}
			if err = requireAuthorization(authorization); err != nil {
				return err
			}
			if authorization.ActorUserID != prepared.actor {
				return bizerr.NewCode(catalog.CodeManagementNotAccessible)
			}
			if err = s.lockDraftVersion(txCtx, versionNo); err != nil {
				return err
			}
			_, err = s.database.Model(do.CatalogProductVersionFulfillmentBinding{}).Ctx(txCtx).Data(do.CatalogProductVersionFulfillmentBinding{
				TenantId: idccap.DefaultTenantID, VersionNo: versionNo, InterfaceInstanceNo: prepared.interfaceNo, InterfaceDisplayNameSnapshot: instance.DisplayName,
				PluginId: evidence.PluginID, ExtensionPointCode: evidence.Point, PluginReleaseVersion: evidence.ReleaseVersion, ContractVersion: evidence.ContractVersion,
				AuthorizedMethods: evidence.AuthorizedMethods, ReleaseDigest: evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest, BoundBy: prepared.actor, BoundAt: gdb.Raw("CURRENT_TIMESTAMP"),
			}).Insert()
			if err != nil {
				return gerror.Wrap(err, "insert imported draft binding")
			}
			_, err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(txCtx).Data(do.CatalogProductVersionConfiguration{
				TenantId: idccap.DefaultTenantID, VersionNo: versionNo, InterfaceInstanceNo: prepared.interfaceNo, InterfaceConfigurationRevision: prepared.interfaceRevision,
				ProductSchemaVersion: configurationSchemaVersion(evidence.ProductConfiguration), ProductDeclarationSnapshot: string(prepared.product.DeclarationJSON),
				MerchantValueSnapshot: string(prepared.product.ValuesJSON), CustomerSchemaVersion: configurationSchemaVersion(prepared.customerSchema), CustomerDeclarationSnapshot: string(prepared.customerJSON), CustomerPresentationSnapshot: string(prepared.customerPresentationJSON),
				DeclarationDigest: configurationDigest(append(append([]byte{}, prepared.product.DeclarationJSON...), prepared.customerJSON...)), MerchantValueDigest: configurationDigest(prepared.product.ValuesJSON), Version: 1, ChangedBy: prepared.actor,
			}).Insert()
			if err != nil {
				return gerror.Wrap(err, "freeze imported draft partial configuration")
			}
			if prepared.portable != nil {
				if err = s.insertImportedFulfillmentPolicy(txCtx, versionNo, prepared.actor); err != nil {
					return err
				}
			}
			bound, err := s.loadBinding(txCtx, versionNo, false)
			if err != nil {
				return err
			}
			if bound == nil {
				return gerror.New("imported binding is missing")
			}
			return s.audit.AppendBusinessTransition(txCtx, bindingAudit(authorization, BindInput{VersionNo: versionNo, InterfaceInstanceNo: prepared.interfaceNo}, nil, instance, bound.Id))
		})
	}))
}
