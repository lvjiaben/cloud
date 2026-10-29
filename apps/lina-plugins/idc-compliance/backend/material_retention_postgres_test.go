// This file verifies that retiring governance preserves mandatory material retention, manual deletion and legal holds.
package backend

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type privacyTestAuthorizer struct {
	permissions []string
	deny        bool
}

func (a *privacyTestAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if a.deny {
		return 0, bizerr.NewCode(privacygovernance.CodeNotAccessible)
	}
	if err := managementcap.ValidateAuthorizationRequest(request); err != nil {
		return 0, err
	}
	a.permissions = append(a.permissions, request.Permission)
	return 87, nil
}

type privacyTestAudit struct {
	transitions []idccap.BusinessAuditTransition
}

type privacyStorageStub struct {
	storagecap.Service
	deleted []string
	fail    bool
}

func (s *privacyStorageStub) Delete(_ context.Context, input storagecap.DeleteInput) error {
	if s.fail {
		return errors.New("test storage removal failed")
	}
	s.deleted = append(s.deleted, input.Path)
	return nil
}

type privacySecretManagerStub struct {
	secretcap.Manager
	revoked           bool
	retainAfterRevoke bool
}

func (s *privacySecretManagerStub) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{Configured: !s.revoked, UpdatedAt: time.Unix(1, 0)}, nil
}
func (s *privacySecretManagerStub) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	s.revoked = !s.retainAfterRevoke
	return secretcap.Metadata{Configured: !s.revoked}, nil
}

type privacyCustomerReaderStub struct {
	subject *identitycap.CustomerSubject
}
type privacyConsentServiceStub struct {
	decisions []identitycap.PrivacyConsentDecision
}

func (s *privacyCustomerReaderStub) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	if s.subject == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeNotAccessible)
	}
	copyValue := *s.subject
	return &copyValue, nil
}

func (s *privacyConsentServiceStub) ListCurrentPrivacyConsents(context.Context) ([]identitycap.PrivacyConsentDecision, error) {
	return append([]identitycap.PrivacyConsentDecision(nil), s.decisions...), nil
}

func (s *privacyConsentServiceStub) WithdrawCurrentPrivacyConsent(
	_ context.Context,
	input identitycap.PrivacyConsentWithdrawalInput,
) (*identitycap.PrivacyConsentDecision, error) {
	decision := identitycap.PrivacyConsentDecision{
		ConsentNo: "CNS_WITHDRAWN_TEST_0001", ConsentType: input.ConsentType,
		PolicyReference: "idc://compliance/privacy-test", PolicyVersion: "v1",
		ActionCode: "withdrawn", Locale: "zh-CN",
	}
	s.decisions = []identitycap.PrivacyConsentDecision{decision}
	return &decision, nil
}

func (a *privacyTestAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	a.transitions = append(a.transitions, transition)
	return nil
}

