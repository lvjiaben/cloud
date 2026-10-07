package configurationcontract

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const (
	DynamicCandidateProtocolV1       = "v1"
	MaximumDynamicCandidateFields    = 128
	MaximumDynamicCandidatesPerField = 512
	MaximumDynamicCandidateRequest   = 60 * 1024
	MaximumDynamicCandidatePayload   = 1 << 20
)

// DynamicCandidateQuery is one member of the governed, method-batched request.
// Values are plugin-declared safe candidate values, never public option tokens.
type DynamicCandidateQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues"`
	SelectedValues []string `json:"selectedValues"`
}

// DynamicCandidateSelection contains only governed selection values. Text,
// secret, file, and every other value arm are deliberately excluded.
type DynamicCandidateSelection struct {
	FieldKey string   `json:"fieldKey"`
	Values   []string `json:"values"`
}

type DynamicCandidateRequest struct {
	SchemaVersion     string                      `json:"schemaVersion"`
	CurrentSelections []DynamicCandidateSelection `json:"currentSelections"`
	Requests          []DynamicCandidateQuery     `json:"requests"`
}

type DynamicCandidateItem struct {
	FieldKey   string                             `json:"fieldKey"`
	Candidates []*capmodel.ConfigurationCandidate `json:"candidates"`
}

type DynamicCandidateResponse struct {
	SchemaVersion string                 `json:"schemaVersion"`
	Items         []DynamicCandidateItem `json:"items"`
}

// DynamicCandidateBatch contains all fields owned by one exact authorized
// method. A caller invokes each batch once instead of calling per field.
type DynamicCandidateBatch struct {
	Method  string
	Payload []byte
	Fields  []string
	Queries []DynamicCandidateQuery
}

type ResolvedCandidates map[string][]*capmodel.ConfigurationCandidate

func BuildDynamicCandidateBatches(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
) ([]DynamicCandidateBatch, error) {
	fields, err := validateConfigurationSchema(schema, surface, authorizedMethods)
	if err != nil {
		return nil, err
	}
	selected, err := selectedCandidateValues(fields, values)
	if err != nil {
		return nil, err
	}
	currentSelections := orderedDynamicCandidateSelections(selected)
	byMethod := make(map[string][]DynamicCandidateQuery)
	for _, field := range schemaFields(schema) {
		if field.DynamicCandidates == nil {
			continue
		}
		current := append([]string(nil), selected[field.Key]...)
		parent := dynamicParentPath(field.Type, current)
		byMethod[field.DynamicCandidates.Method] = append(byMethod[field.DynamicCandidates.Method], DynamicCandidateQuery{
			FieldKey: field.Key, ParentValues: parent, SelectedValues: current,
		})
	}
	methods := make([]string, 0, len(byMethod))
	for method := range byMethod {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	batches := make([]DynamicCandidateBatch, 0, len(methods))
	for _, method := range methods {
		queries := byMethod[method]
		sort.Slice(queries, func(i, j int) bool { return queries[i].FieldKey < queries[j].FieldKey })
		if len(queries) == 0 || len(queries) > MaximumDynamicCandidateFields {
			return nil, gerror.New("dynamic candidate request field bound exceeded")
		}
		payload, marshalErr := json.Marshal(DynamicCandidateRequest{
			SchemaVersion: DynamicCandidateProtocolV1, CurrentSelections: currentSelections, Requests: queries,
		})
		if marshalErr != nil {
			return nil, gerror.Wrap(marshalErr, "encode dynamic candidate request")
		}
		if len(payload) > MaximumDynamicCandidateRequest {
			return nil, gerror.New("dynamic candidate request payload bound exceeded")
		}
		fieldKeys := make([]string, len(queries))
		for index := range queries {
			fieldKeys[index] = queries[index].FieldKey
		}
		batches = append(batches, DynamicCandidateBatch{Method: method, Payload: payload, Fields: fieldKeys, Queries: queries})
	}
	return batches, nil
}

func DecodeDynamicCandidateResponse(payload []byte, expectedQueries []DynamicCandidateQuery) (ResolvedCandidates, error) {
	if len(payload) == 0 || len(payload) > MaximumDynamicCandidatePayload || len(expectedQueries) == 0 || len(expectedQueries) > MaximumDynamicCandidateFields {
		return nil, gerror.New("dynamic candidate response bound is invalid")
	}
	var response DynamicCandidateResponse
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return nil, gerror.Wrap(err, "decode dynamic candidate response")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, gerror.New("dynamic candidate response contains trailing data")
	}
	if response.SchemaVersion != DynamicCandidateProtocolV1 || len(response.Items) != len(expectedQueries) {
		return nil, gerror.New("dynamic candidate response version or field set is invalid")
	}
	expected := make(map[string]DynamicCandidateQuery, len(expectedQueries))
	for _, query := range expectedQueries {
		if !validConfigurationKey(query.FieldKey) {
			return nil, gerror.New("dynamic candidate expected field is invalid")
		}
		if _, duplicate := expected[query.FieldKey]; duplicate {
			return nil, gerror.New("dynamic candidate expected fields are duplicated")
		}
		expected[query.FieldKey] = query
	}
	resolved := make(ResolvedCandidates, len(response.Items))
	for _, item := range response.Items {
		query, ok := expected[item.FieldKey]
		if !ok {
			return nil, gerror.New("dynamic candidate response contains an unknown field")
		}
		if _, duplicate := resolved[item.FieldKey]; duplicate {
			return nil, gerror.New("dynamic candidate response fields are duplicated")
		}
		if len(item.Candidates) > MaximumDynamicCandidatesPerField {
			return nil, gerror.New("dynamic candidate response candidate bound exceeded")
		}
		candidates := cloneCandidates(item.Candidates)
		if err := validateDynamicCandidateSet(candidates, query.ParentValues); err != nil {
			return nil, err
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].ParentValue != candidates[j].ParentValue {
				return candidates[i].ParentValue < candidates[j].ParentValue
			}
			return candidates[i].Value < candidates[j].Value
		})
		resolved[item.FieldKey] = candidates
	}
	return resolved, nil
}

