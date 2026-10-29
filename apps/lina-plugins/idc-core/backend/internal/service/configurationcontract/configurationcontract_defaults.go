// Merchant visibility never removes required provisioning values. Hidden
// fields use a typed, frozen default at every quote and purchase boundary.
package configurationcontract

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
)

func hiddenDefault(field *capmodel.ConfigurationFieldDeclaration, p Presentation) (*ConfigurationValue, error) {
	if p.CustomerVisible == nil || *p.CustomerVisible {
		return nil, nil
	}
	if field == nil || field.Type == capmodel.ConfigurationFieldSecret || field.Type == capmodel.ConfigurationFieldFile || len(p.DefaultValue) == 0 || len(p.DefaultValue) > 8192 {
		return nil, gerror.New("hidden purchase field requires a valid non-secret default")
	}
	decoder := json.NewDecoder(bytes.NewReader(p.DefaultValue))
	decoder.UseNumber()
	var raw any
	if err := decoder.Decode(&raw); err != nil || !json.Valid(p.DefaultValue) {
		return nil, gerror.New("hidden purchase default has an invalid shape")
	}
	value, err := decodeCompiledValue(field, field.Key, raw)
	if err != nil {
		return nil, gerror.Wrap(err, "hidden purchase default has an invalid type")
	}
	check := *field
	// Directory membership is validated with the resolved snapshot at runtime.
	// Saving still checks the exact typed shape and all static bounds.
	if field.DynamicCandidates != nil {
		check.DynamicCandidates = nil
		check.StaticCandidates = make([]*capmodel.ConfigurationCandidate, 0)
		for _, token := range append(append(append([]string{}, value.OptionTokens...), value.TreeOptionTokens...), value.AddressOptionTokens...) {
			check.StaticCandidates = append(check.StaticCandidates, &capmodel.ConfigurationCandidate{Value: token})
		}
	}
	normalized, _, err := validateConfigurationValue(&check, value, SurfaceCustomer)
	if err != nil || (field.Required && emptyValue(normalized)) {
		return nil, gerror.New("hidden purchase default violates field bounds")
	}
	if len(PresentationIssues(&capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{field}}, map[string]Presentation{field.Key: p}, []ConfigurationValue{value})) > 0 {
		return nil, gerror.New("hidden purchase default violates its sellable controls")
	}
	return &value, nil
}

// ApplyHiddenDefaults replaces caller input for hidden fields with the exact
// merchant default. Other input remains unchanged and is validated normally.
// It performs no I/O; resolved candidate/rule validation still follows.
func ApplyHiddenDefaults(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]Presentation, values []ConfigurationValue) ([]ConfigurationValue, error) {
	byKey := make(map[string]ConfigurationValue, len(values))
	for _, value := range values {
		if _, exists := byKey[value.FieldKey]; exists {
			return nil, gerror.New("duplicate purchase configuration field")
		}
		byKey[value.FieldKey] = value
	}
	if schema != nil {
		for _, field := range schema.Fields {
			if field == nil {
				continue
			}
			value, err := hiddenDefault(field, presentations[field.Key])
			if err != nil {
				return nil, err
			}
			if value != nil {
				byKey[field.Key] = *value
			}
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]ConfigurationValue, 0, len(keys))
	for _, key := range keys {
		result = append(result, byKey[key])
	}
	return result, nil
}

// ValidateHiddenValues checks immutable purchase/fulfillment evidence without
// repairing it. Old snapshots with no visibility metadata retain their behavior.
func ValidateHiddenValues(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]Presentation, values []ConfigurationValue) error {
	current := make(map[string]ConfigurationValue, len(values))
	for _, value := range values {
		current[value.FieldKey] = value
	}
	if schema == nil {
		return nil
	}
	for _, field := range schema.Fields {
		if field == nil {
			continue
		}
		defaultValue, err := hiddenDefault(field, presentations[field.Key])
		if err != nil {
			return err
		}
		if defaultValue == nil {
			continue
		}
		actual, exists := current[field.Key]
		if !exists {
			return gerror.New("frozen hidden purchase default is missing")
		}
		check := *field
		if field.DynamicCandidates != nil {
			check.DynamicCandidates = nil
			check.StaticCandidates = []*capmodel.ConfigurationCandidate{}
			for _, token := range append(append(append([]string{}, defaultValue.OptionTokens...), defaultValue.TreeOptionTokens...), defaultValue.AddressOptionTokens...) {
				check.StaticCandidates = append(check.StaticCandidates, &capmodel.ConfigurationCandidate{Value: token})
			}
		}
		got, _, gotErr := validateConfigurationValue(&check, actual, SurfaceCustomer)
		want, _, wantErr := validateConfigurationValue(&check, *defaultValue, SurfaceCustomer)
		if gotErr != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
			return gerror.New("frozen hidden purchase default has changed")
		}
	}
	return nil
}
