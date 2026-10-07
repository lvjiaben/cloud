// This file validates the bounded declarative configuration protocol carried
// by business-extension manifests. It rejects executable or open-ended input.

package catalog

import (
	"bytes"
	"encoding/json"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"gopkg.in/yaml.v3"

	"lina-core/pkg/plugin/capability/capmodel"
)

const (
	maxConfigurationFields                 = 128
	maxConfigurationCandidates             = 256
	maxConfigurationRules                  = 64
	maxConfigurationActionsPerRule         = 8
	maxConfigurationConditionDepth         = 4
	maxConfigurationConditionNodes         = 64
	maxConfigurationCandidateDepth         = 16
	maxConfigurationLiteralBytes           = 512
	maxConfigurationLabelRunes             = 256
	maxConfigurationTextLength             = 32768
	maxConfigurationUploadPolicies         = 32
	maxConfigurationUploadFiles            = 100
	maxConfigurationUploadFileBytes  int64 = 5 * 1024 * 1024 * 1024
	maxConfigurationUploadTotalBytes int64 = 10 * 1024 * 1024 * 1024
	maxConfigurationUploadTypes            = 32
)

var (
	configurationFieldKeyPattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
	configurationValuePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	configurationPolicyPattern    = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	configurationIntegerPattern   = regexp.MustCompile(`^-?(?:0|[1-9][0-9]{0,17})$`)
	configurationDecimalPattern   = regexp.MustCompile(`^-?(?:0|[1-9][0-9]{0,17})(?:\.[0-9]{1,8})?$`)
	configurationMIMEPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]{0,63}/[a-z0-9][a-z0-9!#$&^_.+-]{0,63}$`)
	configurationExtensionPattern = regexp.MustCompile(`^\.[a-z0-9][a-z0-9._+-]{0,15}$`)
)

type configurationSurface string

const (
	configurationSurfaceInterface configurationSurface = "interface"
	configurationSurfaceProduct   configurationSurface = "product"
	configurationSurfaceCustomer  configurationSurface = "customer"
)

// ValidateConfigurationDeclarations normalizes and validates all configuration
// surfaces attached to one business-extension declaration.
func ValidateConfigurationDeclarations(declaration *capmodel.BusinessExtensionDeclaration) error {
	if declaration == nil {
		return nil
	}
	declaration.InterfaceConfigurationCardinality = strings.ToLower(strings.TrimSpace(declaration.InterfaceConfigurationCardinality))
	if declaration.InterfaceConfiguration == nil {
		if declaration.InterfaceConfigurationCardinality != "" {
			return gerror.New("interface configuration cardinality requires an interface configuration declaration")
		}
	} else if declaration.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalityMultiple &&
		declaration.InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalitySingleton {
		return gerror.New("interface configuration cardinality must be multiple or singleton")
	}
	if declaration.Role == capmodel.BusinessExtensionRoleOwner &&
		(declaration.InterfaceConfiguration != nil || declaration.ProductConfiguration != nil || declaration.CustomerConfiguration != nil) {
		return gerror.New("business extension owners cannot declare provider configuration surfaces")
	}
	if declaration.Point != capmodel.BusinessExtensionProductFulfillment &&
		(declaration.ProductConfiguration != nil || declaration.CustomerConfiguration != nil) {
		return gerror.New("product and customer configuration are only supported by product-fulfillment providers")
	}
	methods := make(map[string]struct{}, len(declaration.Methods))
	for _, method := range declaration.Methods {
		methods[method] = struct{}{}
	}
	for _, item := range []struct {
		surface configurationSurface
		schema  *capmodel.ConfigurationSchemaDeclaration
	}{
		{configurationSurfaceInterface, declaration.InterfaceConfiguration},
		{configurationSurfaceProduct, declaration.ProductConfiguration},
		{configurationSurfaceCustomer, declaration.CustomerConfiguration},
	} {
		if err := validateConfigurationSchema(item.surface, item.schema, methods); err != nil {
			return gerror.Wrapf(err, "%s configuration declaration is invalid", item.surface)
		}
	}
	return nil
}