func validateDynamicCandidateSet(candidates []*capmodel.ConfigurationCandidate, allowedExternalParents []string) error {
	byValue := make(map[string]*capmodel.ConfigurationCandidate, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.Value == "" || len(candidate.Value) > 256 || !validLocalizedText(candidate.Label) {
			return gerror.New("dynamic candidate response contains an invalid candidate")
		}
		if _, duplicate := byValue[candidate.Value]; duplicate {
			return gerror.New("dynamic candidate response contains a duplicate candidate")
		}
		byValue[candidate.Value] = candidate
	}
	for _, candidate := range candidates {
		if candidate.ParentValue == "" {
			continue
		}
		if candidate.ParentValue == candidate.Value || byValue[candidate.ParentValue] == nil {
			return gerror.New("dynamic candidate response contains an invalid parent")
		}
		seen := map[string]struct{}{candidate.Value: {}}
		current := candidate
		for current.ParentValue != "" {
			if _, cycle := seen[current.ParentValue]; cycle {
				return gerror.New("dynamic candidate response contains a parent cycle")
			}
			seen[current.ParentValue] = struct{}{}
			current = byValue[current.ParentValue]
			if current == nil || len(seen) > maximumConditionDepth {
				return gerror.New("dynamic candidate response parent depth is invalid")
			}
		}
	}
	for index, value := range allowedExternalParents {
		candidate := byValue[value]
		if candidate == nil || index == 0 && candidate.ParentValue != "" ||
			index > 0 && candidate.ParentValue != allowedExternalParents[index-1] {
			return gerror.New("dynamic candidate response does not match the requested parent path")
		}
	}
	return nil
}

func CompileResolved(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
	resolved ResolvedCandidates,
) (*CompiledConfiguration, error) {
	return compileResolvedConfiguration(schema, values, surface, authorizedMethods, resolved, false)
}

// CompileImportedDraftResolved freezes a partial imported product draft. Only
// absent required fields are tolerated; publication must use CompileResolved.
func CompileImportedDraftResolved(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	authorizedMethods []string,
	resolved ResolvedCandidates,
) (*CompiledConfiguration, error) {
	return compileResolvedConfiguration(schema, values, SurfaceProduct, authorizedMethods, resolved, true)
}

func compileResolvedConfiguration(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
	resolved ResolvedCandidates,
	allowMissing bool,
) (*CompiledConfiguration, error) {
	runtimeSchema, err := runtimeSchemaWithCandidates(schema, surface, authorizedMethods, resolved)
	if err != nil {
		return nil, err
	}
	compiled, err := compileConfigurationValues(runtimeSchema, values, surface, nil, allowMissing)
	if err != nil {
		return nil, err
	}
	declarationJSON := []byte("{}")
	if schema != nil {
		declarationJSON, err = json.Marshal(schema)
	}
	if err != nil {
		return nil, gerror.Wrap(err, "encode configuration declaration")
	}
	compiled.DeclarationJSON = declarationJSON
	return compiled, nil
}

func EvaluateResolved(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
	resolved ResolvedCandidates,
) (*Evaluation, error) {
	runtimeSchema, err := runtimeSchemaWithCandidates(schema, surface, authorizedMethods, resolved)
	if err != nil {
		return nil, err
	}
	return Evaluate(runtimeSchema, values, surface, nil)
}

// MaterializeResolvedSchema returns a presentation-only declaration copy. It
// replaces authorized dynamic methods with their already validated safe
// bilingual candidates so a trusted host form never receives method names.
func MaterializeResolvedSchema(
	schema *capmodel.ConfigurationSchemaDeclaration,
	surface ConfigurationSurface,
	authorizedMethods []string,
	resolved ResolvedCandidates,
) (*capmodel.ConfigurationSchemaDeclaration, error) {
	return runtimeSchemaWithCandidates(schema, surface, authorizedMethods, resolved)
}

