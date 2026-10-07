// This file validates and canonicalizes the shared structured-configuration
// protocol before interface or product-version data reaches persistence.

package configurationcontract

import (
	"encoding/json"
	"net/mail"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const (
	maximumConfigurationFields               = 128
	maximumConfigurationRules                = 128
	maximumConfigurationActionsPerRule       = 32
	maximumConditionDepth                    = 8
	maximumConditionNodes                    = 128
	maximumCandidatesPerField                = 512
	maximumFieldKeyBytes                     = 64
	maximumTextValueBytes                    = 8192
	maximumUploadPolicies                    = 32
	maximumUploadFiles                       = 100
	maximumUploadFileBytes             int64 = 5 * 1024 * 1024 * 1024
	maximumUploadTotalBytes            int64 = 10 * 1024 * 1024 * 1024
	maximumUploadPolicyChoices               = 32
)

// ConfigurationSurface identifies the owner-governed place that consumes a
// schema. Secret fields are permitted only on the interface surface.
type ConfigurationSurface string

const (
	configurationSurfaceInterface ConfigurationSurface = "interface"
	configurationSurfaceProduct   ConfigurationSurface = "product"
	configurationSurfaceCustomer  ConfigurationSurface = "customer"
)

const (
	SurfaceInterface = configurationSurfaceInterface
	SurfaceProduct   = configurationSurfaceProduct
	SurfaceCustomer  = configurationSurfaceCustomer
)

// ConfigurationValue is a typed field value. Exactly one matching value arm
// is accepted; it never accepts arbitrary JSON or executable expressions.
type ConfigurationValue struct {
	FieldKey            string
	TextValue           *string
	IntegerValue        *int64
	IntegerTextValue    *string
	DecimalValue        *string
	BooleanValue        *bool
	DateValue           *string
	DateTimeValue       *string
	OptionTokens        []string
	TreeOptionTokens    []string
	AddressOptionTokens []string
	FileTokens          []string
}

// canonicalConfiguration contains deterministic declaration and non-secret
// value documents suitable for digesting and persistence. Secrets are returned
// separately so callers can pass them directly to secretcap.
type canonicalConfiguration struct {
	DeclarationJSON []byte
	ValuesJSON      []byte
	SecretValues    map[string]string
}

type CompiledConfiguration = canonicalConfiguration

func Compile(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (*CompiledConfiguration, error) {
	return compileConfiguration(schema, values, surface, authorizedMethods)
}

func ValidateSchema(
	schema *capmodel.ConfigurationSchemaDeclaration,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (map[string]*capmodel.ConfigurationFieldDeclaration, error) {
	return validateConfigurationSchema(schema, surface, authorizedMethods)
}

func compileConfiguration(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (*canonicalConfiguration, error) {
	return compileConfigurationValues(schema, values, surface, authorizedMethods, false)
}

// compileConfigurationValues preserves the ordinary strict compiler. The
// import-only partial path may retain missing required values, never malformed
// supplied values, disabled choices or incompatible relations.
func compileConfigurationValues(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
	allowMissing bool,
) (*canonicalConfiguration, error) {
	if schema == nil {
		if len(values) != 0 {
			return nil, gerror.New("configuration values require a schema")
		}
		return &canonicalConfiguration{DeclarationJSON: []byte("{}"), ValuesJSON: []byte("{}"), SecretValues: map[string]string{}}, nil
	}
	fields, err := validateConfigurationSchema(schema, surface, authorizedMethods)
	if err != nil {
		return nil, err
	}
	evaluation, err := Evaluate(schema, values, surface, authorizedMethods)
	if err != nil {
		return nil, err
	}
	for _, issue := range evaluation.Issues {
		if !allowMissing || (issue.Code != IssueRequired && issue.Code != IssueRequiredRelation) {
			return nil, gerror.New("configuration values do not satisfy the active declaration")
		}
	}
	valueDocument := make(map[string]any, len(values))
	secretValues := make(map[string]string)
	seen := make(map[string]struct{}, len(values))
	for _, supplied := range values {
		field := fields[supplied.FieldKey]
		if field == nil {
			return nil, gerror.New("configuration contains an unknown field")
		}
		if _, exists := seen[supplied.FieldKey]; exists {
			return nil, gerror.New("configuration contains a duplicate field")
		}
		seen[supplied.FieldKey] = struct{}{}
		value, secret, valueErr := validateConfigurationValue(field, supplied, surface)
		if valueErr != nil {
			return nil, valueErr
		}
		if secret {
			secretValue, ok := value.(string)
			if !ok {
				return nil, gerror.New("configuration secret value has an invalid internal type")
			}
			secretValues[field.Key] = secretValue
			continue
		}
		valueDocument[field.Key] = value
	}
	declarationJSON, err := json.Marshal(schema)
	if err != nil {
		return nil, gerror.Wrap(err, "encode configuration declaration")
	}
	valuesJSON, err := json.Marshal(valueDocument)
	if err != nil {
		return nil, gerror.Wrap(err, "encode configuration values")
	}
	return &canonicalConfiguration{DeclarationJSON: declarationJSON, ValuesJSON: valuesJSON, SecretValues: secretValues}, nil
}

func validateConfigurationSchema(
	schema *capmodel.ConfigurationSchemaDeclaration,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (map[string]*capmodel.ConfigurationFieldDeclaration, error) {
	if schema == nil {
		return map[string]*capmodel.ConfigurationFieldDeclaration{}, nil
	}
	if schema.Version == "" || len(schema.Version) > 32 || len(schema.Fields) > maximumConfigurationFields || len(schema.Conditions) > maximumConfigurationRules {
		return nil, gerror.New("configuration schema bounds are invalid")
	}
	policies, err := validateUploadPolicies(schema.UploadPolicies)
	if err != nil {
		return nil, err
	}
	methods := make(map[string]struct{}, len(authorizedMethods))
	for _, method := range authorizedMethods {
		methods[method] = struct{}{}
	}
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration, len(schema.Fields))
	for _, field := range schema.Fields {
		if err := validateConfigurationField(field, surface, methods); err != nil {
			return nil, err
		}
		if _, exists := fields[field.Key]; exists {
			return nil, gerror.New("configuration field keys must be unique")
		}
		fields[field.Key] = field
		if field.Type == capmodel.ConfigurationFieldFile {
			if policies[field.UploadPolicy] == nil {
				return nil, gerror.New("configuration file field requires a declared upload policy")
			}
		}
	}
	for _, rule := range schema.Conditions {
		if err := validateConfigurationRule(rule, fields); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func validateUploadPolicies(input []*capmodel.ConfigurationUploadPolicyDeclaration) (map[string]*capmodel.ConfigurationUploadPolicyDeclaration, error) {
	if len(input) > maximumUploadPolicies {
		return nil, gerror.New("configuration upload policy bound exceeded")
	}
	result := make(map[string]*capmodel.ConfigurationUploadPolicyDeclaration, len(input))
	for _, policy := range input {
		if policy == nil || !validUploadPolicyID(policy.ID) ||
			policy.MaximumFiles < 1 || policy.MaximumFiles > maximumUploadFiles ||
			policy.MaximumFileBytes < 1 || policy.MaximumFileBytes > maximumUploadFileBytes ||
			policy.MaximumTotalBytes < policy.MaximumFileBytes || policy.MaximumTotalBytes > maximumUploadTotalBytes ||
			len(policy.AllowedMIMETypes) < 1 || len(policy.AllowedMIMETypes) > maximumUploadPolicyChoices ||
			len(policy.AllowedExtensions) < 1 || len(policy.AllowedExtensions) > maximumUploadPolicyChoices {
			return nil, gerror.New("configuration upload policy is invalid")
		}
		if _, exists := result[policy.ID]; exists {
			return nil, gerror.New("configuration upload policy identifiers must be unique")
		}
		seenMIME := make(map[string]struct{}, len(policy.AllowedMIMETypes))
		for _, value := range policy.AllowedMIMETypes {
			if value != strings.ToLower(strings.TrimSpace(value)) || !validUploadMIME(value) {
				return nil, gerror.New("configuration upload MIME type is invalid")
			}
			if _, exists := seenMIME[value]; exists {
				return nil, gerror.New("configuration upload MIME types must be unique")
			}
			seenMIME[value] = struct{}{}
		}
		seenExtension := make(map[string]struct{}, len(policy.AllowedExtensions))
		for _, value := range policy.AllowedExtensions {
			if value != strings.ToLower(strings.TrimSpace(value)) || !validUploadExtension(value) {
				return nil, gerror.New("configuration upload extension is invalid")
			}
			if _, exists := seenExtension[value]; exists {
				return nil, gerror.New("configuration upload extensions must be unique")
			}
			seenExtension[value] = struct{}{}
		}
		result[policy.ID] = policy
	}
	return result, nil
}

func validUploadMIME(value string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 2 && validUploadAtom(parts[0], 64, true) && validUploadAtom(parts[1], 64, true)
}

func validUploadPolicyID(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	previousSeparator := false
	for _, character := range value {
		separator := character == '.' || character == '_' || character == '-'
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			previousSeparator = false
			continue
		}
		if separator && !previousSeparator {
			previousSeparator = true
			continue
		}
		return false
	}
	return !previousSeparator
}

func validUploadExtension(value string) bool {
	return len(value) >= 2 && len(value) <= 17 && value[0] == '.' && validUploadAtom(value[1:], 16, true)
}

func validUploadAtom(value string, limit int, allowDot bool) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune("!#$&^_+-", character) || allowDot && character == '.' {
			continue
		}
		return false
	}
	return true
}

func validateConfigurationField(
	field *capmodel.ConfigurationFieldDeclaration,
	surface ConfigurationSurface,
	authorizedMethods map[string]struct{},
) error {
	if err := validateConfigurationFieldBasics(field, surface); err != nil {
		return err
	}
	if err := validateStaticCandidates(field); err != nil {
		return err
	}
	if err := validateDynamicCandidates(field, authorizedMethods); err != nil {
		return err
	}
	if field.UploadPolicy != "" && field.Type != capmodel.ConfigurationFieldFile {
		return gerror.New("configuration upload policy requires a file field")
	}
	return nil
}

func validateConfigurationFieldBasics(field *capmodel.ConfigurationFieldDeclaration, surface ConfigurationSurface) error {
	if field == nil || !validConfigurationKey(field.Key) || !validLocalizedText(field.Label) || !validOptionalLocalizedText(field.Description) {
		return gerror.New("configuration field identity or bilingual label is invalid")
	}
	if !validConfigurationFieldType(field.Type) || field.Type == capmodel.ConfigurationFieldSecret && surface != configurationSurfaceInterface {
		return gerror.New("configuration field type is unavailable on this surface")
	}
	if field.TextFormat != "" && (field.Type != capmodel.ConfigurationFieldSingleLine || !validTextFormat(field.TextFormat)) {
		return gerror.New("configuration text format is invalid")
	}
	if field.MinimumLength < 0 || field.MaximumLength < 0 ||
		field.MaximumLength > 0 && field.MinimumLength > field.MaximumLength ||
		field.MaximumLength > maximumTextValueBytes {
		return gerror.New("configuration length bounds are invalid")
	}
	if (field.Minimum != "" || field.Maximum != "") &&
		field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldDecimal {
		return gerror.New("configuration numeric bounds require a numeric field")
	}
	return validateNumericBounds(field)
}

func validateStaticCandidates(field *capmodel.ConfigurationFieldDeclaration) error {
	if len(field.StaticCandidates) > maximumCandidatesPerField || len(field.StaticCandidates) > 0 && field.DynamicCandidates != nil {
		return gerror.New("configuration candidates are invalid")
	}
	if len(field.StaticCandidates) > 0 && !candidateFieldType(field.Type) {
		return gerror.New("configuration candidates require a selection field")
	}
	seenCandidates := make(map[string]struct{}, len(field.StaticCandidates))
	candidateByValue := make(map[string]*capmodel.ConfigurationCandidate, len(field.StaticCandidates))
	for _, candidate := range field.StaticCandidates {
		if candidate == nil || candidate.Value == "" || len(candidate.Value) > 256 || !validLocalizedText(candidate.Label) {
			return gerror.New("configuration candidate is invalid")
		}
		if _, exists := seenCandidates[candidate.Value]; exists {
			return gerror.New("configuration candidate values must be unique")
		}
		seenCandidates[candidate.Value] = struct{}{}
		candidateByValue[candidate.Value] = candidate
	}
	return validateCandidateHierarchy(field, candidateByValue)
}

func validateCandidateHierarchy(
	field *capmodel.ConfigurationFieldDeclaration,
	candidateByValue map[string]*capmodel.ConfigurationCandidate,
) error {
	for _, candidate := range field.StaticCandidates {
		if candidate.ParentValue == "" {
			continue
		}
		if field.Type != capmodel.ConfigurationFieldTreeSelect && field.Type != capmodel.ConfigurationFieldAddress {
			return gerror.New("configuration candidate parent requires a hierarchical field")
		}
		if candidate.ParentValue == candidate.Value || candidateByValue[candidate.ParentValue] == nil {
			return gerror.New("configuration candidate parent is invalid")
		}
		seenParents := map[string]struct{}{candidate.Value: {}}
		current := candidate
		for current.ParentValue != "" {
			if _, cycle := seenParents[current.ParentValue]; cycle {
				return gerror.New("configuration candidate hierarchy contains a cycle")
			}
			seenParents[current.ParentValue] = struct{}{}
			current = candidateByValue[current.ParentValue]
			if current == nil || len(seenParents) > maximumConditionDepth {
				return gerror.New("configuration candidate hierarchy is invalid")
			}
		}
	}
	return nil
}

func validateDynamicCandidates(field *capmodel.ConfigurationFieldDeclaration, authorizedMethods map[string]struct{}) error {
	if dynamic := field.DynamicCandidates; dynamic != nil {
		if !candidateFieldType(field.Type) || dynamic.Version == "" || len(dynamic.Version) > 32 || !validConfigurationKey(dynamic.Method) {
			return gerror.New("dynamic configuration candidate declaration is invalid")
		}
		if _, authorized := authorizedMethods[dynamic.Method]; !authorized {
			return gerror.New("dynamic configuration candidate method is not authorized")
		}
	}
	return nil
}

func validateNumericBounds(field *capmodel.ConfigurationFieldDeclaration) error {
	if field.Type == capmodel.ConfigurationFieldInteger {
		minimum, minimumErr := parseOptionalInteger(field.Minimum)
		maximum, maximumErr := parseOptionalInteger(field.Maximum)
		if minimumErr != nil || maximumErr != nil || (minimum != nil && maximum != nil && *minimum > *maximum) {
			return gerror.New("configuration integer bounds are invalid")
		}
	}
	if field.Type == capmodel.ConfigurationFieldDecimal {
		minimum, minimumOK := normalizeDecimal(field.Minimum)
		maximum, maximumOK := normalizeDecimal(field.Maximum)
		if !minimumOK || !maximumOK || (minimum != "" && maximum != "" && compareDecimals(minimum, maximum) > 0) {
			return gerror.New("configuration decimal bounds are invalid")
		}
	}
	return nil
}

func validateConfigurationRule(rule *capmodel.ConfigurationRuleDeclaration, fields map[string]*capmodel.ConfigurationFieldDeclaration) error {
	if rule == nil || rule.When == nil || len(rule.Actions) == 0 || len(rule.Actions) > maximumConfigurationActionsPerRule {
		return gerror.New("configuration rule is invalid")
	}
	nodes := 0
	if err := validateConfigurationCondition(rule.When, fields, 1, &nodes); err != nil {
		return err
	}
	for _, action := range rule.Actions {
		if action == nil || fields[action.Field] == nil || !validConfigurationAction(action.Action) {
			return gerror.New("configuration action is invalid")
		}
		if len(action.Values) > maximumCandidatesPerField {
			return gerror.New("configuration action values exceed the bound")
		}
		if len(action.Values) > 0 && action.Action != capmodel.ConfigurationActionFilterCandidates && action.Action != capmodel.ConfigurationActionRequires && action.Action != capmodel.ConfigurationActionExcludes {
			return gerror.New("configuration action values are not allowed")
		}
	}
	return nil
}

func validateConfigurationCondition(condition *capmodel.ConfigurationConditionDeclaration, fields map[string]*capmodel.ConfigurationFieldDeclaration, depth int, nodes *int) error {
	*nodes++
	if condition == nil || depth > maximumConditionDepth || *nodes > maximumConditionNodes {
		return gerror.New("configuration condition exceeds the bound")
	}
	if condition.Operator == capmodel.ConfigurationConditionAnd || condition.Operator == capmodel.ConfigurationConditionOr {
		if condition.Field != "" || condition.Value != "" || len(condition.Children) < 2 || len(condition.Children) > maximumConditionNodes {
			return gerror.New("configuration condition group is invalid")
		}
		for _, child := range condition.Children {
			if err := validateConfigurationCondition(child, fields, depth+1, nodes); err != nil {
				return err
			}
		}
		return nil
	}
	field := fields[condition.Field]
	if field == nil || len(condition.Children) != 0 || !validConfigurationConditionOperator(condition.Operator) {
		return gerror.New("configuration condition leaf is invalid")
	}
	if field.Type == capmodel.ConfigurationFieldSecret {
		return gerror.New("secret configuration fields cannot be condition sources")
	}
	if (condition.Operator == capmodel.ConfigurationConditionEmpty || condition.Operator == capmodel.ConfigurationConditionNotEmpty) && condition.Value != "" {
		return gerror.New("empty configuration condition cannot contain a value")
	}
	if numericCondition(condition.Operator) && field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldDecimal {
		return gerror.New("numeric configuration condition requires a numeric field")
	}
	if condition.Value != "" && len(condition.Value) > maximumTextValueBytes {
		return gerror.New("configuration condition value exceeds the bound")
	}
	return nil
}

func validateConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, supplied ConfigurationValue, surface ConfigurationSurface) (any, bool, error) {
	if configurationValueArmCount(supplied) != 1 {
		return nil, false, gerror.New("configuration value must use exactly one typed arm")
	}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		return validateIntegerConfigurationValue(field, supplied)
	case capmodel.ConfigurationFieldBoolean:
		if supplied.BooleanValue == nil {
			return nil, false, gerror.New("configuration boolean value is invalid")
		}
		return *supplied.BooleanValue, false, nil
	case capmodel.ConfigurationFieldMultiSelect:
		return validateMultiSelectConfigurationValue(field, supplied.OptionTokens)
	case capmodel.ConfigurationFieldSingleSelect:
		return validateSingleCandidateConfigurationValue(field, supplied.OptionTokens, "single-select")
	case capmodel.ConfigurationFieldTreeSelect:
		return validateSingleCandidateConfigurationValue(field, supplied.TreeOptionTokens, "tree")
	case capmodel.ConfigurationFieldAddress:
		return validateAddressConfigurationValue(field, supplied.AddressOptionTokens)
	case capmodel.ConfigurationFieldFile:
		return validateFileConfigurationValue(supplied.FileTokens)
	case capmodel.ConfigurationFieldDecimal:
		return validateStringArmConfigurationValue(field, supplied.DecimalValue, false, "decimal")
	case capmodel.ConfigurationFieldDate:
		return validateStringArmConfigurationValue(field, supplied.DateValue, false, "date")
	case capmodel.ConfigurationFieldDateTime:
		return validateStringArmConfigurationValue(field, supplied.DateTimeValue, false, "datetime")
	default:
		return validateStringArmConfigurationValue(
			field, supplied.TextValue,
			field.Type == capmodel.ConfigurationFieldSecret && surface == configurationSurfaceInterface,
			"string",
		)
	}
}

func configurationValueArmCount(supplied ConfigurationValue) int {
	arms := 0
	for _, present := range []bool{
		supplied.TextValue != nil,
		supplied.IntegerValue != nil || supplied.IntegerTextValue != nil,
		supplied.BooleanValue != nil,
		supplied.DecimalValue != nil,
		supplied.DateValue != nil,
		supplied.DateTimeValue != nil,
		supplied.OptionTokens != nil,
		supplied.TreeOptionTokens != nil,
		supplied.AddressOptionTokens != nil,
		supplied.FileTokens != nil,
	} {
		if present {
			arms++
		}
	}
	return arms
}

func validateIntegerConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, supplied ConfigurationValue) (any, bool, error) {
	if supplied.IntegerValue != nil && supplied.IntegerTextValue != nil {
		return nil, false, gerror.New("configuration integer value is invalid")
	}
	integer, ok := int64(0), false
	if supplied.IntegerValue != nil {
		integer, ok = *supplied.IntegerValue, true
	}
	if supplied.IntegerTextValue != nil {
		parsed, err := strconv.ParseInt(*supplied.IntegerTextValue, 10, 64)
		if err != nil || strconv.FormatInt(parsed, 10) != *supplied.IntegerTextValue {
			return nil, false, gerror.New("configuration integer value is invalid")
		}
		integer, ok = parsed, true
	}
	if !ok || !integerWithinBounds(integer, field) {
		return nil, false, gerror.New("configuration integer value is invalid")
	}
	return integer, false, nil
}

func validateMultiSelectConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, tokens []string) (any, bool, error) {
	if tokens == nil || !validSelectedCandidates(field, tokens) {
		return nil, false, gerror.New("configuration multi-select value is invalid")
	}
	values := append([]string{}, tokens...)
	sort.Strings(values)
	return values, false, nil
}

func validateSingleCandidateConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, tokens []string, kind string) (any, bool, error) {
	if len(tokens) != 1 || !validSelectedCandidates(field, tokens) {
		return nil, false, gerror.Newf("configuration %s value is invalid", kind)
	}
	return tokens[0], false, nil
}

func validateAddressConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, tokens []string) (any, bool, error) {
	if len(tokens) == 0 || len(tokens) > maximumCandidatesPerField || !validAddressPath(field, tokens) {
		return nil, false, gerror.New("configuration address value is invalid")
	}
	return append([]string{}, tokens...), false, nil
}

func validateFileConfigurationValue(tokens []string) (any, bool, error) {
	if len(tokens) < 1 || len(tokens) > maximumUploadFiles {
		return nil, false, gerror.New("configuration file token count is invalid")
	}
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if !validConfigurationFileToken(token) {
			return nil, false, gerror.New("configuration file token is invalid")
		}
		if _, duplicate := seen[token]; duplicate {
			return nil, false, gerror.New("configuration file tokens must be unique")
		}
		seen[token] = struct{}{}
	}
	return append([]string(nil), tokens...), false, nil
}

