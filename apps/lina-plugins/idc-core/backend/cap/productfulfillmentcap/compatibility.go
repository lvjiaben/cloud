// This file proves narrowly additive product configuration upgrades. Execution
// still uses the host's real active release and authorization evidence.

package productfulfillmentcap

import (
	"cmp"
	"encoding/json"
	"slices"

	"lina-core/pkg/plugin/capability/capmodel"
)

// IsAdditiveConfigurationUpgrade permits only optional product/customer fields
// while preserving the old fields, interface, methods and behavioral metadata.
// Both inputs must be authentic host evidence; callers must separately fence
// the previous release/digests and keep current host admission checks intact.
// Unknown, removed, changed or newly required configuration returns false.
func IsAdditiveConfigurationUpgrade(previous, current *capmodel.BusinessExtensionProviderEvidence) bool {
	if previous == nil || current == nil || previous.PluginID != current.PluginID || previous.Point != capmodel.BusinessExtensionProductFulfillment || current.Point != previous.Point || previous.ContractVersion != current.ContractVersion || previous.ProviderMetadata == nil || current.ProviderMetadata == nil {
		return false
	}
	if !sameConfigurationValue(previous.InterfaceConfiguration, current.InterfaceConfiguration) || previous.InterfaceConfigurationCardinality != current.InterfaceConfigurationCardinality || !sameConfigurationValue(previous.AuthorizedMethods, current.AuthorizedMethods) {
		return false
	}
	if !IsAdditiveConfigurationSchema(previous.ProductConfiguration, current.ProductConfiguration) || !IsAdditiveConfigurationSchema(previous.CustomerConfiguration, current.CustomerConfiguration) {
		return false
	}
	oldMeta, newMeta := *previous.ProviderMetadata, *current.ProviderMetadata
	oldFields, oldOK := merchantFields(oldMeta.Values["merchantCustomerFields"])
	newFields, newOK := merchantFields(newMeta.Values["merchantCustomerFields"])
	if !oldOK || !newOK {
		return false
	}
	for _, key := range oldFields {
		if !slices.Contains(newFields, key) {
			return false
		}
	}
	for _, key := range newFields {
		if slices.Contains(oldFields, key) {
			continue
		}
		found := false
		if current.CustomerConfiguration != nil {
			for _, field := range current.CustomerConfiguration.Fields {
				if field != nil && field.Key == key && !field.Required {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	newMeta.Values = make(map[string]any, len(current.ProviderMetadata.Values))
	for key, value := range current.ProviderMetadata.Values {
		newMeta.Values[key] = value
	}
	if value, exists := previous.ProviderMetadata.Values["merchantCustomerFields"]; exists {
		newMeta.Values["merchantCustomerFields"] = value
	} else {
		delete(newMeta.Values, "merchantCustomerFields")
	}
	if !normalizeProductChoiceFields(previous, current, &newMeta) || !normalizeCustomerProductSources(previous, current, &newMeta) {
		return false
	}
	return sameConfigurationValue(oldMeta, newMeta)
}

// IsAdditiveConfigurationSchema proves an optional product/customer schema
// extension. Interface/credential schemas must instead match exactly. Conditions
// and upload policies must remain unchanged. Existing technical field contracts
// cannot change; explanatory descriptions do not affect accepted input values.
func IsAdditiveConfigurationSchema(previous, current *capmodel.ConfigurationSchemaDeclaration) bool {
	if previous == nil || current == nil {
		return previous == nil && current == nil
	}
	if previous.Version != current.Version ||
		((len(previous.Conditions) > 0 || len(current.Conditions) > 0) && !sameConfigurationValue(previous.Conditions, current.Conditions)) ||
		((len(previous.UploadPolicies) > 0 || len(current.UploadPolicies) > 0) && !sameConfigurationValue(previous.UploadPolicies, current.UploadPolicies)) {
		return false
	}
	oldFields := make(map[string]*capmodel.ConfigurationFieldDeclaration, len(previous.Fields))
	for _, field := range previous.Fields {
		if field == nil || field.Key == "" || oldFields[field.Key] != nil {
			return false
		}
		oldFields[field.Key] = field
	}
	seen := make(map[string]bool, len(current.Fields))
	for _, field := range current.Fields {
		if field == nil || field.Key == "" || seen[field.Key] {
			return false
		}
		seen[field.Key] = true
		if old, exists := oldFields[field.Key]; exists {
			if !sameConfigurationField(old, field) {
				return false
			}
		} else if field.Required || field.Type == capmodel.ConfigurationFieldSecret || field.Type == capmodel.ConfigurationFieldFile {
			return false
		}
	}
	for key := range oldFields {
		if !seen[key] {
			return false
		}
	}
	return true
}

func merchantFields(value any) ([]string, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var fields []string
	if json.Unmarshal(encoded, &fields) != nil {
		return nil, false
	}
	return fields, true
}

func sameConfigurationValue(left, right any) bool {
	a, err := json.Marshal(left)
	if err != nil {
		return false
	}
	b, err := json.Marshal(right)
	return err == nil && string(a) == string(b)
}

// sameConfigurationField treats static choices as a value-indexed set. Host
// normalization may reorder them; presentation order cannot invalidate saved
// product values or force merchants to rebuild their products.
func sameConfigurationField(previous, current *capmodel.ConfigurationFieldDeclaration) bool {
	left, right := *previous, *current
	// Correcting help text must not invalidate saved products. Compare detached
	// values so historical snapshots and the current provider stay untouched.
	left.Description, right.Description = nil, nil
	normalize := func(items []*capmodel.ConfigurationCandidate) []*capmodel.ConfigurationCandidate {
		result := append([]*capmodel.ConfigurationCandidate(nil), items...)
		slices.SortFunc(result, func(a, b *capmodel.ConfigurationCandidate) int {
			if a == nil {
				if b == nil {
					return 0
				}
				return -1
			}
			if b == nil {
				return 1
			}
			return cmp.Compare(a.Value, b.Value)
		})
		return result
	}
	left.StaticCandidates = normalize(left.StaticCandidates)
	right.StaticCandidates = normalize(right.StaticCandidates)
	return sameConfigurationValue(left, right)
}

// normalizeCustomerProductSources permits only additive, declared override identities.
func normalizeCustomerProductSources(previous, current *capmodel.BusinessExtensionProviderEvidence, normalized *capmodel.BusinessExtensionProviderMetadata) bool {
	oldFields, oldOK := customerProductSources(previous.ProviderMetadata.Values["customerProductFields"])
	newFields, newOK := customerProductSources(current.ProviderMetadata.Values["customerProductFields"])
	if !oldOK || !newOK {
		return false
	}
	for key, target := range newFields {
		if !declaredConfigurationKey(current.CustomerConfiguration, key) || !declaredConfigurationKey(current.ProductConfiguration, target) {
			return false
		}
	}
	for key, target := range oldFields {
		if newFields[key] != target {
			return false
		}
	}
	if value, exists := previous.ProviderMetadata.Values["customerProductFields"]; exists {
		normalized.Values["customerProductFields"] = value
	} else {
		delete(normalized.Values, "customerProductFields")
	}
	return true
}

func customerProductSources(value any) (map[string]string, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var fields map[string]string
	if json.Unmarshal(raw, &fields) != nil {
		return nil, false
	}
	return fields, true
}

func declaredConfigurationKey(schema *capmodel.ConfigurationSchemaDeclaration, key string) bool {
	if schema == nil {
		return false
	}
	for _, field := range schema.Fields {
		if field != nil && field.Key == key && field.Type != capmodel.ConfigurationFieldSecret && field.Type != capmodel.ConfigurationFieldFile {
			return true
		}
	}
	return false
}

// normalizeProductChoiceFields permits a provider to add explicit pure profile
// validation support, while rejecting removal or unknown/non-text targets.
func normalizeProductChoiceFields(previous, current *capmodel.BusinessExtensionProviderEvidence, normalized *capmodel.BusinessExtensionProviderMetadata) bool {
	oldFields, oldOK := merchantFields(previous.ProviderMetadata.Values["productChoiceFields"])
	newFields, newOK := merchantFields(current.ProviderMetadata.Values["productChoiceFields"])
	if !oldOK || !newOK {
		return false
	}
	for _, key := range oldFields {
		if !slices.Contains(newFields, key) {
			return false
		}
	}
	for _, key := range newFields {
		found := false
		if current.ProductConfiguration != nil {
			for _, field := range current.ProductConfiguration.Fields {
				if field != nil && field.Key == key && field.Type == capmodel.ConfigurationFieldSingleLine {
					found = true
				}
			}
		}
		if !found || !slices.Contains(current.AuthorizedMethods, MethodValidateProductConfiguration) {
			return false
		}
	}
	if value, exists := previous.ProviderMetadata.Values["productChoiceFields"]; exists {
		normalized.Values["productChoiceFields"] = value
	} else {
		delete(normalized.Values, "productChoiceFields")
	}
	return true
}
