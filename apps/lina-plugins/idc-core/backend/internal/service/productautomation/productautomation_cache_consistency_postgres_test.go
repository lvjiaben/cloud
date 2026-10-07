// This file proves that configuration revisions and dynamic-candidate caches
// remain safe across process-like product automation service instances.

package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/glog"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type automationCacheQuerySampleKey struct{}

type automationCacheQueryLogger struct {
	glog.ILogger
	mu     sync.Mutex
	counts map[int]int
}

func (l *automationCacheQueryLogger) Debug(ctx context.Context, _ ...any) { l.record(ctx) }

func (l *automationCacheQueryLogger) Error(ctx context.Context, _ ...any) { l.record(ctx) }

func (l *automationCacheQueryLogger) record(ctx context.Context) {
	sample, ok := ctx.Value(automationCacheQuerySampleKey{}).(int)
	if !ok {
		return
	}
	l.mu.Lock()
	l.counts[sample]++
	l.mu.Unlock()
}

func (l *automationCacheQueryLogger) count(sample int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counts[sample]
}

func TestConfigurationRevisionsAndDynamicCandidateCachesConvergeAcrossPostgreSQLInstances(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schemaName := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schemaName)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`); cleanupErr != nil {
			t.Errorf("drop cache consistency schema: %v", cleanupErr)
		}
		if cleanupErr := admin.Close(ctx); cleanupErr != nil {
			t.Errorf("close cache consistency admin database: %v", cleanupErr)
		}
	})
	databaseA := openAutomationDatabase(t, ctx, dsn, schemaName)
	databaseB := openAutomationDatabase(t, ctx, dsn, schemaName)
	if _, err = databaseA.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version (
 tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,status_code VARCHAR(16) NOT NULL,
 UNIQUE(tenant_id,version_no));
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy (
 tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,
 strategy_code VARCHAR(32) NOT NULL,granularity_code VARCHAR(32) NOT NULL,
 maximum_attempts INT NOT NULL,retry_base_seconds INT NOT NULL,policy_digest CHAR(64) NOT NULL,
 created_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
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
		if _, err = databaseA.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	installResourceProjectionTestColumns(t, ctx, databaseA)

	exactV1 := cacheConsistencyEvidence("1.0.0", strings.Repeat("1", 64), strings.Repeat("a", 64))
	exactV2 := cacheConsistencyEvidence("2.0.0", strings.Repeat("2", 64), strings.Repeat("b", 64))
	if !validProviderEvidence(exactV1) || !validProviderEvidence(exactV2) {
		t.Fatal("cache consistency provider evidence is invalid")
	}
	extensionsA := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exactV1}}
	extensionsB := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exactV2}}
	generation := 1
	configureCandidateInvoker(t, extensionsA, "node-a", &generation)
	configureCandidateInvoker(t, extensionsB, "node-b", &generation)
	serviceA, err := newWithRandom(databaseA, extensionsA, &automationTestAuthorizer{}, &automationTestAudit{}, &automationTestCodec{evidence: exactV1}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serviceB, err := newWithRandom(databaseB, extensionsB, &automationTestAuthorizer{}, &automationTestAudit{}, &automationTestCodec{evidence: exactV2}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	endpointA := "https://supplier-a.example"
	interfaceA, err := serviceA.CreateInterface(ctx, CreateInterfaceInput{
		DisplayName: "供应商 A", SelectionToken: "opaque",
		Configuration: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointA}},
	})
	if err != nil {
		t.Fatalf("create release v1 interface: %v", err)
	}
	endpointB := "https://supplier-b.example"
	interfaceB, err := serviceB.CreateInterface(ctx, CreateInterfaceInput{
		DisplayName: "供应商 B", SelectionToken: "opaque",
		Configuration: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointB}},
	})
	if err != nil {
		t.Fatalf("create release v2 interface: %v", err)
	}
	// The second process now serves the same frozen release as the first while
	// retaining its own database connection and candidate cache.
	extensionsB.evidence = []*capmodel.BusinessExtensionProviderEvidence{exactV1}

	regionSchema := cacheConsistencyCandidateSchema("region", "candidate.region")
	regionScopeV1 := dynamicCandidateScope{interfaceInstanceNo: interfaceA.InterfaceInstanceNo, configurationRevision: 1}
	firstA, err := serviceA.resolveDynamicCandidates(ctx, exactV1, regionSchema, cacheConsistencyCountry("cn"), configurationSurfaceCustomer, regionScopeV1)
	if err != nil || firstA["region"][0].Value != "node-a-candidate.region-cn-r1" {
		t.Fatalf("first process candidate resolve=%#v err=%v", firstA, err)
	}
	firstB, err := serviceB.resolveDynamicCandidates(ctx, exactV1, regionSchema, cacheConsistencyCountry("cn"), configurationSurfaceCustomer, regionScopeV1)
	if err != nil || firstB["region"][0].Value != "node-b-candidate.region-cn-r1" {
		t.Fatalf("second process reused first process cache: resolved=%#v err=%v", firstB, err)
	}
	if _, err = serviceB.resolveDynamicCandidates(ctx, exactV1, regionSchema, cacheConsistencyCountry("cn"), configurationSurfaceCustomer, regionScopeV1); err != nil || extensionsB.invokeCalls != 1 {
		t.Fatalf("same process exact cache miss: calls=%d err=%v", extensionsB.invokeCalls, err)
	}
	if _, err = serviceB.resolveDynamicCandidates(ctx, exactV1, regionSchema, cacheConsistencyCountry("us"), configurationSurfaceCustomer, regionScopeV1); err != nil || extensionsB.invokeCalls != 2 {
		t.Fatalf("different input digest reused candidates: calls=%d err=%v", extensionsB.invokeCalls, err)
	}
	zoneSchema := cacheConsistencyCandidateSchema("zone", "candidate.zone")
	if _, err = serviceB.resolveDynamicCandidates(ctx, exactV1, zoneSchema, cacheConsistencyCountry("cn"), configurationSurfaceCustomer, regionScopeV1); err != nil || extensionsB.invokeCalls != 3 {
		t.Fatalf("different authorized method reused candidates: calls=%d err=%v", extensionsB.invokeCalls, err)
	}
	releaseScope := dynamicCandidateScope{interfaceInstanceNo: interfaceB.InterfaceInstanceNo, configurationRevision: 1}
	if _, err = serviceB.resolveDynamicCandidates(ctx, exactV2, regionSchema, cacheConsistencyCountry("cn"), configurationSurfaceCustomer, releaseScope); err != nil || extensionsB.invokeCalls != 4 {
		t.Fatalf("different plugin release or interface reused candidates: calls=%d err=%v", extensionsB.invokeCalls, err)
	}

	if _, err = serviceA.Bind(ctx, BindInput{VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", InterfaceInstanceNo: interfaceA.InterfaceInstanceNo}); err != nil {
		t.Fatalf("bind draft version: %v", err)
	}
	planBasic := "basic"
	if err = serviceA.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 0,
		MerchantValues: []ConfigurationValue{{FieldKey: "plan", TextValue: &planBasic}},
	}); err != nil {
		t.Fatalf("save product configuration revision 1: %v", err)
	}
	planPlus := "plus"
	if err = serviceA.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 1,
		MerchantValues: []ConfigurationValue{{FieldKey: "plan", TextValue: &planPlus}},
	}); err != nil {
		t.Fatalf("save product configuration revision 2: %v", err)
	}
	if _, staleErr := serviceB.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 1,
		MerchantValues: []ConfigurationValue{{FieldKey: "plan", TextValue: &planPlus}},
	}); staleErr == nil || !bizerr.Is(staleErr, catalog.CodeManagementConflict) {
		t.Fatalf("stale product configuration revision did not fail closed: %v", staleErr)
	}
	if current, currentErr := serviceB.EvaluateVersionConfiguration(ctx, EvaluateVersionConfigurationInput{
		VersionNo: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExpectedVersion: 2,
		MerchantValues: []ConfigurationValue{{FieldKey: "plan", TextValue: &planPlus}},
	}); currentErr != nil || current == nil || !current.Valid || current.Revision != 2 {
		t.Fatalf("second process did not immediately see product revision 2: value=%+v err=%v", current, currentErr)
	}

	endpointA2 := "https://supplier-a-v2.example"
	updated, err := serviceA.UpdateInterfaceConfiguration(ctx, UpdateInterfaceConfigurationInput{
		InterfaceInstanceNo: interfaceA.InterfaceInstanceNo, ExpectedRevision: 1,
		Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointA2}},
	})
	if err != nil || updated == nil || updated.Revision != 2 {
		t.Fatalf("publish interface configuration revision 2: value=%+v err=%v", updated, err)
	}
	if _, staleErr := serviceB.EvaluateInterfaceConfiguration(ctx, EvaluateInterfaceConfigurationInput{
		InterfaceInstanceNo: interfaceA.InterfaceInstanceNo, ExpectedRevision: 1,
		Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointA2}},
	}); staleErr == nil || !bizerr.Is(staleErr, catalog.CodeManagementConflict) {
		t.Fatalf("stale interface revision did not fail closed in second process: %v", staleErr)
	}
	if current, currentErr := serviceB.EvaluateInterfaceConfiguration(ctx, EvaluateInterfaceConfigurationInput{
		InterfaceInstanceNo: interfaceA.InterfaceInstanceNo, ExpectedRevision: 2,
		Values: []ConfigurationValue{{FieldKey: "endpoint", TextValue: &endpointA2}},
	}); currentErr != nil || current == nil || !current.Valid || current.Revision != 2 {
		t.Fatalf("second process did not immediately see interface revision 2: value=%+v err=%v", current, currentErr)
	}
	generation = 2
	regionScopeV2 := dynamicCandidateScope{interfaceInstanceNo: interfaceA.InterfaceInstanceNo, configurationRevision: 2}
	newRevisionCandidates, err := serviceB.resolveDynamicCandidates(ctx, exactV1, regionSchema, cacheConsistencyCountry("cn"), configurationSurfaceCustomer, regionScopeV2)
	if err != nil || newRevisionCandidates["region"][0].Value != "node-b-candidate.region-cn-r2" || extensionsB.invokeCalls != 5 {
		t.Fatalf("new interface revision reused stale candidate cache: resolved=%#v calls=%d err=%v", newRevisionCandidates, extensionsB.invokeCalls, err)
	}

	queryLogger := &automationCacheQueryLogger{ILogger: databaseB.GetLogger(), counts: make(map[int]int)}
	previousLogger, previousDebug := databaseB.GetLogger(), databaseB.GetDebug()
	databaseB.SetLogger(queryLogger)
	databaseB.SetDebug(true)
	t.Cleanup(func() {
		databaseB.SetDebug(previousDebug)
		databaseB.SetLogger(previousLogger)
	})
	oneContext := context.WithValue(ctx, automationCacheQuerySampleKey{}, 1)
	if _, err = serviceB.resolveDynamicCandidates(oneContext, exactV1, cacheConsistencyBulkSchema(1), nil, configurationSurfaceCustomer, regionScopeV2); err != nil {
		t.Fatalf("resolve one dynamic field: %v", err)
	}
	manyContext := context.WithValue(ctx, automationCacheQuerySampleKey{}, 64)
	if _, err = serviceB.resolveDynamicCandidates(manyContext, exactV1, cacheConsistencyBulkSchema(64), nil, configurationSurfaceCustomer, regionScopeV2); err != nil {
		t.Fatalf("resolve many dynamic fields: %v", err)
	}
	// One fixed query loads the interface configuration and one fixed query
	// checks whether its credentials are resource-owned. Field count must not
	// change that total.
	if oneQueries, manyQueries := queryLogger.count(1), queryLogger.count(64); oneQueries != 2 || manyQueries != oneQueries {
		t.Fatalf("dynamic candidate database queries grew with fields: one=%d many=%d", oneQueries, manyQueries)
	}
}

func cacheConsistencyEvidence(releaseVersion, releaseDigest, declarationDigest string) *capmodel.BusinessExtensionProviderEvidence {
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "generic-fulfillment", DisplayName: "Generic fulfillment", ReleaseVersion: releaseVersion,
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
		AuthorizedMethods: []string{"candidate.bulk", "candidate.region", "candidate.zone", "provision", "query.status"},
		ReleaseDigest:     releaseDigest, DeclarationDigest: declarationDigest, ProviderMetadata: testProviderMetadata(),
		InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine,
			Label: &capmodel.LocalizedText{ZhCN: "接口地址", EnUS: "Endpoint"},
		}}},
		ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "plan", Type: capmodel.ConfigurationFieldSingleLine,
			Label: &capmodel.LocalizedText{ZhCN: "方案", EnUS: "Plan"},
		}}},
	}
}

func cacheConsistencyCandidateSchema(fieldKey, method string) *capmodel.ConfigurationSchemaDeclaration {
	return &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "country", Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicCandidateLabel("国家", "Country"), StaticCandidates: []*capmodel.ConfigurationCandidate{
			{Value: "cn", Label: dynamicCandidateLabel("中国", "China")},
			{Value: "us", Label: dynamicCandidateLabel("美国", "United States")},
		}},
		{Key: fieldKey, Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicCandidateLabel("动态选项", "Dynamic option"),
			DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: method}},
	}}
}

func cacheConsistencyCountry(value string) []ConfigurationValue {
	return []ConfigurationValue{{FieldKey: "country", OptionTokens: []string{value}}}
}

func cacheConsistencyBulkSchema(fieldCount int) *capmodel.ConfigurationSchemaDeclaration {
	fields := make([]*capmodel.ConfigurationFieldDeclaration, 0, fieldCount)
	for index := 0; index < fieldCount; index++ {
		fields = append(fields, &capmodel.ConfigurationFieldDeclaration{
			Key:  "field_" + strings.Repeat("x", index/10) + string(rune('a'+index%10)),
			Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicCandidateLabel("批量选项", "Bulk option"),
			DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.bulk"},
		})
	}
	return &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: fields}
}

func configureCandidateInvoker(t *testing.T, extensions *automationTestExtensions, node string, generation *int) {
	t.Helper()
	extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var request configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &request); err != nil {
			return nil, err
		}
		country := "none"
		for _, selection := range request.CurrentSelections {
			if selection.FieldKey == "country" && len(selection.Values) == 1 {
				country = selection.Values[0]
			}
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(request.Requests))
		for _, query := range request.Requests {
			items = append(items, configurationcontract.DynamicCandidateItem{FieldKey: query.FieldKey, Candidates: []*capmodel.ConfigurationCandidate{{
				Value: node + "-" + input.Method + "-" + country + "-r" + string(rune('0'+*generation)),
				Label: dynamicCandidateLabel("可用选项", "Available option"),
			}}})
		}
		payload, err := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: payload}, err
	}
}
