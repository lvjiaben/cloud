// This file verifies transactional admission, frozen method evidence, audit rollback,
// bounded reads, draft replacement, unbinding, and publication serialization.

package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestServicePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schema)+` CASCADE`); cleanupErr != nil {
			t.Errorf("drop product automation schema: %v", cleanupErr)
		}
		if cleanupErr := admin.Close(ctx); cleanupErr != nil {
			t.Errorf("close product automation admin database: %v", cleanupErr)
		}
	})
	scoped := openAutomationDatabase(t, ctx, dsn, schema)
	contender := openAutomationDatabase(t, ctx, dsn, schema)
	if _, err = scoped.Exec(ctx, `
CREATE TABLE plugin_idc_core_catalog_product_version (
    tenant_id INT NOT NULL DEFAULT 0,
    version_no VARCHAR(40) NOT NULL PRIMARY KEY,
    status_code VARCHAR(16) NOT NULL,
    UNIQUE (tenant_id, version_no)
);
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_billing_rule (
    tenant_id INT NOT NULL DEFAULT 0,
    version_no VARCHAR(40) NOT NULL,
    charge_type_code VARCHAR(16) NOT NULL,
    status_code VARCHAR(16) NOT NULL
);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy (
    tenant_id INT NOT NULL DEFAULT 0,
    version_no VARCHAR(40) NOT NULL PRIMARY KEY,
    strategy_code VARCHAR(32) NOT NULL,
    granularity_code VARCHAR(32) NOT NULL,
    ongoing_management BOOLEAN NOT NULL DEFAULT FALSE,
    renewal_mode_code VARCHAR(32) NOT NULL DEFAULT 'unavailable',
    renewal_method VARCHAR(128) NOT NULL DEFAULT '',
    entitlement_owner_id VARCHAR(64) NOT NULL DEFAULT '',
    provision_method VARCHAR(128) NOT NULL DEFAULT '',
    query_method VARCHAR(128) NOT NULL DEFAULT '',
    maximum_attempts INT NOT NULL,
    retry_base_seconds INT NOT NULL,
    policy_digest CHAR(64) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE product_automation_test_audit (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    idempotency_scope VARCHAR(128) NOT NULL,
    idempotency_key VARCHAR(256) NOT NULL,
    evidence_digest CHAR(64) NOT NULL,
    UNIQUE (idempotency_scope,idempotency_key)
);
INSERT INTO plugin_idc_core_catalog_product_version (version_no,status_code) VALUES
('ver_11111111111111111111111111111111','draft'),
('ver_22222222222222222222222222222222','draft'),
('ver_33333333333333333333333333333333','draft'),
('ver_44444444444444444444444444444444','draft'),
('ver_55555555555555555555555555555555','draft'),
('ver_66666666666666666666666666666666','draft'),
('ver_77777777777777777777777777777777','draft');`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "074-product-automation-binding.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	configurationMigration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "075-structured-product-configuration.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if sidecar := bytes.Index(configurationMigration, []byte("-- Generic transaction evidence is a sidecar")); sidecar >= 0 {
		configurationMigration = configurationMigration[:sidecar]
	}
	if _, err = scoped.Exec(ctx, string(configurationMigration)); err != nil {
		t.Fatal(err)
	}
	installResourceProjectionTestColumns(t, ctx, scoped)
	deleteMigration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "128-interface-delete-session-operations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = scoped.Exec(ctx, string(deleteMigration)); err != nil {
			t.Fatal(err)
		}
	}

	evidence := automationEvidence(nil)
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}}
	authorizer := &automationTestAuthorizer{}
	audit := &automationTestAudit{strict: true, database: scoped}
	codec := &automationTestCodec{evidence: evidence, identityOnly: true}
	randomBytes := append(bytes.Repeat([]byte{0x11}, 16), bytes.Repeat([]byte{0x22}, 16)...)
	service, err := newWithRandom(scoped, extensions, authorizer, audit, codec, bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatal(err)
	}
	emptyBinding, err := service.GetBinding(ctx, "ver_22222222222222222222222222222222")
	if err != nil || emptyBinding != nil {
		t.Fatalf("visible version without automation binding must be a legal empty result: binding=%+v err=%v", emptyBinding, err)
	}
	manualPolicy, err := catalog.BuildFulfillmentPolicyEvidence(catalog.FulfillmentStrategyManual, nil)
	if err != nil {
		t.Fatalf("build unbound manual publication policy: %v", err)
	}
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
		(tenant_id,version_no,strategy_code,granularity_code,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,created_by)
		VALUES(0,?,?,?,?,?,?,?,?,101)`, "ver_22222222222222222222222222222222", manualPolicy.StrategyCode,
		manualPolicy.GranularityCode, manualPolicy.ProvisionMethod, manualPolicy.QueryMethod,
		manualPolicy.MaximumAttempts, manualPolicy.RetryBaseSeconds, manualPolicy.PolicyDigest); err != nil {
		t.Fatalf("insert unbound manual publication policy: %v", err)
	}
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_billing_rule
        (tenant_id,version_no,charge_type_code,status_code)
        VALUES(0,'ver_22222222222222222222222222222222','trial','active')`); err != nil {
		t.Fatal(err)
	}
	unsafePublicationCalled := false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_22222222222222222222222222222222", func(context.Context) error {
		unsafePublicationCalled = true
		return nil
	}); !bizerr.Is(publicationErr, catalog.CodeTrialFulfillmentUnavailable) || unsafePublicationCalled {
		t.Fatalf("manual trial reached publication: called=%v err=%v", unsafePublicationCalled, publicationErr)
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_billing_rule WHERE version_no='ver_22222222222222222222222222222222'`); err != nil {
		t.Fatal(err)
	}
	manualPublicationCalled := false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_22222222222222222222222222222222", func(context.Context) error {
		manualPublicationCalled = true
		return nil
	}); publicationErr != nil || !manualPublicationCalled {
		t.Fatalf("unbound manual policy failed publication admission: called=%v err=%v", manualPublicationCalled, publicationErr)
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_product_version_fulfillment_policy WHERE version_no=?`, "ver_22222222222222222222222222222222"); err != nil {
		t.Fatalf("reset unbound manual policy fixture: %v", err)
	}
	first, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "魔方云 A 接口", SelectionToken: "opaque-a"})
	if err != nil {
		t.Fatal(err)
	}
	if extensions.lastAdmission.Method != "" {
		t.Fatalf("empty owner method must remain empty, got %q", extensions.lastAdmission.Method)
	}
	frozenMethodCount, err := scoped.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_core_product_automation_interface_instance