func validateConfigurationSchema(surface configurationSurface, schema *capmodel.ConfigurationSchemaDeclaration, methods map[string]struct{}) error {
	if schema == nil {
		return nil
	}
	schema.Version = strings.TrimSpace(schema.Version)
	if !contractVersionPattern.MatchString(schema.Version) {
		return gerror.New("schema version must use v1-style format")
	}
	if len(schema.Fields) == 0 || len(schema.Fields) > maxConfigurationFields {
		return gerror.Newf("schema fields must contain between 1 and %d entries", maxConfigurationFields)
	}
	if len(schema.Conditions) > maxConfigurationRules {
		return gerror.Newf("schema conditions exceed limit %d", maxConfigurationRules)
	}
	policies, err := validateConfigurationUploadPolicies(schema.UploadPolicies)
	if err != nil {
		return err
	}
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration, len(schema.Fields))
	for index, field := range schema.Fields {
		if err := validateConfigurationField(surface, field, methods); err != nil {
			return gerror.Wrapf(err, "field %d is invalid", index+1)
		}
		if _, exists := fields[field.Key]; exists {
			return gerror.Newf("field key is duplicated: %s", field.Key)
		}
		fields[field.Key] = field
		if field.Type == capmodel.ConfigurationFieldFile {
			if _, exists := policies[field.UploadPolicy]; !exists {
				return gerror.Newf("file field references an unknown upload policy: %s", field.UploadPolicy)
			}
		}
	}
	for index, rule := range schema.Conditions {
		if err := validateConfigurationRule(rule, fields); err != nil {
			return gerror.Wrapf(err, "condition rule %d is invalid", index+1)
		}
	}
	return nil
}

func validateConfigurationUploadPolicies(input []*capmodel.ConfigurationUploadPolicyDeclaration) (map[string]*capmodel.ConfigurationUploadPolicyDeclaration, error) {
	if len(input) > maxConfigurationUploadPolicies {
		return nil, gerror.Newf("configuration upload policies exceed limit %d", maxConfigurationUploadPolicies)
	}
	policies := make(map[string]*capmodel.ConfigurationUploadPolicyDeclaration, len(input))
	for index, policy := range input {
		if policy == nil {
			return nil, gerror.Newf("upload policy %d cannot be nil", index+1)
		}
		policy.ID = strings.ToLower(strings.TrimSpace(policy.ID))
		if !configurationPolicyPattern.MatchString(policy.ID) || policy.MaximumFiles < 1 || policy.MaximumFiles > maxConfigurationUploadFiles ||
			policy.MaximumFileBytes < 1 || policy.MaximumFileBytes > maxConfigurationUploadFileBytes ||
			policy.MaximumTotalBytes < policy.MaximumFileBytes || policy.MaximumTotalBytes > maxConfigurationUploadTotalBytes ||
			len(policy.AllowedMIMETypes) < 1 || len(policy.AllowedMIMETypes) > maxConfigurationUploadTypes ||
			len(policy.AllowedExtensions) < 1 || len(policy.AllowedExtensions) > maxConfigurationUploadTypes {
			return nil, gerror.Newf("upload policy %d has invalid bounds", index+1)
		}
		if _, exists := policies[policy.ID]; exists {
			return nil, gerror.Newf("upload policy id is duplicated: %s", policy.ID)
		}
		seenMIME := make(map[string]struct{}, len(policy.AllowedMIMETypes))
		for mimeIndex, value := range policy.AllowedMIMETypes {
			value = strings.ToLower(strings.TrimSpace(value))
			if !configurationMIMEPattern.MatchString(value) {
				return nil, gerror.Newf("upload policy %d MIME type %d is invalid", index+1, mimeIndex+1)
			}
			if _, exists := seenMIME[value]; exists {
				return nil, gerror.Newf("upload policy %d contains duplicate MIME types", index+1)
			}
			seenMIME[value] = struct{}{}
			policy.AllowedMIMETypes[mimeIndex] = value
		}
		seenExtension := make(map[string]struct{}, len(policy.AllowedExtensions))
		for extensionIndex, value := range policy.AllowedExtensions {
			value = strings.ToLower(strings.TrimSpace(value))
			if !configurationExtensionPattern.MatchString(value) {
				return nil, gerror.Newf("upload policy %d extension %d is invalid", index+1, extensionIndex+1)
			}
			if _, exists := seenExtension[value]; exists {
				return nil, gerror.Newf("upload policy %d contains duplicate extensions", index+1)
			}
			seenExtension[value] = struct{}{}
			policy.AllowedExtensions[extensionIndex] = value
		}
		policies[policy.ID] = policy
	}
	return policies, nil
}

