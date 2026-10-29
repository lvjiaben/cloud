// This file projects a published purchase schema into portable opaque keys.
// Conditions, candidate hierarchy and price references must use the same map;
// provider field names and executable candidate methods never cross the export.

package catalog

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const importedCandidateMethod = "peer.configuration.candidates"

// importSchemaProjection retains source keys only while constructing an export.
type importSchemaProjection struct {
	schema        *capmodel.ConfigurationSchemaDeclaration
	mappings      []channelcommercecap.ImportFieldMapping
	fields        map[string]string
	codec         *publicConfigurationRevisionCodec
	revision      string
	presentations map[string]configurationcontract.Presentation
}

func projectImportSchema(codec *publicConfigurationRevisionCodec, snapshot *publicStructuredConfigurationSnapshot) (*importSchemaProjection, error) {
	if codec == nil || snapshot == nil || snapshot.Revision == "" {
		return nil, gerror.New("import schema revision is unavailable")
	}
	if _, err := configurationcontract.ValidateSchema(snapshot.Schema, configurationcontract.SurfaceCustomer, authorizedDynamicMethods(snapshot.Schema)); err != nil {
		return nil, err
	}
	presentations, err := configurationcontract.DecodePresentations(snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, err
	}
	if err = configurationcontract.ValidatePresentations(snapshot.Schema, presentations); err != nil {
		return nil, err
	}
	result := &importSchemaProjection{presentations: presentations, fields: make(map[string]string), codec: codec, revision: snapshot.Revision, mappings: []channelcommercecap.ImportFieldMapping{}}
	if snapshot.Schema == nil {
		return result, nil
	}
	encoded, err := json.Marshal(snapshot.Schema)
	if err != nil {
		return nil, gerror.Wrap(err, "encode import schema")
	}
	if err = json.Unmarshal(encoded, &result.schema); err != nil {
		return nil, gerror.Wrap(err, "copy import schema")
	}
	for _, field := range result.schema.Fields {
		if err = result.projectField(field); err != nil {
			return nil, err
		}
	}
	for _, rule := range result.schema.Conditions {
		if err = result.projectCondition(rule.When, snapshot.Schema); err != nil {
			return nil, err
		}
		for _, action := range rule.Actions {
			sourceKey := action.Field
			action.Field = result.fields[sourceKey]
			for index, value := range action.Values {
				action.Values[index], err = result.optionKey(sourceKey, value)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if _, err = configurationcontract.ValidateSchema(result.schema, configurationcontract.SurfaceCustomer, []string{importedCandidateMethod}); err != nil {
		return nil, gerror.Wrap(err, "validate portable import schema")
	}
	return result, nil
}

func (p *importSchemaProjection) projectField(field *capmodel.ConfigurationFieldDeclaration) error {
	var (
		sourceKey = field.Key
		err       error
	)
	field.Key, err = p.key("field", sourceKey)
	if err != nil {
		return err
	}
	p.fields[sourceKey] = field.Key
	fieldToken, err := p.codec.encodeStructuredField(p.revision, sourceKey)
	if err != nil {
		return err
	}
	mapping := channelcommercecap.ImportFieldMapping{FieldKey: field.Key, FieldToken: fieldToken, Options: []channelcommercecap.ImportOptionMapping{}}
	for _, candidate := range field.StaticCandidates {
		value := candidate.Value
		candidate.Value, err = p.optionKey(sourceKey, value)
		if err != nil {
			return err
		}
		if candidate.ParentValue != "" {
			candidate.ParentValue, err = p.optionKey(sourceKey, candidate.ParentValue)
			if err != nil {
				return err
			}
		}
		token, tokenErr := p.codec.encodeStructuredOption(p.revision, sourceKey, value)
		if tokenErr != nil {
			return tokenErr
		}
		mapping.Options = append(mapping.Options, channelcommercecap.ImportOptionMapping{Value: candidate.Value, OptionToken: token})
	}
	if field.DynamicCandidates != nil {
		field.DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: importedCandidateMethod}
	}
	presentation := p.presentations[sourceKey]
	mapping.CustomerVisible = presentation.CustomerVisible
	if len(presentation.DefaultValue) > 0 {
		mapping.DefaultValue, err = p.projectDefault(sourceKey, field.Type, presentation.DefaultValue)
		if err != nil {
			return err
		}
	}
	p.mappings = append(p.mappings, mapping)
	return nil
}

func (p *importSchemaProjection) projectCondition(condition *capmodel.ConfigurationConditionDeclaration, source *capmodel.ConfigurationSchemaDeclaration) error {
	if condition.Field != "" {
		field := configurationFieldByKey(source, condition.Field)
		if field == nil {
			return gerror.New("import condition field is unavailable")
		}
		if condition.Value != "" && importCandidateField(field.Type) {
			value, err := p.optionKey(condition.Field, condition.Value)
			if err != nil {
				return err
			}
			condition.Value = value
		}
		condition.Field = p.fields[condition.Field]
	}
	for _, child := range condition.Children {
		if err := p.projectCondition(child, source); err != nil {
			return err
		}
	}
	return nil
}

func importCandidateField(kind string) bool {
	switch kind {
	case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress:
		return true
	default:
		return false
	}
}

func (p *importSchemaProjection) optionKey(field, value string) (string, error) {
	return p.key("option", field, value)
}

func (p *importSchemaProjection) key(kind string, parts ...string) (string, error) {
	parts = append([]string{p.revision, kind}, parts...)
	token, err := p.codec.encodeOpaque([]byte("idc-core/portable-import-key/v1"), parts...)
	if err != nil {
		return "", err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return "", gerror.New("import key encoding is invalid")
	}
	return "k_" + hex.EncodeToString(decoded[:24]), nil
}

// projectDefault uses the exact option projection used by conditions and prices.
func (p *importSchemaProjection) projectDefault(sourceKey, kind string, raw json.RawMessage) (json.RawMessage, error) {
	if !importCandidateField(kind) {
		return append(json.RawMessage(nil), raw...), nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case string:
		mapped, err := p.optionKey(sourceKey, v)
		if err != nil {
			return nil, err
		}
		return json.Marshal(mapped)
	case []any:
		mapped := make([]string, len(v))
		for i, item := range v {
			text, ok := item.(string)
			if !ok {
				return nil, gerror.New("import default option is invalid")
			}
			var err error
			mapped[i], err = p.optionKey(sourceKey, text)
			if err != nil {
				return nil, err
			}
		}
		return json.Marshal(mapped)
	default:
		return nil, gerror.New("import default option is invalid")
	}
}
