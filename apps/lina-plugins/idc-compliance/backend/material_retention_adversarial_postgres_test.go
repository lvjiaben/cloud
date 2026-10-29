// These tests challenge retained-material transactions with independent PostgreSQL schemas and storage faults.
package backend

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type retentionAdversarialAuthorizer struct{ actor int }

func (a retentionAdversarialAuthorizer) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	return a.actor, managementcap.ValidateAuthorizationRequest(request)
}

type retentionAdversarialStore struct {
	storagecap.Service
	mu      sync.Mutex
	removed bool
	calls   int
}

func (s *retentionAdversarialStore) Delete(context.Context, storagecap.DeleteInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.removed = true // The host local provider treats an already absent object as successful deletion.
	return nil
}

type retentionAdversarialAudit struct {
	database       gdb.DB
	enter          chan struct{}
	resume         chan struct{}
	blockOperation string
	failOperation  string
}

func (a *retentionAdversarialAudit) AppendBusinessTransition(ctx context.Context, input idccap.BusinessAuditTransition) error {
	if input.OperationCode == a.failOperation {
		return errors.New("injected audit failure")
	}
	if input.OperationCode == a.blockOperation && a.enter != nil {
		close(a.enter)
		select {
		case <-a.resume:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_, err := a.database.Exec(ctx, `INSERT INTO retention_test_audit(operation_code,target_no) VALUES(?,?)`, input.OperationCode, input.TargetNo)
	return err
}

func (*retentionAdversarialAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type retentionAdversarialFixture struct {
	database   gdb.DB
	service    privacygovernance.Service
	storage    *retentionAdversarialStore
	audit      *retentionAdversarialAudit
	schema     string
	authorizer *retentionAdversarialAuthorizer
}

// retentionFirstScanGateDB pauses exact database boundaries without injecting production hooks.
type retentionFirstScanGateDB struct {
	gdb.DB
	taskRead          chan struct{}
	resumeTaskRead    chan struct{}
	approvalCommitted chan struct{}
	resumeApproval    chan struct{}
	readOnce          sync.Once
	commitOnce        sync.Once
}

func (d *retentionFirstScanGateDB) GetScan(ctx context.Context, output any, query string, args ...any) error {
	if err := d.DB.GetScan(ctx, output, query, args...); err != nil {
		return err
	}
	if d.taskRead != nil && strings.Contains(query, "WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? FOR UPDATE") {
		d.readOnce.Do(func() {
			close(d.taskRead)
			select {
			case <-d.resumeTaskRead:
			case <-ctx.Done():
			}
		})
	}
	return ctx.Err()
}

func (d *retentionFirstScanGateDB) Transaction(ctx context.Context, callback func(context.Context, gdb.TX) error) error {
	if err := d.DB.Transaction(ctx, callback); err != nil {
		return err
	}
	if d.approvalCommitted != nil {
		d.commitOnce.Do(func() {
			close(d.approvalCommitted)
			select {
			case <-d.resumeApproval:
			case <-ctx.Done():
			}
		})
	}
	return ctx.Err()
}

func (f retentionAdversarialFixture) withDatabase(t *testing.T, database gdb.DB) privacygovernance.Service {
	t.Helper()
	service, err := privacygovernance.NewComplete(database, managementcap.Runtime{Authorizer: f.authorizer, Audit: f.audit}, new(privacyCustomerReaderStub), new(privacyConsentServiceStub), f.audit, f.storage, &privacySecretManagerStub{})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func newRetentionAdversarialFixture(t *testing.T) retentionAdversarialFixture {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(complianceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s for adversarial PostgreSQL verification", complianceIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newComplianceIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendComplianceDSNParameter(appendComplianceDSNParameter(dsn, "search_path="+schema), "application_name="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	createComplianceDictionaryFixtures(t, ctx, database)
	executeComplianceMigrations(t, ctx, database)
	if _, err = database.Exec(ctx, `CREATE TABLE retention_test_audit(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,operation_code TEXT NOT NULL,target_no TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	store := &retentionAdversarialStore{}
	audit := &retentionAdversarialAudit{database: database}
	authorizer := &retentionAdversarialAuthorizer{actor: 87}
	service, err := privacygovernance.NewComplete(database, managementcap.Runtime{Authorizer: authorizer, Audit: audit}, new(privacyCustomerReaderStub), new(privacyConsentServiceStub), audit, store, &privacySecretManagerStub{})
	if err != nil {
		t.Fatal(err)
	}
	return retentionAdversarialFixture{database: database, service: service, storage: store, audit: audit, schema: schema, authorizer: authorizer}
}

func (f retentionAdversarialFixture) enterpriseTask(t *testing.T) privacygovernance.DeletionTask {
	t.Helper()
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_enterprise_encryption_key(key_no,secret_ref) VALUES('enterprise_material_key_v1','retention_adversarial_key')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,status_code,consent_no,legal_profile_snapshot_ref,legal_profile_sha256,request_idempotency_key_digest,request_fingerprint,encryption_key_no,snapshot_stored_byte_size,retention_until) VALUES('EVC_adversarial_test','CUS_adversarial','PRN_adversarial','enterprise','manual_enterprise','canceled','CONS_adversarial','private/adversarial/snapshot',repeat('a',64),repeat('b',64),repeat('c',64),'enterprise_material_key_v1',128,CURRENT_TIMESTAMP-INTERVAL '1 day')`); err != nil {
		t.Fatal(err)
	}
	if created, err := f.service.ScanDueDeletionTasks(ctx, 100); err != nil || created != 1 {
		t.Fatalf("scan created=%d error=%v", created, err)
	}
	return f.task(t)
}

func (f retentionAdversarialFixture) task(t *testing.T) privacygovernance.DeletionTask {
	t.Helper()
	page, err := f.service.ListDeletionTasks(context.Background(), 1, 20)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("task page=%+v error=%v", page, err)
	}
	return page.Items[0]
}

func TestRetentionAdversarialMultiHoldPriorityPostgreSQL(t *testing.T) {
	f := newRetentionAdversarialFixture(t)
	task := f.enterpriseTask(t)
	ctx := context.Background()
	for _, reason := range []string{"tax", "legal_hold"} {
		if err := f.service.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: task.TargetTypeCode, TargetNo: task.TargetNo, ReasonCode: reason, OwnerCode: "identity.retention", IdempotencyKey: "adversarial-place-" + reason}); err != nil {
			t.Fatal(err)
		}
	}
	held := f.task(t)
	if held.StatusCode != "blocked" || held.HoldReasonCode != "legal_hold" {
		t.Fatalf("highest protection not projected: %+v", held)
	}
	if err := f.service.ReleaseRetentionHold(ctx, privacygovernance.RetentionHoldReleaseInput{TargetTypeCode: task.TargetTypeCode, TargetNo: task.TargetNo, ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: "adversarial-release-legal"}); err != nil {
		t.Fatal(err)
	}
	held = f.task(t)
	if held.StatusCode != "blocked" || held.HoldReasonCode != "tax" {
		t.Fatalf("remaining protection lost: %+v", held)
	}
	if _, err := f.service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: held.TaskNo, ExpectedVersion: held.Version, Confirmed: true, IdempotencyKey: "adversarial-held-delete"}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("held deletion error=%v", err)
	}
	if f.storage.calls != 0 {
		t.Fatal("protected original reached storage deletion")
	}
}

