// This file verifies the bounded structured-configuration compiler shared by
// interface, product-version, and customer purchase surfaces.

package productautomation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func compileConfiguration(
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (*canonicalConfiguration, error) {
	return configurationcontract.Compile(schema, values, surface, authorizedMethods)
}

type configurationSecretManager struct {
	mu                                     sync.Mutex
	created                                map[string]secretcap.CreateResult
	metadata                               map[string]secretcap.Metadata
	plaintext                              map[string][]byte
	createCalls, replaceCalls, revokeCalls int
	createFailureAt                        int
	createUnknownAt                        int
	recoverCalls                           int
	recoverFailure                         error
	revokeFailure                          error
	revokeFailures                         map[string]error
	revokedRefs                            []string
}

func newConfigurationSecretManager() *configurationSecretManager {
	return &configurationSecretManager{created: map[string]secretcap.CreateResult{}, metadata: map[string]secretcap.Metadata{}, plaintext: map[string][]byte{}, revokeFailures: map[string]error{}}
}
func (m *configurationSecretManager) Create(_ context.Context, key string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.created[key]; ok {
		existing.Reused = true
		return existing, nil
	}
	var plaintext []byte
	if err := produce(func(value []byte) error { plaintext = append([]byte(nil), value...); return nil }); err != nil {
		return secretcap.CreateResult{}, err
	}
	m.createCalls++
	if m.createFailureAt > 0 && m.createCalls == m.createFailureAt {
		return secretcap.CreateResult{}, errors.New("injected secret create failure")
	}
	handle, err := secretcap.NewHandle(fmt.Sprintf("secret-config-%d", m.createCalls))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	tail := string(plaintext)
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	m.plaintext[handle.OpaqueRef()] = append([]byte(nil), plaintext...)
	clear(plaintext)
	metadata := secretcap.Metadata{Configured: true, MaskedTail: tail, UpdatedAt: time.Unix(int64(m.createCalls), 0).UTC()}
	result := secretcap.CreateResult{Handle: handle, Metadata: metadata}
	m.created[key], m.metadata[handle.OpaqueRef()] = result, metadata
	if m.createUnknownAt > 0 && m.createCalls == m.createUnknownAt {
		return secretcap.CreateResult{}, errors.New("injected unknown secret create result")
	}
	return result, nil
}
func (m *configurationSecretManager) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	m.mu.Lock()
	value := append([]byte(nil), m.plaintext[request.Handle.OpaqueRef()]...)
	m.mu.Unlock()
	if len(value) == 0 || consume == nil {
		return errors.New("not found")
	}
	defer clear(value)
	return consume(value)
}
func (m *configurationSecretManager) RecoverCreate(_ context.Context, key string) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recoverCalls++
	if m.recoverFailure != nil {
		return secretcap.CreateResult{}, m.recoverFailure
	}
	value, ok := m.created[key]
	if !ok {
		return secretcap.CreateResult{}, errors.New("not found")
	}
	value.Reused = true
	return value, nil
}
func (m *configurationSecretManager) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.replaceCalls++
	return secretcap.MutationResult{}, errors.New("historical handle replacement is forbidden")
}
func (m *configurationSecretManager) Metadata(_ context.Context, handle secretcap.Handle) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.metadata[handle.OpaqueRef()]
	if !ok {
		return secretcap.Metadata{}, errors.New("not found")
	}
	return value, nil
}
func (m *configurationSecretManager) Revoke(_ context.Context, handle secretcap.Handle, _ time.Time) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeCalls++
	m.revokedRefs = append(m.revokedRefs, handle.OpaqueRef())
	if m.revokeFailure != nil {
		return secretcap.Metadata{}, m.revokeFailure
	}
	if err := m.revokeFailures[handle.OpaqueRef()]; err != nil {
		return secretcap.Metadata{}, err
	}
	value := m.metadata[handle.OpaqueRef()]
	value.Configured = false
	m.metadata[handle.OpaqueRef()] = value
	return value, nil
}
func (*configurationSecretManager) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func TestCreateConfigurationSecretsRecoversUnknownCreateResult(t *testing.T) {
	first := secretOperationIdempotencyKey("pso_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "api_secret")
	second := secretOperationIdempotencyKey("pso_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "api_secret")
	if first == second || first != "pso_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:api_secret" {
		t.Fatalf("secret operation key is not stable and operation scoped: first=%q second=%q", first, second)
	}
}

func TestUpdateSecretRecoveryIsCompensatedWhenLaterSecretFails(t *testing.T) {
	statusA := secretConfiguredStatusDigest(map[string]bool{"api_secret": true})
	statusB := secretConfiguredStatusDigest(map[string]bool{"api_secret": false})
	if statusA == statusB {
		t.Fatal("safe secret status digest did not distinguish configured state")
	}
}

func TestConfigurationAuditKeysDistinguishSecretRotationValueRoundTripAndMerchantValues(t *testing.T) {
	interfaceKeys := []string{
		interfaceConfigurationAuditKey("pfi_test", 2, "declaration", "value-a"),
		interfaceConfigurationAuditKey("pfi_test", 3, "declaration", "value-b"),
		interfaceConfigurationAuditKey("pfi_test", 4, "declaration", "value-a"),
	}
	if interfaceKeys[0] == interfaceKeys[1] || interfaceKeys[0] == interfaceKeys[2] || interfaceKeys[1] == interfaceKeys[2] {
		t.Fatalf("interface secret/value revisions reused an audit identity: %v", interfaceKeys)
	}
	versionA := versionConfigurationAuditKey("ver_test", 1, 7, "same-declaration", "merchant-a")
	versionB := versionConfigurationAuditKey("ver_test", 2, 7, "same-declaration", "merchant-b")
	if versionA == versionB {
		t.Fatal("same declaration with different merchant values reused an audit identity")
	}
}

func TestCompileConfigurationAcceptsAllGovernedFieldTypes(t *testing.T) {
	t.Parallel()
	text := func(value string) *string { return &value }
	integer := func(value int64) *int64 { return &value }
	boolean := func(value bool) *bool { return &value }
	label := func(enUS, zhCN string) *capmodel.LocalizedText {
		return &capmodel.LocalizedText{EnUS: enUS, ZhCN: zhCN}
	}
	candidates := []*capmodel.ConfigurationCandidate{
		{Value: "one", Label: label("One", "一")},
		{Value: "two", Label: label("Two", "二")},
	}
	fields := []*capmodel.ConfigurationFieldDeclaration{
		{Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: label("Name", "名称"), Required: true},
		{Key: "notes", Type: capmodel.ConfigurationFieldMultiLine, Label: label("Notes", "说明")},
		{Key: "count", Type: capmodel.ConfigurationFieldInteger, Label: label("Count", "数量"), Minimum: "1", Maximum: "20"},
		{Key: "ratio", Type: capmodel.ConfigurationFieldDecimal, Label: label("Ratio", "比例"), Minimum: "0.1", Maximum: "2.5"},
		{Key: "enabled", Type: capmodel.ConfigurationFieldBoolean, Label: label("Enabled", "启用")},
		{Key: "choice", Type: capmodel.ConfigurationFieldSingleSelect, Label: label("Choice", "单选"), StaticCandidates: candidates},
		{Key: "choices", Type: capmodel.ConfigurationFieldMultiSelect, Label: label("Choices", "多选"), StaticCandidates: candidates},
		{Key: "start_date", Type: capmodel.ConfigurationFieldDate, Label: label("Date", "日期")},
		{Key: "start_time", Type: capmodel.ConfigurationFieldDateTime, Label: label("Time", "时间")},
		{Key: "credential", Type: capmodel.ConfigurationFieldSecret, Label: label("Credential", "凭据")},
		{Key: "tree", Type: capmodel.ConfigurationFieldTreeSelect, Label: label("Tree", "树选择"), StaticCandidates: candidates},
		{Key: "address", Type: capmodel.ConfigurationFieldAddress, Label: label("Address", "地址"), StaticCandidates: candidates},
		{Key: "attachment", Type: capmodel.ConfigurationFieldFile, Label: label("File", "文件"), UploadPolicy: "document-v1"},
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields:  fields,
		UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{
			ID: "document-v1", MaximumFiles: 2, MaximumFileBytes: 5 * 1024 * 1024,
			MaximumTotalBytes: 10 * 1024 * 1024, AllowedMIMETypes: []string{"application/pdf"},
			AllowedExtensions: []string{".pdf"}, SafetyScanRequired: true,
		}},
		Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionAnd, Children: []*capmodel.ConfigurationConditionDeclaration{
				{Operator: capmodel.ConfigurationConditionEqual, Field: "enabled", Value: "true"},
				{Operator: capmodel.ConfigurationConditionGreaterOrEqual, Field: "count", Value: "1"},
			}},
			Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionShow, Field: "notes"}},
		}},
	}
	values := []ConfigurationValue{
		{FieldKey: "name", TextValue: text("Example")},
		{FieldKey: "notes", TextValue: text("two\nlines")},
		{FieldKey: "count", IntegerValue: integer(2)},
		{FieldKey: "ratio", DecimalValue: text("1.25")},
		{FieldKey: "enabled", BooleanValue: boolean(true)},
		{FieldKey: "choice", OptionTokens: []string{"one"}},
		{FieldKey: "choices", OptionTokens: []string{"two", "one"}},
		{FieldKey: "start_date", DateValue: text("2026-08-26")},
		{FieldKey: "start_time", DateTimeValue: text("2026-08-26T12:00:00+08:00")},
		{FieldKey: "credential", TextValue: text("do-not-persist")},
		{FieldKey: "tree", TreeOptionTokens: []string{"one"}},
		{FieldKey: "address", AddressOptionTokens: []string{"one"}},
	}
	compiled, err := compileConfiguration(schema, values, configurationSurfaceInterface, []string{"candidates.address", "candidates.tree"})
	if err != nil {
		t.Fatalf("compile all governed field types: %v", err)
	}
	if compiled.SecretValues["credential"] != "do-not-persist" {
		t.Fatalf("secret was not separated for secretcap: %#v", compiled.SecretValues)
	}
	if strings.Contains(string(compiled.ValuesJSON), "do-not-persist") || strings.Contains(string(compiled.DeclarationJSON), "do-not-persist") {
		t.Fatal("plaintext secret leaked into persistence documents")
	}
	if !strings.Contains(string(compiled.ValuesJSON), `"choices":["one","two"]`) {
		t.Fatalf("multi-select values were not canonicalized: %s", compiled.ValuesJSON)
	}
}

