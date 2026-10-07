// This file owns the bounded anonymous configuration graph and authoritative selection validation.

package catalog

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	maximumPublicConfigurationGroups     = 5000
	maximumPublicConfigurationOptions    = 5000
	maximumPublicCompatibilityRules      = 8000
	maximumPublicConfigurationViolations = 256
	maximumPublicConfigurationBytes      = 1024 * 1024
	publicConfigurationEnvelopeReserve   = 1024
)

const (
	PublicConfigurationViolationGroupSelection = "group_selection_out_of_range"
	PublicConfigurationViolationRequiresTarget = "requires_target_quantity"
	PublicConfigurationViolationExcludesTarget = "excludes_target"
	PublicConfigurationViolationAllowsTarget   = "allows_target_quantity"
)

// PublicConfigurationOption is one safe selectable specification.
type PublicConfigurationOption struct {
	OptionNo        string
	ComponentType   string
	NameZhCN        string
	NameEnUS        string
	MinimumQuantity int
	MaximumQuantity int
	DefaultQuantity int
}

// PublicConfigurationGroup is one bilingual selection boundary.
type PublicConfigurationGroup struct {
	GroupNo         string
	SelectionMode   SelectionMode
	NameZhCN        string
	NameEnUS        string
	MinimumSelected int
	MaximumSelected int
	Options         []PublicConfigurationOption
}

// PublicCompatibilityRule is one directional, version-local public constraint.
type PublicCompatibilityRule struct {
	RuleType              CompatibilityRuleType
	SourceOptionNo        string
	TargetOptionNo        string
	MinimumTargetQuantity int
	MaximumTargetQuantity *int
}

// PublicConfigurationGraph omits the internal product version and raw normalized values.
type PublicConfigurationGraph struct {
	Revision          string
	ConfigurationMode ConfigurationMode
	Groups            []PublicConfigurationGroup
	Rules             []PublicCompatibilityRule
}

// PublicConfigurationSelection is one explicitly selected option quantity.
type PublicConfigurationSelection struct {
	OptionNo string
	Quantity int
}

// PublicConfigurationViolation is a stable, localizable selection problem.
type PublicConfigurationViolation struct {
	Code            string
	GroupNo         string
	OptionNo        string
	RelatedOptionNo string
}

// PublicConfigurationValidation is the bounded result of authoritative graph evaluation.
type PublicConfigurationValidation struct {
	Valid               bool
	Violations          []PublicConfigurationViolation
	ViolationsTruncated bool
}

// PublicConfigurationValidationInput binds the complete selection to one graph revision.
type PublicConfigurationValidationInput struct {
	PublicSlug string
	Revision   string
	Selections []PublicConfigurationSelection
}