// SQL-level failure occurs after storage deletion, so retry must retain one final transition and the history.
func TestRetentionAdversarialStorageThenDatabaseFailurePostgreSQL(t *testing.T) {
	f := newRetentionAdversarialFixture(t)
	task := f.enterpriseTask(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, `CREATE TABLE retention_test_fault(enabled BOOLEAN NOT NULL); INSERT INTO retention_test_fault VALUES(TRUE); CREATE FUNCTION retention_test_fail_completion() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.status_code='completed' AND (SELECT enabled FROM retention_test_fault) THEN RAISE EXCEPTION 'injected completion failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER retention_test_completion_failure BEFORE UPDATE ON plugin_idc_compliance_privacy_deletion_task FOR EACH ROW EXECUTE FUNCTION retention_test_fail_completion()`); err != nil {
		t.Fatal(err)
	}
	input := privacygovernance.DeletionConfirmInput{TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true, IdempotencyKey: "adversarial-recover-delete"}
	if _, err := f.service.ConfirmDeletion(ctx, input); !bizerr.Is(err, privacygovernance.CodeUnavailable) {
		t.Fatalf("fault error=%v", err)
	}
	if !f.storage.removed || f.storage.calls != 1 {
		t.Fatal("fault did not occur after external removal")
	}
	confirmed := f.task(t)
	if confirmed.StatusCode != "confirmed" || confirmed.ConfirmedByUserID != 87 || confirmed.ConfirmedAt == nil {
		t.Fatalf("durable approval lost: %+v", confirmed)
	}
	if count, err := f.database.Model("retention_test_audit").Ctx(ctx).Where("operation_code", "compliance.privacy.deletion-authorize").Count(); err != nil || count != 1 {
		t.Fatalf("durable confirmation audit count=%d error=%v", count, err)
	}
	if _, err := f.database.Exec(ctx, `UPDATE retention_test_fault SET enabled=FALSE`); err != nil {
		t.Fatal(err)
	}
	result, err := f.service.ConfirmDeletion(ctx, input)
	if err != nil || result.StatusCode != "completed" {
		t.Fatalf("recovery result=%+v error=%v", result, err)
	}
	if result.ConfirmedByUserID != confirmed.ConfirmedByUserID || !result.ConfirmedAt.Equal(*confirmed.ConfirmedAt) {
		t.Fatal("recovery replaced the original approval")
	}
	if _, err = f.service.ConfirmDeletion(ctx, input); err != nil {
		t.Fatal(err)
	}
	count, err := f.database.Model("retention_test_audit").Ctx(ctx).Where("operation_code", "compliance.privacy.deletion-confirm").Count()
	if err != nil || count != 1 {
		t.Fatalf("completion audit count=%d error=%v", count, err)
	}
}

