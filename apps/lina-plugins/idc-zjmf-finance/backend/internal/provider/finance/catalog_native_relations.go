// This file translates source-proven finance advanced relations into safe host
// conditions. Multiple active inclusion sets retain union semantics; unsupported
// mixed actions are rejected rather than silently widening the source choices.
package finance

import (
	"encoding/json"
	"sort"
	"strconv"

	"lina-core/pkg/plugin/capability/capmodel"
)

type nativeRelationKind string
type nativeRelationAction string

const (
	nativeRelationEqual     nativeRelationKind   = "seq"
	nativeRelationNotEqual  nativeRelationKind   = "sneq"
	nativeRelationCondition nativeRelationAction = "condition"
	nativeRelationResult    nativeRelationAction = "result"
)

type nativeRelationWire struct {
	ID       json.RawMessage                `json:"id"`
	FieldID  json.RawMessage                `json:"config_id"`
	Relation nativeRelationKind             `json:"relation"`
	Type     nativeRelationAction           `json:"type"`
	ParentID json.RawMessage                `json:"relation_id"`
	Values   map[string]nativeRelationRange `json:"sub_id"`
}
type nativeRelationRange struct {
	Minimum json.RawMessage `json:"qty_minimum"`
	Maximum json.RawMessage `json:"qty_maximum"`
}
type nativeRelationRule struct {
	when   *capmodel.ConfigurationConditionDeclaration
	result nativeRelationWire
}

func nativeAdvancedRules(source []nativeRelationWire, schema *capmodel.ConfigurationSchemaDeclaration) ([]*capmodel.ConfigurationRuleDeclaration, bool, error) {
	if len(source) > 128 {
		return nil, false, errInvalidResponse
	}
	var (
		fields  = map[string]*capmodel.ConfigurationFieldDeclaration{}
		roots   = map[string]*capmodel.ConfigurationConditionDeclaration{}
		grouped = map[string][]nativeRelationRule{}
		ids     = map[string]bool{}
	)
	for _, field := range schema.Fields {
		fields[field.Key] = field
		nativeTreeAliases(fields, field)
	}
	for _, wire := range source {
		id := scalar(wire.ID)
		if !positiveDecimal(id) || ids[id] || len(wire.Values) == 0 || len(wire.Values) > 512 {
			return nil, false, errInvalidResponse
		}
		ids[id] = true
		if wire.Type != nativeRelationCondition {
			continue
		}
		if scalar(wire.ParentID) != "0" {
			return nil, false, nil
		}
		condition, err := nativeSourceCondition(wire, fields["cfg_"+scalar(wire.FieldID)])
		if err != nil {
			return nil, false, err
		}
		if condition == nil {
			return nil, false, nil
		}
		roots[id] = condition
	}
	for _, wire := range source {
		if wire.Type == nativeRelationCondition {
			continue
		}
		if wire.Type != nativeRelationResult || roots[scalar(wire.ParentID)] == nil {
			return nil, false, nil
		}
		key := "cfg_" + scalar(wire.FieldID)
		if fields[key] == nil {
			return nil, false, errInvalidResponse
		}
		if wire.Relation != nativeRelationEqual && wire.Relation != nativeRelationNotEqual {
			return nil, false, nil
		}
		grouped[key] = append(grouped[key], nativeRelationRule{when: roots[scalar(wire.ParentID)], result: wire})
	}
	result := []*capmodel.ConfigurationRuleDeclaration{}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rules, supported, err := nativeTargetRules(fields[key], grouped[key])
		if err != nil || !supported {
			return nil, false, err
		}
		result = append(result, rules...)
	}
	if len(result) > 128 {
		return nil, false, nil
	}
	for _, rule := range result {
		nodes := 0
		if !nativeConditionBound(rule.When, 1, &nodes) {
			return nil, false, nil
		}
	}
	return result, true, nil
}

