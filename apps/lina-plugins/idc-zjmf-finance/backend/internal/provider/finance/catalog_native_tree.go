// This file preserves Finance option_type=20 as an actual candidate hierarchy.
// A terminal token retains every source configuration/suboption pair so cart
// procurement restores the full branch, including each ancestor's charge.
package finance

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type nativeTreeNode struct {
	Value, Parent, Label string
	Choices              []nativeCatalogChoice
	Leaf                 bool
}
type nativeTree struct {
	Root  nativeCatalogOption
	Nodes []nativeTreeNode
}

func nativeTrees(definition nativeCatalogDefinition) ([]nativeTree, error) {
	options := map[string]nativeCatalogOption{}
	children := map[string][]nativeCatalogOption{}
	roots := []nativeCatalogOption{}
	seenChoices := map[string]bool{}
	for _, group := range definition.Groups {
		for _, option := range group.Options {
			if scalar(option.Type) != "20" {
				continue
			}
			id, parent := scalar(option.ID), scalar(option.Parent)
			if !positiveDecimal(id) || options[id].ID != nil {
				return nil, errInvalidResponse
			}
			if len(option.Choices) > 512 {
				return nil, errInvalidResponse
			}
			for _, choice := range option.Choices {
				sub := scalar(choice.ID)
				if !positiveDecimal(sub) || seenChoices[sub] {
					return nil, errInvalidResponse
				}
				seenChoices[sub] = true
			}
			option.Choices = append([]nativeCatalogChoice(nil), option.Choices...)
			for _, choice := range option.Choices {
				if _, err := nativeTreeOrder(choice.Order); err != nil {
					return nil, err
				}
			}
			if _, err := nativeTreeOrder(option.Order); err != nil {
				return nil, err
			}
			sort.SliceStable(option.Choices, func(i, j int) bool {
				left, _ := nativeTreeOrder(option.Choices[i].Order)
				right, _ := nativeTreeOrder(option.Choices[j].Order)
				if left != right {
					return left < right
				}
				a, _ := strconv.ParseInt(scalar(option.Choices[i].ID), 10, 64)
				b, _ := strconv.ParseInt(scalar(option.Choices[j].ID), 10, 64)
				return a < b
			})
			options[id] = option
			if parent == "" || parent == "0" {
				roots = append(roots, option)
			} else if !positiveDecimal(parent) {
				return nil, errInvalidResponse
			} else {
				children[parent] = append(children[parent], option)
			}
		}
	}
	sort.SliceStable(roots, func(i, j int) bool {
		left, _ := nativeTreeOrder(roots[i].Order)
		right, _ := nativeTreeOrder(roots[j].Order)
		if left != right {
			return left < right
		}
		a, _ := strconv.ParseInt(scalar(roots[i].ID), 10, 64)
		b, _ := strconv.ParseInt(scalar(roots[j].ID), 10, 64)
		return a < b
	})
	visited := map[string]bool{}
	result := []nativeTree{}
	for _, root := range roots {
		tree := nativeTree{Root: root, Nodes: []nativeTreeNode{}}
		active := map[string]bool{}
		if err := nativeTreeBranch(&tree, root, "", "", nil, children, active, visited, 0); err != nil {
			return nil, err
		}
		if len(tree.Nodes) == 0 || len(tree.Nodes) > 512 {
			return nil, errInvalidResponse
		}
		result = append(result, tree)
	}
	for id := range options {
		if !visited[id] {
			return nil, errInvalidResponse
		}
	}
	return result, nil
}
func nativeTreeBranch(tree *nativeTree, option nativeCatalogOption, parentValue, parentSub string, path []nativeCatalogChoice, children map[string][]nativeCatalogOption, active, visited map[string]bool, depth int) error {
	id := scalar(option.ID)
	if depth >= 8 || active[id] || len(option.Choices) == 0 {
		return errInvalidResponse
	}
	active[id] = true
	defer delete(active, id)
	visited[id] = true
	matched := 0
	for _, choice := range option.Choices {
		parent := scalar(choice.Parent)
		if parent == "0" {
			parent = ""
		}
		if parent != parentSub {
			continue
		}
		matched++
		sub := scalar(choice.ID)
		if !positiveDecimal(sub) {
			return errInvalidResponse
		}
		value := id + ":" + sub
		if parentValue != "" {
			value = parentValue + "/" + value
		}
		if len(value) > 256 {
			return errInvalidResponse
		}
		label, err := catalogPlainText(choice.Name, 256, false)
		if err != nil {
			return err
		}
		branch := append(append([]nativeCatalogChoice(nil), path...), choice)
		node := nativeTreeNode{Value: value, Parent: parentValue, Label: label, Choices: branch, Leaf: true}
		next := []nativeCatalogOption{}
		for _, candidate := range children[id] {
			for _, child := range candidate.Choices {
				if scalar(child.Parent) == sub {
					next = append(next, candidate)
					break
				}
			}
		}
		if len(next) > 1 {
			return errInvalidResponse
		}
		node.Leaf = len(next) == 0
		if len(tree.Nodes) >= 512 {
			return errInvalidResponse
		}
		tree.Nodes = append(tree.Nodes, node)
		if len(next) == 1 {
			if err = nativeTreeBranch(tree, next[0], value, sub, branch, children, active, visited, depth+1); err != nil {
				return err
			}
		}
	}
	if matched == 0 {
		return errInvalidResponse
	}
	return nil
}
func nativeTreeSchema(tree nativeTree) (*capmodel.ConfigurationFieldDeclaration, channelcommercecap.ImportFieldMapping, []*capmodel.ConfigurationRuleDeclaration, error) {
	id := scalar(tree.Root.ID)
	name, err := catalogPlainText(tree.Root.Name, 256, false)
	if err != nil {
		return nil, channelcommercecap.ImportFieldMapping{}, nil, err
	}
	field := &capmodel.ConfigurationFieldDeclaration{Key: "cfg_" + id, Type: capmodel.ConfigurationFieldTreeSelect, Label: nativeLabel(name), Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{}}
	mapping := channelcommercecap.ImportFieldMapping{FieldKey: field.Key, FieldToken: "config:" + id, Options: []channelcommercecap.ImportOptionMapping{}}
	conditions := []*capmodel.ConfigurationConditionDeclaration{}
	for _, node := range tree.Nodes {
		field.StaticCandidates = append(field.StaticCandidates, &capmodel.ConfigurationCandidate{Value: node.Value, ParentValue: node.Parent, Label: nativeLabel(node.Label)})
		mapping.Options = append(mapping.Options, channelcommercecap.ImportOptionMapping{Value: node.Value, OptionToken: node.Value})
		if node.Leaf {
			if len(mapping.DefaultValue) == 0 {
				raw, encodeErr := json.Marshal(node.Value)
				if encodeErr != nil {
					return nil, mapping, nil, encodeErr
				}
				mapping.DefaultValue = raw
			}
		} else {
			conditions = append(conditions, &capmodel.ConfigurationConditionDeclaration{Field: field.Key, Operator: capmodel.ConfigurationConditionEqual, Value: node.Value})
		}
	}
	if len(mapping.DefaultValue) == 0 || len(conditions) > 120 {
		return nil, mapping, nil, errInvalidResponse
	}
	rules := []*capmodel.ConfigurationRuleDeclaration{}
	if len(conditions) > 0 {
		rules = append(rules, &capmodel.ConfigurationRuleDeclaration{When: nativeConditionGroup(capmodel.ConfigurationConditionOr, conditions), Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionExcludes, Field: field.Key, Values: []string{}}}})
	}
	return field, mapping, rules, nil
}
func nativeTreePath(value, rootID string) (map[string]string, error) {
	path := strings.Split(value, "/")
	if len(path) == 0 || len(path) > 8 {
		return nil, errInvalidConfig
	}
	result := map[string]string{}
	for index, step := range path {
		id, sub, ok := strings.Cut(step, ":")
		if !ok || !positiveDecimal(id) || !positiveDecimal(sub) || index == 0 && id != rootID || result[id] != "" {
			return nil, errInvalidConfig
		}
		result[id] = sub
	}
	return result, nil
}

