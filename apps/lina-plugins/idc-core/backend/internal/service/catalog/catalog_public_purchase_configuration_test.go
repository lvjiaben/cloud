// This file verifies opaque public projection and generic configuration
// evaluation without relying on legacy server option structures.

package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type publicPurchaseAvailabilityStub struct {
	available bool
	err       error
	calls     int
}

// Storefront options and crafted quotes must obey the same frozen commercial
// restriction while exposing only opaque public tokens and bilingual labels.
func TestPublicCommercialChoiceRulesFilterAndRejectInvalidPairs(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x74}, 32))
	if err != nil {
		t.Fatal(err)
	}
	field := func(key string, choices ...string) *capmodel.ConfigurationFieldDeclaration {
		result := &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: key, EnUS: key}}
		for _, choice := range choices {
			result.StaticCandidates = append(result.StaticCandidates, &capmodel.ConfigurationCandidate{Value: choice, Label: &capmodel.LocalizedText{ZhCN: choice, EnUS: choice}})
		}
		return result
	}
	presentations := map[string]configurationcontract.Presentation{"local_validation": {ChoiceRules: []*capmodel.ConfigurationRuleDeclaration{{When: &capmodel.ConfigurationConditionDeclaration{Field: "local_certificate", Operator: capmodel.ConfigurationConditionEqual, Value: "wildcard"}, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionFilterCandidates, Field: "local_validation", Values: []string{"dns"}}}}}}}
	raw, err := json.Marshal(presentations)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &publicStructuredConfigurationSnapshot{Revision: "commercial-rules", Target: publicStructuredConfigurationTarget{CustomerPresentation: string(raw)}, Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{field("local_certificate", "single", "wildcard"), field("local_validation", "file", "dns")}}}
	service := &publicCatalogReaderImpl{revision: codec}
	for _, child := range []string{"dns", "file"} {
		values := []configurationcontract.ConfigurationValue{{FieldKey: "local_certificate", OptionTokens: []string{"wildcard"}}, {FieldKey: "local_validation", OptionTokens: []string{child}}}
		evaluation, candidates, evaluateErr := service.evaluateStructuredConfiguration(t.Context(), snapshot, values)
		if evaluateErr != nil || evaluation.Valid != (child == "dns") {
			t.Fatal("crafted customer combination bypassed frozen rules", evaluateErr)
		}
		view, projectErr := service.projectStructuredConfiguration(snapshot, evaluation, candidates)
		if projectErr != nil || len(view.Fields) != 2 || len(view.Fields[1].Options) != 1 || view.Fields[1].Options[0].Label.EnUS != "dns" {
			t.Fatal("storefront exposed an incompatible child option", projectErr)
		}
		if view.Fields[1].FieldToken == "local_validation" || view.Fields[1].Options[0].OptionToken == "dns" {
			t.Fatal("storefront exposed internal choice identities")
		}
	}
	for _, test := range []struct{ parent, child, expected string }{{"single", "dns", "dns"}, {"wildcard", "file", "dns"}, {"wildcard", "dns", "dns"}} {
		values := []configurationcontract.ConfigurationValue{{FieldKey: "local_certificate", OptionTokens: []string{test.parent}}, {FieldKey: "local_validation", OptionTokens: []string{test.child}}}
		evaluation, _, interactiveErr := service.evaluateInteractiveStructuredConfiguration(t.Context(), snapshot, values)
		if interactiveErr != nil || !evaluation.Valid {
			t.Fatal("interactive relationship did not stabilize", interactiveErr)
		}
		for _, value := range evaluation.Values {
			if value.FieldKey == "local_validation" && (len(value.OptionTokens) != 1 || value.OptionTokens[0] != test.expected) {
				t.Fatal("interactive change discarded a legal child or kept an invalid child")
			}
		}
		if values[1].OptionTokens[0] != test.child {
			t.Fatal("interactive defaults changed the caller's submitted values")
		}
	}
	initial, _, err := service.evaluateInteractiveStructuredConfiguration(t.Context(), snapshot, nil)
	if err != nil || !initial.Valid || len(initial.Values) != 2 || initial.Values[0].OptionTokens[0] != "single" || initial.Values[1].OptionTokens[0] != "file" {
		t.Fatal("initial selection did not use the merchant's first legal choices", err)
	}
	if len(snapshot.Schema.Conditions) != 0 {
		t.Fatal("public evaluation rewrote the provider declaration")
	}
}

