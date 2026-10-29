// These tests keep presentation roles bound to real candidate declarations and
// verify that historical products retain automatic server login generation.
package catalog

import (
	"bytes"
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"strings"
	"testing"
)

func TestPurchasePresentationUsesCandidateSourceInsteadOfTranslatedLabel(t *testing.T) {
	field := &capmodel.ConfigurationFieldDeclaration{Key: "operating_system", Label: &capmodel.LocalizedText{ZhCN: "自定义名称", EnUS: "Custom name"}, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: resourceCandidateMethod}}
	if got := publicPurchasePresentation(field); got != "operating_system" {
		t.Fatal(got)
	}
	field.Key = "custom_option"
	if publicPurchasePresentation(field) != "" {
		t.Fatal("ordinary field acquired resource semantics")
	}
	field.Key = "region"
	field.DynamicCandidates.Method = "candidate.other"
	if publicPurchasePresentation(field) != "" {
		t.Fatal("unrelated candidate source acquired region semantics")
	}
	field.DynamicCandidates.Method = resourceCandidateMethod
	if got := publicPurchasePresentation(field); got != "region" {
		t.Fatal(got)
	}
}

func TestPublishedServerLoginSwitchesDefaultOffAndKeepGenerationRules(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"hostname_length":12,"password_length":16}`, `{"hostname_customer_input":false,"password_customer_input":false}`} {
		if publicServerLoginPolicy(raw) != nil {
			t.Fatal("historical generation unexpectedly allowed customer input")
		}
	}
	policy := publicServerLoginPolicy(`{"hostname_customer_input":true,"password_customer_input":true,"hostname_prefix":"srv","hostname_length":12,"password_length":16,"password_characters":["digits"]}`)
	if policy == nil || !policy.HostnameEnabled || !policy.PasswordEnabled || policy.Hostname.Prefix != "srv" || policy.Password.Length != 16 || len(policy.Password.Characters) != 1 {
		t.Fatal("published login rules were lost")
	}
}

func TestMaterializedCloudPurchaseRolesFollowFrozenSources(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{0x63}, 32))
	if err != nil {
		t.Fatal(err)
	}
	label := &capmodel.LocalizedText{ZhCN: "自定义显示名称", EnUS: "Custom display name"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "operating_system", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "os-a", Label: label}}},
		{Key: "local_product_region", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "area-a", Label: label}}},
		{Key: "unrelated", Type: capmodel.ConfigurationFieldSingleSelect, Label: &capmodel.LocalizedText{ZhCN: "操作系统", EnUS: "Operating system"}, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "other", Label: label}}},
	}}
	target := publicStructuredConfigurationTarget{BindingPluginID: managedCloudPresentationProvider, ProductDeclaration: `{"version":"v1","fields":[{"key":"region","type":"single_select","label":{"zhCN":"源地区","enUS":"Source region"},"dynamicCandidates":{"version":"v1","method":"configuration.candidates"}}]}`, CustomerPresentation: `{"local_product_region":{"productFieldKey":"region"}}`}
	snapshot := &publicStructuredConfigurationSnapshot{Target: target, Revision: "materialized-revision", Schema: schema}
	evaluation, err := configurationcontract.Evaluate(schema, nil, configurationcontract.SurfaceCustomer, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader := &publicCatalogReaderImpl{revision: codec}
	view, err := reader.projectStructuredConfiguration(snapshot, evaluation, nil)
	if err != nil {
		t.Fatal(err)
	}
	tokens := map[string]string{}
	for _, field := range schema.Fields {
		token, encodeErr := codec.encodeStructuredField(snapshot.Revision, field.Key)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		tokens[token] = field.Key
	}
	roles := map[string]string{}
	for _, field := range view.Fields {
		key := tokens[field.FieldToken]
		roles[key] = field.Presentation
	}
	if roles["operating_system"] != "operating_system" || roles["local_product_region"] != "region" || roles["unrelated"] != "" {
		t.Fatal("materialized roles did not follow owned source identities")
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "local_product_region") {
		t.Fatal("private source mapping leaked into public projection")
	}
	snapshot.Target.BindingPluginID = "unrelated-provider"
	view, err = reader.projectStructuredConfiguration(snapshot, evaluation, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range view.Fields {
		key := tokens[field.FieldToken]
		if key == "operating_system" && field.Presentation != "" {
			t.Fatal("unrelated provider acquired cloud OS semantics")
		}
	}
}
