// This file evaluates the bounded declarative condition protocol and returns
// authoritative field state without executing plugin code.

package configurationcontract

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const (
	IssueInvalidValue     = "invalid_value"
	IssueRequired         = "required"
	IssueDisabledValue    = "disabled_value"
	IssueFilteredOption   = "filtered_option"
	IssueRequiredRelation = "required_relation"
	IssueExcludedRelation = "excluded_relation"
)

// FieldState is the server-owned result of all active declaration rules.
type FieldState struct {
	Visible               bool
	Required              bool
	Disabled              bool
	CandidateValues       []string
	candidateFilterActive bool
	requiredValues        []string
	excludedValues        []string
	requiresActive        bool
	excludesActive        bool
}

// Issue is one stable, localizable customer-configuration problem.
type Issue struct {
	FieldKey string
	Code     string
}

// Evaluation is the canonical partial value set and its authoritative state.
type Evaluation struct {
	Values []ConfigurationValue
	States map[string]FieldState
	Issues []Issue
	Valid  bool
}

// Evaluate validates a partial interactive value set and evaluates every
// bounded condition using the same schema compiler as persistence boundaries.
func Evaluate(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (*Evaluation, error) {
	fields, err := validateConfigurationSchema(schema, surface, authorizedMethods)
	if err != nil {
		return nil, err
	}
	if schema == nil {
		if len(values) != 0 {
			return nil, gerror.New("configuration values require a schema")
		}
		return &Evaluation{Values: []ConfigurationValue{}, States: map[string]FieldState{}, Issues: []Issue{}, Valid: true}, nil
	}
	if len(values) > maximumConfigurationFields {
		return nil, gerror.New("configuration value bound exceeded")
	}
	normalized, normalizedRows, issues, err := normalizeEvaluationValues(fields, values, surface)
	if err != nil {
		return nil, err
	}
	states := initialConfigurationFieldStates(schema, fields)
	if err = applyConfigurationRules(schema, normalized, states); err != nil {
		return nil, err
	}
	issues = append(issues, configurationStateIssues(schema, normalized, states)...)
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].FieldKey != issues[j].FieldKey {
			return issues[i].FieldKey < issues[j].FieldKey
		}
		return issues[i].Code < issues[j].Code
	})
	return &Evaluation{
		Values: authoritativeConfigurationRows(normalizedRows, states),
		States: states, Issues: issues, Valid: len(issues) == 0,
	}, nil
}

func normalizeEvaluationValues(
	fields map[string]*capmodel.ConfigurationFieldDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
) (map[string]any, []ConfigurationValue, []Issue, error) {
	normalized := make(map[string]any, len(values))
	normalizedRows := make([]ConfigurationValue, 0, len(values))
	issues := make([]Issue, 0)
	seen := make(map[string]struct{}, len(values))
	for _, supplied := range values {
		field := fields[supplied.FieldKey]
		if field == nil {
			return nil, nil, nil, gerror.New("configuration contains an unknown field")
		}
		if _, duplicate := seen[supplied.FieldKey]; duplicate {
			return nil, nil, nil, gerror.New("configuration contains a duplicate field")
		}
		seen[supplied.FieldKey] = struct{}{}
		value, secret, valueErr := validateConfigurationValue(field, supplied, surface)
		if valueErr != nil {
			issues = append(issues, Issue{FieldKey: field.Key, Code: IssueInvalidValue})
			continue
		}
		normalized[field.Key] = value
		if !secret {
			row, rowErr := valueRow(field, value)
			if rowErr != nil {
				return nil, nil, nil, rowErr
			}
			normalizedRows = append(normalizedRows, row)
		}
	}
	return normalized, normalizedRows, issues, nil
}

func initialConfigurationFieldStates(
	schema *capmodel.ConfigurationSchemaDeclaration,
	fields map[string]*capmodel.ConfigurationFieldDeclaration,
) map[string]FieldState {
	states := make(map[string]FieldState, len(fields))
	showTargets := make(map[string]struct{})
	for _, rule := range schema.Conditions {
		for _, action := range rule.Actions {
			if action.Action == capmodel.ConfigurationActionShow {
				showTargets[action.Field] = struct{}{}
			}
		}
	}
	for _, field := range schema.Fields {
		candidateValues := make([]string, 0, len(field.StaticCandidates))
		for _, candidate := range field.StaticCandidates {
			candidateValues = append(candidateValues, candidate.Value)
		}
		_, shownConditionally := showTargets[field.Key]
		states[field.Key] = FieldState{
			Visible: !shownConditionally, Required: field.Required,
			CandidateValues: candidateValues,
		}
	}
	return states
}

