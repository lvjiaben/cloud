package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/pkg/plugin/capability/capmodel"
)

func TestValidateConfigurationDeclarationsAcceptsThirteenGovernedFieldTypes(t *testing.T) {
	declaration := validConfigurationProviderDeclaration()
	if err := ValidateConfigurationDeclarations(declaration); err != nil {
		t.Fatalf("validate complete configuration declaration: %v", err)
	}
	types := make(map[string]struct{})
	for _, schema := range []*capmodel.ConfigurationSchemaDeclaration{
		declaration.InterfaceConfiguration,
		declaration.ProductConfiguration,
		declaration.CustomerConfiguration,
	} {
		for _, field := range schema.Fields {
			types[field.Type] = struct{}{}
		}
	}
	if len(types) != 13 {
		t.Fatalf("expected all 13 governed field types, got %d: %#v", len(types), types)
	}
}

func TestValidateConfigurationDeclarationsRejectsUnsafeOrUnboundedForms(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*capmodel.BusinessExtensionDeclaration)
	}{
		{"secret outside interface", func(item *capmodel.BusinessExtensionDeclaration) {
			item.ProductConfiguration.Fields[0].Type = capmodel.ConfigurationFieldSecret
		}},
		{"unknown field type", func(item *capmodel.BusinessExtensionDeclaration) {
			item.ProductConfiguration.Fields[0].Type = "javascript"
		}},
		{"missing interface cardinality", func(item *capmodel.BusinessExtensionDeclaration) {
			item.InterfaceConfigurationCardinality = ""
		}},
		{"unsupported interface cardinality", func(item *capmodel.BusinessExtensionDeclaration) {
			item.InterfaceConfigurationCardinality = "unlimited"
		}},
		{"unbounded regular expression format", func(item *capmodel.BusinessExtensionDeclaration) {
			item.ProductConfiguration.Fields[0].TextFormat = "regex:^.*$"
		}},
		{"undeclared dynamic method", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Fields[1].DynamicCandidates.Method = "script.execute"
		}},
		{"static and dynamic candidates together", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Fields[0].DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidates.list"}
		}},
		{"arbitrary upload URL", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Fields[5].UploadPolicy = "https://uploads.invalid"
		}},
		{"unknown upload policy", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Fields[5].UploadPolicy = "customer.missing.v1"
		}},
		{"unsafe MIME type", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.UploadPolicies[0].AllowedMIMETypes[0] = "text/html;script=true"
		}},
		{"unsafe extension", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.UploadPolicies[0].AllowedExtensions[0] = "../pdf"
		}},
		{"file count exceeds policy", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.UploadPolicies[0].MaximumFiles = maxConfigurationUploadFiles + 1
		}},
		{"condition references unknown field", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Conditions[0].When.Children[0].Field = "missing"
		}},
		{"numeric comparison on text", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Conditions[0].When.Children[0].Operator = capmodel.ConfigurationConditionGreaterThan
		}},
		{"unsafe action", func(item *capmodel.BusinessExtensionDeclaration) {
			item.CustomerConfiguration.Conditions[0].Actions[0].Action = "eval"
		}},
		{"too many fields", func(item *capmodel.BusinessExtensionDeclaration) {
			for index := len(item.ProductConfiguration.Fields); index <= maxConfigurationFields; index++ {
				item.ProductConfiguration.Fields = append(item.ProductConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{
					Key: fmt.Sprintf("field_%d", index), Type: capmodel.ConfigurationFieldBoolean,
					Label: bilingual(fmt.Sprintf("Field %d", index), fmt.Sprintf("字段%d", index)),
				})
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			declaration := cloneBusinessExtensionDeclaration(validConfigurationProviderDeclaration())
			test.mutate(declaration)
			if err := ValidateConfigurationDeclarations(declaration); err == nil {
				t.Fatal("expected invalid configuration declaration to be rejected")
			}
		})
	}
}

func TestValidateBusinessExtensionsRestrictsConfigurationOwnershipAndSurface(t *testing.T) {
	owner := ownerManifest("idc-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query", "candidates.list"})
	owner.BusinessExtensions[0].InterfaceConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine, Label: bilingual("Endpoint", "接口地址"),
	}}}
	if err := ValidateBusinessExtensions(owner); err == nil {
		t.Fatal("owner configuration must be rejected")
	}

	provider := validConfigurationProviderManifest()
	provider.BusinessExtensions[0].Point = capmodel.BusinessExtensionSMSChannel
	if err := ValidateBusinessExtensions(provider); err == nil {
		t.Fatal("non-product extension must not declare product/customer configuration")
	}
}

