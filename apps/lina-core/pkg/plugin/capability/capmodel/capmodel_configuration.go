// This file defines the transport-neutral structured configuration declaration
// protocol shared by plugin manifests, frozen release snapshots, and discovery.

package capmodel

// Governed configuration field types supported by the first protocol version.
const (
	ConfigurationFieldSingleLine   = "single_line"
	ConfigurationFieldMultiLine    = "multi_line"
	ConfigurationFieldInteger      = "integer"
	ConfigurationFieldDecimal      = "decimal"
	ConfigurationFieldBoolean      = "boolean"
	ConfigurationFieldSingleSelect = "single_select"
	ConfigurationFieldMultiSelect  = "multi_select"
	ConfigurationFieldDate         = "date"
	ConfigurationFieldDateTime     = "datetime"
	ConfigurationFieldSecret       = "secret"
	ConfigurationFieldTreeSelect   = "tree_select"
	ConfigurationFieldAddress      = "address"
	ConfigurationFieldFile         = "file"
)

// Host-governed text formats. They refine text validation without defining a
// product type or accepting plugin-provided regular expressions.
const (
	ConfigurationTextFormatPhone  = "phone"
	ConfigurationTextFormatEmail  = "email"
	ConfigurationTextFormatDomain = "domain"
	ConfigurationTextFormatURL    = "url"
)

// Safe condition operators understood by the host compiler.
const (
	ConfigurationConditionAnd            = "and"
	ConfigurationConditionOr             = "or"
	ConfigurationConditionEqual          = "equal"
	ConfigurationConditionNotEqual       = "not_equal"
	ConfigurationConditionContains       = "contains"
	ConfigurationConditionGreaterThan    = "greater_than"
	ConfigurationConditionGreaterOrEqual = "greater_or_equal"
	ConfigurationConditionLessThan       = "less_than"
	ConfigurationConditionLessOrEqual    = "less_or_equal"
	ConfigurationConditionEmpty          = "empty"
	ConfigurationConditionNotEmpty       = "not_empty"
)

// Safe condition-result actions understood by the host compiler.
const (
	ConfigurationActionShow             = "show"
	ConfigurationActionHide             = "hide"
	ConfigurationActionRequire          = "require"
	ConfigurationActionDisable          = "disable"
	ConfigurationActionFilterCandidates = "filter_candidates"
	ConfigurationActionRequires         = "requires"
	ConfigurationActionExcludes         = "excludes"
)

// LocalizedText carries the two required first-release display labels. Stable
// field keys and candidate values remain machine-only identifiers.
type LocalizedText struct {
	EnUS string `json:"enUS" yaml:"en-US"`
	ZhCN string `json:"zhCN" yaml:"zh-CN"`
}

// ConfigurationSchemaDeclaration declares one independently versioned field
// surface. The three supported surfaces are attached to a business extension.
type ConfigurationSchemaDeclaration struct {
	Version        string                                  `json:"version" yaml:"version"`
	Fields         []*ConfigurationFieldDeclaration        `json:"fields,omitempty" yaml:"fields,omitempty"`
	Conditions     []*ConfigurationRuleDeclaration         `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	UploadPolicies []*ConfigurationUploadPolicyDeclaration `json:"uploadPolicies,omitempty" yaml:"upload_policies,omitempty"`
}

// ConfigurationUploadPolicyDeclaration defines bounded, non-executable file
// constraints enforced by the host before a file token becomes usable.
type ConfigurationUploadPolicyDeclaration struct {
	ID                 string   `json:"id" yaml:"id"`
	MaximumFiles       int      `json:"maximumFiles" yaml:"maximum_files"`
	MaximumFileBytes   int64    `json:"maximumFileBytes" yaml:"maximum_file_bytes"`
	MaximumTotalBytes  int64    `json:"maximumTotalBytes" yaml:"maximum_total_bytes"`
	AllowedMIMETypes   []string `json:"allowedMimeTypes" yaml:"allowed_mime_types"`
	AllowedExtensions  []string `json:"allowedExtensions" yaml:"allowed_extensions"`
	SafetyScanRequired bool     `json:"safetyScanRequired" yaml:"safety_scan_required"`
}

// ConfigurationFieldDeclaration describes one host-rendered and host-validated
// field. It contains no script, expression, executable callback, or upload URL.
type ConfigurationFieldDeclaration struct {
	Key               string                       `json:"key" yaml:"key"`
	Type              string                       `json:"type" yaml:"type"`
	Label             *LocalizedText               `json:"label" yaml:"label"`
	Description       *LocalizedText               `json:"description,omitempty" yaml:"description,omitempty"`
	Required          bool                         `json:"required,omitempty" yaml:"required,omitempty"`
	TextFormat        string                       `json:"textFormat,omitempty" yaml:"text_format,omitempty"`
	Minimum           string                       `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum           string                       `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinimumLength     int                          `json:"minimumLength,omitempty" yaml:"minimum_length,omitempty"`
	MaximumLength     int                          `json:"maximumLength,omitempty" yaml:"maximum_length,omitempty"`
	StaticCandidates  []*ConfigurationCandidate    `json:"staticCandidates,omitempty" yaml:"static_candidates,omitempty"`
	DynamicCandidates *DynamicCandidateDeclaration `json:"dynamicCandidates,omitempty" yaml:"dynamic_candidates,omitempty"`
	UploadPolicy      string                       `json:"uploadPolicy,omitempty" yaml:"upload_policy,omitempty"`
}

// ConfigurationCandidate is one safe opaque choice with bilingual labels.
// ParentValue is available only to governed tree and address fields.
type ConfigurationCandidate struct {
	Value       string         `json:"value" yaml:"value"`
	ParentValue string         `json:"parentValue,omitempty" yaml:"parent_value,omitempty"`
	Label       *LocalizedText `json:"label" yaml:"label"`
}

// DynamicCandidateDeclaration names one versioned method already requested by
// the same business extension. Runtime authorization remains method-scoped.
type DynamicCandidateDeclaration struct {
	Version string `json:"version" yaml:"version"`
	Method  string `json:"method" yaml:"method"`
}

// ConfigurationRuleDeclaration applies safe actions when its bounded
// condition tree evaluates true.
type ConfigurationRuleDeclaration struct {
	When    *ConfigurationConditionDeclaration `json:"when" yaml:"when"`
	Actions []*ConfigurationActionDeclaration  `json:"actions" yaml:"actions"`
}

// ConfigurationConditionDeclaration is either an and/or group or one leaf
// comparison. Value is an opaque bounded literal, never an expression.
type ConfigurationConditionDeclaration struct {
	Operator string                               `json:"operator" yaml:"operator"`
	Field    string                               `json:"field,omitempty" yaml:"field,omitempty"`
	Value    string                               `json:"value,omitempty" yaml:"value,omitempty"`
	Children []*ConfigurationConditionDeclaration `json:"children,omitempty" yaml:"children,omitempty"`
}

// ConfigurationActionDeclaration is a host-owned result operation. Values are
// used only by candidate filtering and requires/excludes constraints.
type ConfigurationActionDeclaration struct {
	Action string   `json:"action" yaml:"action"`
	Field  string   `json:"field" yaml:"field"`
	Values []string `json:"values,omitempty" yaml:"values,omitempty"`
}