func nativeTreePrices(currency string, cycle catalogCycle, trees []nativeTree) ([]channelcommercecap.ImportOptionPrice, error) {
	result := []channelcommercecap.ImportOptionPrice{}
	for _, tree := range trees {
		for _, node := range tree.Nodes {
			total := channelcommercecap.ImportPriceBand{}
			for _, choice := range node.Choices {
				band, err := nativeChoiceBand(currency, cycle, choice, false)
				if err != nil || band == nil {
					return nil, errInvalidResponse
				}
				const maximum = int64(1<<63 - 1)
				if total.AmountMinor > maximum-band.AmountMinor || total.SetupAmountMinor > maximum-band.SetupAmountMinor {
					return nil, errInvalidResponse
				}
				total.AmountMinor += band.AmountMinor
				total.SetupAmountMinor += band.SetupAmountMinor
			}
			renewal := total.AmountMinor
			total.RenewalAmountMinor = &renewal
			result = append(result, channelcommercecap.ImportOptionPrice{FieldKey: "cfg_" + scalar(tree.Root.ID), Value: node.Value, PricingMode: channelcommercecap.ImportConfigurationPriceFixed, Bands: []channelcommercecap.ImportPriceBand{total}})
		}
	}
	return result, nil
}
func nativePublicOptions(field *capmodel.ConfigurationFieldDeclaration) []channelcommercecap.ConfigurationOption {
	children := map[string][]*capmodel.ConfigurationCandidate{}
	for _, candidate := range field.StaticCandidates {
		children[candidate.ParentValue] = append(children[candidate.ParentValue], candidate)
	}
	var branch func(string, int) []channelcommercecap.ConfigurationOption
	branch = func(parent string, depth int) []channelcommercecap.ConfigurationOption {
		result := []channelcommercecap.ConfigurationOption{}
		if depth > 8 {
			return result
		}
		for _, candidate := range children[parent] {
			result = append(result, channelcommercecap.ConfigurationOption{OptionToken: candidate.Value, Label: channelcommercecap.LocalizedText{ZhCN: candidate.Label.ZhCN, EnUS: candidate.Label.EnUS}, Children: branch(candidate.Value, depth+1)})
		}
		return result
	}
	return branch("", 0)
}

func nativeTreeOrder(raw json.RawMessage) (int64, error) {
	text := scalar(raw)
	if text == "" {
		return 0, nil
	}
	number, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, errInvalidResponse
	}
	return number, nil
}