type publicDynamicCandidateResolverStub struct {
	calls  int
	values [][]configurationcontract.ConfigurationValue
}

type publicPurchaseFileVerifierStub struct {
	purchaseconfigurationfile.Service
	calls  int
	inputs []purchaseconfigurationfile.VerifyInput
}

func (s *publicPurchaseFileVerifierStub) VerifyFields(_ context.Context, inputs []purchaseconfigurationfile.VerifyInput) ([]purchaseconfigurationfile.VerifiedField, error) {
	s.calls++
	s.inputs = inputs
	result := make([]purchaseconfigurationfile.VerifiedField, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, purchaseconfigurationfile.VerifiedField{Target: input.Target})
	}
	return result, nil
}

func (s *publicDynamicCandidateResolverStub) ResolvePublicPurchaseCandidates(
	_ context.Context,
	_ string,
	_ *capmodel.ConfigurationSchemaDeclaration,
	values []configurationcontract.ConfigurationValue,
) (configurationcontract.ResolvedCandidates, error) {
	s.calls++
	s.values = append(s.values, values)
	return configurationcontract.ResolvedCandidates{"region": {
		{Value: "cn", Label: &capmodel.LocalizedText{ZhCN: "中国", EnUS: "China"}},
		{Value: "cn-zj", ParentValue: "cn", Label: &capmodel.LocalizedText{ZhCN: "浙江", EnUS: "Zhejiang"}},
	}}, nil
}

func TestPublicDynamicCandidatesUseOpaqueReversibleTokens(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x71}, 32))
	if err != nil {
		t.Fatal(err)
	}
	resolver := &publicDynamicCandidateResolverStub{}
	service := &publicCatalogReaderImpl{revision: codec, dynamicCandidateResolver: resolver}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "region", Type: capmodel.ConfigurationFieldAddress,
		Label:             &capmodel.LocalizedText{ZhCN: "地区", EnUS: "Region"},
		DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.region"},
	}}}
	snapshot := &publicStructuredConfigurationSnapshot{
		Target:   publicStructuredConfigurationTarget{VersionNo: "ver_public_dynamic_1"},
		Revision: "revision_dynamic_1", Schema: schema,
	}
	evaluation, resolved, err := service.evaluateStructuredConfiguration(context.Background(), snapshot, nil)
	if err != nil {
		t.Fatalf("evaluate initial dynamic fields: %v", err)
	}
	view, err := service.projectStructuredConfiguration(snapshot, evaluation, resolved)
	if err != nil || len(view.Fields) != 1 || len(view.Fields[0].Options) != 1 || len(view.Fields[0].Options[0].Children) != 1 {
		t.Fatalf("project dynamic hierarchy: view=%#v err=%v", view, err)
	}
	rootToken := view.Fields[0].Options[0].OptionToken
	childToken := view.Fields[0].Options[0].Children[0].OptionToken
	if strings.Contains(rootToken, "cn") || strings.Contains(childToken, "cn-zj") {
		t.Fatal("dynamic safe value leaked in public option token")
	}
	decoded, err := service.decodePublicStructuredValues(snapshot, []PublicPurchaseConfigurationValue{{
		FieldToken: view.Fields[0].FieldToken, Type: capmodel.ConfigurationFieldAddress,
		AddressOptionTokens: []string{rootToken, childToken},
	}})
	if err != nil || len(decoded) != 1 || len(decoded[0].AddressOptionTokens) != 2 || decoded[0].AddressOptionTokens[1] != "cn-zj" {
		t.Fatalf("decode dynamic option tokens: %#v err=%v", decoded, err)
	}
	secondEvaluation, secondResolved, err := service.evaluateStructuredConfiguration(context.Background(), snapshot, decoded)
	if err != nil || resolver.calls != 2 {
		t.Fatalf("reevaluate dynamic selection: calls=%d err=%v", resolver.calls, err)
	}
	secondView, err := service.projectStructuredConfiguration(snapshot, secondEvaluation, secondResolved)
	if err != nil || len(secondView.Values) != 1 || len(secondView.Values[0].AddressOptionTokens) != 2 ||
		secondView.Values[0].AddressOptionTokens[1] != secondView.Fields[0].Options[0].Children[0].OptionToken {
		t.Fatalf("authoritative selected token does not match its projected option: view=%#v err=%v", secondView, err)
	}
	replacement := "A"
	if childToken[len(childToken)-1:] == replacement {
		replacement = "B"
	}
	tampered := childToken[:len(childToken)-1] + replacement
	if _, err = service.decodePublicStructuredValues(snapshot, []PublicPurchaseConfigurationValue{{
		FieldToken: view.Fields[0].FieldToken, Type: capmodel.ConfigurationFieldAddress,
		AddressOptionTokens: []string{rootToken, tampered},
	}}); err == nil {
		t.Fatal("tampered dynamic option token was accepted")
	}
}

