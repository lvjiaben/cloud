// This file applies only the safe condition operations emitted by the native
// finance translator. It never evaluates provider scripts or expressions and
// invalid source relationships cannot reach an authoritative quote request.
package finance

import (
	"strconv"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func applyNativeSelectionRules(snapshot *channelcommercecap.ImportSnapshot, result *channelcommercecap.ConfigurationEvaluation) error {
	if len(snapshot.Schema.Conditions) == 0 {
		return nil
	}
	var (
		selected = map[string]string{}
		indices  = map[string]int{}
		values   = map[string]string{}
		invalid  = map[string]bool{}
	)
	for index, mapping := range snapshot.Mappings {
		indices[mapping.FieldKey] = index
		value, err := nativePurchaseValue(result.Values[index])
		if err == nil {
			selected[mapping.FieldKey] = value
			values[mapping.FieldToken] = value
		}
	}
	for _, rule := range snapshot.Schema.Conditions {
		matches, err := nativeConditionMatches(rule.When, selected)
		if err != nil {
			return err
		}
		if !matches {
			continue
		}
		for _, action := range rule.Actions {
			index, exists := indices[action.Field]
			if !exists {
				return errInvalidResponse
			}
			field := &result.Fields[index]
			switch action.Action {
			case capmodel.ConfigurationActionFilterCandidates:
				if field.Type == capmodel.ConfigurationFieldTreeSelect {
					field.Options = nativeFilterTreeOptions(field.Options, action.Values)
					continue
				}
				options := []channelcommercecap.ConfigurationOption{}
				for _, option := range field.Options {
					for _, allowed := range action.Values {
						if option.OptionToken == allowed {
							options = append(options, option)
							break
						}
					}
				}
				field.Options = options
			case capmodel.ConfigurationActionExcludes:
				if len(action.Values) == 0 {
					invalid[field.FieldToken] = true
				} else {
					for _, excluded := range action.Values {
						if values[field.FieldToken] == excluded {
							invalid[field.FieldToken] = true
						}
					}
				}
			default:
				return errUnsupported
			}
		}
	}
	for _, field := range result.Fields {
		if field.Type == capmodel.ConfigurationFieldSingleSelect {
			found := false
			for _, option := range field.Options {
				if option.OptionToken == values[field.FieldToken] {
					found = true
				}
			}
			if !found {
				invalid[field.FieldToken] = true
			}
		}
		if field.Type == capmodel.ConfigurationFieldTreeSelect && !nativeTreeOptionPresent(field.Options, values[field.FieldToken]) {
			invalid[field.FieldToken] = true
		}
		if invalid[field.FieldToken] {
			result.Valid = false
			result.Issues = append(result.Issues, channelcommercecap.ConfigurationIssue{FieldToken: field.FieldToken, IssueCode: "invalid_selection"})
		}
	}
	return nil
}

func nativeConditionMatches(condition *capmodel.ConfigurationConditionDeclaration, values map[string]string) (bool, error) {
	if condition == nil {
		return false, errInvalidResponse
	}
	switch condition.Operator {
	case capmodel.ConfigurationConditionAnd:
		for _, child := range condition.Children {
			matched, err := nativeConditionMatches(child, values)
			if err != nil || !matched {
				return false, err
			}
		}
		return true, nil
	case capmodel.ConfigurationConditionOr:
		for _, child := range condition.Children {
			matched, err := nativeConditionMatches(child, values)
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
	if !exists {
		return false, nil
	}
	switch condition.Operator {
	case capmodel.ConfigurationConditionEqual:
		return value == condition.Value, nil
	case capmodel.ConfigurationConditionNotEqual:
		return value != condition.Value, nil
	}
	actual, actualErr := strconv.ParseInt(value, 10, 64)
	expected, expectedErr := strconv.ParseInt(condition.Value, 10, 64)
	if actualErr != nil || expectedErr != nil {
		return false, errInvalidResponse
	}
	switch condition.Operator {
	case capmodel.ConfigurationConditionGreaterOrEqual:
		return actual >= expected, nil
	case capmodel.ConfigurationConditionGreaterThan:
		return actual > expected, nil
	case capmodel.ConfigurationConditionLessOrEqual:
		return actual <= expected, nil
	case capmodel.ConfigurationConditionLessThan:
		return actual < expected, nil
	default:
		return false, errUnsupported
	}
}
