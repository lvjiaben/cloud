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

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
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
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version (tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,status_code VARCHAR(16) NOT NULL,UNIQUE(tenant_id,version_no));
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy (
 tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,
 strategy_code VARCHAR(32) NOT NULL,granularity_code VARCHAR(32) NOT NULL,
 ongoing_management BOOLEAN NOT NULL DEFAULT FALSE,
 renewal_mode_code VARCHAR(32) NOT NULL DEFAULT 'unavailable',renewal_method VARCHAR(128) NOT NULL DEFAULT '',entitlement_owner_id VARCHAR(64) NOT NULL DEFAULT '',
 provision_method VARCHAR(128) NOT NULL DEFAULT '',query_method VARCHAR(128) NOT NULL DEFAULT '',
 maximum_attempts INT NOT NULL,retry_base_seconds INT NOT NULL,policy_digest CHAR(64) NOT NULL,
 created_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO plugin_idc_core_catalog_product_version VALUES (0,'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','draft');`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	for _, migrationName := range []string{"074-product-automation-binding.sql", "075-structured-product-configuration.sql"} {
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
	label := &capmodel.LocalizedText{EnUS: "API secret", ZhCN: "接口密钥"}
	evidence := automationEvidence([]string{"candidate.product", "provision", "query.status"})
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
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			return nil, err
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(request.Requests))
		for _, query := range request.Requests {
			items = append(items, configurationcontract.DynamicCandidateItem{FieldKey: query.FieldKey, Candidates: []*capmodel.ConfigurationCandidate{{
				Value: "ssl-basic", Label: &capmodel.LocalizedText{EnUS: "Basic SSL", ZhCN: "基础证书"},
			}}})
		}
		payload, err := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, err
	}
	authorizer := &automationTestAuthorizer{}
	audit := &automationTestAudit{}
	codec := &automationTestCodec{evidence: evidence}
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
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || ready {
		t.Fatalf("bound version without a frozen product/customer configuration passed publication: ready=%v err=%v", ready, readinessErr)
	}
	publicationActionCalled := false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(context.Context) error {
		publicationActionCalled = true
		return nil
	}); publicationErr == nil || publicationActionCalled {
		t.Fatalf("final publication crossed a missing configuration gate: called=%v err=%v", publicationActionCalled, publicationErr)
	}
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
	fileVerifier := &frozenCustomerFileVerifierStub{}
	frozenValidator, err := NewFrozenCustomerConfigurationValidator(database, extensions, fileVerifier)
	if err != nil {
		t.Fatal(err)
	}
	customerDeclarationJSON, err := configurationDeclarationJSON(evidence.CustomerConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	frozenCandidates := configurationcontract.ResolvedCandidates{}
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
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || !ready {
		t.Fatalf("complete frozen configuration failed publication: ready=%v err=%v", ready, readinessErr)
	}
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(context.Context) error {
		publicationActionCalled = true
		return nil
	}); publicationErr != nil || !publicationActionCalled {
		t.Fatalf("complete final publication admission failed: called=%v err=%v", publicationActionCalled, publicationErr)
	}
	if extensions.invokeCalls != 4 {
		t.Fatalf("dynamic product candidates produced repeated per-boundary calls: calls=%d want=4 distinct instance/revision inputs", extensions.invokeCalls)
	}
	publicationActionCalled = false
	originalMerchantDigest, err := database.GetValue(ctx, `SELECT merchant_value_digest FROM plugin_idc_core_catalog_product_version_configuration WHERE version_no=?`, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET merchant_value_digest=?,version=version+1 WHERE version_no=?`, strings.Repeat("f", 64), "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || ready {
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
	if ready, readinessErr := service.CheckPublicationReadiness(ctx, "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); readinessErr != nil || ready {
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
}