WHERE interface_instance_no=? AND authorized_methods=ARRAY['provision','query.status']::TEXT[]`, first.InterfaceInstanceNo)
	if err != nil || frozenMethodCount.Int() != 1 {
		t.Fatalf("authorized fulfillment methods were not frozen: count=%d err=%v", frozenMethodCount.Int(), err)
	}

	binding, err := service.Bind(ctx, BindInput{VersionNo: "ver_11111111111111111111111111111111", InterfaceInstanceNo: first.InterfaceInstanceNo})
	if err != nil || binding.InterfaceInstanceNo != first.InterfaceInstanceNo {
		t.Fatalf("bind first interface: binding=%+v err=%v", binding, err)
	}
	frozenContract, err := service.GetFrozenFulfillmentContract(ctx, "ver_11111111111111111111111111111111")
	if err != nil || frozenContract == nil || frozenContract.Granularity != productfulfillmentcap.GranularityPerOrderItem ||
		frozenContract.ProvisionMethod != "provision" || frozenContract.QueryMethod != "query.status" ||
		frozenContract.DeclarationDigest != evidence.DeclarationDigest {
		t.Fatalf("read frozen fulfillment contract: contract=%+v err=%v", frozenContract, err)
	}
	policyAuthorization, err := service.AuthorizeFulfillmentPolicyWrite(ctx, "ver_11111111111111111111111111111111")
	if err != nil || policyAuthorization.ActorUserID != 101 || policyAuthorization.Contract == nil ||
		policyAuthorization.Contract.DeclarationDigest != evidence.DeclarationDigest {
		t.Fatalf("authorize fulfillment policy write: authorization=%+v err=%v", policyAuthorization, err)
	}
	renamed, err := service.RenameInterface(ctx, RenameInterfaceInput{InterfaceInstanceNo: first.InterfaceInstanceNo, DisplayName: "魔方云 A 新名称", ExpectedVersion: 2})
	if err != nil || renamed.DisplayName != "魔方云 A 新名称" || renamed.Version != 3 {
		t.Fatalf("rename future interface display only: item=%+v err=%v", renamed, err)
	}
	frozenName, err := service.GetBinding(ctx, "ver_11111111111111111111111111111111")
	if err != nil || frozenName == nil || frozenName.InterfaceDisplayName != "魔方云 A 接口" {
		t.Fatalf("rename changed historical binding snapshot: binding=%+v err=%v", frozenName, err)
	}
	stoppedInterface, err := service.UpdateNewBusinessState(ctx, InterfaceStateInput{InterfaceInstanceNo: first.InterfaceInstanceNo, NewBusinessState: InterfaceStateStopped, ExpectedVersion: 3})
	if err != nil || stoppedInterface.Version != 4 {
		t.Fatalf("stop interface: item=%+v err=%v", stoppedInterface, err)
	}
	stoppedInterface, err = service.UpdateNewBusinessState(ctx, InterfaceStateInput{InterfaceInstanceNo: first.InterfaceInstanceNo, NewBusinessState: InterfaceStateStopped, ExpectedVersion: 4})
	if err != nil || stoppedInterface.Version != 4 {
		t.Fatalf("idempotent optimistic stop interface: item=%+v err=%v", stoppedInterface, err)
	}
	stopped, err := service.GetBinding(ctx, "ver_11111111111111111111111111111111")
	if err != nil || stopped == nil || stopped.InterfaceNewBusinessState != InterfaceStateStopped {
		t.Fatalf("stopped interface history must remain visible: binding=%+v err=%v", stopped, err)
	}
	extensions.admissionErr = errors.New("provider is not ready")
	if _, err = service.UpdateNewBusinessState(ctx, InterfaceStateInput{InterfaceInstanceNo: first.InterfaceInstanceNo, NewBusinessState: InterfaceStateAccepting, ExpectedVersion: 4}); !bizerr.Is(err, CodeEvidenceUnavailable) {
		t.Fatalf("failed host revalidation must reject resume with automation evidence context: %v", err)
	}
	stateAfterFailure, queryErr := scoped.GetValue(ctx, `SELECT new_business_state||':'||version::text
FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, first.InterfaceInstanceNo)
	if queryErr != nil || stateAfterFailure.String() != "stopped:4" {
		t.Fatalf("failed resume changed local state: state=%q err=%v", stateAfterFailure.String(), queryErr)
	}
	extensions.admissionErr = nil
	resumedInterface, err := service.UpdateNewBusinessState(ctx, InterfaceStateInput{InterfaceInstanceNo: first.InterfaceInstanceNo, NewBusinessState: InterfaceStateAccepting, ExpectedVersion: 4})
	if err != nil || resumedInterface.Version != 5 {
		t.Fatalf("resume interface: item=%+v err=%v", resumedInterface, err)
	}
	resumedInterface, err = service.UpdateNewBusinessState(ctx, InterfaceStateInput{InterfaceInstanceNo: first.InterfaceInstanceNo, NewBusinessState: InterfaceStateAccepting, ExpectedVersion: 5})
	if err != nil || resumedInterface.Version != 5 {
		t.Fatalf("idempotent resume must still pass host revalidation: item=%+v err=%v", resumedInterface, err)
	}
	extensions.listErr = errors.New("provider no longer accepts new business")
	historical, err := service.GetBinding(ctx, "ver_11111111111111111111111111111111")
	if err != nil || !historical.EvidenceVerified {
		t.Fatalf("historical exact read depended on current candidate state: binding=%+v err=%v", historical, err)
	}
	extensions.listErr = nil
	extensions.exactErr = errors.New("historical release temporarily unavailable")
	unverified, err := service.GetBinding(ctx, "ver_11111111111111111111111111111111")
	if err != nil || unverified == nil || unverified.EvidenceVerified || unverified.InterfaceDisplayName != historical.InterfaceDisplayName || unverified.InterfaceNewBusinessState != InterfaceStateAccepting {
		t.Fatalf("safe historical summary must remain readable: binding=%+v err=%v", unverified, err)
	}
	if _, err = service.GetFrozenFulfillmentContract(ctx, "ver_11111111111111111111111111111111"); err == nil {
		t.Fatal("unverified summary must not authorize fulfillment")
	}
	extensions.exactErr = nil

	second, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "魔方云 B 接口", SelectionToken: "opaque-b"})
	if err != nil {
		t.Fatal(err)
	}
	stopTx, err := contender.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stopTx.Exec(`UPDATE plugin_idc_core_product_automation_interface_instance
SET new_business_state='stopped',version=version+1,changed_by=101,changed_at=CURRENT_TIMESTAMP
WHERE interface_instance_no=?`, second.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	bindStarted := make(chan struct{})
	bindAfterStop := make(chan error, 1)
	go func() {
		close(bindStarted)
		_, bindErr := service.Bind(ctx, BindInput{VersionNo: "ver_33333333333333333333333333333333", InterfaceInstanceNo: second.InterfaceInstanceNo})
		bindAfterStop <- bindErr
	}()
	<-bindStarted
	select {
	case bindErr := <-bindAfterStop:
		t.Fatalf("bind returned before the concurrent stop released its interface lock: %v", bindErr)
	case <-time.After(200 * time.Millisecond):
	}
	if err = stopTx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case bindErr := <-bindAfterStop:
		if !bizerr.Is(bindErr, catalog.CodeManagementConflict) {
			t.Fatalf("binding and stopping new business must not both succeed: %v", bindErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("binding did not finish after concurrent stop committed")
	}
	if err = service.ResumeNewBusiness(ctx, second.InterfaceInstanceNo); err != nil {
		t.Fatalf("resume second interface after bind race: %v", err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: "ver_44444444444444444444444444444444", InterfaceInstanceNo: second.InterfaceInstanceNo}); err != nil {
		t.Fatalf("prepare publication binding: %v", err)
	}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: "ver_44444444444444444444444444444444", ExpectedVersion: 0}); err != nil {
		t.Fatalf("prepare publication generic configuration: %v", err)
	}
	missingPolicyCallback := false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_44444444444444444444444444444444", func(context.Context) error {
		missingPolicyCallback = true
		return nil
	}); !bizerr.Is(publicationErr, catalog.CodeManagementConflict) || missingPolicyCallback {
		t.Fatalf("publication crossed a missing fulfillment policy: called=%v err=%v", missingPolicyCallback, publicationErr)
	}
	publicationContract, err := service.GetFrozenFulfillmentContract(ctx, "ver_44444444444444444444444444444444")
	if err != nil || publicationContract == nil {
		t.Fatalf("load publication fulfillment contract: contract=%+v err=%v", publicationContract, err)
	}
	publicationPolicy, err := catalog.BuildFulfillmentPolicyEvidence(catalog.FulfillmentStrategyPaymentConfirmedPlugin, publicationContract)
	if err != nil {
		t.Fatalf("build publication fulfillment policy: %v", err)
	}
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
		(tenant_id,version_no,strategy_code,granularity_code,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,created_by)
		VALUES(0,?,?,?,?,?,?,?,?,101)`, "ver_44444444444444444444444444444444", publicationPolicy.StrategyCode,
		publicationPolicy.GranularityCode, publicationPolicy.ProvisionMethod, publicationPolicy.QueryMethod,
		publicationPolicy.MaximumAttempts, publicationPolicy.RetryBaseSeconds, publicationPolicy.PolicyDigest); err != nil {
		t.Fatalf("insert publication fulfillment policy: %v", err)
	}
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy
		SET policy_digest=? WHERE version_no=?`, strings.Repeat("b", 64), "ver_44444444444444444444444444444444"); err != nil {
		t.Fatalf("tamper publication fulfillment policy: %v", err)
	}
	tamperedPolicyCallback := false
	if publicationErr := service.WithPublicationAdmission(ctx, "ver_44444444444444444444444444444444", func(context.Context) error {
		tamperedPolicyCallback = true
		return nil
	}); !bizerr.Is(publicationErr, catalog.CodeFulfillmentPolicyContractChanged) || tamperedPolicyCallback {
		t.Fatalf("publication crossed a tampered fulfillment policy: called=%v err=%v", tamperedPolicyCallback, publicationErr)
	}
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy
		SET policy_digest=? WHERE version_no=?`, publicationPolicy.PolicyDigest, "ver_44444444444444444444444444444444"); err != nil {
		t.Fatalf("restore publication fulfillment policy: %v", err)
	}
	publicationService, err := newWithRandom(contender, extensions, authorizer, audit, codec, bytes.NewReader(bytes.Repeat([]byte{0x44}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	publicationEntered := make(chan struct{})
	releasePublication := make(chan struct{})
	publicationResult := make(chan error, 1)
	go func() {
		publicationResult <- publicationService.WithPublicationAdmission(ctx, "ver_44444444444444444444444444444444", func(publicationCtx context.Context) error {
			if publicationCtx.Value(automationLockedContextKey{}) != true {
				return errors.New("publication callback lost host admission context")
			}
			close(publicationEntered)
			<-releasePublication
			_, updateErr := contender.Model("plugin_idc_core_catalog_product_version").Ctx(publicationCtx).
				Where("version_no", "ver_44444444444444444444444444444444").Data("status_code", "published").Update()
			return updateErr
		})
	}()
	select {
	case <-publicationEntered:
	case publicationErr := <-publicationResult:
		t.Fatalf("publication admission failed before callback: %v", publicationErr)
	case <-time.After(3 * time.Second):
		t.Fatal("publication admission did not enter callback")
	}
	stopResult := make(chan error, 1)
	go func() { stopResult <- service.StopNewBusiness(ctx, second.InterfaceInstanceNo) }()
	select {
	case stopErr := <-stopResult:
		t.Fatalf("stop returned before publication released its interface lock: %v", stopErr)
	case <-time.After(200 * time.Millisecond):
	}
	close(releasePublication)
	if publicationErr := <-publicationResult; publicationErr != nil {
		t.Fatalf("publication helper failed: %v", publicationErr)
	}
	select {
	case stopErr := <-stopResult:
		if stopErr != nil {
			t.Fatalf("stop after publication failed: %v", stopErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not finish after publication committed")
	}
	publishedHistory, err := service.GetBinding(ctx, "ver_44444444444444444444444444444444")
	if err != nil || publishedHistory == nil || publishedHistory.InterfaceNewBusinessState != InterfaceStateStopped {
		t.Fatalf("published history disappeared after stop: binding=%+v err=%v", publishedHistory, err)
	}
	if err = service.ResumeNewBusiness(ctx, second.InterfaceInstanceNo); err != nil {
		t.Fatalf("resume second interface for remaining draft tests: %v", err)
	}
	const lifecycleVersion = "ver_66666666666666666666666666666666"
	if _, err = service.Bind(ctx, BindInput{VersionNo: lifecycleVersion, InterfaceInstanceNo: first.InterfaceInstanceNo}); err != nil {
		t.Fatalf("bind lifecycle interface A: %v", err)
	}
	transitionsBeforeRetry := len(audit.snapshotTransitions())
	if _, err = service.Bind(ctx, BindInput{VersionNo: lifecycleVersion, InterfaceInstanceNo: first.InterfaceInstanceNo}); err != nil {
		t.Fatalf("repeat lifecycle interface A binding: %v", err)
	}
	if got := len(audit.snapshotTransitions()); got != transitionsBeforeRetry {
		t.Fatalf("repeated identical binding appended another audit transition: before=%d after=%d", transitionsBeforeRetry, got)
	}
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
		(tenant_id,version_no,strategy_code,granularity_code,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,created_by)
		VALUES(0,?,?,?,?,?,?,?,?,101)`, lifecycleVersion, manualPolicy.StrategyCode, manualPolicy.GranularityCode,
		manualPolicy.ProvisionMethod, manualPolicy.QueryMethod, manualPolicy.MaximumAttempts,
		manualPolicy.RetryBaseSeconds, manualPolicy.PolicyDigest); err != nil {
		t.Fatalf("insert lifecycle fulfillment policy: %v", err)
	}
	if err = service.Unbind(ctx, lifecycleVersion); err != nil {
		t.Fatalf("unbind lifecycle interface A: %v", err)
	}
	policyCount, err := scoped.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_catalog_product_version_fulfillment_policy WHERE version_no=?`, lifecycleVersion)
	if err != nil || policyCount.Int() != 0 {
		t.Fatalf("unbind retained a stale fulfillment policy: count=%d err=%v", policyCount.Int(), err)
	}
	transitionsAfterFirstUnbind := len(audit.snapshotTransitions())
	if err = service.Unbind(ctx, lifecycleVersion); err != nil {
		t.Fatalf("repeat lifecycle interface A unbind: %v", err)
	}
	if got := len(audit.snapshotTransitions()); got != transitionsAfterFirstUnbind {
		t.Fatalf("repeated unbind appended another audit transition: before=%d after=%d", transitionsAfterFirstUnbind, got)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: lifecycleVersion, InterfaceInstanceNo: second.InterfaceInstanceNo}); err != nil {
		t.Fatalf("bind lifecycle interface B: %v", err)
	}
	if err = service.Unbind(ctx, lifecycleVersion); err != nil {
		t.Fatalf("unbind lifecycle interface B: %v", err)
	}
	if err = service.Unbind(ctx, lifecycleVersion); err != nil {
		t.Fatalf("repeat lifecycle interface B unbind: %v", err)
	}
	lifecycleTransitions := make([]idccap.BusinessAuditTransition, 0, 4)
	identities := map[string]struct{}{}
	for _, transition := range audit.snapshotTransitions() {
		if transition.TargetNo != lifecycleVersion || !strings.HasPrefix(transition.OperationCode, "product-automation.binding.") {
			continue
		}
		lifecycleTransitions = append(lifecycleTransitions, transition)
		identity := transition.IdempotencyScope + ":" + transition.IdempotencyKey
		if _, exists := identities[identity]; exists {
			t.Fatalf("binding lifecycle reused audit identity %q", identity)
		}
		identities[identity] = struct{}{}
	}
	if len(lifecycleTransitions) != 4 {
		t.Fatalf("binding lifecycle audit transitions=%d want=4: %+v", len(lifecycleTransitions), lifecycleTransitions)
	}
	persistedLifecycleAudits, err := scoped.GetValue(ctx, `