func applyConfigurationRules(
	schema *capmodel.ConfigurationSchemaDeclaration,
	normalized map[string]any,
	states map[string]FieldState,
) error {
	for _, rule := range schema.Conditions {
		matched, evaluateErr := evaluateCondition(rule.When, normalized)
		if evaluateErr != nil {
			return evaluateErr
		}
		if !matched {
			continue
		}
		for _, action := range rule.Actions {
			state := states[action.Field]
			switch action.Action {
			case capmodel.ConfigurationActionShow:
				state.Visible = true
			case capmodel.ConfigurationActionHide:
				state.Visible = false
			case capmodel.ConfigurationActionRequire:
				state.Required = true
			case capmodel.ConfigurationActionDisable:
				state.Disabled = true
			case capmodel.ConfigurationActionFilterCandidates:
				state.candidateFilterActive = true
				state.CandidateValues = intersectValues(state.CandidateValues, action.Values)
			case capmodel.ConfigurationActionRequires:
				state.requiresActive = true
				state.requiredValues = appendUnique(state.requiredValues, action.Values...)
			case capmodel.ConfigurationActionExcludes:
				state.excludesActive = true
				state.excludedValues = appendUnique(state.excludedValues, action.Values...)
			}
			states[action.Field] = state
		}
	}
	return nil
}

func configurationStateIssues(
	schema *capmodel.ConfigurationSchemaDeclaration,
	normalized map[string]any,
	states map[string]FieldState,
) []Issue {
	issues := make([]Issue, 0)
	for _, field := range schema.Fields {
		issues = append(issues, configurationFieldStateIssues(field.Key, normalized, states[field.Key])...)
	}
	return issues
}

func configurationFieldStateIssues(fieldKey string, normalized map[string]any, state FieldState) []Issue {
	value, supplied := normalized[fieldKey]
	if supplied && (!state.Visible || state.Disabled) {
		return []Issue{{FieldKey: fieldKey, Code: IssueDisabledValue}}
	}
	if state.Visible && !state.Disabled && state.Required && (!supplied || emptyValue(value)) {
		return []Issue{{FieldKey: fieldKey, Code: IssueRequired}}
	}
	if !supplied {
		if state.requiresActive {
			return []Issue{{FieldKey: fieldKey, Code: IssueRequiredRelation}}
		}
		return nil
	}
	issues := make([]Issue, 0, 3)
	selected := selectedValues(value)
	if (state.candidateFilterActive || len(state.CandidateValues) > 0) && hasValueOutside(selected, state.CandidateValues) {
		issues = append(issues, Issue{FieldKey: fieldKey, Code: IssueFilteredOption})
	}
	if state.requiresActive && !relationSatisfied(selected, state.requiredValues, true) {
		issues = append(issues, Issue{FieldKey: fieldKey, Code: IssueRequiredRelation})
	}
	if state.excludesActive && !relationSatisfied(selected, state.excludedValues, false) {
		issues = append(issues, Issue{FieldKey: fieldKey, Code: IssueExcludedRelation})
	}
	return issues
}

func authoritativeConfigurationRows(normalizedRows []ConfigurationValue, states map[string]FieldState) []ConfigurationValue {
	authoritativeRows := normalizedRows[:0]
	for _, row := range normalizedRows {
		state := states[row.FieldKey]
		if state.Visible && !state.Disabled {
			authoritativeRows = append(authoritativeRows, row)
		}
	}
	return authoritativeRows
}