func validateConfigurationField(surface configurationSurface, field *capmodel.ConfigurationFieldDeclaration, methods map[string]struct{}) error {
	if field == nil {
		return gerror.New("field cannot be nil")
	}
	field.Key = strings.ToLower(strings.TrimSpace(field.Key))
	field.Type = strings.ToLower(strings.TrimSpace(field.Type))
	field.TextFormat = strings.ToLower(strings.TrimSpace(field.TextFormat))
	field.Minimum = strings.TrimSpace(field.Minimum)
	field.Maximum = strings.TrimSpace(field.Maximum)
	field.UploadPolicy = strings.TrimSpace(field.UploadPolicy)
	if !configurationFieldKeyPattern.MatchString(field.Key) {
		return gerror.Newf("field key must use safe snake_case: %s", field.Key)
	}
	if !supportedConfigurationFieldType(field.Type) {
		return gerror.Newf("field type is not supported: %s", field.Type)
	}
	if field.Type == capmodel.ConfigurationFieldSecret && surface != configurationSurfaceInterface {
		return gerror.New("secret fields are allowed only in interface configuration")
	}
	if err := validateLocalizedText(field.Label, true); err != nil {
		return gerror.Wrap(err, "field label is invalid")
	}
	if err := validateLocalizedText(field.Description, false); err != nil {
		return gerror.Wrap(err, "field description is invalid")
	}
	if err := validateConfigurationTextConstraints(field); err != nil {
		return err
	}
	if err := validateConfigurationNumberConstraints(field); err != nil {
		return err
	}
	if err := validateConfigurationCandidates(field, methods); err != nil {
		return err
	}
	if field.Type == capmodel.ConfigurationFieldFile {
		if !configurationPolicyPattern.MatchString(field.UploadPolicy) || strings.Contains(field.UploadPolicy, "://") {
			return gerror.New("file fields require a safe host-governed upload_policy identifier")
		}
	} else if field.UploadPolicy != "" {
		return gerror.New("upload_policy is allowed only for file fields")
	}
	return nil
}

func validateConfigurationTextConstraints(field *capmodel.ConfigurationFieldDeclaration) error {
	isText := field.Type == capmodel.ConfigurationFieldSingleLine || field.Type == capmodel.ConfigurationFieldMultiLine || field.Type == capmodel.ConfigurationFieldSecret
	if field.TextFormat != "" {
		if field.Type != capmodel.ConfigurationFieldSingleLine || !supportedConfigurationTextFormat(field.TextFormat) {
			return gerror.New("text_format only supports the host phone/email/domain/url whitelist on single_line fields")
		}
	}
	if !isText && (field.MinimumLength != 0 || field.MaximumLength != 0) {
		return gerror.New("length limits are allowed only for text and secret fields")
	}
	if field.MinimumLength < 0 || field.MaximumLength < 0 || field.MinimumLength > maxConfigurationTextLength || field.MaximumLength > maxConfigurationTextLength {
		return gerror.Newf("text length limits must be between 0 and %d", maxConfigurationTextLength)
	}
	if field.MaximumLength > 0 && field.MinimumLength > field.MaximumLength {
		return gerror.New("minimum_length cannot exceed maximum_length")
	}
	return nil
}

func validateConfigurationNumberConstraints(field *capmodel.ConfigurationFieldDeclaration) error {
	isInteger := field.Type == capmodel.ConfigurationFieldInteger
	isDecimal := field.Type == capmodel.ConfigurationFieldDecimal
	if !isInteger && !isDecimal && (field.Minimum != "" || field.Maximum != "") {
		return gerror.New("numeric bounds are allowed only for integer and decimal fields")
	}
	pattern := configurationDecimalPattern
	if isInteger {
		pattern = configurationIntegerPattern
	}
	if (isInteger || isDecimal) && field.Minimum != "" && !pattern.MatchString(field.Minimum) {
		return gerror.New("minimum is not a bounded canonical number")
	}
	if (isInteger || isDecimal) && field.Maximum != "" && !pattern.MatchString(field.Maximum) {
		return gerror.New("maximum is not a bounded canonical number")
	}
	if field.Minimum != "" && field.Maximum != "" {
		minimum, minimumOK := new(big.Rat).SetString(field.Minimum)
		maximum, maximumOK := new(big.Rat).SetString(field.Maximum)
		if !minimumOK || !maximumOK || minimum.Cmp(maximum) > 0 {
			return gerror.New("minimum cannot exceed maximum")
		}
	}
	return nil
}

