package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/providerexecution"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type frozenCustomerFileVerifierStub struct {
	purchaseconfigurationfile.Service
	inputs []purchaseconfigurationfile.VerifyInput
	owner  purchaseconfigurationfile.OrderEvidenceOwner
	err    error
}

func (s *frozenCustomerFileVerifierStub) VerifyOrderEvidence(_ context.Context, owner purchaseconfigurationfile.OrderEvidenceOwner, inputs []purchaseconfigurationfile.VerifyInput) ([]purchaseconfigurationfile.VerifiedField, error) {
	s.owner = owner
	s.inputs = inputs
	if s.err != nil {
		return nil, s.err
	}
	result := make([]purchaseconfigurationfile.VerifiedField, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, purchaseconfigurationfile.VerifiedField{
			Target: input.Target,
			Files:  []purchaseconfigurationfile.VerifiedFile{{FileToken: input.FileTokens[0], SHA256Hex: strings.Repeat("a", 64)}},
		})
	}
	return result, nil
}

func (s *frozenCustomerFileVerifierStub) VerifyOrderEvidenceBatch(ctx context.Context, inputs []purchaseconfigurationfile.OrderEvidenceBatchInput) ([][]purchaseconfigurationfile.VerifiedField, error) {
	result := make([][]purchaseconfigurationfile.VerifiedField, len(inputs))
	for index, input := range inputs {
		verified, err := s.VerifyOrderEvidence(ctx, input.Owner, input.Fields)
		if err != nil {
			return nil, err
		}
		result[index] = verified
	}
	return result, nil
}

