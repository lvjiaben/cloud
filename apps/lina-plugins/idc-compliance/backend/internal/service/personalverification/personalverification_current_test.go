// Manual owner status overrides historical cases without touching retained evidence or providers.
package personalverification

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
	identity := &fakeIdentityService{subject: identitycap.PersonalSubject{CustomerNo: "customer_test", ManualVerification: true, VerificationStatus: identitycap.StatusVerified}}
	service := &serviceImpl{identities: identity}
	value, err := service.Current(context.Background())
	if err != nil || value.Status != StatusVerified || value.CaseNo != "" {
		t.Fatalf("manual approval: %+v %v", value, err)
	}
	identity.subject.VerificationStatus = identitycap.StatusUnverified
	value, err = service.Current(context.Background())
	if err != nil || value.Status != StatusUnverified || value.MaskedName != "" {
		t.Fatalf("cleared status: %+v %v", value, err)
	}
	value, err = service.QueryCurrent(context.Background(), "attempt_old_0001")
	if err != nil || value.Status != StatusUnverified {
		t.Fatalf("old browser return: %+v %v", value, err)
	}
}

// Retire only pre-reset in-flight cases; keep encrypted references and never cancel fresh requests.
func TestCancelClearedCasesPostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := personalVerificationTestDatabase(t, ctx)
	const customerNo = "customer_clear_history_001"
	resetAt := time.Now().UTC().Add(-time.Minute)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,status_code,consent_no,created_at,legal_profile_snapshot_ref,legal_profile_sha256) VALUES('case_reset_old_001',?,'usr:7101','personal','aliyun_face','pending','consent_reset_001',?,'encrypted/history/reference',repeat('a',64))`, customerNo, resetAt.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	writer := &fakeAuditWriter{}
	service := &serviceImpl{database: database, audit: writer}
	subject := identitycap.PersonalSubject{CustomerNo: customerNo, ManualVerification: true, VerificationStatus: identitycap.StatusUnverified, VerificationUpdatedAt: &resetAt}
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
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_compliance_verification_case(case_no,customer_no,principal_no,subject_type_code,provider_code,status_code,consent_no,created_at) VALUES('case_reset_new_001',?,'usr:7101','personal','aliyun_face','draft','consent_reset_002',?)`, customerNo, resetAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := service.cancelClearedCases(ctx, subject); err != nil {
		t.Fatal(err)
	}
	var fresh *entity.VerificationCase
	if err := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{CaseNo: "case_reset_new_001"}).Scan(&fresh); err != nil {
		t.Fatal(err)
	}
	if fresh == nil || fresh.StatusCode != "draft" || len(writer.transitions) != 1 {
		t.Fatalf("stale reset canceled a new request or duplicated audit: %+v events=%d", fresh, len(writer.transitions))
	}
}