SELECT COUNT(1) FROM product_automation_test_audit
WHERE idempotency_scope LIKE 'product-automation.binding.%'
  AND idempotency_key LIKE ?`, lifecycleVersion+":%")
	if err != nil || persistedLifecycleAudits.Int() != 4 {
		t.Fatalf("persisted binding lifecycle audits=%d want=4 err=%v", persistedLifecycleAudits.Int(), err)
	}

	const concurrentVersion = "ver_77777777777777777777777777777777"
	concurrentService, err := newWithRandom(contender, extensions, authorizer, audit, codec, bytes.NewReader(bytes.Repeat([]byte{0x77}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	concurrentResults := make(chan error, 2)
	for _, bindingService := range []*serviceImpl{service, concurrentService} {
		go func(candidate *serviceImpl) {
			_, bindErr := candidate.Bind(ctx, BindInput{VersionNo: concurrentVersion, InterfaceInstanceNo: second.InterfaceInstanceNo})
			concurrentResults <- bindErr
		}(bindingService)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if bindErr := <-concurrentResults; bindErr != nil {
			t.Fatalf("concurrent identical binding attempt %d: %v", attempt+1, bindErr)
		}
	}
	concurrentAuditCount := 0
	for _, transition := range audit.snapshotTransitions() {
		if transition.TargetNo == concurrentVersion && transition.OperationCode == "product-automation.binding.bind" {
			concurrentAuditCount++
		}
	}
	if concurrentAuditCount != 1 {
		t.Fatalf("concurrent identical binding audits=%d want=1", concurrentAuditCount)
	}
	persistedConcurrentAudits, err := scoped.GetValue(ctx, `