func runtimeSchemaWithCandidates(
	schema *capmodel.ConfigurationSchemaDeclaration,
	surface ConfigurationSurface,
	authorizedMethods []string,
	resolved ResolvedCandidates,
) (*capmodel.ConfigurationSchemaDeclaration, error) {
	if _, err := validateConfigurationSchema(schema, surface, authorizedMethods); err != nil {
		return nil, err
	}
	if schema == nil {
		if len(resolved) != 0 {
			return nil, gerror.New("dynamic candidates require a schema")
		}
		return nil, nil
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, gerror.Wrap(err, "clone configuration schema")
	}
	var runtime capmodel.ConfigurationSchemaDeclaration
	if err = json.Unmarshal(encoded, &runtime); err != nil {
		return nil, gerror.Wrap(err, "clone configuration schema")
	}
	dynamicFields := make(map[string]struct{})
	for _, field := range runtime.Fields {
		if field.DynamicCandidates == nil {
			continue
		}
		dynamicFields[field.Key] = struct{}{}
		candidates, exists := resolved[field.Key]
		if !exists {
			return nil, gerror.New("dynamic configuration candidates are unavailable")
		}
		field.DynamicCandidates = nil
		field.StaticCandidates = cloneCandidates(candidates)
	}
	for fieldKey := range resolved {
		if _, exists := dynamicFields[fieldKey]; !exists {
			return nil, gerror.New("resolved candidates contain an unknown field")
		}
	}
	if _, err = validateConfigurationSchema(&runtime, surface, nil); err != nil {
		return nil, err
	}
	return &runtime, nil
}

func selectedCandidateValues(fields map[string]*capmodel.ConfigurationFieldDeclaration, values []ConfigurationValue) (map[string][]string, error) {
	selected := make(map[string][]string)
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		field := fields[value.FieldKey]
		if field == nil {
			return nil, gerror.New("dynamic candidate input contains an unknown field")
		}
		if _, duplicate := seen[value.FieldKey]; duplicate {
			return nil, gerror.New("dynamic candidate input contains a duplicate field")
		}
		seen[value.FieldKey] = struct{}{}
		switch field.Type {
		case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect:
			selected[field.Key] = append([]string(nil), value.OptionTokens...)
		case capmodel.ConfigurationFieldTreeSelect:
			selected[field.Key] = append([]string(nil), value.TreeOptionTokens...)
		case capmodel.ConfigurationFieldAddress:
			selected[field.Key] = append([]string(nil), value.AddressOptionTokens...)
		}
		if len(selected[field.Key]) > MaximumDynamicCandidatesPerField || duplicateStrings(selected[field.Key]) {
			return nil, gerror.New("dynamic candidate current selection is invalid")
		}
	}
	if len(selected) > MaximumDynamicCandidateFields {
		return nil, gerror.New("dynamic candidate current selection field bound exceeded")
	}
	return selected, nil
}

func orderedDynamicCandidateSelections(selected map[string][]string) []DynamicCandidateSelection {
	fieldKeys := make([]string, 0, len(selected))
	for fieldKey := range selected {
		fieldKeys = append(fieldKeys, fieldKey)
	}
	sort.Strings(fieldKeys)
	result := make([]DynamicCandidateSelection, 0, len(fieldKeys))
	for _, fieldKey := range fieldKeys {
		result = append(result, DynamicCandidateSelection{
			FieldKey: fieldKey, Values: append([]string(nil), selected[fieldKey]...),
		})
	}
	return result
}

func dynamicParentPath(fieldType string, selected []string) []string {
	if fieldType != capmodel.ConfigurationFieldTreeSelect && fieldType != capmodel.ConfigurationFieldAddress || len(selected) < 2 {
		return []string{}
	}
	return append([]string(nil), selected[:len(selected)-1]...)
}

func schemaFields(schema *capmodel.ConfigurationSchemaDeclaration) []*capmodel.ConfigurationFieldDeclaration {
	if schema == nil {
		return nil
	}
	return schema.Fields
}

func duplicateStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || len(value) > 256 {
			return true
		}
		if _, duplicate := seen[value]; duplicate {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func cloneCandidates(values []*capmodel.ConfigurationCandidate) []*capmodel.ConfigurationCandidate {
	result := make([]*capmodel.ConfigurationCandidate, len(values))
	for index, candidate := range values {
		if candidate == nil {
			continue
		}
		copyCandidate := *candidate
		if candidate.Label != nil {
			copyLabel := *candidate.Label
			copyCandidate.Label = &copyLabel
		}
		result[index] = &copyCandidate
	}
	return result
}