func TestConfigurationSnapshotDecodingRejectsUnknownAndTrailingData(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{
		`{"version":"v1","unknown":true}`,
		`{"version":"v1"} {"version":"v2"}`,
	} {
		if _, err := decodeConfigurationDeclaration(payload); err == nil {
			t.Fatalf("unsafe declaration snapshot was accepted: %s", payload)
		}
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{EnUS: "Name", ZhCN: "名称"}}}}
	for _, payload := range []string{`{"unknown":"value"}`, `{"name":"one"} {"name":"two"}`} {
		if _, err := configurationValuesFromJSON(schema, []byte(payload)); err == nil {
			t.Fatalf("unsafe value snapshot was accepted: %s", payload)
		}
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decodeConfigurationDeclaration(string(encoded)); err != nil {
		t.Fatalf("valid declaration snapshot was rejected: %v", err)
	}
}

func TestAddressConfigurationAcceptsOnlyBoundedStaticParentPath(t *testing.T) {
	t.Parallel()
	label := func(value string) *capmodel.LocalizedText { return &capmodel.LocalizedText{EnUS: value, ZhCN: value} }
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "delivery_address", Type: capmodel.ConfigurationFieldAddress, Label: label("Address"),
		StaticCandidates: []*capmodel.ConfigurationCandidate{
			{Value: "country-cn", Label: label("China")},
			{Value: "province-gd", ParentValue: "country-cn", Label: label("Guangdong")},
			{Value: "city-sz", ParentValue: "province-gd", Label: label("Shenzhen")},
		},
	}}}
	valid := []ConfigurationValue{{FieldKey: "delivery_address", AddressOptionTokens: []string{"country-cn", "province-gd", "city-sz"}}}
	if _, err := compileConfiguration(schema, valid, configurationSurfaceCustomer, nil); err != nil {
		t.Fatalf("valid linked address path was rejected: %v", err)
	}
	invalid := []ConfigurationValue{{FieldKey: "delivery_address", AddressOptionTokens: []string{"country-cn", "city-sz"}}}
	if _, err := compileConfiguration(schema, invalid, configurationSurfaceCustomer, nil); err == nil {
		t.Fatal("address path that skipped its declared parent was accepted")
	}
}