func (s *publicPurchaseAvailabilityStub) CheckPublicPurchaseAvailability(context.Context, string) (bool, error) {
	s.calls++
	return s.available, s.err
}

func TestPublicStructuredProjectionNeverExposesFieldOrCandidateKeys(t *testing.T) {
	t.Parallel()
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := &publicCatalogReaderImpl{revision: codec}
	schema := publicStructuredTestSchema()
	snapshot := &publicStructuredConfigurationSnapshot{
		Target:   publicStructuredConfigurationTarget{VersionNo: "version_public_1", ConfigurationVersion: 3},
		Revision: "revision_public_test", Schema: schema,
	}
	evaluation, err := configurationcontract.Evaluate(schema, nil, configurationcontract.SurfaceCustomer, nil)
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.projectStructuredConfiguration(snapshot, evaluation, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Fields) != 3 || view.Fields[0].FieldToken == "certificate_type" {
		t.Fatalf("field identity leaked or projection changed: %#v", view.Fields)
	}
	if len(view.Fields[0].Options) != 2 || view.Fields[0].Options[0].OptionToken == "single_domain" {
		t.Fatalf("candidate value leaked or projection changed: %#v", view.Fields[0].Options)
	}
	if view.Fields[2].Visible {
		t.Fatal("field targeted by show must remain hidden before its condition matches")
	}
}

func TestPublicStructuredInputAcceptsOnlyRevisionScopedOpaqueTokens(t *testing.T) {
	t.Parallel()
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x51}, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := &publicCatalogReaderImpl{revision: codec}
	schema := publicStructuredTestSchema()
	snapshot := &publicStructuredConfigurationSnapshot{Revision: "revision_one", Schema: schema}
	fieldToken, _ := codec.encodeStructuredField(snapshot.Revision, "certificate_type")
	optionToken, _ := codec.encodeStructuredOption(snapshot.Revision, "certificate_type", "single_domain")
	values, err := service.decodePublicStructuredValues(snapshot, []PublicPurchaseConfigurationValue{{
		FieldToken: fieldToken, Type: capmodel.ConfigurationFieldSingleSelect,
		OptionTokens: []string{optionToken},
	}})
	if err != nil || len(values) != 1 || values[0].FieldKey != "certificate_type" || values[0].OptionTokens[0] != "single_domain" {
		t.Fatalf("opaque tokens did not map to the frozen declaration: %#v, %v", values, err)
	}
	if _, err = service.decodePublicStructuredValues(snapshot, []PublicPurchaseConfigurationValue{{
		FieldToken: "certificate_type", Type: capmodel.ConfigurationFieldSingleSelect,
		OptionTokens: []string{"single_domain"},
	}}); err == nil {
		t.Fatal("raw plugin field and candidate identities were accepted")
	}
	otherRevisionToken, _ := codec.encodeStructuredField("revision_two", "certificate_type")
	if _, err = service.decodePublicStructuredValues(snapshot, []PublicPurchaseConfigurationValue{{
		FieldToken: otherRevisionToken, Type: capmodel.ConfigurationFieldSingleSelect,
		OptionTokens: []string{optionToken},
	}}); err == nil {
		t.Fatal("field token from another revision was accepted")
	}
}

