// This file verifies opaque public projection and generic configuration
// evaluation without relying on legacy server option structures.

package catalog

import (
	"bytes"
	"context"
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
