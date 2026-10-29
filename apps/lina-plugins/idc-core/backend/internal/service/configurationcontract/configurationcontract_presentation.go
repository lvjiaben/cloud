// This file validates merchant-owned purchase controls without changing provider
// field types. The same numeric constraints protect evaluation and order freeze.

package configurationcontract

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
)

// Local purchase control vocabulary is independent of provider declarations.
const (
	ControlAuto            = "auto"
	ControlInput           = "input"
	ControlSlider          = "slider"
	ControlSelect          = "select"
	ControlRadio           = "radio"
	NumericPricingQuantity = "quantity"
	NumericPricingChoice   = "per_choice"
)

// Presentation is a version-frozen local renderer and numeric selection policy.
// NumericChoices are ordinary integer values, never provider option identifiers.
type Presentation struct {
	CustomerVisible *bool           `json:"customerVisible,omitempty"`
	DefaultValue    json.RawMessage `json:"defaultValue,omitempty"`

	// UpgradeEnabled is nil for existing offers, retaining their previous upgrade behavior.
	UpgradeEnabled         *bool    `json:"upgradeEnabled,omitempty"`
	UpgradeMinimum         string   `json:"upgradeMinimum,omitempty"`
	UpgradeMaximum         string   `json:"upgradeMaximum,omitempty"`
	UpgradeCandidateValues []string `json:"upgradeCandidateValues,omitempty"`

	FixedPriceIncluded       bool                                     `json:"fixedPriceIncluded,omitempty"`
	BasePriceIncluded        bool                                     `json:"basePriceIncluded,omitempty"`
	AutomationMappingPending bool                                     `json:"automationMappingPending,omitempty"`
	ChoiceRules              []*capmodel.ConfigurationRuleDeclaration `json:"choiceRules,omitempty"`
	ProductFieldKey          string                                   `json:"productFieldKey,omitempty"`
	ProductChoiceBindings    map[string]string                        `json:"productChoiceBindings,omitempty"`
	SortOrder                int                                      `json:"sortOrder,omitempty"`
	NumericPricing           string                                   `json:"numericPricing,omitempty"`
	GroupZhCN                string                                   `json:"groupZhCN,omitempty"`
	GroupEnUS                string                                   `json:"groupEnUS,omitempty"`
	Unit                     string                                   `json:"unit,omitempty"`
	DescriptionZhCN          string                                   `json:"descriptionZhCN,omitempty"`
	DescriptionEnUS          string                                   `json:"descriptionEnUS,omitempty"`
	Advanced                 bool                                     `json:"advanced,omitempty"`
	NumericLabels            map[string]*capmodel.LocalizedText       `json:"numericLabels,omitempty"`
	Control                  string                                   `json:"displayControl,omitempty"`
	Step                     string                                   `json:"quantityStep,omitempty"`
	NumericChoices           []string                                 `json:"numericChoices,omitempty"`
}

// DecodePresentations reads bounded local metadata; missing legacy data is empty.
func DecodePresentations(raw string) (map[string]Presentation, error) {
	result := map[string]Presentation{}
	if raw == "" {
		return result, nil
	}
	if len(raw) > 131072 {
		return nil, gerror.New("purchase presentation exceeds its bound")
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil || result == nil || len(result) > 128 {
		return nil, gerror.New("purchase presentation is invalid")
	}
	return result, nil
}

// ValidatePresentations checks controls against the already-narrowed schema.
func ValidatePresentations(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]Presentation) error {
	if _, err := WithPresentationChoiceRules(schema, presentations); err != nil {
		return err
	}
	fields := map[string]*capmodel.ConfigurationFieldDeclaration{}
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				fields[field.Key] = field
			}
		}
	}
	if len(presentations) > 128 {
		return gerror.New("too many purchase controls")
	}
	for key, p := range presentations {
		field := fields[key]
		if err := validatePresentationMetadata(field, p); err != nil {
			return err
		}
		if field == nil {
			return gerror.New("purchase control has no enabled field")
		}
		switch p.Control {
		case "", ControlAuto, ControlInput, ControlSlider, ControlSelect, ControlRadio:
		default:
			return gerror.New("purchase control is invalid")
		}
		if field.Type != capmodel.ConfigurationFieldInteger {
			choice := field.Type == capmodel.ConfigurationFieldSingleSelect || field.Type == capmodel.ConfigurationFieldMultiSelect
			allowed := p.Control == ControlAuto || p.Control == "" || (choice && (p.Control == ControlSelect || (p.Control == ControlRadio && field.Type == capmodel.ConfigurationFieldSingleSelect)))
			if !allowed || p.Step != "" || len(p.NumericChoices) > 0 || len(p.NumericLabels) > 0 {
				return gerror.New("purchase control does not match its field")
			}
			continue
		}
		if err := validateNumericPresentation(field, p); err != nil {
			return err
		}
	}
	return nil
}