// waitForBlockedRequest observes a real PostgreSQL lock waiter instead of guessing timing with sleeps.
func (f retentionAdversarialFixture) waitForBlockedRequest(t *testing.T, ctx context.Context) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		value, err := f.database.GetValue(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE application_name=? AND wait_event_type='Lock' AND pid<>pg_backend_pid()`, f.schema)
		if err != nil {
			t.Fatal(err)
		}
		if value.Int() > 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("no PostgreSQL lock waiter: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestRetentionAdversarialApprovalAndCompletionAuditFailuresPostgreSQL(t *testing.T) {
	for _, operation := range []string{"compliance.privacy.deletion-authorize", "compliance.privacy.deletion-confirm"} {
		t.Run(operation, func(t *testing.T) {
			f := newRetentionAdversarialFixture(t)
			task := f.enterpriseTask(t)
			ctx := context.Background()
			f.audit.failOperation = operation
			input := privacygovernance.DeletionConfirmInput{TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true, IdempotencyKey: "adversarial-audit-fault"}
			if _, err := f.service.ConfirmDeletion(ctx, input); !bizerr.Is(err, privacygovernance.CodeUnavailable) {
				t.Fatalf("audit fault error=%v", err)
			}
			current := f.task(t)
			if operation == "compliance.privacy.deletion-authorize" {
				if f.storage.calls != 0 || current.StatusCode != "pending" {
					t.Fatalf("uncommitted approval removed original: calls=%d task=%+v", f.storage.calls, current)
				}
			} else if !f.storage.removed || current.StatusCode != "confirmed" {
				t.Fatalf("completion failure lost durable intent: %+v", current)
			}
			f.audit.failOperation = ""
			f.authorizer.actor = 98
			input.ExpectedVersion = current.Version
			input.IdempotencyKey = "adversarial-audit-recovery"
			result, err := f.service.ConfirmDeletion(ctx, input)
			if err != nil || result.StatusCode != "completed" {
				t.Fatalf("recovery=%+v error=%v", result, err)
			}
			if operation == "compliance.privacy.deletion-confirm" && (result.ConfirmedByUserID != current.ConfirmedByUserID || !result.ConfirmedAt.Equal(*current.ConfirmedAt)) {
				t.Fatal("another administrator's recovery replaced the approver")
			}
			for _, code := range []string{"compliance.privacy.deletion-authorize", "compliance.privacy.deletion-confirm"} {
				count, loadErr := f.database.Model("retention_test_audit").Ctx(ctx).Where("operation_code", code).Count()
				if loadErr != nil || count != 1 {
					t.Fatalf("audit %s count=%d error=%v", code, count, loadErr)
				}
			}
		})
	}
}

func TestRetentionAdversarialHoldAndApprovalLinearizationPostgreSQL(t *testing.T) {
	for _, holdFirst := range []bool{true, false} {
		name := "approval-first"
		if holdFirst {
			name = "hold-first"
		}
		t.Run(name, func(t *testing.T) {
			f := newRetentionAdversarialFixture(t)
			task := f.enterpriseTask(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			f.audit.enter = make(chan struct{})
			f.audit.resume = make(chan struct{})
			f.audit.blockOperation = "compliance.privacy.deletion-authorize"
			if holdFirst {
				f.audit.blockOperation = "compliance.privacy.place_retention_hold"
			}
			hold := func() error {
				return f.service.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: task.TargetTypeCode, TargetNo: task.TargetNo, ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: "adversarial-concurrent-hold"})
			}
			confirm := func() error {
				_, err := f.service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true, IdempotencyKey: "adversarial-concurrent-confirm"})
				return err
			}
			first, second := make(chan error, 1), make(chan error, 1)
			if holdFirst {
				go func() { first <- hold() }()
			} else {
				go func() { first <- confirm() }()
			}
			select {
			case <-f.audit.enter:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if holdFirst {
				go func() { second <- confirm() }()
			} else {
				go func() { second <- hold() }()
			}
			f.waitForBlockedRequest(t, ctx)
			close(f.audit.resume)
			if err := <-first; err != nil {
				t.Fatalf("first operation failed: %v", err)
			}
			if err := <-second; !bizerr.Is(err, privacygovernance.CodeConflict) {
				t.Fatalf("second operation must conflict: %v", err)
			}
			if holdFirst && f.storage.calls != 0 {
				t.Fatal("committed hold did not prevent deletion")
			}
			if !holdFirst {
				count, err := f.database.Model("plugin_idc_compliance_privacy_retention_hold").Ctx(ctx).Where("status_code", "active").Count()
				if err != nil || count != 0 {
					t.Fatalf("new hold accepted after approval: count=%d error=%v", count, err)
				}
			}
		})
	}
}

func TestRetentionAdversarialConcurrentConfirmAndResumePostgreSQL(t *testing.T) {
	for _, resume := range []bool{false, true} {
		name := "confirm"
		if resume {
			name = "resume"
		}
		t.Run(name, func(t *testing.T) {
			f := newRetentionAdversarialFixture(t)
			task := f.enterpriseTask(t)
			ctx := context.Background()
			input := privacygovernance.DeletionConfirmInput{TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true, IdempotencyKey: "adversarial-parallel-confirm"}
			if resume {
				f.audit.failOperation = "compliance.privacy.deletion-confirm"
				if _, err := f.service.ConfirmDeletion(ctx, input); !bizerr.Is(err, privacygovernance.CodeUnavailable) {
					t.Fatalf("prepare failed completion: %v", err)
				}
				task = f.task(t)
				f.authorizer.actor = 98
				input.ExpectedVersion = task.Version
				input.IdempotencyKey = "adversarial-parallel-resume"
				f.audit.failOperation = ""
				replayComplianceMigrationAfterCurrentSchema(t, ctx, f.database, "029-enterprise-snapshot-retention.sql")
				replayComplianceMigrationAfterCurrentSchema(t, ctx, f.database, "029-enterprise-snapshot-retention.sql")
			}
			results := make(chan error, 2)
			start := make(chan struct{})
			for range 2 {
				go func() { <-start; _, err := f.service.ConfirmDeletion(ctx, input); results <- err }()
			}
			close(start)
			for range 2 {
				if err := <-results; err != nil {
					t.Fatal(err)
				}
			}
			completed := f.task(t)
			if completed.StatusCode != "completed" {
				t.Fatalf("not converged: %+v", completed)
			}
			if resume && (completed.ConfirmedByUserID != task.ConfirmedByUserID || !completed.ConfirmedAt.Equal(*task.ConfirmedAt)) {
				t.Fatal("resume changed the approval")
			}
			for _, operation := range []string{"compliance.privacy.deletion-authorize", "compliance.privacy.deletion-confirm"} {
				count, err := f.database.Model("retention_test_audit").Ctx(ctx).Where("operation_code", operation).Count()
				if err != nil || count != 1 {
					t.Fatalf("duplicate or missing %s: count=%d error=%v", operation, count, err)
				}
			}
			if !resume && f.storage.calls != 1 {
				t.Fatalf("duplicate external deletion calls=%d", f.storage.calls)
			}
		})
	}
}

func TestRetentionAdversarialScanBoundariesAndPagesPostgreSQL(t *testing.T) {
	f := newRetentionAdversarialFixture(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,consent_no,status_code,retention_until) SELECT 'PVC_page_'||i,'CUS_page_'||i,'PRN_page_'||i,'personal','aliyun_face','CONS_page_'||i,'expired',CURRENT_TIMESTAMP-INTERVAL '1 day' FROM generate_series(1,525) i`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{500, 25, 0} {
		created, err := f.service.ScanDueDeletionTasks(ctx, 500)
		if err != nil || created != want {
			t.Fatalf("bounded scan created=%d want=%d error=%v", created, want, err)
		}
	}
	seen := make(map[string]bool)
	for page := 1; page <= 11; page++ {
		result, err := f.service.ListDeletionTasks(ctx, page, 50)
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 525 {
			t.Fatalf("page %d total=%d", page, result.Total)
		}
		for _, item := range result.Items {
			if seen[item.TaskNo] {
				t.Fatalf("duplicate page row %s", item.TaskNo)
			}
			seen[item.TaskNo] = true
		}
	}
	if len(seen) != 525 {
		t.Fatalf("pagination lost rows: %d", len(seen))
	}
}