func TestManifestDecodersRejectUnknownExecutableConfigurationKeys(t *testing.T) {
	yamlContent := `id: phone-provider
name: Phone provider
version: v0.1.0
type: dynamic
business_extensions:
  - point: product-fulfillment
    version: v1
    role: provider
    owner: idc-core
    methods: [fulfill]
    customer_configuration:
      version: v1
      fields:
        - key: color
          type: single_line
          label: {en-US: Color, zh-CN: 颜色}
          script: "return true"
`
	directory := t.TempDir()
	manifestPath := filepath.Join(directory, "plugin.yaml")
	if err := os.WriteFile(manifestPath, []byte(yamlContent), 0o600); err != nil {
		t.Fatal(err)
	}
	service := &serviceImpl{}
	if err := service.LoadManifestFromYAML(manifestPath, &Manifest{}); err == nil || !strings.Contains(err.Error(), "unsupported fields") {
		t.Fatalf("unknown YAML script key must be rejected: %v", err)
	}

	jsonContent := []byte(`{"business_extensions":[{"point":"product-fulfillment","version":"v1","role":"provider","owner":"idc-core","methods":["fulfill"],"customerConfiguration":{"version":"v1","fields":[{"key":"color","type":"single_line","label":{"enUS":"Color","zhCN":"颜色"},"expression":"unsafe"}]}}]}`)
	if err := validateJSONBusinessExtensionSchema(jsonContent); err == nil {
		t.Fatal("unknown JSON expression key must be rejected")
	}
	snapshotContent := []byte(`authorizedBusinessExtensions:
  - point: product-fulfillment
    version: v1
    role: provider
    owner: idc-core
    customer_configuration:
      version: v1
      fields:
        - key: color
          type: single_line
          label: {en-US: Color, zh-CN: 颜色}
          script: unsafe
`)
	if err := ValidateYAMLBusinessExtensionSnapshotSchema(snapshotContent); err == nil {
		t.Fatal("unknown persisted snapshot script key must be rejected")
	}
}

func TestConfigurationCloneAndArtifactMatchCoverCompleteDeclaration(t *testing.T) {
	registerPermissiveOwnerValidator(t, "idc-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	manifest := validConfigurationProviderManifest()
	clone := CloneManifest(manifest)
	clone.BusinessExtensions[0].CustomerConfiguration.Fields[0].Label.ZhCN = "新颜色"
	clone.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedMIMETypes[0] = "image/jpeg"
	clone.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedExtensions[0] = ".jpg"
	if manifest.BusinessExtensions[0].CustomerConfiguration.Fields[0].Label.ZhCN == "新颜色" {
		t.Fatal("manifest clone must detach nested configuration declarations")
	}
	if manifest.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedMIMETypes[0] != "application/pdf" ||
		manifest.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedExtensions[0] != ".pdf" {
		t.Fatal("manifest clone must detach upload policy MIME types and extensions")
	}
	artifact := &ArtifactManifest{
		ID: manifest.ID, Dependencies: plugintypes.CloneDependencySpec(manifest.Dependencies),
		BusinessExtensions: CloneBusinessExtensionDeclarations(manifest.BusinessExtensions),
	}
	if !ManifestBusinessDeclarationsMatch(manifest, artifact) {
		t.Fatal("identical complete configuration declarations must match")
	}
	artifact.BusinessExtensions[0].CustomerConfiguration.Fields[0].Key = "different"
	if ManifestBusinessDeclarationsMatch(manifest, artifact) {
		t.Fatal("artifact comparison must cover nested configuration declarations")
	}
}

func validConfigurationProviderManifest() *Manifest {
	return &Manifest{
		ID: "phone-provider", Name: "Phone provider", Version: "v0.1.0", Type: "dynamic", Distribution: "managed",
		Dependencies:       &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{ID: "idc-core", Version: ">=0.1.0 <1.0.0"}}},
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{validConfigurationProviderDeclaration()},
	}
}