func TestPublicStructuredSchemaAcceptsDeclaredFilesAndDynamicCandidates(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Field", ZhCN: "字段"}
	dynamic := &capmodel.ConfigurationFieldDeclaration{
		Key: "region", Type: capmodel.ConfigurationFieldTreeSelect, Label: label,
		DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.region"},
	}
	file := &capmodel.ConfigurationFieldDeclaration{Key: "document", Type: capmodel.ConfigurationFieldFile, Label: label, UploadPolicy: "document-v1"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{dynamic, file}, UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{
		ID: "document-v1", MaximumFiles: 1, MaximumFileBytes: 1024, MaximumTotalBytes: 1024,
		AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"},
	}}}
	if _, err := configurationcontract.ValidateSchema(schema, configurationcontract.SurfaceCustomer, []string{"candidate.region"}); err != nil {
		t.Fatalf("governed dynamic and file schema was rejected: %v", err)
	}
}

func TestPublicStructuredFilePolicyIsSafeAndVerificationIsOneBatch(t *testing.T) {
	t.Parallel()
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatal(err)
	}
	verifier := &publicPurchaseFileVerifierStub{}
	service := &publicCatalogReaderImpl{revision: codec, purchaseFiles: verifier}
	policy := &capmodel.ConfigurationUploadPolicyDeclaration{
		ID: "identity.v1", MaximumFiles: 2, MaximumFileBytes: 1024, MaximumTotalBytes: 2048,
		AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}, SafetyScanRequired: true,
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{policy}, Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "identity_file", Type: capmodel.ConfigurationFieldFile,
		Label: &capmodel.LocalizedText{EnUS: "Identity file", ZhCN: "身份证明文件"}, UploadPolicy: policy.ID,
	}}}
	snapshot := &publicStructuredConfigurationSnapshot{
		Target:   publicStructuredConfigurationTarget{VersionNo: "PV_FILE_PUBLIC"},
		Revision: "revision_file_public", Schema: schema,
	}
	evaluation, err := configurationcontract.Evaluate(schema, nil, configurationcontract.SurfaceCustomer, nil)
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.projectStructuredConfiguration(snapshot, evaluation, nil)
	if err != nil || len(view.Fields) != 1 || view.Fields[0].UploadPolicy == nil || !view.Fields[0].UploadPolicy.SafetyScanRequired {
		t.Fatalf("safe file requirements=%+v err=%v", view, err)
	}
	fieldToken := view.Fields[0].FieldToken
	if strings.Contains(fieldToken, "identity_file") {
		t.Fatal("file field key leaked through public token")
	}
	values, err := service.decodePublicStructuredValues(snapshot, []PublicPurchaseConfigurationValue{{
		FieldToken: fieldToken, Type: capmodel.ConfigurationFieldFile,
		FileTokens: []string{"CFU_0123456789ABCDEF0123456789ABCDEF"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.verifyStructuredFiles(context.Background(), snapshot, values); err != nil {
		t.Fatal(err)
	}
	if verifier.calls != 1 || len(verifier.inputs) != 1 || verifier.inputs[0].Target.ProductVersionNo != snapshot.Target.VersionNo || verifier.inputs[0].Target.ConfigurationRevision != snapshot.Revision || verifier.inputs[0].Target.FieldKey != "identity_file" {
		t.Fatalf("file verification was not one exact batch: calls=%d inputs=%+v", verifier.calls, verifier.inputs)
	}
}

func TestPublicStructuredAutomationAvailabilityFailsClosed(t *testing.T) {
	t.Parallel()
	target := publicStructuredConfigurationTarget{VersionNo: "ver_public_1", HasAutomationBinding: true}
	if err := (&publicCatalogReaderImpl{}).ensurePublicPurchaseAvailability(context.Background(), target); err == nil {
		t.Fatal("bound product passed without the automation owner's availability service")
	}
	stopped := &publicPurchaseAvailabilityStub{available: false}
	if err := (&publicCatalogReaderImpl{newBusinessAvailability: stopped}).ensurePublicPurchaseAvailability(context.Background(), target); err == nil {
		t.Fatal("stopped automation interface remained available for a new purchase")
	}
	unavailable := &publicPurchaseAvailabilityStub{err: errors.New("host unavailable")}
	if err := (&publicCatalogReaderImpl{newBusinessAvailability: unavailable}).ensurePublicPurchaseAvailability(context.Background(), target); err == nil {
		t.Fatal("host availability failure did not fail closed")
	}
	accepting := &publicPurchaseAvailabilityStub{available: true}
	if err := (&publicCatalogReaderImpl{newBusinessAvailability: accepting}).ensurePublicPurchaseAvailability(context.Background(), target); err != nil {
		t.Fatalf("accepting exact automation evidence was rejected: %v", err)
	}
	manual := publicStructuredConfigurationTarget{VersionNo: "ver_manual_1"}
	if err := (&publicCatalogReaderImpl{}).ensurePublicPurchaseAvailability(context.Background(), manual); err != nil {
		t.Fatalf("manual product incorrectly required an automation dependency: %v", err)
	}
}

func publicStructuredTestSchema() *capmodel.ConfigurationSchemaDeclaration {
	label := func(enUS, zhCN string) *capmodel.LocalizedText {
		return &capmodel.LocalizedText{EnUS: enUS, ZhCN: zhCN}
	}
	return &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields: []*capmodel.ConfigurationFieldDeclaration{
			{
				Key: "certificate_type", Type: capmodel.ConfigurationFieldSingleSelect,
				Label: label("Certificate type", "证书类型"), Required: true,
				StaticCandidates: []*capmodel.ConfigurationCandidate{
					{Value: "single_domain", Label: label("Single domain", "单域名")},
					{Value: "wildcard", Label: label("Wildcard", "通配符")},
				},
			},
			{Key: "needs_company", Type: capmodel.ConfigurationFieldBoolean, Label: label("Company", "企业")},
			{Key: "company_name", Type: capmodel.ConfigurationFieldSingleLine, Label: label("Company name", "企业名称")},
		},
		Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When: &capmodel.ConfigurationConditionDeclaration{
				Operator: capmodel.ConfigurationConditionEqual, Field: "needs_company", Value: "true",
			},
			Actions: []*capmodel.ConfigurationActionDeclaration{
				{Action: capmodel.ConfigurationActionShow, Field: "company_name"},
				{Action: capmodel.ConfigurationActionRequire, Field: "company_name"},
			},
		}},
	}
}