func validatePresentationMetadata(field *capmodel.ConfigurationFieldDeclaration, p Presentation) error {
	if _, err := hiddenDefault(field, p); err != nil {
		return err
	}
	if err := validateUpgradeMetadata(field, p); err != nil {
		return err
	}
	if err := validateProductChoiceBindings(field, p); err != nil {
		return err
	}
	if p.ProductFieldKey != "" && (len(p.ProductFieldKey) > 128 || field == nil || !strings.HasPrefix(field.Key, "local_") || !field.Required || (field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldSingleSelect && field.Type != capmodel.ConfigurationFieldBoolean && field.Type != capmodel.ConfigurationFieldSingleLine && field.Type != capmodel.ConfigurationFieldMultiLine)) {
		return gerror.New("purchase parameter presentation binding is invalid")
	}
	if p.NumericPricing != "" && p.NumericPricing != NumericPricingQuantity && p.NumericPricing != NumericPricingChoice {
		return gerror.New("invalid numeric pricing policy")
	}
	if p.NumericPricing != "" && (field == nil || field.Type != capmodel.ConfigurationFieldInteger) {
		return gerror.New("numeric pricing requires integer field")
	}
	if p.NumericPricing == NumericPricingChoice && (p.Control != ControlSelect && p.Control != ControlRadio || len(p.NumericChoices) == 0) {
		return gerror.New("per-choice prices require discrete numeric specifications")
	}
	if p.SortOrder < 0 || p.SortOrder > 1024 {
		return gerror.New("invalid purchase field sort order")
	}
	if utf8.RuneCountInString(p.GroupZhCN) > 128 || utf8.RuneCountInString(p.GroupEnUS) > 128 || utf8.RuneCountInString(p.Unit) > 32 || utf8.RuneCountInString(p.DescriptionZhCN) > 2048 || utf8.RuneCountInString(p.DescriptionEnUS) > 2048 {
		return gerror.New("purchase field metadata exceeds its bound")
	}
	return nil
}

// ProductChoiceValue preserves commercial identities everywhere except the
// explicitly bound provider input. Missing overrides retain legacy identity.
func ProductChoiceValue(bindings map[string]string, value string) string {
	if mapped, exists := bindings[value]; exists {
		return mapped
	}
	return value
}

func validateProductChoiceBindings(field *capmodel.ConfigurationFieldDeclaration, p Presentation) error {
	if len(p.ProductChoiceBindings) == 0 {
		return nil
	}
	if field == nil || field.Type != capmodel.ConfigurationFieldSingleSelect || !field.Required || !strings.HasPrefix(field.Key, "local_") || p.ProductFieldKey == "" || len(p.ProductChoiceBindings) > 512 {
		return gerror.New("purchase choice bindings require an explicit local select parameter")
	}
	choices := make(map[string]bool, len(field.StaticCandidates))
	for _, choice := range field.StaticCandidates {
		if choice != nil {
			choices[choice.Value] = true
		}
	}
	for choice, value := range p.ProductChoiceBindings {
		if !choices[choice] || value == "" || len(value) > 256 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
			return gerror.New("purchase choice binding is invalid")
		}
		for _, char := range value {
			if unicode.IsControl(char) {
				return gerror.New("purchase choice binding contains a control character")
			}
		}
	}
	return nil
}