func (*privacyTestAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestMaterialRetentionPostgreSQLWithoutGovernanceAssessment(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(complianceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL privacy governance tests", complianceIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create privacy governance admin database: %v", err)
	}
	defer adminDB.Close(ctx)
	schema := newComplianceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)); err != nil {
		t.Fatalf("create privacy governance schema: %v", err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "idc_compliance_test_") {
			t.Errorf("refusing unexpected privacy governance schema %q", schema)
			return
		}
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCompliancePostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop privacy governance schema: %v", dropErr)
		}
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendComplianceDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create privacy governance scoped database: %v", err)
	}
	defer database.Close(ctx)
	createComplianceDictionaryFixtures(t, ctx, database)
	executeComplianceMigrations(t, ctx, database)
	// Exercise upgrade from the historical target constraints, including archived evidence.
	for _, target := range [][2]string{{"plugin_idc_compliance_privacy_retention_hold", "ck_idc_compliance_privacy_hold_target_type"}, {"plugin_idc_compliance_privacy_deletion_task", "ck_idc_compliance_privacy_deletion_target_type"}} {
		if _, err = database.Exec(ctx, `ALTER TABLE `+target[0]+` DROP CONSTRAINT `+target[1]); err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, `ALTER TABLE `+target[0]+` ADD CONSTRAINT `+target[1]+` CHECK (target_type_code IN ('personal_verification_case','enterprise_verification_material','electronic_contract_file','tax_invoice_file','sensitive_access_audit','local_sensitive_export'))`); err != nil {
			t.Fatal(err)
		}
	}
	// These are the pre-retirement CHECK definitions, not the freshly installed confirmed-capable checks.
	for _, constraint := range []struct{ table, name, expression string }{
		{"plugin_idc_compliance_privacy_deletion_task", "ck_idc_compliance_privacy_deletion_status", `status_code IN ('pending','blocked','completed')`},
		{"plugin_idc_compliance_privacy_deletion_task", "ck_idc_compliance_privacy_deletion_state", `(status_code='pending' AND hold_reason_code='' AND confirmed_at IS NULL AND completed_at IS NULL) OR (status_code='blocked' AND hold_reason_code<>'' AND completed_at IS NULL) OR (status_code='completed' AND hold_reason_code='' AND confirmed_by_user_id>0 AND confirmed_at IS NOT NULL AND completed_at IS NOT NULL)`},
		{"plugin_idc_compliance_privacy_deletion_task", "ck_idc_compliance_privacy_deletion_data_type", `data_type_code IN ('personal_verification_material','enterprise_verification_material','electronic_contract_evidence','tax_invoice_evidence','sensitive_access_audit','local_sensitive_export')`},
	} {
		if _, err = database.Exec(ctx, `ALTER TABLE `+constraint.table+` DROP CONSTRAINT `+constraint.name); err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, `ALTER TABLE `+constraint.table+` ADD CONSTRAINT `+constraint.name+` CHECK (`+constraint.expression+`)`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_deletion_task(task_no,data_type_code,target_type_code,target_no,status_code,due_at) VALUES('PDT_retired_history','electronic_contract_evidence','electronic_contract_file','ECF_retired_history','pending',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	replayComplianceMigrationAfterCurrentSchema(t, ctx, database, "029-enterprise-snapshot-retention.sql")
	replayComplianceMigrationAfterCurrentSchema(t, ctx, database, "029-enterprise-snapshot-retention.sql")
	if count, loadErr := database.Model("plugin_idc_compliance_privacy_deletion_task").Ctx(ctx).Where("task_no", "PDT_retired_history").Count(); loadErr != nil || count != 1 {
		t.Fatalf("historical task lost: count=%d error=%v", count, loadErr)
	}

	authorizer := new(privacyTestAuthorizer)
	audit := new(privacyTestAudit)
	secrets := &privacySecretManagerStub{}
	storage := &privacyStorageStub{}
	service, err := privacygovernance.NewComplete(database, managementcap.Runtime{Authorizer: authorizer, Audit: audit}, new(privacyCustomerReaderStub), new(privacyConsentServiceStub), audit, storage, secrets)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RequireSensitiveCollection(ctx, "personal_verification", "personal_verification_material"); !bizerr.Is(err, privacygovernance.CodePolicyRequired) {
		t.Fatalf("missing policy error=%v", err)
	}
	if _, err = service.SetRetentionPolicy(ctx, privacygovernance.RetentionPolicyInput{DataType: privacygovernance.DataPersonalVerificationMaterial, RetentionDays: 365, IdempotencyKey: "retention-test-policy-0001"}); err != nil {
		t.Fatal(err)
	}
	until, err := service.RequireSensitiveCollection(ctx, "personal_verification", "personal_verification_material")
	if err != nil || until.IsZero() {
		t.Fatalf("retention without retired assessment: until=%v error=%v", until, err)
	}
	if _, err = service.RequireSensitiveCollection(ctx, "enterprise_verification", "personal_verification_material"); !bizerr.Is(err, privacygovernance.CodePolicyRequired) {
		t.Fatalf("cross-category collection error=%v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case (case_no,customer_no,principal_no,subject_type_code,provider_code,consent_no,status_code,verified_at,masked_name,masked_identity_no,retention_until) VALUES ('PVC_retention_test','CUST_retention_test','PRN_retention_test','personal','aliyun_face','CONS_retention_test','verified',CURRENT_TIMESTAMP-INTERVAL '2 days','*某','110**********001',CURRENT_TIMESTAMP-INTERVAL '1 day')`); err != nil {
		t.Fatal(err)
	}
	created, err := service.ScanDueDeletionTasks(ctx, 100)
	if err != nil || created != 1 {
		t.Fatalf("created=%d error=%v", created, err)
	}
	tasks, err := service.ListDeletionTasks(ctx, 1, 20)
	if err != nil || len(tasks.Items) != 1 || tasks.Items[0].StatusCode != "pending" {
		t.Fatalf("tasks=%+v error=%v", tasks, err)
	}
	secondPage, pageErr := service.ListDeletionTasks(ctx, 2, 20)
	if pageErr != nil || secondPage.Total != 1 || len(secondPage.Items) != 0 {
		t.Fatalf("second page=%+v error=%v", secondPage, pageErr)
	}
	// A queued first record must not consume every subsequent scan's bounded page.
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case (case_no,customer_no,principal_no,subject_type_code,provider_code,consent_no,status_code,retention_until) VALUES ('PVC_retention_second','CUST_retention_second','PRN_retention_second','personal','aliyun_face','CONS_retention_second','expired',CURRENT_TIMESTAMP-INTERVAL '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if created, err = service.ScanDueDeletionTasks(ctx, 1); err != nil || created != 1 {
		t.Fatalf("second bounded scan created=%d error=%v", created, err)
	}
	// Confirmation must re-evaluate current expiry, not trust a stale queue entry.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_verification_case SET retention_until=CURRENT_TIMESTAMP+INTERVAL '1 day' WHERE case_no='PVC_retention_test'`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: tasks.Items[0].TaskNo, ExpectedVersion: tasks.Items[0].Version, Confirmed: true, IdempotencyKey: "retention-not-due-0001"}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("not-due deletion error=%v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_verification_case SET retention_until=CURRENT_TIMESTAMP-INTERVAL '1 day',legal_profile_snapshot_ref='retention-test-snapshot' WHERE case_no='PVC_retention_test'`); err != nil {
		t.Fatal(err)
	}
	if err = service.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: "personal_verification_case", TargetNo: "PVC_retention_test", ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: "retention-test-hold-0001"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: tasks.Items[0].TaskNo, ExpectedVersion: tasks.Items[0].Version, Confirmed: true, IdempotencyKey: "retention-test-delete-0001"}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("held deletion error=%v", err)
	}
	tasks, err = service.ListDeletionTasks(ctx, 1, 20)
	if err != nil || tasks.Items[0].StatusCode != "blocked" || tasks.Items[0].HoldReasonCode != "legal_hold" {
		t.Fatalf("held tasks=%+v error=%v", tasks, err)
	}
	if err = service.ReleaseRetentionHold(ctx, privacygovernance.RetentionHoldReleaseInput{TargetTypeCode: "personal_verification_case", TargetNo: "PVC_retention_test", ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: "retention-test-release-0001"}); err != nil {
		t.Fatal(err)
	}
	// The management owner cannot release tax or another business owner's protection.
	if err = service.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: "personal_verification_case", TargetNo: "PVC_retention_test", ReasonCode: "tax", OwnerCode: "tax.retention", IdempotencyKey: "retention-tax-hold-0001"}); err != nil {
		t.Fatal(err)
	}
	if err = service.ReleaseRetentionHold(ctx, privacygovernance.RetentionHoldReleaseInput{TargetTypeCode: "personal_verification_case", TargetNo: "PVC_retention_test", ReasonCode: "tax", OwnerCode: "identity.retention", IdempotencyKey: "retention-tax-release-wrong"}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("foreign owner release error=%v", err)
	}
	if err = service.ReleaseRetentionHold(ctx, privacygovernance.RetentionHoldReleaseInput{TargetTypeCode: "personal_verification_case", TargetNo: "PVC_retention_test", ReasonCode: "tax", OwnerCode: "tax.retention", IdempotencyKey: "retention-tax-release-right"}); err != nil {
		t.Fatal(err)
	}
	tasks, err = service.ListDeletionTasks(ctx, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	secrets.retainAfterRevoke = true
	if _, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: tasks.Items[0].TaskNo, ExpectedVersion: tasks.Items[0].Version, Confirmed: true, IdempotencyKey: "retention-revoke-still-live"}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("unrevoked snapshot error=%v", err)
	}
	secrets.retainAfterRevoke = false
	tasks, err = service.ListDeletionTasks(ctx, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: tasks.Items[0].TaskNo, ExpectedVersion: tasks.Items[0].Version, Confirmed: true, IdempotencyKey: "retention-test-delete-0002"})
	if err != nil || result.StatusCode != "completed" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if !secrets.revoked {
		t.Fatal("private snapshot was not revoked")
	}
	var receipt []struct {
		StatusCode string     `json:"status_code"`
		MaskedName string     `json:"masked_name"`
		VerifiedAt *time.Time `json:"verified_at"`
	}
	if err = database.GetScan(ctx, &receipt, `SELECT status_code,masked_name,verified_at FROM plugin_idc_compliance_verification_case WHERE case_no='PVC_retention_test'`); err != nil || len(receipt) != 1 || receipt[0].StatusCode != "verified" || receipt[0].MaskedName != "*某" || receipt[0].VerifiedAt == nil {
		t.Fatalf("material disposal changed valid identity receipt: %+v error=%v", receipt, err)
	}
	// Enterprise encrypted legal snapshots share the material lifecycle, but use storage deletion.
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_compliance_enterprise_encryption_key(key_no,secret_ref) VALUES('enterprise_material_key_v1','retention_key_ref')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,status_code,consent_no,legal_profile_snapshot_ref,legal_profile_sha256,request_idempotency_key_digest,request_fingerprint,encryption_key_no,snapshot_stored_byte_size,retention_until) VALUES('EVC_retention_test','CUST_retention_enterprise','PRN_retention_enterprise','enterprise','manual_enterprise','canceled','CONS_retention_enterprise','private/enterprise/retention-snapshot',repeat('a',64),repeat('b',64),repeat('c',64),'enterprise_material_key_v1',128,CURRENT_TIMESTAMP-INTERVAL '2 days')`); err != nil {
		t.Fatal(err)
	}
	if created, err = service.ScanDueDeletionTasks(ctx, 1); err != nil || created != 1 {
		t.Fatalf("enterprise snapshot scan created=%d error=%v", created, err)
	}
	tasks, err = service.ListDeletionTasks(ctx, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	var enterprise privacygovernance.DeletionTask
	for _, item := range tasks.Items {
		if item.TargetTypeCode == "enterprise_verification_case" {
			enterprise = item
		}
	}
	if enterprise.TaskNo == "" {
		t.Fatal("enterprise snapshot not reachable in material directory")
	}
	if err = service.PlaceRetentionHold(ctx, privacygovernance.RetentionHoldInput{TargetTypeCode: "personal_verification_case", TargetNo: enterprise.TargetNo, ReasonCode: "legal_hold", OwnerCode: "identity.retention", IdempotencyKey: "retention-mismatched-type"}); !bizerr.Is(err, privacygovernance.CodeNotAccessible) {
		t.Fatalf("mismatched target hold error=%v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_verification_case SET legal_hold=TRUE WHERE case_no='EVC_retention_test'`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: enterprise.TaskNo, ExpectedVersion: enterprise.Version, Confirmed: true, IdempotencyKey: "enterprise-retention-held"}); !bizerr.Is(err, privacygovernance.CodeConflict) {
		t.Fatalf("held enterprise snapshot error=%v", err)
	}
	if len(storage.deleted) != 0 {
		t.Fatal("held enterprise snapshot was removed")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_verification_case SET legal_hold=FALSE WHERE case_no='EVC_retention_test'`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_deletion_task SET status_code='pending',hold_reason_code='',version=version+1 WHERE task_no=?`, enterprise.TaskNo); err != nil {
		t.Fatal(err)
	}
	tasks, err = service.ListDeletionTasks(ctx, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range tasks.Items {
		if item.TaskNo == enterprise.TaskNo {
			enterprise = item
		}
	}
	storage.fail = true
	if _, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: enterprise.TaskNo, ExpectedVersion: enterprise.Version, Confirmed: true, IdempotencyKey: "enterprise-retention-failed"}); !bizerr.Is(err, privacygovernance.CodeUnavailable) {
		t.Fatalf("failed storage error=%v", err)
	}
	storage.fail = false
	tasks, err = service.ListDeletionTasks(ctx, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range tasks.Items {
		if item.TaskNo == enterprise.TaskNo {
			enterprise = item
		}
	}
	result, err = service.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: enterprise.TaskNo, ExpectedVersion: enterprise.Version, Confirmed: true, IdempotencyKey: "enterprise-retention-success"})
	if err != nil || result.StatusCode != "completed" || len(storage.deleted) != 1 || storage.deleted[0] != "private/enterprise/retention-snapshot" {
		t.Fatalf("enterprise removal result=%+v removed=%v error=%v", result, storage.deleted, err)
	}
	authorizer.deny = true
	if _, err = service.ListDeletionTasks(ctx, 1, 20); !bizerr.Is(err, privacygovernance.CodeNotAccessible) {
		t.Fatalf("unauthorized list error=%v", err)
	}
	if _, err = service.GenerateDueDeletionTasks(ctx, 100); !bizerr.Is(err, privacygovernance.CodeNotAccessible) {
		t.Fatalf("unauthorized scan error=%v", err)
	}
	if count, loadErr := database.Model("plugin_idc_compliance_privacy_deletion_task").Ctx(ctx).Where("task_no", "PDT_retired_history").Where("data_type_code", "electronic_contract_evidence").Count(); loadErr != nil || count != 1 {
		t.Fatalf("upgrade or recovery changed historical evidence: count=%d error=%v", count, loadErr)
	}
}