func validateConfigurationCandidates(field *capmodel.ConfigurationFieldDeclaration, methods map[string]struct{}) error {
	isChoice := field.Type == capmodel.ConfigurationFieldSingleSelect || field.Type == capmodel.ConfigurationFieldMultiSelect ||
		field.Type == capmodel.ConfigurationFieldTreeSelect || field.Type == capmodel.ConfigurationFieldAddress
	if !isChoice && (len(field.StaticCandidates) > 0 || field.DynamicCandidates != nil) {
		return gerror.New("candidate declarations are allowed only for select, tree_select, and address fields")
	}
	if isChoice && (len(field.StaticCandidates) == 0) == (field.DynamicCandidates == nil) {
		return gerror.New("choice fields require exactly one static or dynamic candidate source")
	}
	if len(field.StaticCandidates) > maxConfigurationCandidates {
		return gerror.Newf("static candidates exceed limit %d", maxConfigurationCandidates)
	}
	if len(field.StaticCandidates) > 0 {
		if err := validateStaticCandidates(field.Type, field.StaticCandidates); err != nil {
			return err
		}
	}
	if field.DynamicCandidates != nil {
		field.DynamicCandidates.Version = strings.TrimSpace(field.DynamicCandidates.Version)
		field.DynamicCandidates.Method = strings.ToLower(strings.TrimSpace(field.DynamicCandidates.Method))
		if !contractVersionPattern.MatchString(field.DynamicCandidates.Version) || !extensionMethodPattern.MatchString(field.DynamicCandidates.Method) {
			return gerror.New("dynamic candidates require a safe versioned method")
		}
		if _, ok := methods[field.DynamicCandidates.Method]; !ok {
			return gerror.New("dynamic candidate method must be declared and authorized by the same business extension")
		}
	}
	return nil
}

func validateStaticCandidates(fieldType string, candidates []*capmodel.ConfigurationCandidate) error {
	seen := make(map[string]struct{}, len(candidates))
	parents := make(map[string]string, len(candidates))
	for index, candidate := range candidates {
		if candidate == nil {
			return gerror.Newf("static candidate %d cannot be nil", index+1)
		}
		candidate.Value = strings.TrimSpace(candidate.Value)
		candidate.ParentValue = strings.TrimSpace(candidate.ParentValue)
		if !configurationValuePattern.MatchString(candidate.Value) {
			return gerror.Newf("static candidate value is unsafe: %s", candidate.Value)
		}
		if _, exists := seen[candidate.Value]; exists {
			return gerror.Newf("static candidate value is duplicated: %s", candidate.Value)
		}
		if err := validateLocalizedText(candidate.Label, true); err != nil {
			return gerror.Wrapf(err, "static candidate %s label is invalid", candidate.Value)
		}
		if candidate.ParentValue != "" && !configurationValuePattern.MatchString(candidate.ParentValue) {
			return gerror.Newf("static candidate parent value is unsafe: %s", candidate.ParentValue)
		}
		if fieldType != capmodel.ConfigurationFieldTreeSelect && fieldType != capmodel.ConfigurationFieldAddress && candidate.ParentValue != "" {
			return gerror.New("parent_value is allowed only for tree_select and address candidates")
		}
		seen[candidate.Value] = struct{}{}
		parents[candidate.Value] = candidate.ParentValue
	}
	if fieldType == capmodel.ConfigurationFieldTreeSelect || fieldType == capmodel.ConfigurationFieldAddress {
		rootCount := 0
		for value, parent := range parents {
			if parent == "" {
				rootCount++
				continue
			}
			if parent == value {
				return gerror.Newf("candidate cannot be its own parent: %s", value)
			}
			if _, exists := parents[parent]; !exists {
				return gerror.Newf("candidate parent is not declared: %s", parent)
			}
			visited := map[string]struct{}{value: {}}
			for next := parent; next != ""; next = parents[next] {
				if _, cycle := visited[next]; cycle {
					return gerror.Newf("candidate hierarchy contains a cycle at %s", next)
				}
				visited[next] = struct{}{}
				if len(visited) > maxConfigurationCandidateDepth {
					return gerror.New("candidate hierarchy exceeds the bounded depth")
				}
			}
		}
		if rootCount == 0 {
			return gerror.New("tree and address candidates require at least one root")
		}
	}
	sort.Slice(candidates, func(left, right int) bool {
		return candidates[left].Value < candidates[right].Value
	})
	return nil
}