func validConfigurationProviderDeclaration() *capmodel.BusinessExtensionDeclaration {
	return &capmodel.BusinessExtensionDeclaration{
		Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1", Role: capmodel.BusinessExtensionRoleProvider, Owner: "idc-core",
		Methods:                           []string{"fulfill", "query", "candidates.list"},
		InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine, Label: bilingual("Endpoint", "接口地址"), TextFormat: capmodel.ConfigurationTextFormatURL, MaximumLength: 2048},
			{Key: "notes", Type: capmodel.ConfigurationFieldMultiLine, Label: bilingual("Notes", "备注"), MaximumLength: 4096},
			{Key: "api_secret", Type: capmodel.ConfigurationFieldSecret, Label: bilingual("API secret", "接口密钥"), Required: true, MaximumLength: 1024},
		}},
		ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "stock", Type: capmodel.ConfigurationFieldInteger, Label: bilingual("Stock", "库存"), Minimum: "0", Maximum: "999999"},
			{Key: "weight", Type: capmodel.ConfigurationFieldDecimal, Label: bilingual("Weight", "重量"), Minimum: "0", Maximum: "9999.99"},
			{Key: "enabled", Type: capmodel.ConfigurationFieldBoolean, Label: bilingual("Enabled", "启用")},
			{Key: "sale_date", Type: capmodel.ConfigurationFieldDate, Label: bilingual("Sale date", "销售日期")},
			{Key: "sale_time", Type: capmodel.ConfigurationFieldDateTime, Label: bilingual("Sale time", "销售时间")},
		}},
		CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{
			ID: "customer.identity.v1", MaximumFiles: 3, MaximumFileBytes: 20 * 1024 * 1024,
			MaximumTotalBytes: 60 * 1024 * 1024, AllowedMIMETypes: []string{"application/pdf", "image/jpeg", "image/png"},
			AllowedExtensions: []string{".pdf", ".jpg", ".jpeg", ".png"}, SafetyScanRequired: true,
		}}, Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "color", Type: capmodel.ConfigurationFieldSingleSelect, Label: bilingual("Color", "颜色"), StaticCandidates: candidates("black", "Black", "黑色", "white", "White", "白色")},
			{Key: "capacity", Type: capmodel.ConfigurationFieldMultiSelect, Label: bilingual("Capacity", "容量"), DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidates.list"}},
			{Key: "model", Type: capmodel.ConfigurationFieldTreeSelect, Label: bilingual("Model", "型号"), StaticCandidates: []*capmodel.ConfigurationCandidate{
				{Value: "phone", Label: bilingual("Phone", "手机")},
				{Value: "phone-pro", ParentValue: "phone", Label: bilingual("Phone Pro", "专业版手机")},
			}},
			{Key: "address", Type: capmodel.ConfigurationFieldAddress, Label: bilingual("Address", "收货地址"), StaticCandidates: []*capmodel.ConfigurationCandidate{
				{Value: "cn", Label: bilingual("China", "中国")},
				{Value: "cn-gd", ParentValue: "cn", Label: bilingual("Guangdong", "广东")},
			}},
			{Key: "delivery_date", Type: capmodel.ConfigurationFieldDate, Label: bilingual("Delivery date", "送货日期")},
			{Key: "identity_file", Type: capmodel.ConfigurationFieldFile, Label: bilingual("Identity file", "身份证明文件"), UploadPolicy: "customer.identity.v1"},
		}, Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionAnd, Children: []*capmodel.ConfigurationConditionDeclaration{
				{Operator: capmodel.ConfigurationConditionEqual, Field: "color", Value: "black"},
				{Operator: capmodel.ConfigurationConditionNotEmpty, Field: "address"},
			}},
			Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionRequire, Field: "identity_file"}},
		}}},
	}
}

func bilingual(enUS, zhCN string) *capmodel.LocalizedText {
	return &capmodel.LocalizedText{EnUS: enUS, ZhCN: zhCN}
}

func candidates(values ...string) []*capmodel.ConfigurationCandidate {
	out := make([]*capmodel.ConfigurationCandidate, 0, len(values)/3)
	for index := 0; index < len(values); index += 3 {
		out = append(out, &capmodel.ConfigurationCandidate{Value: values[index], Label: bilingual(values[index+1], values[index+2])})
	}
	return out
}