func TestInterfaceConfigurationRevisionSecretsAreCompensatable(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	schemaName := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schemaName)); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`)
	database := openAutomationDatabase(t, ctx, dsn, schemaName)
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version (tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,status_code VARCHAR(16) NOT NULL,source_code VARCHAR(16) NOT NULL DEFAULT 'manual',UNIQUE(tenant_id,version_no));
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_billing_rule (
    tenant_id INT NOT NULL DEFAULT 0,
    version_no VARCHAR(40) NOT NULL,
    charge_type_code VARCHAR(16) NOT NULL,
    status_code VARCHAR(16) NOT NULL
);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy (
 tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,
 strategy_code VARCHAR(32) NOT NULL,granularity_code VARCHAR(32) NOT NULL,
 ongoing_management BOOLEAN NOT NULL DEFAULT FALSE,
 renewal_mode_code VARCHAR(32) NOT NULL DEFAULT 'unavailable',renewal_method VARCHAR(128) NOT NULL DEFAULT '',entitlement_owner_id VARCHAR(64) NOT NULL DEFAULT '',
 provision_method VARCHAR(128) NOT NULL DEFAULT '',query_method VARCHAR(128) NOT NULL DEFAULT '',
 maximum_attempts INT NOT NULL,retry_base_seconds INT NOT NULL,policy_digest CHAR(64) NOT NULL,
 created_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES (0,'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','draft');`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	for _, migrationName := range []string{"074-product-automation-binding.sql", "075-structured-product-configuration.sql", "134-local-purchase-controls.sql"} {
		migration, readErr := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", migrationName))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if migrationName == "075-structured-product-configuration.sql" {
			if sidecar := bytes.Index(migration, []byte("-- Generic transaction evidence is a sidecar")); sidecar >= 0 {
				migration = migration[:sidecar]
			}
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	installResourceProjectionTestColumns(t, ctx, database)
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision ADD COLUMN resource_owner_configuration_revision VARCHAR(40) NOT NULL DEFAULT ''`); err != nil {
		t.Fatal(err)
	}
	deleteMigration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "128-interface-delete-session-operations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(deleteMigration)); err != nil {
		t.Fatal(err)
	}
	label := &capmodel.LocalizedText{EnUS: "API secret", ZhCN: "接口密钥"}
	evidence := automationEvidence([]string{"candidate.product", productfulfillmentcap.MethodValidateProductConfiguration, "provision", "query.status"})
	evidence.InterfaceConfigurationCardinality = capmodel.InterfaceConfigurationCardinalityMultiple
	evidence.InterfaceConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "api_secret", Type: capmodel.ConfigurationFieldSecret, Label: label},
		{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{EnUS: "Endpoint", ZhCN: "接口地址"}},
	}}
	evidence.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "certificate_profile", Type: capmodel.ConfigurationFieldSingleSelect,
		Label:             &capmodel.LocalizedText{EnUS: "Certificate profile", ZhCN: "证书方案"},
		DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.product"},
	}}}
	evidence.CustomerConfiguration = &capmodel.ConfigurationSchemaDeclaration{
		Version: "v1",
		Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "cores", Type: capmodel.ConfigurationFieldInteger, Minimum: "1", Maximum: "8", Label: &capmodel.LocalizedText{EnUS: "CPU cores", ZhCN: "处理器核心数"}},
			{Key: "region", Type: capmodel.ConfigurationFieldSingleSelect, Label: &capmodel.LocalizedText{EnUS: "Region", ZhCN: "地区"}, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.product"}},
			{Key: "domain", Type: capmodel.ConfigurationFieldSingleLine, TextFormat: capmodel.ConfigurationTextFormatDomain, Required: true, Label: &capmodel.LocalizedText{EnUS: "Domain", ZhCN: "域名"}},
			{Key: "csr", Type: capmodel.ConfigurationFieldFile, Required: true, UploadPolicy: "csr-v1", Label: &capmodel.LocalizedText{EnUS: "CSR", ZhCN: "申请文件"}},
		},
		UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{
			ID: "csr-v1", MaximumFiles: 1, MaximumFileBytes: 1024 * 1024,
			MaximumTotalBytes: 1024 * 1024, AllowedMIMETypes: []string{"application/pkcs10"},
			AllowedExtensions: []string{".csr"}, SafetyScanRequired: true,
		}},
	}
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}}
	var (
		providerConfigurationValid  = true
		providerConfigurationReason productfulfillmentcap.ConfigurationValidationReason
		validationCalls             int
		productCandidateCalls       int
	)
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		if input.Method == productfulfillmentcap.MethodValidateProductConfiguration {
			validationCalls++
			payload, marshalErr := json.Marshal(productfulfillmentcap.ConfigurationValidationResult{Version: productfulfillmentcap.ContractVersion, Valid: providerConfigurationValid, Reason: providerConfigurationReason})
			return &capmodel.BusinessExtensionMethodResult{Version: productfulfillmentcap.ContractVersion, Payload: payload}, marshalErr
		}
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			return nil, err
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(request.Requests))
		for _, query := range request.Requests {
			if query.FieldKey == "certificate_profile" {
				productCandidateCalls++
			}
			items = append(items, configurationcontract.DynamicCandidateItem{FieldKey: query.FieldKey, Candidates: []*capmodel.ConfigurationCandidate{{
				Value: "ssl-basic", Label: &capmodel.LocalizedText{EnUS: "Basic SSL", ZhCN: "基础证书"},
			}}})
		}
		payload, err := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, err
	}
	authorizer := &automationTestAuthorizer{}
	audit := &automationTestAudit{}
	codec := &automationTestCodec{evidence: evidence, identityOnly: true}
	randomInput := bytes.NewReader(append(bytes.Repeat([]byte{0x4a}, 16), bytes.Repeat([]byte{0x4b}, 240)...))
	service, err := newWithRandom(database, extensions, authorizer, audit, codec, randomInput)
	if err != nil {
		t.Fatal(err)
	}
	service.runtimeContexts = newSupplierRuntimeContextStore(rand.Reader)
	secrets := newConfigurationSecretManager()
	service.secrets = secrets
	service.secretAccessor = secrets
	endpointA := "https://supplier-a.example"
	created, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "SSL 供应商 A", SelectionToken: "opaque", Configuration: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointA}}})
	if err != nil {
		t.Fatalf("create configured interface: %+v", err)
	}
	createdRevision, revisionErr := service.loadLatestInterfaceConfigurationRevision(ctx, created.InterfaceInstanceNo, false)
	if revisionErr != nil || createdRevision == nil {
		t.Fatalf("load created interface configuration revision: value=%+v err=%v", createdRevision, revisionErr)
	}
	if validationErr := service.validateInterfaceConfigurationRevision(ctx, created.InterfaceInstanceNo, evidence, createdRevision, nil); validationErr != nil {
		t.Fatalf("created interface configuration is not self-consistent: row=%+v declaration=%s values=%s err=%v", createdRevision, createdRevision.DeclarationSnapshot, createdRevision.PublicValueSnapshot, validationErr)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InterfaceInstanceNo: created.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	unsaved, err := service.GetVersionConfiguration(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil || unsaved == nil || unsaved.Version != 0 || unsaved.ProductDeclaration == nil || unsaved.CustomerDeclaration == nil || len(unsaved.MerchantValues) != 0 {
		t.Fatalf("bound unsaved version did not expose exact empty configuration: value=%+v err=%v", unsaved, err)
	}
	serializedPresentation, err := json.Marshal(unsaved.ProductDeclaration)
	if err != nil || bytes.Contains(serializedPresentation, []byte("candidate.product")) ||
		unsaved.ProductDeclaration.Fields[0].DynamicCandidates != nil ||
		len(unsaved.ProductResolvedCandidates["certificate_profile"]) != 1 ||
		unsaved.ProductResolvedCandidates["certificate_profile"][0].Label.ZhCN != "基础证书" {
		t.Fatalf("management read leaked a dynamic method or omitted safe bilingual candidates: declaration=%s candidates=%#v err=%v", serializedPresentation, unsaved.ProductResolvedCandidates, err)
	}
	evaluatedVersion, err := service.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{
		VersionNo:       "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MerchantValues:  []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"ssl-basic"}}},
		ExpectedVersion: 0,
	})
	if err != nil || !evaluatedVersion.Valid || evaluatedVersion.Revision != 0 ||
		len(evaluatedVersion.Candidates["certificate_profile"]) != 1 {
		t.Fatalf("side-effect-free product configuration evaluation failed: value=%+v err=%v", evaluatedVersion, err)
	}
	invalidEvaluation, err := service.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{
		VersionNo:       "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MerchantValues:  []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"forged-option"}}},
		ExpectedVersion: 0,
	})
	if err != nil || invalidEvaluation.Valid || len(invalidEvaluation.Issues) != 1 ||
		invalidEvaluation.Issues[0].Code != configurationcontract.IssueInvalidValue {
		t.Fatalf("invalid dynamic selection did not return one stable field issue: value=%+v err=%v", invalidEvaluation, err)
	}
	evidence.ProductConfiguration.Fields[0].Required = true
	emptyEvaluation, err := service.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MerchantValues: []ConfigurationValue{}, ExpectedVersion: 0,
	})
	if err != nil || emptyEvaluation.Valid {
		t.Fatalf("empty array bypassed declared required merchant field: %+v %v", emptyEvaluation, err)
	}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MerchantValues: []ConfigurationValue{}, ExpectedVersion: 0}); err == nil {
		t.Fatal("empty array saved despite declared required merchant field")
	}
	evidence.ProductConfiguration.Fields[0].Required = false
	evaluatedInterface, err := service.EvaluateInterfaceConfiguration(ctx, EvaluateInterfaceConfigurationInput{
		InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 1,
	})
	if err != nil || !evaluatedInterface.Valid || evaluatedInterface.Revision != 1 {
		t.Fatalf("side-effect-free interface configuration evaluation failed: value=%+v err=%v", evaluatedInterface, err)
	}
	firstValue := "first-secret"
	first, err := service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 1, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &firstValue}, {FieldKey: "endpoint", TextValue: &endpointA}}})
	if err != nil || first.Revision != 2 {
		t.Fatalf("first secret revision: value=%+v err=%v", first, err)
	}
	firstRefValue, err := database.GetValue(ctx, `SELECT secret_ref FROM plugin_idc_core_product_automation_interface_secret_reference r JOIN plugin_idc_core_product_automation_interface_configuration_revision c ON c.id=r.configuration_revision_id WHERE c.interface_instance_no=? AND c.revision=2`, created.InterfaceInstanceNo)
	firstRef := firstRefValue.String()
	if err != nil || firstRef == "" {
		t.Fatalf("load first secret reference: %q %v", firstRef, err)
	}
	secondValue := "second-secret"
	second, err := service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 2, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &secondValue}, {FieldKey: "endpoint", TextValue: &endpointA}}})
	if err != nil || second.Revision != 3 {
		t.Fatalf("second secret revision: value=%+v err=%v", second, err)
	}
	var refRows []struct {
		SecretRef string `orm:"secret_ref"`
	}
	if err = database.GetScan(ctx, &refRows, `SELECT r.secret_ref FROM plugin_idc_core_product_automation_interface_secret_reference r JOIN plugin_idc_core_product_automation_interface_configuration_revision c ON c.id=r.configuration_revision_id WHERE c.interface_instance_no=? ORDER BY c.revision`, created.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	refs := make([]string, 0, len(refRows))
	for _, row := range refRows {
		refs = append(refs, row.SecretRef)
	}
	if len(refs) != 2 || refs[0] != firstRef || refs[0] == refs[1] || secrets.replaceCalls != 0 {
		t.Fatalf("historical secret was replaced: refs=%v replaceCalls=%d", refs, secrets.replaceCalls)
	}
	transitions := audit.snapshotTransitions()
	lastInterfaceUpdate := transitions[len(transitions)-1]
	beforeInterface, beforeOK := lastInterfaceUpdate.BeforeSnapshot.(map[string]any)
	afterInterface, afterOK := lastInterfaceUpdate.AfterSnapshot.(map[string]any)
	if !beforeOK || !afterOK || beforeInterface["secret_status_digest"] == "" || afterInterface["secret_status_digest"] == "" {
		t.Fatalf("interface secret status audit evidence is incomplete: before=%#v after=%#v", lastInterfaceUpdate.BeforeSnapshot, lastInterfaceUpdate.AfterSnapshot)
	}
	replacedFields, fieldsOK := afterInterface["replaced_secret_fields"].([]string)
	if !fieldsOK || len(replacedFields) != 1 || replacedFields[0] != "api_secret" {
		t.Fatalf("interface secret replacement audit evidence is incomplete: %#v", afterInterface)
	}
	secrets.createUnknownAt = secrets.createCalls + 1
	audit.err = errors.New("injected audit transaction failure")
	rollbackSecret := "rollback-secret"
	revokesBeforeRollback := secrets.revokeCalls
	if _, updateErr := service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 3, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &rollbackSecret}, {FieldKey: "endpoint", TextValue: &endpointA}}}); updateErr == nil {
		t.Fatal("audit transaction failure did not roll back interface revision")
	}
	audit.err = nil
	secrets.createUnknownAt = 0
	latestAfterRollback, rollbackErr := service.loadLatestInterfaceConfigurationRevision(ctx, created.InterfaceInstanceNo, false)
	if rollbackErr != nil || latestAfterRollback == nil || latestAfterRollback.Revision != 3 || secrets.revokeCalls != revokesBeforeRollback+1 {
		t.Fatalf("unknown recovered secret was not compensated after transaction rollback: revision=%#v revoke=%d err=%v", latestAfterRollback, secrets.revokeCalls, rollbackErr)
	}
	endpointB := "https://supplier-b.example"
	supplierBSecret := "supplier-b-secret"
	secrets.createUnknownAt = secrets.createCalls + 1
	createdB, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "SSL 供应商 B", SelectionToken: "opaque", Configuration: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointB}, {FieldKey: "api_secret", TextValue: &supplierBSecret}}})
	secrets.createUnknownAt = 0
	if err != nil {
		t.Fatalf("create supplier B from recovered unknown secret result: %v", err)
	}
	runtimeInvoker := NewRuntimeCapabilityInvoker()
	if err = runtimeInvoker.Bind(service); err != nil {
		t.Fatal(err)
	}
	assertRuntimeEndpoint := func(instanceNo string, revision int64, expected, expectedSecret string) {
		t.Helper()
		scope := dynamicCandidateScope{interfaceInstanceNo: instanceNo, configurationRevision: revision}
		runtimeView, loadErr := service.loadRuntimeConfigurationView(ctx, evidence, scope)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		issued, issueErr := service.runtimeContexts.issue(evidence, scope, capmodel.BusinessExtensionInvocationPurposeNewBusiness, "candidate.product", runtimeView)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		defer service.runtimeContexts.remove(issued.id)
		result, invokeErr := runtimeInvoker.Invoke(ctx, capregistry.Invocation{CallerPluginID: evidence.PluginID, OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodConfigurationGet, ResourceRef: issued.id, Payload: []byte("{}")})
		if invokeErr != nil {
			t.Fatalf("read runtime configuration: %v", invokeErr)
		}
		if bytes.Contains(result.Payload, []byte("api_secret")) || bytes.Contains(result.Payload, []byte("configuration-secret-preserved-by-owner")) || bytes.Contains(result.Payload, []byte("secret_ref")) {
			t.Fatalf("runtime configuration leaked secret evidence: %s", result.Payload)
		}
		var view supplierinterfacecap.ConfigurationView
		if decodeErr := json.Unmarshal(result.Payload, &view); decodeErr != nil || len(view.Values) != 1 || view.Values[0].TextValue == nil || *view.Values[0].TextValue != expected {
			t.Fatalf("runtime endpoint=%s view=%#v err=%v", expected, view, decodeErr)
		}
		if _, invokeErr = runtimeInvoker.Invoke(ctx, capregistry.Invocation{CallerPluginID: "other-provider", OwnerPluginID: supplierinterfacecap.OwnerPluginID, Service: supplierinterfacecap.ServiceName, Version: supplierinterfacecap.ContractVersion, Method: supplierinterfacecap.MethodConfigurationGet, ResourceRef: issued.id, Payload: []byte("{}")}); invokeErr == nil {
			t.Fatal("another provider reused the opaque runtime context")
		}
		sourceClient, sourceErr := supplierinterfacecap.NewSourceClient(runtimeInvoker, runtimeInvoker)
		if sourceErr != nil {
			t.Fatal(sourceErr)
		}
		declarations := pluginhost.NewDeclarations(evidence.PluginID)
		if sourceErr = declarations.Providers().ProvideBusinessExtensionMethods(func(sourceCtx context.Context, _ *capmodel.BusinessExtensionMethodInvocation) (*capmodel.BusinessExtensionMethodResult, error) {
			accessErr := sourceClient.AccessSecret(sourceCtx, issued.id, "api_secret", func(value []byte) error {
				if string(value) != expectedSecret {
					t.Fatalf("runtime secret does not match exact A/B revision")
				}
				return nil
			})
			return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1}, accessErr
		}); sourceErr != nil {
			t.Fatal(sourceErr)
		}
		if _, sourceErr = declarations.(pluginhost.SourcePluginDefinition).GetBusinessExtensionMethodHandler()(ctx, &capmodel.BusinessExtensionMethodInvocation{}); sourceErr != nil {
			t.Fatalf("source callback secret access: %v", sourceErr)
		}
		if sourceErr = sourceClient.AccessSecret(ctx, issued.id, "api_secret", func([]byte) error { return nil }); sourceErr == nil {
			t.Fatal("ordinary context accessed a supplier secret")
		}
		if _, sourceErr = declarations.(pluginhost.SourcePluginDefinition).GetBusinessExtensionMethodHandler()(ctx, &capmodel.BusinessExtensionMethodInvocation{}); sourceErr != nil {
			t.Fatalf("source callback repeat: %v", sourceErr)
		}
	}
	assertRuntimeEndpoint(created.InterfaceInstanceNo, 3, endpointA, secondValue)
	assertRuntimeEndpoint(createdB.InterfaceInstanceNo, 1, endpointB, supplierBSecret)
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || ready.Passed {
		t.Fatalf("bound version without a frozen product/customer configuration passed publication: ready=%v err=%v", ready, readinessErr)
	}
	publicationActionCalled := false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(context.Context) error {
		publicationActionCalled = true
		return nil
	}); publicationErr == nil || publicationActionCalled {
		t.Fatalf("final publication crossed a missing configuration gate: called=%v err=%v", publicationActionCalled, publicationErr)
	}
	providerConfigurationValid = false
	if rejected, evaluationErr := service.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 0,
		MerchantValues: []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"ssl-basic"}}},
	}); evaluationErr == nil || rejected != nil {
		t.Fatalf("provider-rejected configuration was shown as valid: result=%+v err=%v", rejected, evaluationErr)
	}
	if rejectedErr := service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 0,
		MerchantValues: []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"ssl-basic"}}},
	}); rejectedErr == nil {
		t.Fatal("provider-rejected merchant configuration was saved")
	}
	if count, countErr := database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product_version_configuration`); countErr != nil || count != 0 {
		t.Fatalf("rejected merchant configuration left persisted data: count=%d err=%v", count, countErr)
	}
	providerConfigurationValid = true
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{
		VersionNo:       "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MerchantValues:  []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"ssl-basic"}}},
		ExpectedVersion: 0,
	}); err != nil {
		t.Fatalf("save frozen version configuration: %v", err)
	}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{
		VersionNo:       "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MerchantValues:  []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"ssl-basic"}}},
		ExpectedVersion: 1,
	}); err != nil {
		t.Fatalf("update frozen version configuration: %v", err)
	}
	t.Run("compatible optional login switches evaluate against current declaration", func(t *testing.T) {
		previousFields := evidence.ProductConfiguration.Fields
		evidence.ProductConfiguration.Fields = append(append([]*capmodel.ConfigurationFieldDeclaration{}, previousFields...), &capmodel.ConfigurationFieldDeclaration{Key: "hostname_customer_input", Type: capmodel.ConfigurationFieldBoolean, Label: &capmodel.LocalizedText{EnUS: "Allow customer hostname", ZhCN: "允许客户填写主机名"}}, &capmodel.ConfigurationFieldDeclaration{Key: "password_customer_input", Type: capmodel.ConfigurationFieldBoolean, Label: &capmodel.LocalizedText{EnUS: "Allow customer password", ZhCN: "允许客户填写主机密码"}})
		defer func() { evidence.ProductConfiguration.Fields = previousFields }()
		enabled := true
		evaluated, evaluationErr := service.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 2, MerchantValues: []ConfigurationValue{{FieldKey: "certificate_profile", OptionTokens: []string{"ssl-basic"}}, {FieldKey: "hostname_customer_input", BooleanValue: &enabled}, {FieldKey: "password_customer_input", BooleanValue: &enabled}}})
		if evaluationErr != nil || !evaluated.Valid {
			t.Fatalf("new compatible optional switches cannot evaluate: %v", evaluationErr)
		}
		found := 0
		for _, value := range evaluated.Values {
			if strings.HasSuffix(value.FieldKey, "_customer_input") && value.BooleanValue != nil && *value.BooleanValue {
				found++
			}
		}
		if found != 2 {
			t.Fatal("evaluation lost customer input switches")
		}
		var stored struct {
			Version     int64
			Declaration string
		}
		if loadErr := database.GetScan(ctx, &stored, `SELECT version,product_declaration_snapshot::text AS declaration FROM plugin_idc_core_catalog_product_version_configuration WHERE version_no=?`, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); loadErr != nil {
			t.Fatal(loadErr)
		}
		if stored.Version != 2 || strings.Contains(stored.Declaration, "customer_input") {
			t.Fatal("evaluation modified the frozen historical declaration")
		}
	})
	fulfillmentContract, err := service.GetFrozenFulfillmentContract(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("load frozen fulfillment contract: %v", err)
	}
	fulfillmentPolicy, err := catalog.BuildFulfillmentPolicyEvidence(
		catalog.FulfillmentStrategyPaymentConfirmedPlugin,
		fulfillmentContract,
	)
	if err != nil {
		t.Fatalf("build frozen fulfillment policy: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
		(tenant_id,version_no,strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,
		 provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,created_by)
		VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
		"ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", fulfillmentPolicy.StrategyCode,
		fulfillmentPolicy.GranularityCode, fulfillmentPolicy.OngoingManagement,
		fulfillmentPolicy.RenewalMode, fulfillmentPolicy.RenewalMethod,
		fulfillmentPolicy.EntitlementOwnerID, fulfillmentPolicy.ProvisionMethod,
		fulfillmentPolicy.QueryMethod, fulfillmentPolicy.MaximumAttempts,
		fulfillmentPolicy.RetryBaseSeconds, fulfillmentPolicy.PolicyDigest,
	); err != nil {
		t.Fatalf("save frozen fulfillment policy: %v", err)
	}
	transitions = audit.snapshotTransitions()
	lastVersionUpdate := transitions[len(transitions)-1]
	beforeVersion, beforeOK := lastVersionUpdate.BeforeSnapshot.(map[string]any)
	afterVersion, afterOK := lastVersionUpdate.AfterSnapshot.(map[string]any)
	if !beforeOK || !afterOK || beforeVersion["version"] != int64(1) || beforeVersion["merchant_value_digest"] == "" || afterVersion["version"] != int64(2) || afterVersion["merchant_value_digest"] == "" {
		t.Fatalf("version configuration audit evidence is incomplete: before=%#v after=%#v", lastVersionUpdate.BeforeSnapshot, lastVersionUpdate.AfterSnapshot)
	}
	// Real database regression: unchanged drafts accept pricing, including retries.
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension (LIKE public.plugin_idc_core_catalog_configuration_price_dimension INCLUDING DEFAULTS INCLUDING IDENTITY);CREATE UNIQUE INDEX test_dimension_identity ON plugin_idc_core_catalog_configuration_price_dimension(version_no,field_key,candidate_value)`); err != nil {
		t.Fatal(err)
	}
	// Candidate connectivity must not block saved local configuration edits.
	originalInvoke, originalCache := extensions.invoke, service.dynamicCandidates
	extensions.invoke = func(pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		return nil, errors.New("candidate provider offline")
	}
	service.dynamicCandidates = newDynamicCandidateCache()
	offlineRollback := errors.New("rollback offline edit fixture")
	if offlineErr := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		view, readErr := service.GetVersionConfiguration(txCtx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if readErr != nil || view == nil || view.ProductDeclaration == nil || view.ProductDeclaration.Fields[0].DynamicCandidates != nil {
			t.Fatalf("offline saved schema read: %+v %v", view, readErr)
		}
		if saveErr := service.SaveVersionConfiguration(txCtx, SaveVersionConfigurationInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 2, MerchantValues: view.MerchantValues, CustomerFieldSelections: []CustomerFieldSelection{{FieldKey: "cores", GroupZhCN: "本地分组", GroupEnUS: "Local group"}}}); saveErr != nil {
			t.Fatalf("offline local edit failed: %v", saveErr)
		}
		return offlineRollback
	}); !errors.Is(offlineErr, offlineRollback) {
		t.Fatal(offlineErr)
	}
	extensions.invoke, service.dynamicCandidates = originalInvoke, originalCache
	extensions.exactErr = errors.New("original provider release unavailable during local pricing")
	candidates, err := service.ListConfigurationPriceCandidates(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil || len(candidates) != 1 {
		t.Fatalf("price candidates: %+v %v", candidates, err)
	}
	dimensionInput := CreateConfigurationPriceDimensionInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CandidateToken: candidates[0].CandidateToken, PricingMode: catalog.ConfigurationPricePerUnit}
	dimension, err := service.CreateConfigurationPriceDimension(ctx, dimensionInput)
	if err != nil {
		t.Fatalf("unchanged draft rejected dimension: %v", err)
	}
	repeated, err := service.CreateConfigurationPriceDimension(ctx, dimensionInput)
	if err != nil || repeated.DimensionNo != dimension.DimensionNo {
		t.Fatalf("dimension retry: %+v %v", repeated, err)
	}
	dimensionInput.PricingMode = catalog.ConfigurationPriceBandTotal
	if _, err = service.CreateConfigurationPriceDimension(ctx, dimensionInput); !bizerr.Is(err, catalog.CodeManagementConflict) {
		t.Fatalf("existing algorithm changed: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension (tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code) VALUES(0,'dimension-saved-region','ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',2,'region','saved-east','地区','Region','华东','East China','fixed','active')`); err != nil {
		t.Fatal(err)
	}
	retained, retainErr := service.ListConfigurationPriceCandidates(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if retainErr != nil || len(retained) != 2 {
		t.Fatalf("offline local quantities and saved dynamic options: %+v %v", retained, retainErr)
	}
	var savedRegion *ConfigurationPriceCandidate
	for index := range retained {
		if retained[index].CandidateLabel.ZhCN == "华东" {
			savedRegion = &retained[index]
		}
	}
	if savedRegion == nil || savedRegion.DimensionNo != "dimension-saved-region" {
		t.Fatalf("saved dynamic label or dimension lost: %+v", savedRegion)
	}
	if _, err = service.CreateConfigurationPriceDimension(ctx, CreateConfigurationPriceDimensionInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CandidateToken: configurationPriceCandidateToken("ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 2, "region", "invented"), PricingMode: catalog.ConfigurationPriceFixed}); !bizerr.Is(err, catalog.CodeManagementInvalid) {
		t.Fatalf("invented dynamic option accepted: %v", err)
	}
	rollbackSnapshot := errors.New("rollback corrupted pricing snapshot fixture")
	if err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, updateErr := database.Exec(txCtx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET declaration_digest=?,version=version+1 WHERE version_no='ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`, strings.Repeat("f", 64)); updateErr != nil {
			return updateErr
		}
		if _, priceErr := service.ListConfigurationPriceCandidates(txCtx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !bizerr.Is(priceErr, catalog.CodeManagementConflict) {
			return errors.New("corrupted local pricing snapshot was accepted")
		}
		return rollbackSnapshot
	}); !errors.Is(err, rollbackSnapshot) {
		t.Fatalf("pricing snapshot integrity guard: %v", err)
	}
	extensions.exactErr = nil
	fileVerifier := &frozenCustomerFileVerifierStub{}
	frozenValidator, err := NewFrozenCustomerConfigurationValidator(database, extensions, fileVerifier)
	if err != nil {
		t.Fatal(err)
	}
	customerDeclarationJSON, err := configurationDeclarationJSON(evidence.CustomerConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	frozenCandidates := configurationcontract.ResolvedCandidates{"region": []*capmodel.ConfigurationCandidate{{Value: "saved-east", Label: &capmodel.LocalizedText{ZhCN: "华东", EnUS: "East China"}}}}
	candidateDigest, err := FrozenCustomerCandidateSnapshotDigest(frozenCandidates)
	if err != nil {
		t.Fatal(err)
	}
	domainValue := "example.com"
	frozenValues := []ConfigurationValue{
		{FieldKey: "domain", TextValue: &domainValue},
		{FieldKey: "csr", FileTokens: []string{"CFU_0123456789ABCDEF0123456789ABCDEF"}},
	}
	compiledFrozenValues, err := configurationcontract.CompileResolved(
		evidence.CustomerConfiguration, frozenValues, configurationSurfaceCustomer,
		evidence.AuthorizedMethods, frozenCandidates,
	)
	if err != nil {
		t.Fatal(err)
	}
	frozenInput := FrozenCustomerConfigurationInput{
		OrderNo: "ORD_FROZEN_001", CustomerNo: "cus_frozen", UploaderUserID: 701,
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InterfaceInstanceNo: created.InterfaceInstanceNo,
		InterfaceConfigurationRevision: 3, CustomerDeclarationDigest: configurationDigest(customerDeclarationJSON),
		CandidateSnapshotDigest: candidateDigest, CustomerValueDigest: configurationDigest(compiledFrozenValues.ValuesJSON),
		ConfigurationRevision: "cfg_rev_test", Values: frozenValues,
		Candidates: frozenCandidates,
	}
	frozenEvidence, err := frozenValidator.Validate(ctx, frozenInput)
	if err != nil || frozenEvidence == nil || frozenEvidence.CustomerValueDigest == "" ||
		len(frozenEvidence.VerifiedFiles) != 1 || len(fileVerifier.inputs) != 1 || fileVerifier.owner.OrderNo != frozenInput.OrderNo || fileVerifier.owner.CustomerNo != frozenInput.CustomerNo || fileVerifier.owner.UploaderUserID != frozenInput.UploaderUserID ||
		fileVerifier.inputs[0].Target.ConfigurationRevision != "cfg_rev_test" {
		t.Fatalf("opening-time frozen customer validation failed: evidence=%+v inputs=%+v err=%v", frozenEvidence, fileVerifier.inputs, err)
	}
	t.Run("native resource-owned interface freezes an opaque owner revision", func(t *testing.T) {
		rollback := errors.New("rollback native resource fixture")
		owner := &capmodel.ConfigurationSchemaDeclaration{Version: "resource-owner-v1"}
		ownerJSON, encodeErr := configurationDeclarationJSON(owner)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		err := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			nativeNo := "pfi_" + strings.Repeat("e", 32)
			if _, updateErr := database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_automation_interface_instance (interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,created_by,changed_by,interface_origin_code,resource_owner_id,resource_channel_ref) SELECT ?,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,created_by,changed_by,'resource','idc-resource','CHN_native_resource' FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, nativeNo, created.InterfaceInstanceNo); updateErr != nil {
				return updateErr
			}
			if _, updateErr := database.Exec(txCtx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding SET interface_instance_no=? WHERE version_no=?`, nativeNo, frozenInput.VersionNo); updateErr != nil {
				return updateErr
			}

			if _, updateErr := database.Model(do.ProductAutomationInterfaceConfigurationRevision{}).Ctx(txCtx).Data(do.ProductAutomationInterfaceConfigurationRevision{InterfaceInstanceNo: nativeNo, Revision: 1, SchemaVersion: "resource-owner-v1", DeclarationSnapshot: string(ownerJSON), PublicValueSnapshot: "{}", DeclarationDigest: configurationDigest(ownerJSON), ValueDigest: configurationDigest([]byte("{}")), ResourceOwnerConfigurationRevision: "rci_" + strings.Repeat("1", 32), CreatedBy: 701}).Insert(); updateErr != nil {
				return updateErr
			}
			if _, updateErr := database.Exec(txCtx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET interface_instance_no=?,interface_configuration_revision=1,version=version+1 WHERE version_no=?`, nativeNo, frozenInput.VersionNo); updateErr != nil {
				return updateErr
			}
			nativeInput := frozenInput
			nativeInput.InterfaceConfigurationRevision = 1
			nativeInput.InterfaceInstanceNo = nativeNo
			// Native interfaces have no imported contract. Real SQL batch reads must
			// use the immutable owner projection instead of connector credential fields.
			got, validateErr := frozenValidator.ValidateBatch(txCtx, []FrozenCustomerConfigurationInput{nativeInput, nativeInput})
			if validateErr != nil {
				return validateErr
			}
			if len(got) != 2 {
				return errors.New("native frozen batch count mismatch")
			}
			tamperedInput := nativeInput
			tamperedInput.CustomerDeclarationDigest = strings.Repeat("f", 64)
			if _, validateErr = frozenValidator.Validate(txCtx, tamperedInput); validateErr == nil {
				return errors.New("tampered native projection accepted")
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatalf("native resource batch: %v", err)
		}
	})

	staleInput := frozenInput
	staleInput.InterfaceConfigurationRevision = 2
	if staleEvidence, staleErr := frozenValidator.Validate(ctx, staleInput); staleErr == nil || staleEvidence != nil {
		t.Fatalf("stale interface revision crossed opening validation: evidence=%+v err=%v", staleEvidence, staleErr)
	}
	fileVerifier.err = errors.New("file ownership changed")
	if missingFileEvidence, fileErr := frozenValidator.Validate(ctx, frozenInput); fileErr == nil || missingFileEvidence != nil {
		t.Fatalf("unverified file crossed opening validation: evidence=%+v err=%v", missingFileEvidence, fileErr)
	}
	fileVerifier.err = nil
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || !ready.Passed {
		t.Fatalf("complete frozen configuration failed publication: ready=%v err=%v", ready, readinessErr)
	}
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(context.Context) error {
		publicationActionCalled = true
		return nil
	}); publicationErr != nil || !publicationActionCalled {
		t.Fatalf("complete final publication admission failed: called=%v err=%v", publicationActionCalled, publicationErr)
	}
	if productCandidateCalls != 4 {
		t.Fatalf("dynamic product candidates produced repeated per-boundary calls: calls=%d want=4 distinct instance/revision inputs", productCandidateCalls)
	}
	t.Run("compatible package update preserves business facts", func(t *testing.T) {
		previousExtensions := service.extensions
		previousEvidence := extensions.evidence
		defer func() { service.extensions = previousExtensions; extensions.evidence = previousEvidence }()
		runtime, wrapErr := providerexecution.New(extensions)
		if wrapErr != nil {
			t.Fatal(wrapErr)
		}
		current := cloneAutomationEvidence(evidence)
		current.ReleaseVersion = "v1.1.0"
		current.ReleaseDigest = strings.Repeat("e", 64)
		extensions.evidence = []*capmodel.BusinessExtensionProviderEvidence{current}
		service.extensions = runtime
		before, loadErr := service.loadBinding(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		readiness, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if readinessErr != nil || !readiness.Passed {
			t.Fatalf("compatible package blocked configured product: %+v %v", readiness, readinessErr)
		}
		if extensions.lastAdmission.ReleaseDigest != current.ReleaseDigest || extensions.lastAdmission.ReleaseVersion != current.ReleaseVersion {
			t.Fatal("publication used historical executable identity")
		}
		view, viewErr := service.loadRuntimeConfigurationView(ctx, current, dynamicCandidateScope{interfaceInstanceNo: created.InterfaceInstanceNo, configurationRevision: 3})
		if viewErr != nil || view == nil {
			t.Fatalf("current package lost original interface configuration: %v", viewErr)
		}
		after, loadErr := service.loadBinding(ctx, before.VersionNo, false)
		if loadErr != nil || !sameEvidence(bindingEvidence(before), bindingEvidence(after)) {
			t.Fatal("runtime resolution rewrote saved binding")
		}
	})
	t.Run("optional schema upgrade preserves saved configuration", func(t *testing.T) {
		previousExtensions, previousEvidence := service.extensions, extensions.evidence
		defer func() {
			service.extensions = previousExtensions
			extensions.evidence = previousEvidence
			extensions.historicalEvidence = nil
		}()
		runtime, wrapErr := providerexecution.New(extensions)
		if wrapErr != nil {
			t.Fatal(wrapErr)
		}
		rawCurrent, marshalErr := json.Marshal(evidence)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		current := &capmodel.BusinessExtensionProviderEvidence{}
		if decodeErr := json.Unmarshal(rawCurrent, current); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		current.ReleaseVersion = "v1.2.0"
		current.ReleaseDigest = strings.Repeat("e", 64)
		current.DeclarationDigest = strings.Repeat("d", 64)
		current.ProductConfiguration.Fields = append(current.ProductConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{Key: "inbound_bandwidth", Label: &capmodel.LocalizedText{EnUS: "Inbound bandwidth", ZhCN: "入站带宽"}, Type: capmodel.ConfigurationFieldInteger, Minimum: "1", Maximum: "1000000"})
		current.CustomerConfiguration.Fields = append(current.CustomerConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{Key: "ip_count", Label: &capmodel.LocalizedText{EnUS: "IP count", ZhCN: "IP数量"}, Type: capmodel.ConfigurationFieldInteger, Minimum: "0", Maximum: "1024"})
		if !productfulfillmentcap.IsAdditiveConfigurationUpgrade(evidence, current) {
			t.Fatal("optional field proof rejected identical prior contract")
		}
		extensions.historicalEvidence = cloneAutomationEvidence(evidence)
		extensions.evidence = []*capmodel.BusinessExtensionProviderEvidence{current}
		service.extensions = runtime
		before, loadErr := database.GetValue(ctx, `SELECT row_to_json(c)::text FROM plugin_idc_core_catalog_product_version_configuration c WHERE version_no=?`, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		readiness, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if readinessErr != nil || !readiness.Passed {
			t.Fatalf("additive schema blocked configured product: %+v %v", readiness, readinessErr)
		}
		view, viewErr := service.GetVersionConfiguration(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if viewErr != nil || view == nil || len(view.ProductDeclaration.Fields) != len(current.ProductConfiguration.Fields) {
			t.Fatalf("optional new merchant field is not editable: view=%+v err=%v", view, viewErr)
		}
		if extensions.lastAdmission.ReleaseDigest != current.ReleaseDigest || extensions.lastAdmission.DeclarationDigest != current.DeclarationDigest {
			t.Fatal("schema upgrade used fabricated historical executable identity")
		}
		after, loadErr := database.GetValue(ctx, `SELECT row_to_json(c)::text FROM plugin_idc_core_catalog_product_version_configuration c WHERE version_no=?`, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		if loadErr != nil || before.String() != after.String() {
			t.Fatal("schema upgrade rewrote merchant configuration")
		}
		// A new product may bind the existing interface after this compatible
		// package upgrade. Database snapshots remain the selected local tuple.
		newVersion := "ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		if _, insertErr := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES (0,?,'draft')`, newVersion); insertErr != nil {
			t.Fatal(insertErr)
		}
		bound, bindErr := service.Bind(ctx, BindInput{VersionNo: newVersion, InterfaceInstanceNo: created.InterfaceInstanceNo})
		if bindErr != nil || bound == nil {
			t.Fatalf("compatible provider upgrade blocked a new product binding: %v", bindErr)
		}
		stored, loadErr := service.loadBinding(ctx, newVersion, false)
		if loadErr != nil || stored.PluginReleaseVersion != evidence.ReleaseVersion || stored.ReleaseDigest != evidence.ReleaseDigest {
			t.Fatal("new binding replaced the selected interface snapshot")
		}
		if extensions.lastAdmission.ReleaseDigest != current.ReleaseDigest {
			t.Fatal("new product bypassed actual package admission")
		}
		current.CustomerConfiguration.Fields[len(current.CustomerConfiguration.Fields)-1].Required = true
		if result, checkErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); checkErr == nil && result.Passed {
			t.Fatal("required schema change passed publication admission")
		}
	})
	providerConfigurationValid = false
	providerConfigurationReason = productfulfillmentcap.ConfigurationValidationSupplierCapabilitiesUnconfirmed
	publicationActionCalled = false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(context.Context) error {
		publicationActionCalled = true
		return nil
	}); !bizerr.Is(publicationErr, codeSupplierCapabilitiesUnconfirmed) || publicationActionCalled {
		t.Fatalf("provider-rejected configuration crossed final publication admission: called=%v err=%v", publicationActionCalled, publicationErr)
	}
	providerConfigurationValid = true
	providerConfigurationReason = ""
	publicationActionCalled = false
	originalMerchantDigest, err := database.GetValue(ctx, `SELECT merchant_value_digest FROM plugin_idc_core_catalog_product_version_configuration WHERE version_no=?`, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET merchant_value_digest=?,version=version+1 WHERE version_no=?`, strings.Repeat("f", 64), "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || ready.Passed {
		t.Fatalf("tampered merchant digest passed publication: ready=%v err=%v", ready, readinessErr)
	}
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(context.Context) error {
		publicationActionCalled = true
		return nil
	}); publicationErr == nil || publicationActionCalled {
		t.Fatalf("final publication crossed a tampered configuration gate: called=%v err=%v", publicationActionCalled, publicationErr)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET merchant_value_digest=?,version=version+1 WHERE version_no=?`, originalMerchantDigest.String(), "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	service.audit = &automationTestAudit{err: errors.New("forced audit rollback")}
	thirdValue := "third-secret"
	revokesBeforeAuditFailure := secrets.revokeCalls
	if _, err = service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 3, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &thirdValue}}}); err == nil {
		t.Fatal("expected transaction failure")
	}
	latestValue, err := database.GetValue(ctx, `SELECT max(revision) FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE interface_instance_no=?`, created.InterfaceInstanceNo)
	latestRevision := latestValue.Int64()
	if err != nil || latestRevision != 3 {
		t.Fatalf("failed transaction persisted revision=%d err=%v", latestRevision, err)
	}
	if secrets.revokeCalls != revokesBeforeAuditFailure+1 || secrets.replaceCalls != 0 {
		t.Fatalf("failed revision cleanup revoke=%d replace=%d", secrets.revokeCalls, secrets.replaceCalls)
	}
	mismatchValue, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_automation_interface_configuration_revision c WHERE c.interface_instance_no=? AND c.secret_field_count <> (SELECT count(*) FROM plugin_idc_core_product_automation_interface_secret_reference r WHERE r.configuration_revision_id=c.id)`, created.InterfaceInstanceNo)
	mismatch := mismatchValue.Int()
	if err != nil || mismatch != 0 {
		t.Fatalf("secret count mismatch=%d err=%v", mismatch, err)
	}
	service.audit = &automationTestAudit{}
	fourthValue := "fourth-secret"
	if _, err = service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 3, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &fourthValue}}}); err != nil {
		t.Fatalf("create a newer interface configuration revision: %v", err)
	}
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || ready.Passed {
		t.Fatalf("stale frozen interface revision passed publication: ready=%v err=%v", ready, readinessErr)
	}

	// A process can lose both the Create result and its immediate Recover
	// result. The persisted operation key lets a new worker recover and revoke
	// the orphan without retaining the submitted plaintext.
	secrets.createUnknownAt = secrets.createCalls + 1
	secrets.recoverFailure = errors.New("injected recover outage")
	fifthValue := "fifth-secret"
	if _, err = service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 4, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &fifthValue}}}); err == nil {
		t.Fatal("unknown create plus recover outage unexpectedly committed")
	}
	secrets.createUnknownAt = 0
	secrets.recoverFailure = nil
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_secret_operation SET next_attempt_at=CURRENT_TIMESTAMP,lease_owner='',lease_expires_at=NULL WHERE interface_instance_no=? AND status_code='compensation_pending'`, created.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	restartedWorker, err := NewSecretOperationRecoveryWorker(database, secrets, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if processed, recoveryErr := restartedWorker.Run(ctx, 8); recoveryErr != nil || processed != 1 {
		t.Fatalf("restart recovery did not converge unknown create: processed=%d err=%v", processed, recoveryErr)
	}
	statusValue, err := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_product_automation_interface_secret_operation WHERE interface_instance_no=? ORDER BY id DESC LIMIT 1`, created.InterfaceInstanceNo)
	if err != nil || statusValue.String() != secretOperationCompensated {
		t.Fatalf("unknown create journal status=%q err=%v", statusValue.String(), err)
	}

	// A revoke outage also remains durable and is retried by a new worker.
	service.audit = &automationTestAudit{err: errors.New("forced transaction rollback")}
	secrets.revokeFailure = errors.New("injected revoke outage")
	sixthValue := "sixth-secret"
	if _, err = service.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{InterfaceInstanceNo: created.InterfaceInstanceNo, ExpectedRevision: 4, Values: []ConfigurationValue{{FieldKey: "api_secret", TextValue: &sixthValue}}}); err == nil {
		t.Fatal("transaction rollback with revoke outage unexpectedly succeeded")
	}
	secrets.revokeFailure = nil
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_secret_operation SET next_attempt_at=CURRENT_TIMESTAMP,lease_owner='',lease_expires_at=NULL WHERE interface_instance_no=? AND status_code='compensation_pending'`, created.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	restartedWorker, err = NewSecretOperationRecoveryWorker(database, secrets, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if processed, recoveryErr := restartedWorker.Run(ctx, 8); recoveryErr != nil || processed != 1 {
		t.Fatalf("restart recovery did not converge revoke outage: processed=%d err=%v", processed, recoveryErr)
	}
	latestValue, err = database.GetValue(ctx, `SELECT max(revision) FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE interface_instance_no=?`, created.InterfaceInstanceNo)
	if err != nil || latestValue.Int64() != 4 {
		t.Fatalf("recovery materialized a rolled-back revision: revision=%d err=%v", latestValue.Int64(), err)
	}
	service.audit = audit
	audit.err = nil
	revokesBeforeDelete := secrets.revokeCalls
	if err = service.DeleteInterface(ctx, DeleteInterfaceInput{InterfaceInstanceNo: createdB.InterfaceInstanceNo, ExpectedVersion: 1}); err != nil {
		t.Fatalf("delete unused configured interface: %v", err)
	}
	if secrets.revokeCalls != revokesBeforeDelete+1 {
		t.Fatal("deletion did not revoke the private credential")
	}
	remaining, remainingErr := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_automation_interface_secret_reference r JOIN plugin_idc_core_product_automation_interface_configuration_revision c ON c.id=r.configuration_revision_id WHERE c.interface_instance_no=?`, createdB.InterfaceInstanceNo)
	if remainingErr != nil || remaining.Int() != 0 {
		t.Fatalf("deleted interface retained credential references: %v %v", remaining, remainingErr)
	}
}