func TestPublicLocalNumericControlsKeepOpaqueFieldsAndTypedValues(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x72}, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := &publicCatalogReaderImpl{revision: codec}
	snapshot := &publicStructuredConfigurationSnapshot{
		Target:   publicStructuredConfigurationTarget{CustomerPresentation: `{"cpu":{"displayControl":"radio","quantityStep":"2","numericChoices":["2","4","8"],"numericLabels":{"4":{"zhCN":"4核","enUS":"4 cores"}},"groupZhCN":"硬件","groupEnUS":"Hardware","unit":"核","advanced":true,"descriptionZhCN":"自定义说明","descriptionEnUS":"Custom note"}}`},
		Revision: "local-control-revision",
		Schema:   &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Label: &capmodel.LocalizedText{ZhCN: "处理器核心数", EnUS: "CPU cores"}, Minimum: "2", Maximum: "16", Required: true}}},
	}
	number := int64(4)
	evaluation, resolved, err := service.evaluateStructuredConfiguration(context.Background(), snapshot, []configurationcontract.ConfigurationValue{{FieldKey: "cpu", IntegerValue: &number}})
	if err != nil || !evaluation.Valid {
		t.Fatalf("valid numeric choice rejected: %v", err)
	}
	view, err := service.projectStructuredConfiguration(snapshot, evaluation, resolved)
	if err != nil || !view.Ready || len(view.Fields) != 1 || view.Fields[0].DisplayControl != "radio" || view.Fields[0].QuantityStep != "2" || len(view.Fields[0].NumericChoices) != 3 {
		t.Fatalf("local control lost: %+v %v", view, err)
	}
	if view.Fields[0].GroupZhCN != "硬件" || view.Fields[0].Unit != "核" || !view.Fields[0].Advanced || view.Fields[0].Description.ZhCN != "自定义说明" || view.Fields[0].NumericLabels["4"].ZhCN != "4核" {
		t.Fatal("merchant presentation metadata lost")
	}
	if len(view.Values) != 1 || view.Values[0].IntegerValue == nil || *view.Values[0].IntegerValue != "4" || len(view.Values[0].OptionTokens) > 0 {
		t.Fatal("numeric control changed provider value type")
	}
	if view.Fields[0].FieldToken == "cpu" {
		t.Fatal("provider field key exposed")
	}
	for _, invalid := range []int64{3, 6} {
		evaluation, _, err = service.evaluateStructuredConfiguration(context.Background(), snapshot, []configurationcontract.ConfigurationValue{{FieldKey: "cpu", IntegerValue: &invalid}})
		if err != nil || evaluation.Valid {
			t.Fatalf("crafted invalid choice %d accepted: %v", invalid, err)
		}
	}
}