// PresentationIssues applies the same explicit step/choice policy to values
// from any widget, preventing handcrafted quotes or orders from bypassing it.
func PresentationIssues(schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]Presentation, values []ConfigurationValue) []Issue {
	fields := map[string]*capmodel.ConfigurationFieldDeclaration{}
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				fields[field.Key] = field
			}
		}
	}
	issues := []Issue{}
	for _, value := range values {
		p, exists := presentations[value.FieldKey]
		field := fields[value.FieldKey]
		if !exists || field == nil || field.Type != capmodel.ConfigurationFieldInteger {
			continue
		}
		var number int64
		if value.IntegerTextValue != nil {
			n, err := strconv.ParseInt(*value.IntegerTextValue, 10, 64)
			if err != nil {
				issues = append(issues, Issue{FieldKey: value.FieldKey, Code: IssueInvalidValue})
				continue
			}
			number = n
		} else if value.IntegerValue != nil {
			number = *value.IntegerValue
		} else {
			continue
		}
		lower, lowerErr := strconv.ParseInt(field.Minimum, 10, 64)
		step := int64(1)
		var stepErr error
		if p.Step != "" {
			step, stepErr = strconv.ParseInt(p.Step, 10, 64)
		}
		if lowerErr != nil || stepErr != nil {
			issues = append(issues, Issue{FieldKey: value.FieldKey, Code: IssueInvalidValue})
			continue
		}
		valid := step > 0 && number >= lower && (number-lower)%step == 0
		if len(p.NumericChoices) > 0 {
			found := false
			for _, choice := range p.NumericChoices {
				if choice == strconv.FormatInt(number, 10) {
					found = true
					break
				}
			}
			valid = valid && found
		}
		if !valid {
			issues = append(issues, Issue{FieldKey: value.FieldKey, Code: IssueInvalidValue})
		}
	}
	return issues
}

func validateNumericPresentation(field *capmodel.ConfigurationFieldDeclaration, p Presentation) error {
	var (
		lower, e1 = strconv.ParseInt(field.Minimum, 10, 64)
		upper, e2 = strconv.ParseInt(field.Maximum, 10, 64)
		step      = int64(1)
	)
	if p.Step != "" {
		var err error
		step, err = strconv.ParseInt(p.Step, 10, 64)
		if err != nil || strconv.FormatInt(step, 10) != p.Step {
			return gerror.New("quantity step is invalid")
		}
	}
	if e1 != nil || e2 != nil || lower > upper || step < 1 || step > 9007199254740991 || lower < 0 || upper > 9007199254740991 {
		return gerror.New("purchase numeric limits are invalid")
	}
	if len(p.NumericChoices) > 512 {
		return gerror.New("too many numeric choices")
	}
	if p.Control == ControlSelect || p.Control == ControlRadio {
		if len(p.NumericChoices) == 0 {
			return gerror.New("numeric choice control needs choices")
		}
	}
	seen := map[string]bool{}
	for _, choice := range p.NumericChoices {
		n, err := strconv.ParseInt(choice, 10, 64)
		if err != nil || strconv.FormatInt(n, 10) != choice || n < lower || n > upper || (n-lower)%step != 0 || seen[choice] {
			return gerror.New("numeric choices do not match sale limits")
		}
		seen[choice] = true
	}
	for value, label := range p.NumericLabels {
		if !seen[value] || label == nil || label.ZhCN == "" || label.EnUS == "" || utf8.RuneCountInString(label.ZhCN) > 128 || utf8.RuneCountInString(label.EnUS) > 128 {
			return gerror.New("invalid numeric choice label")
		}
	}
	if p.Control != ControlSelect && p.Control != ControlRadio && len(p.NumericChoices) > 0 {
		return gerror.New("numeric choices require a choice control")
	}
	return nil
}
