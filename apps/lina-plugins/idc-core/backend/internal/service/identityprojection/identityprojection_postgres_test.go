// This file verifies consent, monotonic projection, and identity uniqueness against real PostgreSQL.

package identityprojection

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const identityProjectionIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type identityProjectionPrincipal struct{ audience *principal.Audience }

func (p *identityProjectionPrincipal) Current(context.Context) (*principal.Audience, error) {
	copyValue := *p.audience
	return &copyValue, nil
}

func TestIdentityProjectionPostgreSQLRecordsSeparatePersonalConsentIdempotently(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	insertIdentityProjectionCustomer(t, ctx, database, "cust_consent_0001")
	insertIdentityProjectionCustomer(t, ctx, database, "cust_consent_0002")
	principalReader := &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7101, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "cust_consent_0001",
	}}
	service, err := New(database, principalReader, bytes.Repeat([]byte{0x7b}, 32))
	if err != nil {
		t.Fatal(err)
	}
	input := identitycap.PersonalConsentInput{
		PolicyVersion: identitycap.PersonalConsentPolicyVersion,
		Locale:        "zh-CN", IdempotencyKey: "personal-consent-01J5M6S2R9Z8X7W6",
	}
	created, err := service.RecordPersonalConsent(ctx, input)
	if err != nil || created == nil || created.Reused || created.ConsentNo == "" || created.OccurredAt.IsZero() {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	replayed, err := service.RecordPersonalConsent(ctx, input)
	if err != nil || replayed == nil || !replayed.Reused || replayed.ConsentNo != created.ConsentNo ||
		!replayed.OccurredAt.Equal(created.OccurredAt) {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	subject, err := service.PreparePersonal(ctx, created.ConsentNo)
	if err != nil || subject == nil || subject.CustomerNo != "cust_consent_0001" ||
		subject.ActorUserID != 7101 {
		t.Fatalf("subject=%+v err=%v", subject, err)
	}
	row, err := database.GetOne(ctx, `
SELECT c.policy_reference, c.policy_version, c.action_code, c.source_code,
       c.locale, c.evidence_digest, c.recorded_by,
       n.idempotency_key_digest, n.owner_type, n.owner_internal_id
FROM plugin_idc_core_customer_consent c
JOIN plugin_idc_core_business_number n ON n.business_no = c.consent_no
WHERE c.consent_no = ?`, created.ConsentNo)
	if err != nil {
		t.Fatal(err)
	}
	if row["policy_reference"].String() != identitycap.PersonalConsentPolicyReference ||
		row["policy_version"].String() != identitycap.PersonalConsentPolicyVersion ||
		row["action_code"].String() != "granted" || row["source_code"].String() != "portal" ||
		row["locale"].String() != "zh-CN" || len(row["evidence_digest"].String()) != 64 ||
		row["recorded_by"].Int() != 7101 || len(row["idempotency_key_digest"].String()) != 64 ||
		row["idempotency_key_digest"].String() == input.IdempotencyKey ||
		row["owner_type"].String() != "customer_consent" || row["owner_internal_id"].Int64() <= 0 {
		t.Fatalf("unexpected personal consent evidence: %+v", row)
	}
	count, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_customer_consent WHERE customer_no = ?`,
		"cust_consent_0001")
	if err != nil || count.Int() != 1 {
		t.Fatalf("consent count=%d err=%v", count.Int(), err)
	}

	principalReader.audience = &principal.Audience{
		UserID: 7102, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "cust_consent_0002",
	}
	if _, err = service.PreparePersonal(ctx, created.ConsentNo); err == nil {
		t.Fatal("another customer reused the first customer's consent")
	}
	second, err := service.RecordPersonalConsent(ctx, identitycap.PersonalConsentInput{
		PolicyVersion: identitycap.PersonalConsentPolicyVersion,
		Locale:        "en-US", IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil || second == nil || second.ConsentNo == created.ConsentNo {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestIdentityProjectionPostgreSQLPrivacyWithdrawalIsAppendOnlyAndCustomerScoped(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	insertIdentityProjectionCustomer(t, ctx, database, "cust_privacy_alpha_0001")
	insertIdentityProjectionCustomer(t, ctx, database, "cust_privacy_bravo_0001")
	principalReader := &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7201, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "cust_privacy_alpha_0001",
	}}
	service, err := New(database, principalReader, bytes.Repeat([]byte{0x5c}, 32))
	if err != nil {
		t.Fatal(err)
	}
	privacyService, ok := service.(identitycap.PrivacyConsentService)
	if !ok {
		t.Fatal("identity projection does not expose the privacy consent capability")
	}
	grant, err := service.RecordPersonalConsent(ctx, identitycap.PersonalConsentInput{
		PolicyVersion: identitycap.PersonalConsentPolicyVersion,
		Locale:        "zh-CN", IdempotencyKey: "privacy-grant-01J5M6S2R9Z8X7W6",
	})
	if err != nil || grant == nil {
		t.Fatalf("RecordPersonalConsent() = %#v error=%v", grant, err)
	}
	before, err := database.GetOne(ctx, `SELECT action_code, evidence_digest, recorded_by FROM plugin_idc_core_customer_consent WHERE consent_no = ?`, grant.ConsentNo)
	if err != nil || before["action_code"].String() != "granted" || len(before["evidence_digest"].String()) != 64 || before["recorded_by"].Int() != 7201 {
		t.Fatalf("grant before withdrawal = %#v error=%v", before, err)
	}

	withdrawalInput := identitycap.PrivacyConsentWithdrawalInput{
		ConsentType:    identitycap.PrivacyConsentSensitivePersonal,
		IdempotencyKey: "privacy-withdraw-01J5M6S2R9Z8X7W6",
	}
	withdrawn, err := privacyService.WithdrawCurrentPrivacyConsent(ctx, withdrawalInput)
	if err != nil || withdrawn == nil || withdrawn.ActionCode != "withdrawn" || withdrawn.ConsentNo == grant.ConsentNo || withdrawn.Reused {
		t.Fatalf("WithdrawCurrentPrivacyConsent() = %#v error=%v", withdrawn, err)
	}
	replayed, err := privacyService.WithdrawCurrentPrivacyConsent(ctx, withdrawalInput)
	if err != nil || replayed == nil || !replayed.Reused || replayed.ConsentNo != withdrawn.ConsentNo {
		t.Fatalf("WithdrawCurrentPrivacyConsent(replay) = %#v error=%v", replayed, err)
	}

	var rows []struct {
		ConsentNo      string `json:"consent_no"`
		ActionCode     string `json:"action_code"`
		EvidenceDigest string `json:"evidence_digest"`
	}
	if err = database.GetScan(ctx, &rows, `SELECT consent_no, action_code, evidence_digest FROM plugin_idc_core_customer_consent WHERE customer_no = ? AND consent_type_code = 'sensitive_personal_information' ORDER BY occurred_at, id`, "cust_privacy_alpha_0001"); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ConsentNo != grant.ConsentNo || rows[0].ActionCode != "granted" || rows[0].EvidenceDigest != before["evidence_digest"].String() || rows[1].ConsentNo != withdrawn.ConsentNo || rows[1].ActionCode != "withdrawn" {
		t.Fatalf("append-only privacy consent rows = %#v", rows)
	}
	current, err := privacyService.ListCurrentPrivacyConsents(ctx)
	if err != nil || len(current) != 1 || current[0].ConsentNo != withdrawn.ConsentNo || current[0].ActionCode != "withdrawn" {
		t.Fatalf("ListCurrentPrivacyConsents(alpha) = %#v error=%v", current, err)
	}

	principalReader.audience = &principal.Audience{
		UserID: 7202, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "cust_privacy_bravo_0001",
	}
	current, err = privacyService.ListCurrentPrivacyConsents(ctx)
	if err != nil || len(current) != 0 {
		t.Fatalf("ListCurrentPrivacyConsents(bravo) = %#v error=%v", current, err)
	}
	if _, err = privacyService.WithdrawCurrentPrivacyConsent(ctx, identitycap.PrivacyConsentWithdrawalInput{
		ConsentType:    identitycap.PrivacyConsentSensitivePersonal,
		IdempotencyKey: "privacy-withdraw-01J5M6S2R9Z8X7W7",
	}); err == nil {
		t.Fatal("another customer withdrew the first customer's consent")
	}
}

func TestIdentityProjectionPostgreSQLConsentMonotonicProjectionAndDuplicateRisk(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	insertIdentityProjectionCustomer(t, ctx, database, "cust_identity_0001")
	insertIdentityProjectionCustomer(t, ctx, database, "cust_identity_0002")
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_consent (
    tenant_id, consent_no, customer_no, consent_type_code, policy_reference,
    policy_version, action_code, source_code, locale, evidence_digest, occurred_at, recorded_by
) VALUES (0, 'consent_identity_0001', 'cust_identity_0001',
          'sensitive_personal_information', ?, ?,
          'granted', 'portal', 'zh-CN', ?, ?, 7001)`,
		identitycap.PersonalConsentPolicyReference, identitycap.PersonalConsentPolicyVersion,
		strings.Repeat("a", 64), now); err != nil {
		t.Fatal(err)
	}
	principalReader := &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7001, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "cust_identity_0001",
	}}
	service, err := New(database, principalReader, bytes.Repeat([]byte{0x6a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := service.PreparePersonal(ctx, "consent_identity_0001")
	if err != nil || subject.CustomerNo != "cust_identity_0001" || subject.PrincipalReference != "usr:7001" {
		t.Fatalf("subject=%+v err=%v", subject, err)
	}
	pending, err := service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: subject.CustomerNo, CaseNo: "case_identity_0001", Status: identitycap.StatusPending,
	})
	if err != nil || pending.Status != identitycap.StatusPending || pending.Version != 2 {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	name := []byte("测试用户")
	identityNumber := []byte("110101200002290019")
	verifiedAt := now.Add(time.Minute)
	verified, err := service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: subject.CustomerNo, CaseNo: "case_identity_0001", Status: identitycap.StatusVerified,
		LegalName: name, MaskedName: "测***", IdentityNumber: identityNumber,
		MaskedIdentityNo: "110***********0019", VerifiedAt: &verifiedAt,
	})
	if err != nil || verified.Status != identitycap.StatusVerified ||
		len(verified.IdentityLookupDigest) != 64 || verified.Version != 3 {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
	assertIdentityBytesCleared(t, name)
	assertIdentityBytesCleared(t, identityNumber)

	principalReader.audience.CustomerNo = "cust_identity_0002"
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_consent (
    tenant_id, consent_no, customer_no, consent_type_code, policy_reference,
    policy_version, action_code, source_code, locale, evidence_digest, occurred_at, recorded_by
) VALUES (0, 'consent_identity_0002', 'cust_identity_0002',
          'sensitive_personal_information', ?, ?,
          'granted', 'portal', 'zh-CN', ?, ?, 7002)`,
		identitycap.PersonalConsentPolicyReference, identitycap.PersonalConsentPolicyVersion,
		strings.Repeat("b", 64), now); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: "cust_identity_0002", CaseNo: "case_identity_0002", Status: identitycap.StatusPending,
	}); err != nil {
		t.Fatal(err)
	}
	duplicateName := []byte("另一个用户")
	duplicateNumber := []byte("110101200002290019")
	duplicate, err := service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: "cust_identity_0002", CaseNo: "case_identity_0002", Status: identitycap.StatusVerified,
		LegalName: duplicateName, MaskedName: "另****", IdentityNumber: duplicateNumber,
		MaskedIdentityNo: "110***********0019", VerifiedAt: &verifiedAt,
	})
	if err != nil || !duplicate.DuplicateRisk || duplicate.Status != identitycap.StatusPending {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	assertIdentityBytesCleared(t, duplicateName)
	assertIdentityBytesCleared(t, duplicateNumber)
	status, err := database.GetValue(ctx, `
SELECT identity_status_code FROM plugin_idc_core_customer_individual_profile
WHERE customer_no = 'cust_identity_0002'`)
	if err != nil || status.String() != "pending" {
		t.Fatalf("duplicate profile status=%q err=%v", status.String(), err)
	}
}

func TestIdentityProjectionPostgreSQLRequiresLatestGrantedConsent(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	insertIdentityProjectionCustomer(t, ctx, database, "cust_identity_0001")
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	for index, action := range []string{"granted", "withdrawn"} {
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_consent (
    tenant_id, consent_no, customer_no, consent_type_code, policy_reference,
    policy_version, action_code, source_code, locale, evidence_digest, occurred_at, recorded_by
		) VALUES (0, ?, 'cust_identity_0001', 'sensitive_personal_information',
		          ?, ?, ?, 'portal', 'zh-CN', ?, ?, 7001)`,
			"consent_identity_000"+string(rune('1'+index)),
			identitycap.PersonalConsentPolicyReference, identitycap.PersonalConsentPolicyVersion, action,
			strings.Repeat(string(rune('a'+index)), 64), now.Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	service, err := New(database, &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7001, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "cust_identity_0001",
	}}, bytes.Repeat([]byte{0x6a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.PreparePersonal(ctx, "consent_identity_0001"); err == nil {
		t.Fatal("superseded sensitive consent was accepted")
	}
	if _, err = service.PreparePersonal(ctx, "consent_identity_0002"); err == nil {
		t.Fatal("withdrawn sensitive consent was accepted")
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_consent (
    tenant_id, consent_no, customer_no, consent_type_code, policy_reference,
    policy_version, action_code, source_code, locale, evidence_digest, occurred_at, recorded_by
) VALUES (0, 'consent_identity_0003', 'cust_identity_0001',
          'sensitive_personal_information', 'idc://compliance/another-policy', '1',
          'granted', 'portal', 'zh-CN', ?, ?, 7001)`, strings.Repeat("c", 64), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.PreparePersonal(ctx, "consent_identity_0003"); err == nil {
		t.Fatal("a grant for another policy was accepted")
	}
}

func TestIdentityProjectionPostgreSQLSerializesConcurrentDuplicateIdentity(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	const (
		firstCustomer  = "cust_identity_concurrent_0001"
		secondCustomer = "cust_identity_concurrent_0002"
	)
	insertIdentityProjectionCustomer(t, ctx, database, firstCustomer)
	insertIdentityProjectionCustomer(t, ctx, database, secondCustomer)
	service, err := New(database, &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7001, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: firstCustomer,
	}}, bytes.Repeat([]byte{0x6a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	for index, customerNo := range []string{firstCustomer, secondCustomer} {
		if _, err = service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
			CustomerNo: customerNo, CaseNo: "case_identity_concurrent_000" + strconv.Itoa(index+1),
			Status: identitycap.StatusPending,
		}); err != nil {
			t.Fatal(err)
		}
	}
	verifiedAt := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	start := make(chan struct{})
	results := make(chan *identitycap.PersonalProjection, 2)
	errors := make(chan error, 2)
	var group sync.WaitGroup
	for index, customerNo := range []string{firstCustomer, secondCustomer} {
		group.Add(1)
		go func(index int, customerNo string) {
			defer group.Done()
			<-start
			name := []byte("并发用户")
			identityNumber := []byte("110101200002290019")
			result, projectErr := service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
				CustomerNo: customerNo, CaseNo: "case_identity_concurrent_000" + strconv.Itoa(index+1),
				Status: identitycap.StatusVerified, LegalName: name, MaskedName: "并***",
				IdentityNumber: identityNumber, MaskedIdentityNo: "110***********0019",
				VerifiedAt: &verifiedAt,
			})
			results <- result
			errors <- projectErr
		}(index, customerNo)
	}
	close(start)
	group.Wait()
	close(results)
	close(errors)
	for projectErr := range errors {
		if projectErr != nil {
			t.Fatal(projectErr)
		}
	}
	verifiedCount := 0
	duplicateCount := 0
	for result := range results {
		if result == nil {
			t.Fatal("concurrent identity projection returned nil")
		}
		if result.DuplicateRisk {
			duplicateCount++
		} else if result.Status == identitycap.StatusVerified {
			verifiedCount++
		}
	}
	if verifiedCount != 1 || duplicateCount != 1 {
		t.Fatalf("verified=%d duplicate=%d", verifiedCount, duplicateCount)
	}
	count, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_customer_individual_profile
WHERE identity_status_code = 'verified' AND identity_lookup_digest <> ''`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("verified database count=%d err=%v", count.Int(), err)
	}
}

func TestIdentityProjectionPostgreSQLJoinsAmbientTransaction(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	const customerNo = "cust_identity_atomic_0001"
	insertIdentityProjectionCustomer(t, ctx, database, customerNo)
	service, err := New(database, &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7001, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: customerNo,
	}}, bytes.Repeat([]byte{0x6a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: customerNo, CaseNo: "case_identity_atomic_0001",
		Status: identitycap.StatusPending,
	}); err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("force ambient rollback")
	verifiedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		projection, projectErr := service.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
			CustomerNo: customerNo, CaseNo: "case_identity_atomic_0001",
			Status: identitycap.StatusVerified, LegalName: []byte("Atomic Customer"),
			MaskedName: "A***", IdentityNumber: []byte("110101200002290019"),
			MaskedIdentityNo: "110***********0019", VerifiedAt: &verifiedAt,
		})
		if projectErr != nil || projection == nil || projection.Status != identitycap.StatusVerified {
			t.Fatalf("projection=%+v err=%v", projection, projectErr)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("ambient transaction returned %v", err)
	}
	row, err := database.GetOne(ctx, `
SELECT identity_status_code, identity_lookup_digest
FROM plugin_idc_core_customer_individual_profile
WHERE customer_no = ?`, customerNo)
	if err != nil {
		t.Fatal(err)
	}
	if row["identity_status_code"].String() != string(identitycap.StatusPending) ||
		strings.TrimSpace(row["identity_lookup_digest"].String()) != "" {
		t.Fatalf("ambient rollback leaked projection: %+v", row)
	}
}

