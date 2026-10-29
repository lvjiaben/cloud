// This file translates advanced rules that address an individual level inside
// a Finance linkage tree. The public choice remains one complete terminal path;
// filtering a source child never discards or rewrites its ancestor identities.
package finance

import (
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func nativeTreeAliases(fields map[string]*capmodel.ConfigurationFieldDeclaration, field *capmodel.ConfigurationFieldDeclaration) {
	if field.Type != capmodel.ConfigurationFieldTreeSelect {
		return
	}
	for _, candidate := range field.StaticCandidates {
		for _, step := range strings.Split(candidate.Value, "/") {
			id, _, _ := strings.Cut(step, ":")
			fields["cfg_"+id] = field
		}
	}
}
func nativeTreePair(value, id string) (string, bool) {
	for _, step := range strings.Split(value, "/") {
		field, choice, ok := strings.Cut(step, ":")
		if ok && field == id {
			return choice, true
		}
	}
	return "", false
}
func nativeTreeParents(field *capmodel.ConfigurationFieldDeclaration) map[string]bool {
	parents := map[string]bool{}
	for _, candidate := range field.StaticCandidates {
		parents[candidate.ParentValue] = true
	}
	return parents
}
func nativeTreeSourceCondition(wire nativeRelationWire, field *capmodel.ConfigurationFieldDeclaration) (*capmodel.ConfigurationConditionDeclaration, error) {
	parents := nativeTreeParents(field)
	conditions := []*capmodel.ConfigurationConditionDeclaration{}
	found := map[string]bool{}
	present := []*capmodel.ConfigurationConditionDeclaration{}
	operator, group := capmodel.ConfigurationConditionEqual, capmodel.ConfigurationConditionOr
	if wire.Relation == nativeRelationNotEqual {
		operator, group = capmodel.ConfigurationConditionNotEqual, capmodel.ConfigurationConditionAnd
	} else if wire.Relation != nativeRelationEqual {
		return nil, errInvalidResponse
	}
	for _, candidate := range field.StaticCandidates {
		if parents[candidate.Value] {
			continue
		}
		sub, exists := nativeTreePair(candidate.Value, scalar(wire.FieldID))
		if !exists {
			continue
		}
		present = append(present, &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionEqual, Field: field.Key, Value: candidate.Value})
		if _, selected := wire.Values[sub]; selected {
			found[sub] = true
			conditions = append(conditions, &capmodel.ConfigurationConditionDeclaration{Operator: operator, Field: field.Key, Value: candidate.Value})
		}
	}
	for id := range wire.Values {
		if !found[id] {
			return nil, errInvalidResponse
		}
	}
	condition := nativeConditionGroup(group, conditions)
	if wire.Relation == nativeRelationNotEqual {
		condition = nativeConditionGroup(capmodel.ConfigurationConditionAnd, []*capmodel.ConfigurationConditionDeclaration{nativeConditionGroup(capmodel.ConfigurationConditionOr, present), condition})
	}
	return condition, nil
}
func nativeTreeTargetRules(field *capmodel.ConfigurationFieldDeclaration, rules []nativeRelationRule) ([]*capmodel.ConfigurationRuleDeclaration, bool, error) {
	relation := rules[0].result.Relation
	for _, rule := range rules {
		if rule.result.Relation != relation {
			return nil, false, nil
		}
		for id := range rule.result.Values {
			found := false
			for _, candidate := range field.StaticCandidates {
				sub, exists := nativeTreePair(candidate.Value, scalar(rule.result.FieldID))
				found = found || exists && sub == id
			}
			if !found {
				return nil, false, errInvalidResponse
			}
		}
	}
	parents := nativeTreeParents(field)
	result := []*capmodel.ConfigurationRuleDeclaration{}
	for _, candidate := range field.StaticCandidates {
		if parents[candidate.Value] {
			continue
		}
		active, accept, deny := []*capmodel.ConfigurationConditionDeclaration{}, []*capmodel.ConfigurationConditionDeclaration{}, []*capmodel.ConfigurationConditionDeclaration{}
		for _, rule := range rules {
			active = append(active, rule.when)
			sub, exists := nativeTreePair(candidate.Value, scalar(rule.result.FieldID))
			_, selected := rule.result.Values[sub]
			if !exists || selected {
				accept = append(accept, rule.when)
			}
			if exists && selected {
				deny = append(deny, rule.when)
			}
		}
		var invalid *capmodel.ConfigurationConditionDeclaration
		if relation == nativeRelationEqual {
			if len(accept) == len(active) {
				continue
			}
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
func nativeFilterTreeOptions(options []channelcommercecap.ConfigurationOption, allowed []string) []channelcommercecap.ConfigurationOption {
	set := map[string]bool{}
	for _, value := range allowed {
		set[value] = true
	}
	var filter func([]channelcommercecap.ConfigurationOption) []channelcommercecap.ConfigurationOption
	filter = func(values []channelcommercecap.ConfigurationOption) []channelcommercecap.ConfigurationOption {
		result := []channelcommercecap.ConfigurationOption{}
		for _, option := range values {
			if set[option.OptionToken] {
				option.Children = filter(option.Children)
				result = append(result, option)
			}
		}
		return result
	}
	return filter(options)
}
func nativeTreeOptionPresent(options []channelcommercecap.ConfigurationOption, token string) bool {
	for _, option := range options {
		if option.OptionToken == token || nativeTreeOptionPresent(option.Children, token) {
			return true
		}
	}
	return false
}