func (s *publicCatalogReaderImpl) GetConfiguration(
	ctx context.Context,
	publicSlug string,
) (*PublicConfigurationGraph, error) {
	slug := strings.ToLower(strings.TrimSpace(publicSlug))
	if !validPublicConfigurationSlug(slug) || s.revision == nil {
		return nil, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	snapshot, err := loadPublicConfigurationSnapshot(ctx, s.database, slug)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if snapshot == nil {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	}
	snapshot.Graph.Revision, err = s.revision.encode(slug, snapshot.VersionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if err = validatePublicConfigurationGraph(&snapshot.Graph); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	s.config.put(snapshot.Graph)
	return &snapshot.Graph, nil
}

func (s *publicCatalogReaderImpl) ValidateConfiguration(
	ctx context.Context,
	input PublicConfigurationValidationInput,
) (*PublicConfigurationValidation, error) {
	slug := strings.ToLower(strings.TrimSpace(input.PublicSlug))
	selections, err := normalizePublicConfigurationSelections(input.Selections)
	if err != nil || !validPublicConfigurationSlug(slug) || s.revision == nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	target, err := loadPublicConfigurationTarget(ctx, s.database, slug)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if target == nil {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	}
	revision := strings.TrimSpace(input.Revision)
	if !s.revision.verify(revision, slug, target.VersionNo) {
		return nil, bizerr.NewCode(CodePublicConfigurationStale)
	}
	graph, cached := s.config.get(revision)
	if !cached {
		snapshot, loadErr := loadPublicConfigurationSnapshotForTarget(ctx, s.database, slug, *target)
		if loadErr != nil {
			return nil, bizerr.WrapCode(loadErr, CodePublicCatalogUnavailable)
		}
		if snapshot == nil || !s.revision.verify(revision, slug, snapshot.VersionNo) {
			return nil, bizerr.NewCode(CodePublicConfigurationStale)
		}
		graph = snapshot.Graph
		graph.Revision = revision
		if err = validatePublicConfigurationGraph(&graph); err != nil {
			return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
		}
		s.config.put(graph)
	}
	result, err := evaluatePublicConfiguration(graph, selections)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func normalizePublicConfigurationSelections(
	input []PublicConfigurationSelection,
) (map[string]int, error) {
	if len(input) > maximumPublicConfigurationOptions {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	result := make(map[string]int, len(input))
	for _, item := range input {
		optionNo := strings.TrimSpace(item.OptionNo)
		if !catalogBusinessNumberPattern.MatchString(optionNo) || item.Quantity < 1 || item.Quantity > 100000 {
			return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
		}
		if _, duplicate := result[optionNo]; duplicate {
			return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
		}
		result[optionNo] = item.Quantity
	}
	return result, nil
}

func evaluatePublicConfiguration(
	graph PublicConfigurationGraph,
	selections map[string]int,
) (*PublicConfigurationValidation, error) {
	optionByNumber := make(map[string]PublicConfigurationOption, len(selections))
	groupByOption := make(map[string]string)
	selectedByGroup := make(map[string]int, len(graph.Groups))
	for _, group := range graph.Groups {
		for _, option := range group.Options {
			optionByNumber[option.OptionNo] = option
			groupByOption[option.OptionNo] = group.GroupNo
		}
	}
	for optionNo, quantity := range selections {
		option, exists := optionByNumber[optionNo]
		if !exists || quantity < option.MinimumQuantity || quantity > option.MaximumQuantity {
			return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
		}
		selectedByGroup[groupByOption[optionNo]]++
	}
	result := &PublicConfigurationValidation{Violations: make([]PublicConfigurationViolation, 0)}
	appendViolation := func(violation PublicConfigurationViolation) {
		if len(result.Violations) < maximumPublicConfigurationViolations {
			result.Violations = append(result.Violations, violation)
			return
		}
		result.ViolationsTruncated = true
	}
	for _, group := range graph.Groups {
		selected := selectedByGroup[group.GroupNo]
		if selected < group.MinimumSelected || selected > group.MaximumSelected {
			appendViolation(PublicConfigurationViolation{
				Code: PublicConfigurationViolationGroupSelection, GroupNo: group.GroupNo,
			})
		}
	}
	for _, rule := range graph.Rules {
		if selections[rule.SourceOptionNo] == 0 {
			continue
		}
		targetQuantity := selections[rule.TargetOptionNo]
		violation := PublicConfigurationViolation{
			GroupNo: groupByOption[rule.TargetOptionNo], OptionNo: rule.TargetOptionNo,
			RelatedOptionNo: rule.SourceOptionNo,
		}
		switch rule.RuleType {
		case CompatibilityRequires:
			if targetQuantity < rule.MinimumTargetQuantity ||
				(rule.MaximumTargetQuantity != nil && targetQuantity > *rule.MaximumTargetQuantity) {
				violation.Code = PublicConfigurationViolationRequiresTarget
				appendViolation(violation)
			}
		case CompatibilityExcludes:
			if targetQuantity > 0 {
				violation.Code = PublicConfigurationViolationExcludesTarget
				appendViolation(violation)
			}
		case CompatibilityAllows:
			if targetQuantity < rule.MinimumTargetQuantity ||
				rule.MaximumTargetQuantity == nil || targetQuantity > *rule.MaximumTargetQuantity {
				violation.Code = PublicConfigurationViolationAllowsTarget
				appendViolation(violation)
			}
		}
	}
	sort.Slice(result.Violations, func(i, j int) bool {
		left, right := result.Violations[i], result.Violations[j]
		if left.GroupNo != right.GroupNo {
			return left.GroupNo < right.GroupNo
		}
		if left.OptionNo != right.OptionNo {
			return left.OptionNo < right.OptionNo
		}
		if left.RelatedOptionNo != right.RelatedOptionNo {
			return left.RelatedOptionNo < right.RelatedOptionNo
		}
		return left.Code < right.Code
	})
	result.Valid = len(result.Violations) == 0 && !result.ViolationsTruncated
	return result, nil
}

func validatePublicConfigurationGraph(graph *PublicConfigurationGraph) error {
	if graph == nil || graph.Groups == nil || graph.Rules == nil ||
		len(graph.Revision) != publicConfigurationRevisionLength ||
		len(graph.Groups) > maximumPublicConfigurationGroups ||
		len(graph.Rules) > maximumPublicCompatibilityRules {
		return gerror.New("public configuration graph is invalid")
	}
	groups := make([]GroupDefinition, 0, len(graph.Groups))
	options := make([]OptionDefinition, 0)
	optionNumbers := make(map[string]struct{})
	for _, group := range graph.Groups {
		if !validPublicConfigurationLabel(group.NameZhCN) ||
			!validPublicConfigurationLabel(group.NameEnUS) {
			return gerror.New("public configuration group projection is invalid")
		}
		groups = append(groups, GroupDefinition{
			GroupNo: group.GroupNo, SelectionMode: group.SelectionMode,
			MinimumSelected: group.MinimumSelected, MaximumSelected: group.MaximumSelected,
		})
		if group.Options == nil || len(options)+len(group.Options) > maximumPublicConfigurationOptions {
			return gerror.New("public configuration option bound exceeded")
		}
		for _, option := range group.Options {
			if !validPublicComponentType(option.ComponentType) ||
				!validPublicConfigurationLabel(option.NameZhCN) ||
				!validPublicConfigurationLabel(option.NameEnUS) {
				return gerror.New("public configuration option projection is invalid")
			}
			options = append(options, OptionDefinition{
				OptionNo: option.OptionNo, GroupNo: group.GroupNo,
				MinimumQuantity: option.MinimumQuantity, MaximumQuantity: option.MaximumQuantity,
				DefaultQuantity: option.DefaultQuantity, Active: true,
			})
			optionNumbers[option.OptionNo] = struct{}{}
		}
	}
	if err := ValidateConfigurationGraph(graph.ConfigurationMode, groups, options); err != nil {
		return err
	}
	for _, rule := range graph.Rules {
		definition := CompatibilityRuleDefinition(rule)
		if err := ValidateCompatibilityRule(definition); err != nil {
			return err
		}
		if _, exists := optionNumbers[rule.SourceOptionNo]; !exists {
			return gerror.New("public compatibility source is unavailable")
		}
		if _, exists := optionNumbers[rule.TargetOptionNo]; !exists {
			return gerror.New("public compatibility target is unavailable")
		}
	}
	encoded, err := json.Marshal(graph)
	if err != nil || len(encoded) > maximumPublicConfigurationBytes-publicConfigurationEnvelopeReserve {
		return gerror.New("public configuration graph exceeds response bound")
	}
	return nil
}

func validPublicConfigurationSlug(value string) bool {
	return utf8.RuneCountInString(value) <= 128 && publicSlugPattern.MatchString(value)
}

func validPublicConfigurationLabel(value string) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) != "" &&
		utf8.RuneCountInString(value) <= 128 && !strings.ContainsRune(value, '\x00')
}

func validPublicComponentType(value string) bool {
	switch ComponentType(value) {
	case ComponentRegion, ComponentLine, ComponentDatacenter, ComponentCPU, ComponentMemory,
		ComponentSystemDisk, ComponentDataDisk, ComponentIP, ComponentBackup, ComponentImage,
		ComponentTraffic, ComponentBandwidth, ComponentOther:
		return true
	default:
		return false
	}
}