func TestHiddenPurchaseFieldsUseFrozenDefaultsAcrossPublicProjection(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x75}, 32))
	if err != nil {
		t.Fatal(err)
	}
	hidden := false
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Required: true, Minimum: "1", Maximum: "16", Label: &capmodel.LocalizedText{ZhCN: "处理器", EnUS: "CPU"}},
		{Key: "os", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: "系统", EnUS: "OS"}, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "linux", Label: &capmodel.LocalizedText{ZhCN: "Linux", EnUS: "Linux"}}, {Value: "windows", Label: &capmodel.LocalizedText{ZhCN: "Windows", EnUS: "Windows"}}}},
	}}
	policies := map[string]configurationcontract.Presentation{"cpu": {CustomerVisible: &hidden, DefaultValue: json.RawMessage(`4`)}, "os": {CustomerVisible: &hidden, DefaultValue: json.RawMessage(`"windows"`)}}
	encoded, err := json.Marshal(policies)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &publicStructuredConfigurationSnapshot{Revision: "hidden-default", Schema: schema, Target: publicStructuredConfigurationTarget{CustomerPresentation: string(encoded)}}
	service := &publicCatalogReaderImpl{revision: codec}
	values, err := service.decodePublicStructuredValues(snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}
	evaluation, resolved, err := service.evaluateStructuredConfiguration(t.Context(), snapshot, values)
	if err != nil || !evaluation.Valid {
		t.Fatalf("hidden required field rejected: %+v %v", evaluation, err)
	}
	view, err := service.projectStructuredConfiguration(snapshot, evaluation, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Fields) != 2 || view.Fields[0].Visible || view.Fields[1].Visible || len(view.Values) != 2 {
		t.Fatalf("hidden field/value projection differs: %+v", view)
	}
	compiled, err := configurationcontract.CompileResolved(schema, values, configurationcontract.SurfaceCustomer, nil, resolved)
	if err != nil || string(compiled.ValuesJSON) != `{"cpu":4,"os":"windows"}` {
		t.Fatalf("hidden values lost before order freeze: %s %v", compiled.ValuesJSON, err)
	}
	initial, _, err := service.evaluateInteractiveStructuredConfiguration(t.Context(), snapshot, nil)
	if err != nil || !initial.Valid || len(initial.Values) != 2 {
		t.Fatalf("initial read/default reconciliation lost hidden values: %+v %v", initial, err)
	}
}