func TestRetentionAdversarialRejectEnclosingTransactionPostgreSQL(t *testing.T) {
	f := newRetentionAdversarialFixture(t)
	task := f.enterpriseTask(t)
	ctx := context.Background()
	err := f.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		_, callErr := f.service.ConfirmDeletion(txCtx, privacygovernance.DeletionConfirmInput{TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true, IdempotencyKey: "adversarial-enclosing-tx"})
		if !bizerr.Is(callErr, privacygovernance.CodeInvalid) {
			t.Fatalf("nested transaction accepted: %v", callErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.storage.calls != 0 || f.task(t).StatusCode != "pending" {
		t.Fatal("uncommitted outer transaction reached external deletion")
	}
}

func TestRetentionAdversarialFirstScanCannotBypassHoldPostgreSQL(t *testing.T) {
	f := newRetentionAdversarialFixture(t)
	original := f.enterpriseTask(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if _, err := f.database.Exec(ctx, `DELETE FROM plugin_idc_compliance_privacy_deletion_task`); err != nil {
		t.Fatal(err)
	}
	holdDB := &retentionFirstScanGateDB{DB: f.database, taskRead: make(chan struct{}), resumeTaskRead: make(chan struct{})}
	confirmDB := &retentionFirstScanGateDB{DB: f.database, approvalCommitted: make(chan struct{}), resumeApproval: make(chan struct{})}
	holder, confirmer := f.withDatabase(t, holdDB), f.withDatabase(t, confirmDB)
	holdResult := make(chan error, 1)
	go func() {
		holdResult <- holder.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: original.TargetTypeCode, TargetNo: original.TargetNo, ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: "adversarial-first-scan-hold"})
	}()
	select {
	case <-holdDB.taskRead:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if created, err := f.service.ScanDueDeletionTasks(ctx, 100); err != nil || created != 1 {
		t.Fatalf("first concurrent scan created=%d error=%v", created, err)
	}
	task := f.task(t)
	confirmResult := make(chan error, 1)
	go func() {
		_, err := confirmer.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: task.TaskNo, ExpectedVersion: task.Version, Confirmed: true, IdempotencyKey: "adversarial-first-scan-confirm"})
		confirmResult <- err
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	observed := false
	for !observed {
		select {
		case <-confirmDB.approvalCommitted:
			observed = true
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			value, err := f.database.GetValue(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE application_name=? AND wait_event_type='Lock' AND pid<>pg_backend_pid()`, f.schema)
			if err != nil {
				t.Fatal(err)
			}
			observed = value.Int() > 0
		}
	}
	close(holdDB.resumeTaskRead)
	if err := <-holdResult; err != nil {
		t.Fatal(err)
	}
	close(confirmDB.resumeApproval)
	if err := <-confirmResult; !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("confirmation bypassed hold: %v", err)
	}
	current := f.task(t)
	if current.StatusCode != "blocked" || current.HoldReasonCode != "legal_hold" {
		t.Fatalf("first scan left an approved task behind an active hold: %+v", current)
	}
	if f.storage.calls != 0 {
		t.Fatal("first scan race reached external deletion")
	}
}
