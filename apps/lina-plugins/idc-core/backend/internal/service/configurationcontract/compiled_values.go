package configurationcontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"

	"lina-core/pkg/plugin/capability/capmodel"
)

// DecodeCompiledValues restores typed compiler output for an immutable order
// snapshot. It rejects unknown keys and non-canonical value shapes; callers
// must still run CompileResolved against the frozen declaration and candidates.
func DecodeCompiledValues(schema *capmodel.ConfigurationSchemaDeclaration, document []byte) ([]ConfigurationValue, error) {
	if len(document) == 0 || len(document) > 1024*1024 {
		return nil, errors.New("compiled configuration value document is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var values map[string]any
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, errors.New("compiled configuration value document is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("compiled configuration value document contains trailing data")
	}
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				fields[field.Key] = field
			}
		}
	}
	if len(values) > len(fields) || len(values) > maximumConfigurationFields {
		return nil, errors.New("compiled configuration value bound exceeded")
	}
	result := make([]ConfigurationValue, 0, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		field := fields[key]
		if field == nil {
			return nil, errors.New("compiled configuration contains an unknown field")
		}
		value, err := decodeCompiledValue(field, key, values[key])
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func decodeCompiledValue(field *capmodel.ConfigurationFieldDeclaration, key string, raw any) (ConfigurationValue, error) {
	value := ConfigurationValue{FieldKey: key}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		number, ok := raw.(json.Number)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled integer shape is invalid")
		}
		parsed, err := strconv.ParseInt(string(number), 10, 64)
		if err != nil {
			return ConfigurationValue{}, err
		}
		value.IntegerValue = &parsed
	case capmodel.ConfigurationFieldBoolean:
		parsed, ok := raw.(bool)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled boolean shape is invalid")
		}
		value.BooleanValue = &parsed
	case capmodel.ConfigurationFieldMultiSelect:
		parsed, err := compiledStringSlice(raw)
		if err != nil {
			return ConfigurationValue{}, err
		}
		value.OptionTokens = parsed
	case capmodel.ConfigurationFieldSingleSelect:
		parsed, ok := raw.(string)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled select shape is invalid")
		}
		value.OptionTokens = []string{parsed}
	case capmodel.ConfigurationFieldTreeSelect:
		parsed, ok := raw.(string)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled tree shape is invalid")
		}
		value.TreeOptionTokens = []string{parsed}
	case capmodel.ConfigurationFieldAddress:
		parsed, err := compiledStringSlice(raw)
		if err != nil {
			return ConfigurationValue{}, err
		}
		value.AddressOptionTokens = parsed
	case capmodel.ConfigurationFieldFile:
		parsed, err := compiledStringSlice(raw)
		if err != nil {
			return ConfigurationValue{}, err
		}
		value.FileTokens = parsed
	case capmodel.ConfigurationFieldDecimal:
		parsed, ok := raw.(string)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled decimal shape is invalid")
		}
		value.DecimalValue = &parsed
	case capmodel.ConfigurationFieldDate:
		parsed, ok := raw.(string)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled date shape is invalid")
		}
		value.DateValue = &parsed
	case capmodel.ConfigurationFieldDateTime:
		parsed, ok := raw.(string)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled datetime shape is invalid")
		}
		value.DateTimeValue = &parsed
	default:
		parsed, ok := raw.(string)
		if !ok {
			return ConfigurationValue{}, errors.New("compiled text shape is invalid")
		}
		value.TextValue = &parsed
	}
	return value, nil
}

func compiledStringSlice(raw any) ([]string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, errors.New("compiled list shape is invalid")
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, errors.New("compiled list entry is invalid")
		}
		result = append(result, value)
	}
	return result, nil
}