func TestIdentityProjectionPostgreSQLEnterpriseConsentAndInitialVerification(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	const customerNo = "cust_enterprise_identity_0001"
	insertEnterpriseIdentityProjectionCustomer(t, ctx, database, customerNo)
	service, err := New(database, &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7201, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: customerNo,
	}}, bytes.Repeat([]byte{0x5c}, 32))
	if err != nil {
		t.Fatal(err)
	}
	consent, err := service.RecordEnterpriseConsent(ctx, identitycap.EnterpriseConsentInput{
		PolicyVersion: identitycap.EnterpriseConsentPolicyVersion, Locale: "zh-CN",
		IdempotencyKey: "enterprise-consent-01J5M6S2R9Z8X7W6",
	})
	if err != nil || consent == nil || consent.Reused || consent.ConsentNo == "" {
		t.Fatalf("consent=%+v err=%v", consent, err)
	}
	prepared, err := service.PrepareEnterprise(ctx, consent.ConsentNo, "")
	if err != nil || prepared == nil || prepared.Subject.CustomerNo != customerNo ||
		prepared.Reverification || prepared.CurrentProfileVersion != 1 {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	if _, err = service.ProjectEnterprise(ctx, identitycap.EnterpriseProjectionInput{
		CustomerNo: customerNo, CaseNo: "case_enterprise_identity_0001",
		Status: identitycap.StatusPending,
	}); err != nil {
		t.Fatal(err)
	}
	legalName := []byte("Example Infrastructure Co., Ltd.")
	registration := []byte("91110000MA0012345X")
	verifiedAt := time.Now().UTC().Truncate(time.Microsecond)
	verified, err := service.ProjectEnterprise(ctx, identitycap.EnterpriseProjectionInput{
		CustomerNo: customerNo, CaseNo: "case_enterprise_identity_0001",
		Status: identitycap.StatusVerified, ReviewActorUserID: 9001,
		DecisionReasonCode: "manual_review_approved", LegalName: legalName,
		RegistrationCountryCode: "CN", RegistrationIdentifier: registration,
		MaskedRegistrationIdentifier: "9111**********345X", IndustryCode: "IDC",
		VerifiedAt: &verifiedAt,
	})
	if err != nil || verified == nil || verified.Status != identitycap.StatusVerified ||
		verified.Version != 3 || len(verified.RegistrationLookupDigest) != 64 {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
	assertIdentityBytesCleared(t, legalName)
	assertIdentityBytesCleared(t, registration)
	row, err := database.GetOne(ctx, `
SELECT profile.verification_status_code, profile.verification_case_no,
       profile.registration_identifier, profile.registration_lookup_digest,
       profile.version, subject.customer_type_code
FROM plugin_idc_core_customer_enterprise_profile profile
JOIN plugin_idc_core_customer_subject subject ON subject.customer_no = profile.customer_no
WHERE profile.customer_no = ?`, customerNo)
	if err != nil || row["verification_status_code"].String() != "verified" ||
		row["verification_case_no"].String() != "case_enterprise_identity_0001" ||
		row["registration_identifier"].String() != "91110000MA0012345X" ||
		len(row["registration_lookup_digest"].String()) != 64 || row["version"].Int64() != 3 ||
		row["customer_type_code"].String() != "enterprise" {
		t.Fatalf("unexpected enterprise projection row=%+v err=%v", row, err)
	}
}

func TestIdentityProjectionPostgreSQLEnterpriseReverificationPreservesCurrentOnRejection(t *testing.T) {
	ctx := context.Background()
	database := identityProjectionTestDatabase(t, ctx)
	const customerNo = "cust_enterprise_review_0001"
	insertEnterpriseIdentityProjectionCustomer(t, ctx, database, customerNo)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_customer_subject
SET customer_type_code = 'enterprise', version = version + 1
WHERE customer_no = ?`, customerNo); err != nil {
		t.Fatal(err)
	}
	service, err := New(database, &identityProjectionPrincipal{audience: &principal.Audience{
		UserID: 7202, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: customerNo,
	}}, bytes.Repeat([]byte{0x4d}, 32))
	if err != nil {
		t.Fatal(err)
	}
	verifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_customer_enterprise_profile
SET legal_name = 'Existing Enterprise', registration_identifier = '91110000MA0099999X',
    registration_identifier_masked = '9111**********999X', registration_lookup_digest = ?,
    verification_case_no = 'case_enterprise_existing_0001', verification_status_code = 'verified',
    verified_at = ?, version = 7
WHERE customer_no = ?`, strings.Repeat("d", 64), verifiedAt, customerNo); err != nil {
		t.Fatal(err)
	}
	submittedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	const requestNo = "LCR_0123456789abcdef0123456789abcdef"
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_legal_change_request (
    tenant_id, request_no, customer_no, customer_type_code, status_code, reason_code,
    change_legal_name, change_legal_identifier, current_verification_status_code,
    current_verification_case_no, current_profile_version, idempotency_key_digest,
    request_fingerprint, submitted_by_user_id, submitted_at
) VALUES (0, ?, ?, 'enterprise', 'submitted', 'enterprise_registration_change',
          TRUE, TRUE, 'verified', 'case_enterprise_existing_0001', 7, ?, ?, 7202, ?)`,
		requestNo, customerNo, strings.Repeat("e", 64), strings.Repeat("f", 64), submittedAt); err != nil {
		t.Fatal(err)
	}
	consent, err := service.RecordEnterpriseConsent(ctx, identitycap.EnterpriseConsentInput{
		PolicyVersion: identitycap.EnterpriseConsentPolicyVersion, Locale: "en-US",
		IdempotencyKey: "enterprise-review-consent-01J5M6S2R9Z8X7W6",
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.PrepareEnterprise(ctx, consent.ConsentNo, requestNo)
	if err != nil || prepared == nil || !prepared.Reverification ||
		prepared.CurrentCaseNo != "case_enterprise_existing_0001" ||
		prepared.CurrentProfileVersion != 7 {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	pending, err := service.ProjectEnterprise(ctx, identitycap.EnterpriseProjectionInput{
		CustomerNo: customerNo, CaseNo: "case_enterprise_review_0002",
		LegalChangeRequestNo: requestNo, Status: identitycap.StatusPending,
	})
	if err != nil || pending == nil || !pending.PreservedCurrent ||
		pending.Status != identitycap.StatusVerified || pending.Version != 7 {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	rejected, err := service.ProjectEnterprise(ctx, identitycap.EnterpriseProjectionInput{
		CustomerNo: customerNo, CaseNo: "case_enterprise_review_0002",
		LegalChangeRequestNo: requestNo, Status: identitycap.StatusRejected,
		ReviewActorUserID: 9002, DecisionReasonCode: "material_inconsistent",
	})
	if err != nil || rejected == nil || !rejected.PreservedCurrent ||
		rejected.Status != identitycap.StatusVerified || rejected.Version != 7 {
		t.Fatalf("rejected=%+v err=%v", rejected, err)
	}
	row, err := database.GetOne(ctx, `
SELECT p.verification_status_code, p.verification_case_no, p.legal_name, p.version,
       r.status_code, r.reviewed_by_user_id, r.resolution_code
FROM plugin_idc_core_customer_enterprise_profile p
JOIN plugin_idc_core_customer_legal_change_request r ON r.customer_no = p.customer_no
WHERE p.customer_no = ? AND r.request_no = ?`, customerNo, requestNo)
	if err != nil || row["verification_status_code"].String() != "verified" ||
		row["verification_case_no"].String() != "case_enterprise_existing_0001" ||
		row["legal_name"].String() != "Existing Enterprise" || row["version"].Int64() != 7 ||
		row["status_code"].String() != "rejected" || row["reviewed_by_user_id"].Int() != 9002 ||
		row["resolution_code"].String() != "material_inconsistent" {
		t.Fatalf("reverification row=%+v err=%v", row, err)
	}
}

func insertIdentityProjectionCustomer(t *testing.T, ctx context.Context, database gdb.DB, customerNo string) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject (
    tenant_id, customer_no, customer_type_code, status_code, display_name,
    country_code, preferred_locale, version, created_by, updated_by, search_name
) VALUES (0, ?, 'individual', 'pending_activation', 'Test Customer',
	      'CN', 'zh-CN', 1, 7001, 7001, 'test customer')`, customerNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile (
    tenant_id, customer_no, customer_type_code, identity_status_code, version
) VALUES (0, ?, 'individual', 'unverified', 1)`, customerNo); err != nil {
		t.Fatal(err)
	}
}

func insertEnterpriseIdentityProjectionCustomer(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	customerNo string,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject (
    tenant_id, customer_no, customer_type_code, status_code, display_name,
    country_code, preferred_locale, version, created_by, updated_by, search_name
) VALUES (0, ?, 'individual', 'active', 'Test Enterprise',
	      'CN', 'zh-CN', 1, 7201, 7201, 'test enterprise')`, customerNo); err != nil {
		t.Fatal(err)
	}
	verifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile (
    tenant_id, customer_no, customer_type_code, legal_name, legal_name_masked,
    identity_identifier_masked, identity_lookup_digest, identity_case_no,
    identity_status_code, identity_verified_at, version
) VALUES (0, ?, 'individual', 'Test Operator', 'T***r',
          '110***********001', ?, 'case_personal_prerequisite_0001',
          'verified', ?, 2)`, strings.Repeat("a", 64), verifiedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_enterprise_profile (
    tenant_id, customer_no, customer_type_code, verification_status_code, version
) VALUES (0, ?, 'enterprise', 'unverified', 1)`, customerNo); err != nil {
		t.Fatal(err)
	}
}

func identityProjectionTestDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(identityProjectionIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run identity projection PostgreSQL tests", identityProjectionIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "linapro_identity_projection_test_" + randomIdentityProjectionHex(t, 8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	original := gdb.GetAllConfig()
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Errorf("close identity projection database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(original); restoreErr != nil {
			t.Errorf("restore database config: %v", restoreErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop identity projection schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close identity projection admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{
			Link: appendIdentityProjectionDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	if _, err = database.Exec(ctx, identityProjectionDictionaryFixtureSQL); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"001-core-foundation.sql", "003-customer-account.sql", "007-customer-account-workflows.sql",
		"040-personal-identity-uniqueness.sql", "041-enterprise-verification-projection.sql",
		"106-personal-first-enterprise-upgrade.sql",
	} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, readErr = database.Exec(ctx, string(content)); readErr != nil {
			t.Fatalf("execute %s: %v", name, readErr)
		}
		if _, readErr = database.Exec(ctx, string(content)); readErr != nil {
			t.Fatalf("repeat %s idempotently: %v", name, readErr)
		}
	}
	return database
}

const identityProjectionDictionaryFixtureSQL = `
CREATE TABLE sys_dict_type (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, name VARCHAR(128) NOT NULL DEFAULT '',
    type VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0, allow_tenant_override BOOL NOT NULL DEFAULT FALSE,
    remark VARCHAR(512) NOT NULL DEFAULT '', created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_identity_test_dict_type ON sys_dict_type (tenant_id, type);
CREATE TABLE sys_dict_data (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, dict_type VARCHAR(128) NOT NULL DEFAULT '',
    label VARCHAR(128) NOT NULL DEFAULT '', value VARCHAR(128) NOT NULL DEFAULT '',
    sort INT NOT NULL DEFAULT 0, tag_style VARCHAR(128) NOT NULL DEFAULT '',
    css_class VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0, remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_identity_test_dict_data ON sys_dict_data (tenant_id, dict_type, value);
`

func randomIdentityProjectionHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func appendIdentityProjectionDSNParameter(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func assertIdentityBytesCleared(t *testing.T, value []byte) {
	t.Helper()
	for _, character := range value {
		if character != 0 {
			t.Fatal("identity projection retained caller raw bytes")
		}
	}
}