func nativeSourceCondition(wire nativeRelationWire, field *capmodel.ConfigurationFieldDeclaration) (*capmodel.ConfigurationConditionDeclaration, error) {
	if field == nil {
		return nil, errInvalidResponse
	}
	if field.Type == capmodel.ConfigurationFieldTreeSelect {
		return nativeTreeSourceCondition(wire, field)
	}
	conditions := []*capmodel.ConfigurationConditionDeclaration{}
	keys := make([]string, 0, len(wire.Values))
	for key := range wire.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		if !positiveDecimal(id) {
			return nil, errInvalidResponse
		}
		if field.Type == capmodel.ConfigurationFieldInteger {
			bounds, err := nativeRangeCondition(field.Key, wire.Values[id], wire.Relation == nativeRelationNotEqual)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, bounds)
		} else if field.Type == capmodel.ConfigurationFieldSingleSelect {
			found := false
			for _, candidate := range field.StaticCandidates {
				if candidate.Value == id {
					found = true
				}
			}
			if !found {
				return nil, errInvalidResponse
			}
			operator := capmodel.ConfigurationConditionEqual
			if wire.Relation == nativeRelationNotEqual {
				operator = capmodel.ConfigurationConditionNotEqual
			}
			conditions = append(conditions, &capmodel.ConfigurationConditionDeclaration{Field: field.Key, Operator: operator, Value: id})
		} else {
			return nil, nil
		}
	}
	switch wire.Relation {
	case nativeRelationEqual:
		return nativeConditionGroup(capmodel.ConfigurationConditionOr, conditions), nil
	case nativeRelationNotEqual:
		return nativeConditionGroup(capmodel.ConfigurationConditionAnd, conditions), nil
	default:
		return nil, nil
	}
}

func nativeRangeCondition(key string, bounds nativeRelationRange, outside bool) (*capmodel.ConfigurationConditionDeclaration, error) {
	minimum, minErr := nativeNonnegative(bounds.Minimum)
	maximum, maxErr := nativeNonnegative(bounds.Maximum)
	if minErr != nil || maxErr != nil || maximum < minimum {
		return nil, errInvalidResponse
	}
	low, high, group := capmodel.ConfigurationConditionGreaterOrEqual, capmodel.ConfigurationConditionLessOrEqual, capmodel.ConfigurationConditionAnd
	// Finance's sneq range test includes both boundaries. Preserve that source
	// behavior rather than replacing it with the inverse of inclusive equality.
	if outside {
		low, high, group = capmodel.ConfigurationConditionLessOrEqual, capmodel.ConfigurationConditionGreaterOrEqual, capmodel.ConfigurationConditionOr
	}
	return nativeConditionGroup(group, []*capmodel.ConfigurationConditionDeclaration{{Field: key, Operator: low, Value: strconv.FormatInt(minimum, 10)}, {Field: key, Operator: high, Value: strconv.FormatInt(maximum, 10)}}), nil
}

func nativeTargetRules(field *capmodel.ConfigurationFieldDeclaration, rules []nativeRelationRule) ([]*capmodel.ConfigurationRuleDeclaration, bool, error) {
	if field.Type == capmodel.ConfigurationFieldInteger {
		return nativeNumericTargetRules(field, rules)
	}
	if field.Type == capmodel.ConfigurationFieldTreeSelect {
		return nativeTreeTargetRules(field, rules)
	}
	if field.Type != capmodel.ConfigurationFieldSingleSelect {
		return nil, false, nil
	}
	relation := rules[0].result.Relation
	for _, rule := range rules {
		if rule.result.Relation != relation {
			return nil, false, nil
		}
		for id := range rule.result.Values {
			found := false
			for _, candidate := range field.StaticCandidates {
				if candidate.Value == id {
					found = true
				}
			}
			if !found {
				return nil, false, errInvalidResponse
			}
		}
	}
	result := []*capmodel.ConfigurationRuleDeclaration{}
	for _, candidate := range field.StaticCandidates {
		active, accept, deny := []*capmodel.ConfigurationConditionDeclaration{}, []*capmodel.ConfigurationConditionDeclaration{}, []*capmodel.ConfigurationConditionDeclaration{}
		for _, rule := range rules {
			active = append(active, rule.when)
			if _, exists := rule.result.Values[candidate.Value]; exists {
				accept = append(accept, rule.when)
				deny = append(deny, rule.when)
			}
		}
		var invalid *capmodel.ConfigurationConditionDeclaration
		if relation == nativeRelationEqual {
			invalid = nativeDisallowedCondition(active, accept)
		} else {
			invalid = nativeConditionGroup(capmodel.ConfigurationConditionOr, deny)
		}
		if invalid == nil {
			continue
		}
		allowed := []string{}
		for _, other := range field.StaticCandidates {
			if other.Value != candidate.Value {
				allowed = append(allowed, other.Value)
			}
		}
		result = append(result, &capmodel.ConfigurationRuleDeclaration{When: invalid, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionFilterCandidates, Field: field.Key, Values: allowed}}})
	}
	return result, true, nil
}