func validateConfigurationRule(rule *capmodel.ConfigurationRuleDeclaration, fields map[string]*capmodel.ConfigurationFieldDeclaration) error {
	if rule == nil || rule.When == nil {
		return gerror.New("rule and when condition are required")
	}
	if len(rule.Actions) == 0 || len(rule.Actions) > maxConfigurationActionsPerRule {
		return gerror.Newf("rule actions must contain between 1 and %d entries", maxConfigurationActionsPerRule)
	}
	nodes := 0
	if err := validateConfigurationCondition(rule.When, fields, 1, &nodes); err != nil {
		return err
	}
	for index, action := range rule.Actions {
		if err := validateConfigurationAction(action, fields); err != nil {
			return gerror.Wrapf(err, "action %d is invalid", index+1)
		}
	}
	return nil
}

func validateConfigurationCondition(condition *capmodel.ConfigurationConditionDeclaration, fields map[string]*capmodel.ConfigurationFieldDeclaration, depth int, nodes *int) error {
	if condition == nil {
		return gerror.New("condition cannot be nil")
	}
	(*nodes)++
	if depth > maxConfigurationConditionDepth || *nodes > maxConfigurationConditionNodes {
		return gerror.New("condition tree exceeds its bounded depth or node count")
	}
	condition.Operator = strings.ToLower(strings.TrimSpace(condition.Operator))
	condition.Field = strings.ToLower(strings.TrimSpace(condition.Field))
	condition.Value = strings.TrimSpace(condition.Value)
	if condition.Operator == capmodel.ConfigurationConditionAnd || condition.Operator == capmodel.ConfigurationConditionOr {
		if condition.Field != "" || condition.Value != "" || len(condition.Children) < 2 || len(condition.Children) > maxConfigurationActionsPerRule {
			return gerror.New("and/or conditions require 2 to 8 children and no field or value")
		}
		for _, child := range condition.Children {
			if err := validateConfigurationCondition(child, fields, depth+1, nodes); err != nil {
				return err
			}
		}
		return nil
	}
	if len(condition.Children) != 0 || !supportedConfigurationLeafOperator(condition.Operator) {
		return gerror.Newf("condition operator is not supported: %s", condition.Operator)
	}
	field := fields[condition.Field]
	if field == nil {
		return gerror.Newf("condition references undeclared field: %s", condition.Field)
	}
	if condition.Operator == capmodel.ConfigurationConditionEmpty || condition.Operator == capmodel.ConfigurationConditionNotEmpty {
		if condition.Value != "" {
			return gerror.New("empty/not_empty conditions cannot carry a value")
		}
		return nil
	}
	if condition.Value == "" || len(condition.Value) > maxConfigurationLiteralBytes || containsControl(condition.Value) {
		return gerror.New("condition value must be a non-empty bounded literal")
	}
	if isNumericConditionOperator(condition.Operator) && field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldDecimal {
		return gerror.New("numeric comparison operators require an integer or decimal field")
	}
	if condition.Operator == capmodel.ConfigurationConditionContains && field.Type != capmodel.ConfigurationFieldSingleLine &&
		field.Type != capmodel.ConfigurationFieldMultiLine && field.Type != capmodel.ConfigurationFieldMultiSelect {
		return gerror.New("contains requires a text or multi_select field")
	}
	return validateConfigurationConditionLiteral(field, condition)
}

