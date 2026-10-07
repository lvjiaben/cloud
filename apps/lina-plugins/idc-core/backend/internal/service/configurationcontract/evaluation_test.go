// This file verifies that persisted configuration and public evaluation share
// one condition runtime and one typed-value validator.

package configurationcontract

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestEvaluateDefaultsShowTargetsToHiddenAndAppliesConditionalRequired(t *testing.T) {
	t.Parallel()
	schema := conditionalSchema()
	initial, err := Evaluate(schema, nil, SurfaceCustomer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if initial.States["details"].Visible || !initial.Valid {
		t.Fatalf("conditional field must start hidden without a required issue: %#v", initial)
	}
	enabled := true
	active, err := Evaluate(schema, []ConfigurationValue{{FieldKey: "enabled", BooleanValue: &enabled}}, SurfaceCustomer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !active.States["details"].Visible || active.Valid || !hasIssue(active.Issues, "details", IssueRequired) {
		t.Fatalf("active conditional requirement was not enforced: %#v", active)
	}
}

func TestCompileRejectsHiddenResidualValueAndUsesSameRuntime(t *testing.T) {
	t.Parallel()
	schema := conditionalSchema()
	details := "must not survive while hidden"
	evaluated, err := Evaluate(schema, []ConfigurationValue{{FieldKey: "details", TextValue: &details}}, SurfaceCustomer, nil)
	if err != nil || len(evaluated.Values) != 0 || !hasIssue(evaluated.Issues, "details", IssueDisabledValue) {
		t.Fatalf("hidden residual was not removed from authoritative values: %#v, %v", evaluated, err)
	}
	if _, err := Compile(schema, []ConfigurationValue{{FieldKey: "details", TextValue: &details}}, SurfaceCustomer, nil); err == nil {
		t.Fatal("hidden residual value passed the complete persistence compiler")
	}
	enabled := true
	if _, err := Compile(schema, []ConfigurationValue{{FieldKey: "enabled", BooleanValue: &enabled}, {FieldKey: "details", TextValue: &details}}, SurfaceCustomer, nil); err != nil {
		t.Fatalf("visible required value failed the shared compiler: %v", err)
	}
}

func TestRequiredTextRejectsAnEmptyPresentArm(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Name", ZhCN: "名称"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: label, Required: true,
	}}}
	empty := ""
	result, err := Evaluate(schema, []ConfigurationValue{{FieldKey: "name", TextValue: &empty}}, SurfaceCustomer, nil)
	if err != nil || !hasIssue(result.Issues, "name", IssueRequired) {
		t.Fatalf("empty required text arm was accepted: %#v, %v", result, err)
	}
	if _, err = Compile(schema, []ConfigurationValue{{FieldKey: "name", TextValue: &empty}}, SurfaceCustomer, nil); err == nil {
		t.Fatal("complete compiler persisted an empty required text arm")
	}
}

func TestRequiresUsesCurrentAllOfRuleAndExcludesAnyMatch(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Choice", ZhCN: "选择"}
	candidates := []*capmodel.ConfigurationCandidate{
		{Value: "a", Label: label}, {Value: "b", Label: label}, {Value: "c", Label: label},
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "trigger", Type: capmodel.ConfigurationFieldBoolean, Label: label},
			{Key: "choices", Type: capmodel.ConfigurationFieldMultiSelect, Label: label, StaticCandidates: candidates},
		},
		Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionEqual, Field: "trigger", Value: "true"},
			Actions: []*capmodel.ConfigurationActionDeclaration{
				{Action: capmodel.ConfigurationActionRequires, Field: "choices", Values: []string{"a", "b"}},
				{Action: capmodel.ConfigurationActionExcludes, Field: "choices", Values: []string{"c"}},
			},
		}},
	}
	trigger := true
	one, err := Evaluate(schema, []ConfigurationValue{
		{FieldKey: "trigger", BooleanValue: &trigger},
		{FieldKey: "choices", OptionTokens: []string{"a"}},
	}, SurfaceCustomer, nil)
	if err != nil || !hasIssue(one.Issues, "choices", IssueRequiredRelation) {
		t.Fatalf("requires must enforce every declared value: %#v, %v", one, err)
	}
	blocked, err := Evaluate(schema, []ConfigurationValue{
		{FieldKey: "trigger", BooleanValue: &trigger},
		{FieldKey: "choices", OptionTokens: []string{"a", "b", "c"}},
	}, SurfaceCustomer, nil)
	if err != nil || !hasIssue(blocked.Issues, "choices", IssueExcludedRelation) {
		t.Fatalf("excludes must reject any declared match: %#v, %v", blocked, err)
	}
}

func TestSchemaRejectsSecretConditionSource(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Secret", ZhCN: "密钥"}
	schema := &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields:  []*capmodel.ConfigurationFieldDeclaration{{Key: "credential", Type: capmodel.ConfigurationFieldSecret, Label: label}},
		Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When:    &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionNotEmpty, Field: "credential"},
			Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionDisable, Field: "credential"}},
		}},
	}
	if _, err := ValidateSchema(schema, SurfaceInterface, nil); err == nil {
		t.Fatal("secret field was accepted as a condition source")
	}
}

func TestFileValuesRequireDeclaredBoundedPolicyAndOpaqueTokens(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Document", ZhCN: "文件"}
	policy := &capmodel.ConfigurationUploadPolicyDeclaration{
		ID: "document.v1", MaximumFiles: 2, MaximumFileBytes: 1024, MaximumTotalBytes: 2048,
		AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"},
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{policy}, Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "document", Type: capmodel.ConfigurationFieldFile, Label: label, Required: true, UploadPolicy: policy.ID,
	}}}
	token := "CFU_0123456789ABCDEF0123456789ABCDEF"
	result, err := Evaluate(schema, []ConfigurationValue{{FieldKey: "document", FileTokens: []string{token}}}, SurfaceCustomer, nil)
	if err != nil || !result.Valid || len(result.Values) != 1 || result.Values[0].FileTokens[0] != token {
		t.Fatalf("bounded file token was rejected: result=%+v err=%v", result, err)
	}
	missingPolicy := *schema
	missingPolicy.UploadPolicies = nil
	if _, err = ValidateSchema(&missingPolicy, SurfaceCustomer, nil); err == nil {
		t.Fatal("file field without a declared upload policy was accepted")
	}
	if result, err = Evaluate(schema, []ConfigurationValue{{FieldKey: "document", FileTokens: []string{"file:///private/path"}}}, SurfaceCustomer, nil); err != nil || !hasIssue(result.Issues, "document", IssueInvalidValue) {
		t.Fatalf("unsafe file token was not rejected as an invalid value: result=%+v err=%v", result, err)
	}
}

func conditionalSchema() *capmodel.ConfigurationSchemaDeclaration {
	label := &capmodel.LocalizedText{EnUS: "Field", ZhCN: "字段"}
	return &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "enabled", Type: capmodel.ConfigurationFieldBoolean, Label: label},
			{Key: "details", Type: capmodel.ConfigurationFieldSingleLine, Label: label},
		},
		Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionEqual, Field: "enabled", Value: "true"},
			Actions: []*capmodel.ConfigurationActionDeclaration{
				{Action: capmodel.ConfigurationActionShow, Field: "details"},
				{Action: capmodel.ConfigurationActionRequire, Field: "details"},
			},
		}},
	}
}

func hasIssue(issues []Issue, field, code string) bool {
	for _, issue := range issues {
		if issue.FieldKey == field && issue.Code == code {
			return true
		}
	}
	return false
}