func nativeNumericTargetRules(field *capmodel.ConfigurationFieldDeclaration, rules []nativeRelationRule) ([]*capmodel.ConfigurationRuleDeclaration, bool, error) {
	active, accept := []*capmodel.ConfigurationConditionDeclaration{}, []*capmodel.ConfigurationConditionDeclaration{}
	for _, rule := range rules {
		if rule.result.Relation != nativeRelationEqual {
			return nil, false, nil
		}
		active = append(active, rule.when)
		ranges := []*capmodel.ConfigurationConditionDeclaration{}
		keys := make([]string, 0, len(rule.result.Values))
		for id := range rule.result.Values {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			condition, err := nativeRangeCondition(field.Key, rule.result.Values[id], false)
			if err != nil {
				return nil, false, err
			}
			ranges = append(ranges, condition)
		}
		accept = append(accept, nativeConditionGroup(capmodel.ConfigurationConditionAnd, []*capmodel.ConfigurationConditionDeclaration{rule.when, nativeConditionGroup(capmodel.ConfigurationConditionOr, ranges)}))
	}
	invalid := nativeDisallowedCondition(active, accept)
	if invalid == nil {
		return []*capmodel.ConfigurationRuleDeclaration{}, true, nil
	}
	// A numeric result outside every active source interval is a real validation
	// error. Empty excludes rejects that selected value without clamping hardware.
	return []*capmodel.ConfigurationRuleDeclaration{{When: invalid, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionExcludes, Field: field.Key}}}}, true, nil
}

func nativeDisallowedCondition(active, accept []*capmodel.ConfigurationConditionDeclaration) *capmodel.ConfigurationConditionDeclaration {
	if len(active) == 0 {
		return nil
	}
	if len(accept) == 0 {
		return nativeConditionGroup(capmodel.ConfigurationConditionOr, active)
	}
	return nativeConditionGroup(capmodel.ConfigurationConditionAnd, []*capmodel.ConfigurationConditionDeclaration{nativeConditionGroup(capmodel.ConfigurationConditionOr, active), nativeNegateCondition(nativeConditionGroup(capmodel.ConfigurationConditionOr, accept))})
}
func nativeConditionGroup(operator string, children []*capmodel.ConfigurationConditionDeclaration) *capmodel.ConfigurationConditionDeclaration {
	if len(children) == 0 {
		return nil
	}
	if len(children) == 1 {
		return children[0]
	}
	return &capmodel.ConfigurationConditionDeclaration{Operator: operator, Children: children}
}
func nativeNegateCondition(condition *capmodel.ConfigurationConditionDeclaration) *capmodel.ConfigurationConditionDeclaration {
	inverse := map[string]string{capmodel.ConfigurationConditionAnd: capmodel.ConfigurationConditionOr, capmodel.ConfigurationConditionOr: capmodel.ConfigurationConditionAnd, capmodel.ConfigurationConditionEqual: capmodel.ConfigurationConditionNotEqual, capmodel.ConfigurationConditionNotEqual: capmodel.ConfigurationConditionEqual, capmodel.ConfigurationConditionGreaterOrEqual: capmodel.ConfigurationConditionLessThan, capmodel.ConfigurationConditionLessOrEqual: capmodel.ConfigurationConditionGreaterThan, capmodel.ConfigurationConditionLessThan: capmodel.ConfigurationConditionGreaterOrEqual, capmodel.ConfigurationConditionGreaterThan: capmodel.ConfigurationConditionLessOrEqual}
	result := &capmodel.ConfigurationConditionDeclaration{Operator: inverse[condition.Operator], Field: condition.Field, Value: condition.Value}
	for _, child := range condition.Children {
		result.Children = append(result.Children, nativeNegateCondition(child))
	}
	return result
}
func nativeConditionBound(condition *capmodel.ConfigurationConditionDeclaration, depth int, nodes *int) bool {
	*nodes++
	if condition == nil || depth > 8 || *nodes > 128 {
		return false
	}
	for _, child := range condition.Children {
		if !nativeConditionBound(child, depth+1, nodes) {
			return false
		}
	}
	return true
}
