// Local presentation edits reuse identical, already validated merchant facts.
package productautomation

import (
	"context"
	"encoding/json"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"reflect"
	"strconv"
	"strings"
)

func (s *serviceImpl) reuseSavedMerchantConfiguration(ctx context.Context, input SaveVersionConfigurationInput, exact *capmodel.BusinessExtensionProviderEvidence, interfaceNo string, revision int64) (*canonicalConfiguration, bool, error) {
	row, err := s.loadVersionConfigurationRow(ctx, input.VersionNo)
	if err != nil {
		return nil, false, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil || row.InterfaceInstanceNo != interfaceNo || row.InterfaceConfigurationRevision != revision {
		return nil, false, nil
	}
	if row.Version != input.ExpectedVersion {
		return nil, false, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	declaration, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil || !sameCompatibleProductConfiguration(declaration, exact.ProductConfiguration) {
		return nil, false, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	document, valuesJSON, err := decodeStoredMerchantValues(row.MerchantValueSnapshot)
	if err != nil {
		return nil, false, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	values, err := configurationValuesFromJSON(declaration, valuesJSON)
	if err != nil || row.MerchantValueDigest != configurationDigest(valuesJSON) {
		return nil, false, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if !sameSavedMerchantValues(values, input.MerchantValues) {
		return nil, false, nil
	}
	if err = configurationcontract.ValidateStoredCompleteness(declaration, document, configurationSurfaceProduct, exact.AuthorizedMethods); err != nil {
		return nil, false, bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	declarationJSON, err := configurationDeclarationJSON(declaration)
	if err != nil {
		return nil, false, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return &canonicalConfiguration{DeclarationJSON: declarationJSON, ValuesJSON: valuesJSON}, true, nil
}

// PostgreSQL JSONB changes spacing and object key order. Preserve exact
// integer/decimal lexemes while recovering the producer's canonical JSON;
// hashing database display text or decoding through float64 loses evidence.
func decodeStoredMerchantValues(raw string) (map[string]any, []byte, error) {
	var document map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, nil, gerror.Wrap(err, "decode stored merchant configuration")
	}
	if document == nil {
		return nil, nil, gerror.New("stored merchant configuration must be an object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, nil, gerror.New("stored merchant configuration contains trailing data")
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "encode stored merchant configuration")
	}
	return document, encoded, nil
}

// A candidate provider outage cannot hide the saved schema. Empty dynamic lists
// are explicit and never authorize new upstream values; execution still resolves
// and validates candidates through its existing runtime path.
func (s *serviceImpl) materializeSavedConfiguration(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration, values []ConfigurationValue, surface ConfigurationSurface, scope dynamicCandidateScope) (*capmodel.ConfigurationSchemaDeclaration, configurationcontract.ResolvedCandidates, error) {
	resolved, err := s.resolveDynamicCandidates(ctx, exact, schema, values, surface, scope)
	if err == nil {
		if _, err = configurationcontract.EvaluateResolved(schema, values, surface, exact.AuthorizedMethods, resolved); err != nil {
			return nil, nil, err
		}
		presentation, materializeErr := configurationcontract.MaterializeResolvedSchema(schema, surface, exact.AuthorizedMethods, resolved)
		return presentation, resolved, materializeErr
	}
	resolved = configurationcontract.ResolvedCandidates{}
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil && field.DynamicCandidates != nil {
				resolved[field.Key] = []*capmodel.ConfigurationCandidate{}
			}
		}
	}
	presentation, err := configurationcontract.MaterializeResolvedSchema(schema, surface, exact.AuthorizedMethods, resolved)
	return presentation, resolved, err
}

func sameSavedMerchantValues(saved, supplied []ConfigurationValue) bool {
	if len(saved) != len(supplied) {
		return false
	}
	byKey := make(map[string]ConfigurationValue, len(saved))
	normalize := func(value ConfigurationValue) ConfigurationValue {
		if value.IntegerValue != nil && value.IntegerTextValue == nil {
			text := strconv.FormatInt(*value.IntegerValue, 10)
			value.IntegerTextValue = &text
			value.IntegerValue = nil
		}
		return value
	}
	for _, value := range saved {
		byKey[value.FieldKey] = normalize(value)
	}
	for _, value := range supplied {
		existing, ok := byKey[value.FieldKey]
		if !ok || !reflect.DeepEqual(existing, normalize(value)) {
			return false
		}
		delete(byKey, value.FieldKey)
	}
	return len(byKey) == 0
}
func (s *serviceImpl) compileEditableMerchantConfiguration(ctx context.Context, input SaveVersionConfigurationInput, exact *capmodel.BusinessExtensionProviderEvidence, interfaceNo string, revision int64, scope dynamicCandidateScope) (*canonicalConfiguration, bool, error) {
	compiled, reused, err := s.reuseSavedMerchantConfiguration(ctx, input, exact, interfaceNo, revision)
	if err != nil || reused {
		return compiled, reused, err
	}
	compiled, err = s.compileConfigurationResolved(ctx, exact, exact.ProductConfiguration, input.MerchantValues, configurationSurfaceProduct, scope)
	if err != nil {
		return nil, false, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return compiled, false, nil
}
func (s *serviceImpl) validateEditableCustomerSchema(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, schema *capmodel.ConfigurationSchemaDeclaration, scope dynamicCandidateScope, reused bool) error {
	if _, err := configurationcontract.ValidateSchema(schema, configurationSurfaceCustomer, exact.AuthorizedMethods); err != nil {
		return err
	}
	if schema != nil && !reused {
		return s.validateConfigurationRuntime(ctx, exact, schema, nil, configurationSurfaceCustomer, scope)
	}
	return nil
}