func validateConfigurationConditionLiteral(field *capmodel.ConfigurationFieldDeclaration, condition *capmodel.ConfigurationConditionDeclaration) error {
	if field.Type == capmodel.ConfigurationFieldSecret || field.Type == capmodel.ConfigurationFieldFile {
		return gerror.New("secret and file fields support only empty/not_empty conditions")
	}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		if !configurationIntegerPattern.MatchString(condition.Value) {
			return gerror.New("integer condition value must be canonical and bounded")
		}
	case capmodel.ConfigurationFieldDecimal:
		if !configurationDecimalPattern.MatchString(condition.Value) {
			return gerror.New("decimal condition value must be canonical and bounded")
		}
	case capmodel.ConfigurationFieldBoolean:
		if condition.Operator != capmodel.ConfigurationConditionEqual && condition.Operator != capmodel.ConfigurationConditionNotEqual {
			return gerror.New("boolean fields support only equal/not_equal or empty/not_empty conditions")
		}
		if condition.Value != "true" && condition.Value != "false" {
			return gerror.New("boolean condition value must be true or false")
		}
	case capmodel.ConfigurationFieldDate:
		if _, err := time.Parse("2006-01-02", condition.Value); err != nil {
			return gerror.New("date condition value must use YYYY-MM-DD")
		}
	case capmodel.ConfigurationFieldDateTime:
		if _, err := time.Parse(time.RFC3339, condition.Value); err != nil {
			return gerror.New("datetime condition value must use RFC3339")
		}
	case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect,
		capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress:
		if !configurationValuePattern.MatchString(condition.Value) {
			return gerror.New("choice condition value must use a safe opaque candidate value")
		}
		if len(field.StaticCandidates) > 0 && !configurationCandidateExists(field.StaticCandidates, condition.Value) {
			return gerror.Newf("choice condition value is not declared: %s", condition.Value)
		}
	}
	return nil
}

