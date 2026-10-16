// Manual verification is read from the core owner rather than an old enterprise case.
package enterpriseverification

import (
	"context"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"testing"
	"time"
)

func TestCurrentManualVerification(t *testing.T) {
	identity := &enterpriseIdentityFake{subject: identitycap.EnterpriseSubject{CustomerNo: "customer_test", ManualVerification: true, VerificationStatus: identitycap.StatusVerified}}
	service := &serviceImpl{identities: identity}
	value, err := service.Current(context.Background())
	if err != nil || value == nil || value.StatusCode != caseStatusVerified || value.CaseNo != "" {
		t.Fatalf("manual approval: %+v %v", value, err)
	}
}

// Retire only pre-reset in-flight cases; keep encrypted references and never cancel fresh requests.
func TestCancelClearedCasesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := enterpriseVerificationTestDatabase(t, ctx)
	const customerNo = "customer_clear_history_001"
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_compliance_enterprise_encryption_key(key_no,secret_ref) VALUES('enterprise_material_key_v1','opaque_key_ref')`); err != nil {
		t.Fatal(err)
	}
	resetAt := time.Now().UTC().Add(-time.Minute)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,status_code,consent_no,created_at,legal_profile_snapshot_ref,legal_profile_sha256,request_idempotency_key_digest,request_fingerprint,encryption_key_no,snapshot_stored_byte_size) VALUES('case_reset_old_001',?,'usr:7101','enterprise','manual_enterprise','pending','consent_reset_001',?,'encrypted/history/reference',repeat('a',64),repeat('b',64),repeat('c',64),'enterprise_material_key_v1',10)`, customerNo, resetAt.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	writer := &enterpriseAuditFake{}
	service := &serviceImpl{database: database, audit: writer}
	subject := identitycap.EnterpriseSubject{CustomerNo: customerNo, ManualVerification: true, VerificationStatus: identitycap.StatusUnverified, VerificationUpdatedAt: &resetAt}
	if err := service.cancelClearedCases(ctx, subject); err != nil {
		t.Fatal(err)
	}
	var old *entity.VerificationCase
	if err := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{CaseNo: "case_reset_old_001"}).Scan(&old); err != nil {
		t.Fatal(err)
	}
	if old == nil || old.StatusCode != "canceled" || old.LegalProfileSnapshotRef == nil || *old.LegalProfileSnapshotRef != "encrypted/history/reference" {
		t.Fatalf("historical evidence lost: %+v", old)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,status_code,consent_no,created_at,request_idempotency_key_digest,request_fingerprint,encryption_key_no) VALUES('case_reset_new_001',?,'usr:7101','enterprise','manual_enterprise','draft','consent_reset_002',?,repeat('d',64),repeat('e',64),'enterprise_material_key_v1')`, customerNo, resetAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := service.cancelClearedCases(ctx, subject); err != nil {
		t.Fatal(err)
	}
	// A cleared enterprise must see the new draft rather than the preserved old case.
	service.identities = &enterpriseIdentityFake{subject: subject}
	current, err := service.Current(ctx)
	if err != nil || current == nil || current.CaseNo != "case_reset_new_001" {
		t.Fatalf("current draft blocked: %+v %v", current, err)
	}
	var fresh *entity.VerificationCase
	if err := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{CaseNo: "case_reset_new_001"}).Scan(&fresh); err != nil {
		t.Fatal(err)
	}
	if fresh == nil || fresh.StatusCode != "draft" || len(writer.transitions) != 1 {
		t.Fatalf("stale reset canceled a new request or duplicated audit: %+v events=%d", fresh, len(writer.transitions))
	}
}