func evaluateCondition(condition *capmodel.ConfigurationConditionDeclaration, values map[string]any) (bool, error) {
	if condition.Operator == capmodel.ConfigurationConditionAnd {
		for _, child := range condition.Children {
			matched, err := evaluateCondition(child, values)
			if err != nil || !matched {
				return false, err
			}
		}
		return true, nil
	}
	if condition.Operator == capmodel.ConfigurationConditionOr {
		for _, child := range condition.Children {
			matched, err := evaluateCondition(child, values)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}
	value, exists := values[condition.Field]
	empty := !exists || emptyValue(value)
	switch condition.Operator {
	case capmodel.ConfigurationConditionEmpty:
		return empty, nil
	case capmodel.ConfigurationConditionNotEmpty:
		return !empty, nil
	}
	if empty {
		return false, nil
	}
	if numericCondition(condition.Operator) {
		left, ok := normalizeDecimal(fmt.Sprint(value))
		right, rightOK := normalizeDecimal(condition.Value)
		if !ok || !rightOK || left == "" || right == "" {
			return false, gerror.New("configuration numeric condition literal is invalid")
		}
		comparison := compareDecimals(left, right)
		switch condition.Operator {
		case capmodel.ConfigurationConditionGreaterThan:
			return comparison > 0, nil
		case capmodel.ConfigurationConditionGreaterOrEqual:
			return comparison >= 0, nil
		case capmodel.ConfigurationConditionLessThan:
			return comparison < 0, nil
		case capmodel.ConfigurationConditionLessOrEqual:
			return comparison <= 0, nil
		}
	}
	valuesToCompare := selectedValues(value)
	contains := containsValue(valuesToCompare, condition.Value)
	switch condition.Operator {
	case capmodel.ConfigurationConditionContains:
		if text, ok := value.(string); ok {
			return strings.Contains(text, condition.Value), nil
		}
		return contains, nil
	case capmodel.ConfigurationConditionEqual:
		return len(valuesToCompare) == 1 && valuesToCompare[0] == condition.Value, nil
	case capmodel.ConfigurationConditionNotEqual:
		return len(valuesToCompare) != 1 || valuesToCompare[0] != condition.Value, nil
	default:
		return false, gerror.New("configuration condition operator is invalid")
	}
}

func valueRow(field *capmodel.ConfigurationFieldDeclaration, value any) (ConfigurationValue, error) {
	row := ConfigurationValue{FieldKey: field.Key}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		typed, ok := value.(int64)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.IntegerValue = &typed
	case capmodel.ConfigurationFieldBoolean:
		typed, ok := value.(bool)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.BooleanValue = &typed
	case capmodel.ConfigurationFieldMultiSelect:
		typed, ok := value.([]string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.OptionTokens = append([]string(nil), typed...)
	case capmodel.ConfigurationFieldSingleSelect:
		typed, ok := value.(string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.OptionTokens = []string{typed}
	case capmodel.ConfigurationFieldTreeSelect:
		typed, ok := value.(string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.TreeOptionTokens = []string{typed}
	case capmodel.ConfigurationFieldAddress:
		typed, ok := value.([]string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.AddressOptionTokens = append([]string(nil), typed...)
	case capmodel.ConfigurationFieldFile:
		typed, ok := value.([]string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.FileTokens = append([]string(nil), typed...)
	case capmodel.ConfigurationFieldDecimal:
		typed, ok := value.(string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.DecimalValue = &typed
	case capmodel.ConfigurationFieldDate:
		typed, ok := value.(string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.DateValue = &typed
	case capmodel.ConfigurationFieldDateTime:
		typed, ok := value.(string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.DateTimeValue = &typed
	default:
		typed, ok := value.(string)
		if !ok {
			return ConfigurationValue{}, invalidNormalizedValueType(field.Key)
		}
		row.TextValue = &typed
	}
	return row, nil
}

func invalidNormalizedValueType(fieldKey string) error {
	return gerror.Newf("configuration field %s has an invalid normalized value type", fieldKey)
}

func selectedValues(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case string:
		return []string{typed}
	case int64:
		return []string{strconv.FormatInt(typed, 10)}
	case bool:
		return []string{strconv.FormatBool(typed)}
	default:
		return nil
	}
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	default:
		return false
	}
}

func intersectValues(current, allowed []string) []string {
	allow := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allow[value] = struct{}{}
	}
	result := make([]string, 0, len(current))
	for _, value := range current {
		if _, ok := allow[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func appendUnique(current []string, values ...string) []string {
	seen := make(map[string]struct{}, len(current)+len(values))
	for _, value := range current {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if _, exists := seen[value]; !exists {
			current = append(current, value)
			seen[value] = struct{}{}
		}
	}
	return current
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasValueOutside(values, allowed []string) bool {
	for _, value := range values {
		if !containsValue(allowed, value) {
			return true
		}
	}
	return false
}

func relationSatisfied(selected, configured []string, requires bool) bool {
	if len(configured) == 0 {
		if requires {
			return len(selected) > 0
		}
		return len(selected) == 0
	}
	if requires {
		// Multiple required values are the confirmed all-of contract.
		for _, value := range configured {
			if !containsValue(selected, value) {
				return false
			}
		}
		return true
	}
	for _, value := range configured {
		if containsValue(selected, value) {
			return false
		}
	}
	return true
}