func validateConfigurationAction(action *capmodel.ConfigurationActionDeclaration, fields map[string]*capmodel.ConfigurationFieldDeclaration) error {
	if action == nil {
		return gerror.New("action cannot be nil")
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	action.Field = strings.ToLower(strings.TrimSpace(action.Field))
	field := fields[action.Field]
	if field == nil {
		return gerror.Newf("action references undeclared field: %s", action.Field)
	}
	if !supportedConfigurationAction(action.Action) {
		return gerror.Newf("action is not supported: %s", action.Action)
	}
	if action.Action == capmodel.ConfigurationActionShow || action.Action == capmodel.ConfigurationActionHide ||
		action.Action == capmodel.ConfigurationActionRequire || action.Action == capmodel.ConfigurationActionDisable {
		if len(action.Values) != 0 {
			return gerror.New("show/hide/require/disable actions cannot carry values")
		}
		return nil
	}
	if field.Type != capmodel.ConfigurationFieldSingleSelect && field.Type != capmodel.ConfigurationFieldMultiSelect &&
		field.Type != capmodel.ConfigurationFieldTreeSelect && field.Type != capmodel.ConfigurationFieldAddress {
		return gerror.New("candidate filter and requires/excludes actions require a choice field")
	}
	if len(action.Values) == 0 || len(action.Values) > maxConfigurationCandidates {
		return gerror.Newf("action values must contain between 1 and %d entries", maxConfigurationCandidates)
	}
	seen := make(map[string]struct{}, len(action.Values))
	for index, value := range action.Values {
		value = strings.TrimSpace(value)
		if !configurationValuePattern.MatchString(value) {
			return gerror.Newf("action value is unsafe: %s", value)
		}
		if _, exists := seen[value]; exists {
			return gerror.Newf("action value is duplicated: %s", value)
		}
		seen[value] = struct{}{}
		action.Values[index] = value
	}
	if len(field.StaticCandidates) > 0 {
		for _, value := range action.Values {
			if !configurationCandidateExists(field.StaticCandidates, value) {
				return gerror.Newf("action candidate value is not declared: %s", value)
			}
		}
	}
	sort.Strings(action.Values)
	return nil
}

func configurationCandidateExists(candidates []*capmodel.ConfigurationCandidate, value string) bool {
	for _, candidate := range candidates {
		if candidate != nil && candidate.Value == value {
			return true
		}
	}
	return false
}

func validateLocalizedText(value *capmodel.LocalizedText, required bool) error {
	if value == nil {
		if required {
			return gerror.New("bilingual text is required")
		}
		return nil
	}
	value.EnUS = strings.TrimSpace(value.EnUS)
	value.ZhCN = strings.TrimSpace(value.ZhCN)
	if value.EnUS == "" || value.ZhCN == "" {
		return gerror.New("both en-US and zh-CN text are required")
	}
	for _, item := range []string{value.EnUS, value.ZhCN} {
		if !utf8.ValidString(item) || utf8.RuneCountInString(item) > maxConfigurationLabelRunes || containsControl(item) {
			return gerror.Newf("localized text must contain at most %d safe characters", maxConfigurationLabelRunes)
		}
	}
	return nil
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func supportedConfigurationFieldType(value string) bool {
	switch value {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine,
		capmodel.ConfigurationFieldInteger, capmodel.ConfigurationFieldDecimal,
		capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldSingleSelect,
		capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldDate,
		capmodel.ConfigurationFieldDateTime, capmodel.ConfigurationFieldSecret,
		capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress,
		capmodel.ConfigurationFieldFile:
		return true
	default:
		return false
	}
}

func supportedConfigurationTextFormat(value string) bool {
	switch value {
	case capmodel.ConfigurationTextFormatPhone, capmodel.ConfigurationTextFormatEmail,
		capmodel.ConfigurationTextFormatDomain, capmodel.ConfigurationTextFormatURL:
		return true
	default:
		return false
	}
}

func supportedConfigurationLeafOperator(value string) bool {
	switch value {
	case capmodel.ConfigurationConditionEqual, capmodel.ConfigurationConditionNotEqual,
		capmodel.ConfigurationConditionContains, capmodel.ConfigurationConditionGreaterThan,
		capmodel.ConfigurationConditionGreaterOrEqual, capmodel.ConfigurationConditionLessThan,
		capmodel.ConfigurationConditionLessOrEqual, capmodel.ConfigurationConditionEmpty,
		capmodel.ConfigurationConditionNotEmpty:
		return true
	default:
		return false
	}
}

func isNumericConditionOperator(value string) bool {
	switch value {
	case capmodel.ConfigurationConditionGreaterThan, capmodel.ConfigurationConditionGreaterOrEqual,
		capmodel.ConfigurationConditionLessThan, capmodel.ConfigurationConditionLessOrEqual:
		return true
	default:
		return false
	}
}

func supportedConfigurationAction(value string) bool {
	switch value {
	case capmodel.ConfigurationActionShow, capmodel.ConfigurationActionHide,
		capmodel.ConfigurationActionRequire, capmodel.ConfigurationActionDisable,
		capmodel.ConfigurationActionFilterCandidates, capmodel.ConfigurationActionRequires,
		capmodel.ConfigurationActionExcludes:
		return true
	default:
		return false
	}
}

// validateYAMLBusinessExtensionSchema rejects unknown nested declaration keys
// before the repository's intentionally lenient top-level YAML decode.
func validateYAMLBusinessExtensionSchema(content []byte) error {
	return validateYAMLBusinessExtensionKeys(content, "business_extensions")
}

// ValidateYAMLBusinessExtensionSnapshotSchema applies the same strict nested
// schema check to persisted release snapshots before lenient YAML decoding.
func ValidateYAMLBusinessExtensionSnapshotSchema(content []byte) error {
	return validateYAMLBusinessExtensionKeys(content, "businessExtensions", "authorizedBusinessExtensions")
}

func validateYAMLBusinessExtensionKeys(content []byte, keys ...string) error {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return err
	}
	document := &root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		document = root.Content[0]
	}
	if document == nil || document.Kind != yaml.MappingNode {
		return nil
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	for index := 0; index+1 < len(document.Content); index += 2 {
		if _, ok := allowed[strings.TrimSpace(document.Content[index].Value)]; !ok {
			continue
		}
		encoded, err := yaml.Marshal(document.Content[index+1])
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(encoded))
		decoder.KnownFields(true)
		var declarations []*capmodel.BusinessExtensionDeclaration
		if err = decoder.Decode(&declarations); err != nil {
			return err
		}
	}
	return nil
}

// validateJSONBusinessExtensionSchema rejects unknown nested declaration keys
// in dynamic WASM manifest sections.
func validateJSONBusinessExtensionSchema(content []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return err
	}
	raw, exists := root["business_extensions"]
	if !exists {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var declarations []*capmodel.BusinessExtensionDeclaration
	return decoder.Decode(&declarations)
}