SELECT COUNT(1) FROM product_automation_test_audit
WHERE idempotency_scope='product-automation.binding.bind'
  AND idempotency_key LIKE ?`, concurrentVersion+":%")
	if err != nil || persistedConcurrentAudits.Int() != 1 {
		t.Fatalf("persisted concurrent binding audits=%d want=1 err=%v", persistedConcurrentAudits.Int(), err)
	}
	if err = service.Unbind(ctx, concurrentVersion); err != nil {
		t.Fatalf("clean concurrent binding: %v", err)
	}
	concurrentUnbindResults := make(chan error, 2)
	if _, err = service.Bind(ctx, BindInput{VersionNo: concurrentVersion, InterfaceInstanceNo: first.InterfaceInstanceNo}); err != nil {
		t.Fatalf("prepare concurrent unbind: %v", err)
	}
	for _, bindingService := range []*serviceImpl{service, concurrentService} {
		go func(candidate *serviceImpl) {
			concurrentUnbindResults <- candidate.Unbind(ctx, concurrentVersion)
		}(bindingService)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if unbindErr := <-concurrentUnbindResults; unbindErr != nil {
			t.Fatalf("concurrent identical unbind attempt %d: %v", attempt+1, unbindErr)
		}
	}
	concurrentUnbindAuditCount := 0
	for _, transition := range audit.snapshotTransitions() {
		if transition.TargetNo == concurrentVersion && transition.OperationCode == "product-automation.binding.unbind" {
			concurrentUnbindAuditCount++
		}
	}
	if concurrentUnbindAuditCount != 2 {
		// One audit belongs to the cleanup of the first concurrent bind and one
		// to the later concurrent unbind. The losing identical unbind is a no-op.
		t.Fatalf("concurrent unbind audits=%d want=2", concurrentUnbindAuditCount)
	}
	replaced, err := service.Bind(ctx, BindInput{VersionNo: "ver_11111111111111111111111111111111", InterfaceInstanceNo: second.InterfaceInstanceNo})
	if err != nil || replaced.InterfaceInstanceNo != second.InterfaceInstanceNo {
		t.Fatalf("replace draft binding: binding=%+v err=%v", replaced, err)
	}
	if err = service.Unbind(ctx, "ver_11111111111111111111111111111111"); err != nil {
		t.Fatalf("unbind draft binding: %v", err)
	}

	const resourceOwnedInterfaceNo = "pfi_88888888888888888888888888888888"
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_instance
(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state,interface_origin_code,resource_owner_id,resource_channel_ref,version,created_by,changed_by)
VALUES (0,?,'服务器上游接口',?,'product-fulfillment','older-release','v1',ARRAY[]::TEXT[],?,?,'accepting','resource','idc-resource','CHN_channel-a',1,101,101)`,
		resourceOwnedInterfaceNo, evidence.PluginID, strings.Repeat("e", 64), strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}

	page, err := service.ListInterfaces(ctx, PageInput{Page: 1, PageSize: 1})
	if err != nil || len(page.Items) != 1 || page.Total != 2 {
		t.Fatalf("bounded managed interface page must exclude resource-owned server interfaces: page=%+v err=%v", page, err)
	}
	allBindings, err := service.ListInterfaces(ctx, PageInput{Page: 1, PageSize: 20, ForProductBinding: true})
	if err != nil || allBindings.Total != 3 || len(allBindings.Items) != 3 {
		t.Fatalf("direct binding selection must include resource projections without choosing a provider: page=%+v err=%v", allBindings, err)
	}
	bindingFiltered, err := service.ListInterfaces(ctx, PageInput{Page: 1, PageSize: 1, ForProductBinding: true, NamePrefix: "服务器", NewBusinessState: InterfaceStateAccepting})
	if err != nil || bindingFiltered.Total != 1 || len(bindingFiltered.Items) != 1 || bindingFiltered.Items[0].InterfaceInstanceNo != resourceOwnedInterfaceNo {
		t.Fatalf("direct binding search must preserve bounded SQL filters: page=%+v err=%v", bindingFiltered, err)
	}
	filtered, err := service.ListInterfaces(ctx, PageInput{Page: 1, PageSize: 20, NamePrefix: "魔方云 A", NewBusinessState: InterfaceStateAccepting})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].DisplayName != "魔方云 A 新名称" || filtered.Items[0].CanDelete {
		t.Fatalf("database-side literal filters or irreversible delete projection failed: page=%+v err=%v", filtered, err)
	}
	const otherProviderInterfaceNo = "pfi_44444444444444444444444444444444"
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_instance
(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state,version,created_by,changed_by)
VALUES (0,?,'其他连接器接口','other-connector','product-fulfillment','v0.1.0','v1',ARRAY[]::TEXT[],?,?,'accepting',1,101,101)`,
		otherProviderInterfaceNo, strings.Repeat("c", 64), strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	// The real short-lived selection token intentionally carries only frozen
	// identity/evidence fields, not the full provider declaration or schemas.
	codec.evidence = &capmodel.BusinessExtensionProviderEvidence{
		PluginID: evidence.PluginID, DisplayName: evidence.DisplayName,
		ReleaseVersion: evidence.ReleaseVersion, Point: evidence.Point,
		ContractVersion: evidence.ContractVersion, AuthorizedMethods: append([]string(nil), evidence.AuthorizedMethods...),
		ReleaseDigest: evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest,
	}
	selectedProvider, err := service.ListInterfaces(ctx, PageInput{
		Page: 1, PageSize: 20, ProviderSelectionToken: "opaque-provider-selection",
	})
	if err != nil || selectedProvider.Total != 3 || len(selectedProvider.Items) != 3 {
		t.Fatalf("connector selection must include compatible resource projections across releases in one database page: page=%+v err=%v", selectedProvider, err)
	}
	for _, item := range selectedProvider.Items {
		if item.InterfaceInstanceNo == otherProviderInterfaceNo {
			t.Fatalf("connector selection leaked another provider interface: %+v", item)
		}
	}

	resourceFound := false
	for _, item := range selectedProvider.Items {
		resourceFound = resourceFound || item.InterfaceInstanceNo == resourceOwnedInterfaceNo
	}
	if !resourceFound {
		t.Fatal("product binding candidates omitted resource-owned interface from an older connector release")
	}
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_instance SET new_business_state='stopped',version=version+1 WHERE interface_instance_no=?`, resourceOwnedInterfaceNo); err != nil {
		t.Fatal(err)
	}
	accepting, err := service.ListInterfaces(ctx, PageInput{Page: 1, PageSize: 20, ProviderSelectionToken: "opaque-provider-selection", NewBusinessState: InterfaceStateAccepting})
	if err != nil || accepting.Total != 2 {
		t.Fatalf("stopped resource interface must not be offered for new binding: page=%+v err=%v", accepting, err)
	}

	projection := fmt.Sprintf("%+v", page.Items[0])
	for _, forbidden := range []string{evidence.PluginID, evidence.ReleaseDigest, evidence.DeclarationDigest} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("interface projection leaked %q: %s", forbidden, projection)
		}
	}

	failingAudit := &automationTestAudit{err: errors.New("audit unavailable")}
	failingService, err := newWithRandom(scoped, extensions, authorizer, failingAudit, codec, bytes.NewReader(bytes.Repeat([]byte{0x33}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = failingService.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "必须回滚接口", SelectionToken: "opaque-c"}); err == nil {
		t.Fatal("expected audit failure to reject interface creation")
	}
	rolledBack, queryErr := scoped.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_automation_interface_instance
WHERE interface_instance_no='pfi_33333333333333333333333333333333'`)
	if queryErr != nil || rolledBack.Int() != 0 {
		t.Fatalf("audit failure did not roll back interface: count=%d err=%v", rolledBack.Int(), queryErr)
	}

	const directInterfaceNo = "pfi_55555555555555555555555555555555"
	unusedService, err := newWithRandom(scoped, extensions, authorizer, audit, codec, bytes.NewReader(bytes.Repeat([]byte{0x99}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	unused, err := unusedService.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "误建可删除接口", SelectionToken: "opaque-unused"})
	if err != nil {
		t.Fatal(err)
	}
	if err = failingService.DeleteInterface(ctx, DeleteInterfaceInput{InterfaceInstanceNo: unused.InterfaceInstanceNo, ExpectedVersion: 1}); err == nil {
		t.Fatal("delete must roll back when audit fails")
	}
	for _, table := range []string{"plugin_idc_core_product_automation_interface_instance", "plugin_idc_core_product_automation_interface_configuration_revision"} {
		count, countErr := scoped.GetValue(ctx, "SELECT count(*) FROM "+table+" WHERE interface_instance_no=?", unused.InterfaceInstanceNo)
		if countErr != nil || count.Int() != 1 {
			t.Fatalf("delete rollback lost %s: count=%v err=%v", table, count, countErr)
		}
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE interface_instance_no=?`, unused.InterfaceInstanceNo); err == nil {
		t.Fatal("direct deletion must preserve append-only configuration history")
	}
	if err = unusedService.DeleteInterface(ctx, DeleteInterfaceInput{InterfaceInstanceNo: unused.InterfaceInstanceNo, ExpectedVersion: 1}); err != nil {
		t.Fatalf("delete unused interface with its initial configuration: %v", err)
	}
	for _, table := range []string{"plugin_idc_core_product_automation_interface_instance", "plugin_idc_core_product_automation_interface_configuration_revision"} {
		count, countErr := scoped.GetValue(ctx, "SELECT count(*) FROM "+table+" WHERE interface_instance_no=?", unused.InterfaceInstanceNo)
		if countErr != nil || count.Int() != 0 {
			t.Fatalf("unused interface cleanup left %s: count=%v err=%v", table, count, countErr)
		}
	}
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_instance
(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state,version,created_by,changed_by)
VALUES (0,?,'直接 SQL 接口',?,'product-fulfillment',?,?,ARRAY[]::TEXT[],?,?,'accepting',1,101,101)`, directInterfaceNo, evidence.PluginID, evidence.ReleaseVersion, evidence.ContractVersion, evidence.ReleaseDigest, evidence.DeclarationDigest); err != nil {
		t.Fatal(err)
	}
	bindTx, err := contender.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bindTx.Exec(`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
(tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by)
VALUES (0,'ver_55555555555555555555555555555555',?,'直接 SQL 接口',?,'product-fulfillment',?,?,ARRAY[]::TEXT[],?,?,101)`, directInterfaceNo, evidence.PluginID, evidence.ReleaseVersion, evidence.ContractVersion, evidence.ReleaseDigest, evidence.DeclarationDigest); err != nil {
		t.Fatal(err)
	}
	deleteResult := make(chan error, 1)
	go func() {
		deleteResult <- service.DeleteInterface(ctx, DeleteInterfaceInput{InterfaceInstanceNo: directInterfaceNo, ExpectedVersion: 1})
	}()
	select {
	case deleteErr := <-deleteResult:
		t.Fatalf("delete returned before direct binding released its lock: %v", deleteErr)
	case <-time.After(200 * time.Millisecond):
	}
	if err = bindTx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case deleteErr := <-deleteResult:
		if !bizerr.Is(deleteErr, CodeInterfaceEverReferenced) {
			t.Fatalf("concurrent first binding must make interface permanently undeletable: %v", deleteErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("delete did not finish after direct binding committed")
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_product_version_fulfillment_binding WHERE version_no='ver_55555555555555555555555555555555'`); err != nil {
		t.Fatal(err)
	}
	marker, markerErr := scoped.GetValue(ctx, `SELECT ever_referenced FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, directInterfaceNo)
	if markerErr != nil || !marker.Bool() {
		t.Fatalf("direct SQL binding did not persist irreversible reference marker: marker=%v err=%v", marker.Bool(), markerErr)
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, directInterfaceNo); err == nil {
		t.Fatal("database guard allowed deleting a directly referenced then unbound interface")
	}

	publishTx, err := contender.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	t.Cleanup(func() {
		if !committed {
			if rollbackErr := publishTx.Rollback(); rollbackErr != nil {
				t.Errorf("rollback publication transaction: %v", rollbackErr)
			}
		}
	})
	if _, err = publishTx.Exec(`UPDATE plugin_idc_core_catalog_product_version SET status_code='published'
WHERE version_no='ver_22222222222222222222222222222222'`); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	bindResult := make(chan error, 1)
	go func() {
		close(started)
		_, bindErr := service.Bind(ctx, BindInput{VersionNo: "ver_22222222222222222222222222222222", InterfaceInstanceNo: second.InterfaceInstanceNo})
		bindResult <- bindErr
	}()
	<-started
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case bindErr := <-bindResult:
		t.Fatalf("binding returned before publication released the version lock: %v", bindErr)
	case <-timer.C:
	}
	if err = publishTx.Commit(); err != nil {
		t.Fatal(err)
	}
	committed = true
	select {
	case bindErr := <-bindResult:
		if !bizerr.Is(bindErr, catalog.CodeManagementConflict) {
			t.Fatalf("concurrent publication must reject binding with conflict, got %v", bindErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("binding did not finish after publication committed")
	}
}

func TestMigrationBackfillsIrreversibleInterfaceReferences(t *testing.T) {
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
	schema := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schema)+` CASCADE`)
	database := openAutomationDatabase(t, ctx, dsn, schema)
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version (tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL PRIMARY KEY,status_code VARCHAR(16) NOT NULL,UNIQUE(tenant_id,version_no)); INSERT INTO plugin_idc_core_catalog_product_version VALUES (0,'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','draft');`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "074-product-automation-binding.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_instance (tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state,ever_referenced,version,created_by,changed_by) VALUES (0,'pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','旧接口','ssl-provider','product-fulfillment','1.0.0','v1',ARRAY[]::TEXT[],repeat('a',64),repeat('b',64),'accepting',FALSE,1,101,101); DROP TRIGGER trg_idc_validate_fulfillment_binding ON plugin_idc_core_catalog_product_version_fulfillment_binding; INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding (tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by) VALUES (0,'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','旧接口','ssl-provider','product-fulfillment','1.0.0','v1',ARRAY[]::TEXT[],repeat('a',64),repeat('b',64),101);`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("reapply migration for legacy backfill: %v", err)
	}
	value, err := database.GetValue(ctx, `SELECT ever_referenced::text||':'||version::text FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no='pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`)
	if err != nil || value.String() != "true:2" {
		t.Fatalf("legacy binding backfill=%q err=%v", value.String(), err)
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_product_version_fulfillment_binding WHERE version_no='ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'; DELETE FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no='pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';`); err == nil {
		t.Fatal("backfilled interface became deletable after unbind")
	}
}

func newAutomationSchema(t *testing.T) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	return "idc_product_automation_service_" + hex.EncodeToString(suffix[:])
}

func openAutomationDatabase(t *testing.T, ctx context.Context, dsn, schema string) gdb.DB {
	t.Helper()
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close product automation database: %v", closeErr)
		}
	})
	return database
}

func quoteAutomationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