func TestCompileConfigurationRejectsUnsafeOrAmbiguousContracts(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Field", ZhCN: "字段"}
	tests := []struct {
		name    string
		schema  *capmodel.ConfigurationSchemaDeclaration
		values  []ConfigurationValue
		surface ConfigurationSurface
		methods []string
	}{
		{name: "secret outside interface", surface: configurationSurfaceCustomer, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "password", Type: capmodel.ConfigurationFieldSecret, Label: label}}}},
		{name: "untranslated label", surface: configurationSurfaceProduct, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{ZhCN: "名称"}}}}},
		{name: "unapproved dynamic method", surface: configurationSurfaceCustomer, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "region", Type: capmodel.ConfigurationFieldTreeSelect, Label: label, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.region"}}}}},
		{name: "numeric condition on text", surface: configurationSurfaceCustomer, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: label}}, Conditions: []*capmodel.ConfigurationRuleDeclaration{{When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionGreaterThan, Field: "name", Value: "1"}, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionShow, Field: "name"}}}}}},
		{name: "two value arms", surface: configurationSurfaceProduct, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "count", Type: capmodel.ConfigurationFieldInteger, Label: label}}}, values: []ConfigurationValue{{FieldKey: "count", IntegerValue: pointerInt64(1), TextValue: pointerString("1")}}},
		{name: "unknown candidate", surface: configurationSurfaceCustomer, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "plan", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "a", Label: label}}}}}, values: []ConfigurationValue{{FieldKey: "plan", OptionTokens: []string{"b"}}}},
		{name: "unresolved dynamic candidate token", surface: configurationSurfaceCustomer, methods: []string{"candidate.region"}, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "region", Type: capmodel.ConfigurationFieldTreeSelect, Label: label, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.region"}}}}, values: []ConfigurationValue{{FieldKey: "region", TreeOptionTokens: []string{"caller-token"}}}},
		{name: "unverified file token", surface: configurationSurfaceCustomer, schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "document", Type: capmodel.ConfigurationFieldFile, Label: label, UploadPolicy: "document-v1"}}}, values: []ConfigurationValue{{FieldKey: "document", FileTokens: []string{"caller-file"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := compileConfiguration(test.schema, test.values, test.surface, test.methods); err == nil {
				t.Fatal("unsafe configuration unexpectedly compiled")
			}
		})
	}
}

func TestProviderEvidenceValidationCoversAllThreeConfigurationSurfaces(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Credential", ZhCN: "凭据"}
	evidence := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "ssl-provider", DisplayName: "SSL Provider", ReleaseVersion: "v1.0.0",
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
		AuthorizedMethods: []string{"provision", "query.status"}, ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		ProviderMetadata: testProviderMetadata(),
		CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "credential", Type: capmodel.ConfigurationFieldSecret, Label: label,
		}}},
	}
	if validProviderEvidence(evidence) {
		t.Fatal("customer secret field unexpectedly passed provider admission")
	}
	evidence.CustomerConfiguration = nil
	evidence.InterfaceConfigurationCardinality = capmodel.InterfaceConfigurationCardinalityMultiple
	evidence.InterfaceConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "credential", Type: capmodel.ConfigurationFieldSecret, Label: label,
	}}}
	if !validProviderEvidence(evidence) {
		t.Fatal("interface secret field should pass provider admission")
	}
}

func pointerString(value string) *string { return &value }
func pointerInt64(value int64) *int64    { return &value }