func validConfigurationFileToken(token string) bool {
	if len(token) != 36 || !(strings.HasPrefix(token, "CFU_") || strings.HasPrefix(token, "MCF_")) {
		return false
	}
	for _, character := range token[4:] {
		if !(character >= '0' && character <= '9' || character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}

func validateStringArmConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, value *string, secret bool, kind string) (any, bool, error) {
	if value == nil || !validStringConfigurationValue(field, *value) {
		return nil, false, gerror.Newf("configuration %s value is invalid", kind)
	}
	return *value, secret, nil
}

func validStringConfigurationValue(field *capmodel.ConfigurationFieldDeclaration, value string) bool {
	if len(value) > maximumTextValueBytes || (field.MinimumLength > 0 && len([]rune(value)) < field.MinimumLength) || (field.MaximumLength > 0 && len([]rune(value)) > field.MaximumLength) {
		return false
	}
	switch field.Type {
	case capmodel.ConfigurationFieldSingleLine:
		if strings.ContainsAny(value, "\r\n") || !validFormattedText(field.TextFormat, value) {
			return false
		}
	case capmodel.ConfigurationFieldDecimal:
		normalized, ok := normalizeDecimal(value)
		if !ok || normalized == "" || !decimalWithinBounds(normalized, field) {
			return false
		}
	case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldTreeSelect:
		if !validSelectedCandidates(field, []string{value}) {
			return false
		}
	case capmodel.ConfigurationFieldDate:
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return false
		}
	case capmodel.ConfigurationFieldDateTime:
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return false
		}
	case capmodel.ConfigurationFieldSecret:
		return value != ""
	case capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldAddress, capmodel.ConfigurationFieldFile:
	default:
		return false
	}
	return true
}