// Optional presentation does not create a separate package-default choice.
// Initial and cleared selections must carry real values into quote requests.
func TestInteractivePurchaseDefaultsIncludeOptionalFirstChoices(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x61}, 32))
	if err != nil {
		t.Fatal(err)
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Minimum: "1", Maximum: "8", Label: &capmodel.LocalizedText{ZhCN: "CPU", EnUS: "CPU"}},
		{Key: "bandwidth", Type: capmodel.ConfigurationFieldInteger, Minimum: "0", Maximum: "100", Label: &capmodel.LocalizedText{ZhCN: "带宽", EnUS: "Bandwidth"}},
		{Key: "ratio", Type: capmodel.ConfigurationFieldDecimal, Minimum: "0.5", Maximum: "2", Label: &capmodel.LocalizedText{ZhCN: "比例", EnUS: "Ratio"}},
		{Key: "plan", Type: capmodel.ConfigurationFieldSingleSelect, Label: &capmodel.LocalizedText{ZhCN: "方案", EnUS: "Plan"}, StaticCandidates: []*capmodel.ConfigurationCandidate{
			{Value: "b", Label: &capmodel.LocalizedText{ZhCN: "B", EnUS: "B"}},
			{Value: "a", Label: &capmodel.LocalizedText{ZhCN: "A", EnUS: "A"}},
		}},
	}}
	presentations, err := json.Marshal(map[string]configurationcontract.Presentation{"cpu": {Control: configurationcontract.ControlRadio, NumericChoices: []string{"4", "2"}}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &publicStructuredConfigurationSnapshot{Revision: "first-choice", Schema: schema, Target: publicStructuredConfigurationTarget{CustomerPresentation: string(presentations)}}
	service := &publicCatalogReaderImpl{revision: codec}
	for _, cleared := range []bool{false, true} {
		values := []configurationcontract.ConfigurationValue(nil)
		if cleared {
			values = []configurationcontract.ConfigurationValue{{FieldKey: "plan"}}
		}
		result, _, evaluateErr := service.evaluateInteractiveStructuredConfiguration(t.Context(), snapshot, values)
		if evaluateErr != nil || !result.Valid || len(result.Values) != 4 {
			t.Fatalf("first choices not initialized: %+v %v", result, evaluateErr)
		}
		actual := make(map[string]configurationcontract.ConfigurationValue)
		for _, value := range result.Values {
			actual[value.FieldKey] = value
		}
		if actual["cpu"].IntegerValue == nil || *actual["cpu"].IntegerValue != 4 || actual["bandwidth"].IntegerValue == nil || *actual["bandwidth"].IntegerValue != 0 || *actual["ratio"].DecimalValue != "0.5" || actual["plan"].OptionTokens[0] != "b" {
			t.Fatalf("merchant order/minimum was not preserved: %+v", actual)
		}
	}
	two, zero := "2", "0"
	values := []configurationcontract.ConfigurationValue{{FieldKey: "cpu", IntegerTextValue: &two}, {FieldKey: "bandwidth", IntegerTextValue: &zero}, {FieldKey: "plan", OptionTokens: []string{"a"}}}
	result, _, err := service.evaluateInteractiveStructuredConfiguration(t.Context(), snapshot, values)
	if err != nil || !result.Valid {
		t.Fatalf("saved selection rejected: %v", err)
	}
	for _, value := range result.Values {
		if value.FieldKey == "cpu" && (value.IntegerValue == nil || *value.IntegerValue != 2) || value.FieldKey == "bandwidth" && (value.IntegerValue == nil || *value.IntegerValue != 0) || value.FieldKey == "plan" && value.OptionTokens[0] != "a" {
			t.Fatal("first defaults replaced the customer's existing legal selection")
		}
	}
	strict, _, err := service.evaluateStructuredConfiguration(t.Context(), snapshot, nil)
	if err != nil || strict.Valid || len(strict.Values) != 0 || len(strict.Issues) != 4 {
		t.Fatal("strict quote/cart evaluator allowed omitted prices or silently inserted defaults", err)
	}
}