func validFormattedText(format, value string) bool {
	switch format {
	case "":
		return true
	case capmodel.ConfigurationTextFormatEmail:
		address, err := mail.ParseAddress(value)
		return err == nil && address.Address == value
	case capmodel.ConfigurationTextFormatURL:
		parsed, err := url.ParseRequestURI(value)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
	case capmodel.ConfigurationTextFormatPhone:
		if len(value) < 7 || len(value) > 16 {
			return false
		}
		for index, character := range value {
			if character == '+' && index == 0 {
				continue
			}
			if !unicode.IsDigit(character) {
				return false
			}
		}
		return true
	case capmodel.ConfigurationTextFormatDomain:
		return validDomain(value)
	default:
		return false
	}
}

func validDomain(value string) bool {
	if len(value) == 0 || len(value) > 253 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' {
				return false
			}
		}
	}
	return true
}

func validSelectedCandidates(field *capmodel.ConfigurationFieldDeclaration, values []string) bool {
	if len(values) > maximumCandidatesPerField {
		return false
	}
	if field.DynamicCandidates != nil {
		// Dynamic candidate tokens require a method-scoped host resolver. Schema
		// admission alone does not prove that a caller-supplied token exists.
		return false
	}
	allowed := make(map[string]struct{}, len(field.StaticCandidates))
	for _, candidate := range field.StaticCandidates {
		allowed[candidate.Value] = struct{}{}
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return len(values) > 0
}

func validAddressPath(field *capmodel.ConfigurationFieldDeclaration, values []string) bool {
	if field.DynamicCandidates != nil {
		return false
	}
	candidates := make(map[string]*capmodel.ConfigurationCandidate, len(field.StaticCandidates))
	for _, candidate := range field.StaticCandidates {
		candidates[candidate.Value] = candidate
	}
	for index, value := range values {
		candidate := candidates[value]
		if candidate == nil {
			return false
		}
		expectedParent := ""
		if index > 0 {
			expectedParent = values[index-1]
		}
		if candidate.ParentValue != expectedParent {
			return false
		}
	}
	return true
}

func validConfigurationKey(value string) bool {
	if value == "" || len(value) > maximumFieldKeyBytes {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (index > 0 && character >= '0' && character <= '9') || (index > 0 && (character == '.' || character == '_' || character == '-')) {
			continue
		}
		return false
	}
	return true
}

func validLocalizedText(value *capmodel.LocalizedText) bool {
	return value != nil && strings.TrimSpace(value.EnUS) != "" && strings.TrimSpace(value.ZhCN) != "" && len(value.EnUS) <= 256 && len(value.ZhCN) <= 256
}
func validOptionalLocalizedText(value *capmodel.LocalizedText) bool {
	return value == nil || validLocalizedText(value)
}
func validTextFormat(value string) bool {
	return value == capmodel.ConfigurationTextFormatPhone || value == capmodel.ConfigurationTextFormatEmail || value == capmodel.ConfigurationTextFormatDomain || value == capmodel.ConfigurationTextFormatURL
}
func candidateFieldType(value string) bool {
	return value == capmodel.ConfigurationFieldSingleSelect || value == capmodel.ConfigurationFieldMultiSelect || value == capmodel.ConfigurationFieldTreeSelect || value == capmodel.ConfigurationFieldAddress
}
func numericCondition(value string) bool {
	return value == capmodel.ConfigurationConditionGreaterThan || value == capmodel.ConfigurationConditionGreaterOrEqual || value == capmodel.ConfigurationConditionLessThan || value == capmodel.ConfigurationConditionLessOrEqual
}
func validConfigurationConditionOperator(value string) bool {
	switch value {
	case capmodel.ConfigurationConditionEqual, capmodel.ConfigurationConditionNotEqual, capmodel.ConfigurationConditionContains, capmodel.ConfigurationConditionGreaterThan, capmodel.ConfigurationConditionGreaterOrEqual, capmodel.ConfigurationConditionLessThan, capmodel.ConfigurationConditionLessOrEqual, capmodel.ConfigurationConditionEmpty, capmodel.ConfigurationConditionNotEmpty:
		return true
	}
	return false
}
func validConfigurationAction(value string) bool {
	switch value {
	case capmodel.ConfigurationActionShow, capmodel.ConfigurationActionHide, capmodel.ConfigurationActionRequire, capmodel.ConfigurationActionDisable, capmodel.ConfigurationActionFilterCandidates, capmodel.ConfigurationActionRequires, capmodel.ConfigurationActionExcludes:
		return true
	}
	return false
}
func validConfigurationFieldType(value string) bool {
	switch value {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldInteger, capmodel.ConfigurationFieldDecimal, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldDate, capmodel.ConfigurationFieldDateTime, capmodel.ConfigurationFieldSecret, capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress, capmodel.ConfigurationFieldFile:
		return true
	}
	return false
}

func parseOptionalInteger(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
func integerWithinBounds(value int64, field *capmodel.ConfigurationFieldDeclaration) bool {
	minimum, minimumErr := parseOptionalInteger(field.Minimum)
	maximum, maximumErr := parseOptionalInteger(field.Maximum)
	return minimumErr == nil && maximumErr == nil && (minimum == nil || value >= *minimum) && (maximum == nil || value <= *maximum)
}
func decimalWithinBounds(value string, field *capmodel.ConfigurationFieldDeclaration) bool {
	minimum, minimumOK := normalizeDecimal(field.Minimum)
	maximum, maximumOK := normalizeDecimal(field.Maximum)
	return minimumOK && maximumOK && (minimum == "" || compareDecimals(value, minimum) >= 0) && (maximum == "" || compareDecimals(value, maximum) <= 0)
}

func normalizeDecimal(value string) (string, bool) {
	if value == "" {
		return "", true
	}
	negative := strings.HasPrefix(value, "-")
	unsigned := strings.TrimPrefix(value, "-")
	parts := strings.Split(unsigned, ".")
	if len(parts) > 2 || parts[0] == "" {
		return "", false
	}
	for _, part := range parts {
		for _, character := range part {
			if character < '0' || character > '9' {
				return "", false
			}
		}
	}
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if integer == "0" && fraction == "" {
		negative = false
	}
	result := integer
	if fraction != "" {
		result += "." + fraction
	}
	if negative {
		result = "-" + result
	}
	return result, true
}

func compareDecimals(left, right string) int {
	leftNegative, rightNegative := strings.HasPrefix(left, "-"), strings.HasPrefix(right, "-")
	if leftNegative != rightNegative {
		if leftNegative {
			return -1
		}
		return 1
	}
	left = strings.TrimPrefix(left, "-")
	right = strings.TrimPrefix(right, "-")
	leftParts, rightParts := strings.SplitN(left, ".", 2), strings.SplitN(right, ".", 2)
	comparison := len(leftParts[0]) - len(rightParts[0])
	if comparison == 0 {
		comparison = strings.Compare(leftParts[0], rightParts[0])
	}
	if comparison == 0 {
		leftFraction, rightFraction := "", ""
		if len(leftParts) == 2 {
			leftFraction = leftParts[1]
		}
		if len(rightParts) == 2 {
			rightFraction = rightParts[1]
		}
		width := len(leftFraction)
		if len(rightFraction) > width {
			width = len(rightFraction)
		}
		leftFraction += strings.Repeat("0", width-len(leftFraction))
		rightFraction += strings.Repeat("0", width-len(rightFraction))
		comparison = strings.Compare(leftFraction, rightFraction)
	}
	if leftNegative {
		return -comparison
	}
	return comparison
}
