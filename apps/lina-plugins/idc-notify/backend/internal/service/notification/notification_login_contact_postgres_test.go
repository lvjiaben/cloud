// This file verifies real database proof consumption, rate limits and rollback.
package notification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-idc-notify/backend/internal/service/loginprofile"
)

type contactTestContext struct{}

func (contactTestContext) Current(ctx context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentFromContext(ctx)
}

type contactTestAudience struct{}

func (contactTestAudience) Current(context.Context) (notificationcap.CurrentAudienceProjection, error) {
	return notificationcap.CurrentAudienceProjection{CustomerNo: "CUS_contact_test", CustomerVersion: 1}, nil
}

type contactTestProfile struct{ database gdb.DB }

func (p contactTestProfile) Current(ctx context.Context) (loginprofile.Snapshot, error) {
	var result loginprofile.Snapshot
	err := p.database.GetScan(ctx, &result, "SELECT id,email,phone FROM contact_test_identity WHERE id=?", bizctxcap.CurrentFromContext(ctx).UserID)
	return result, err
}

func contactFixture(t *testing.T) (*serviceImpl, *presalesAdapterStub, context.Context, gdb.DB) {
	t.Helper()
	database, cleanup := openPresalesPostgreSQL(t)
	t.Cleanup(cleanup)
	_, file, _, _ := runtime.Caller(0)
	for _, name := range []string{"006-multi-provider-sms.sql", "008-customer-onboarding-verification.sql", "010-login-contact-change.sql", "010-login-contact-change.sql"} {
		sql, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(t.Context(), string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(t.Context(), `CREATE TABLE contact_test_identity(id INTEGER PRIMARY KEY,email TEXT,phone TEXT);INSERT INTO contact_test_identity VALUES(42,'old@example.com','13800138000'),(43,'other@example.com','13900139000')`); err != nil {
		t.Fatal(err)
	}
	service, _, adapter, _ := presalesServiceFixture(t, database)
	service.bizctx = contactTestContext{}
	service.sources.Audience = contactTestAudience{}
	service.loginProfiles = contactTestProfile{database}
	ctx := bizctxcap.WithCurrentContext(t.Context(), bizctxcap.CurrentContext{UserID: 42, TenantID: 0, TokenID: "session-contact-42"})
	return service, adapter, ctx, database
}

func contactReady(t *testing.T, s *serviceImpl, adapter *presalesAdapterStub, ctx context.Context, key, destination string) profilechangecap.Input {
	t.Helper()
	oldInput := LoginContactInput{Kind: ContactEmail, Locale: "en-US", IdempotencyKey: key + "-old"}
	old, err := s.SendLoginContactCode(ctx, oldInput)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.VerifyLoginContactCode(ctx, LoginContactVerifyInput{LoginContactInput: oldInput, VerificationNo: old.VerificationNo, Code: adapter.lastCode()}); err != nil {
		t.Fatal(err)
	}
	newInput := LoginContactInput{Kind: ContactEmail, OldVerification: old.VerificationNo, Destination: destination, Locale: "en-US", IdempotencyKey: key + "-new"}
	fresh, err := s.SendLoginContactCode(ctx, newInput)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.VerifyLoginContactCode(ctx, LoginContactVerifyInput{LoginContactInput: newInput, VerificationNo: fresh.VerificationNo, Code: adapter.lastCode()}); err != nil {
		t.Fatal(err)
	}
	return profilechangecap.Input{Email: &destination, OldVerification: old.VerificationNo, NewVerification: fresh.VerificationNo}
}

func TestLoginContactVerifiedSaveRollbackReplayPostgreSQL(t *testing.T) {
	service, adapter, ctx, database := contactFixture(t)
	input := contactReady(t, service, adapter, ctx, "contact-positive-0001", "new@example.com")
	var calls atomic.Int32
	write := func(ctx context.Context) error {
		calls.Add(1)
		_, err := database.Exec(ctx, "UPDATE contact_test_identity SET email=? WHERE id=42", *input.Email)
		return err
	}
	// A failure after the write must roll back both the profile and proof use.
	err := service.Apply(ctx, input, func(ctx context.Context) error {
		if err := write(ctx); err != nil {
			return err
		}
		return errors.New("failure after profile write")
	})
	if err == nil {
		t.Fatal("accepted failing save")
	}
	email, _ := database.GetValue(ctx, "SELECT email FROM contact_test_identity WHERE id=42")
	if email.String() != "old@example.com" {
		t.Fatal("failed save changed contact")
	}
	if err = service.Apply(ctx, input, write); err != nil {
		t.Fatal(err)
	}
	email, _ = database.GetValue(ctx, "SELECT email FROM contact_test_identity WHERE id=42")
	if email.String() != *input.Email {
		t.Fatal("successful save not persisted")
	}
	if err = service.Apply(ctx, input, write); err == nil {
		t.Fatal("replayed consumed proofs")
	}
	if calls.Load() != 2 {
		t.Fatalf("unexpected route calls: %d", calls.Load())
	}
}

func TestLoginContactWrongCodeLimitsAndIdentityPostgreSQL(t *testing.T) {
	service, adapter, ctx, database := contactFixture(t)
	input := LoginContactInput{Kind: ContactEmail, Locale: "en-US", IdempotencyKey: "contact-attempts-0001", Destination: "attacker@example.com"}
	proof, err := service.SendLoginContactCode(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if proof.MaskedDestination == "attacker@example.com" {
		t.Fatal("client chose old destination")
	}
	before := adapter.deliveryCount()
	replay, err := service.SendLoginContactCode(ctx, input)
	if err != nil || replay.VerificationNo != proof.VerificationNo || adapter.deliveryCount() != before {
		t.Fatal("idempotent send repeated delivery", err)
	}
	code := adapter.lastCode()
	wrong := "000000"
	if wrong == code {
		wrong = "999999"
	}
	verify := LoginContactVerifyInput{LoginContactInput: input, VerificationNo: proof.VerificationNo, Code: wrong}
	for range 5 {
		if err = service.VerifyLoginContactCode(ctx, verify); err == nil {
			t.Fatal("accepted wrong code")
		}
	}
	attempts, _ := database.GetValue(ctx, "SELECT failed_attempts FROM plugin_idc_notify_contact_verification WHERE verification_no=?", proof.VerificationNo)
	if attempts.Int() != 5 {
		t.Fatalf("wrong attempts rolled back: %d", attempts.Int())
	}
	verify.Code = code
	if err = service.VerifyLoginContactCode(ctx, verify); err == nil {
		t.Fatal("locked proof accepted")
	}
	other := bizctxcap.WithCurrentContext(ctx, bizctxcap.CurrentContext{UserID: 43, TenantID: 0, TokenID: "session-contact-42"})
	if err = service.VerifyLoginContactCode(other, verify); err == nil {
		t.Fatal("cross-user proof accepted")
	}
}

func TestLoginContactConcurrentAndStaffChangesPostgreSQL(t *testing.T) {
	service, adapter, ctx, database := contactFixture(t)
	input := contactReady(t, service, adapter, ctx, "contact-concurrent-0001", "next@example.com")
	if err := service.Serialize(ctx, 42, func(ctx context.Context) error {
		_, err := database.Exec(ctx, "UPDATE contact_test_identity SET email='staff@example.com' WHERE id=42")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.Apply(ctx, input, func(context.Context) error { t.Fatal("stale proof reached write"); return nil }); err == nil {
		t.Fatal("accepted proof after staff change")
	}
	// A separate fixture isolates cooldown and proves simultaneous submissions.
	service, adapter, ctx, database = contactFixture(t)
	input = contactReady(t, service, adapter, ctx, "contact-concurrent-0002", "final@example.com")
	var success, calls atomic.Int32
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			err := service.Apply(ctx, input, func(ctx context.Context) error {
				calls.Add(1)
				time.Sleep(10 * time.Millisecond)
				_, err := database.Exec(ctx, "UPDATE contact_test_identity SET email=? WHERE id=42", *input.Email)
				return err
			})
			if err == nil {
				success.Add(1)
			}
		}()
	}
	wait.Wait()
	if success.Load() != 1 || calls.Load() != 1 {
		t.Fatalf("success=%d calls=%d", success.Load(), calls.Load())
	}
}

func TestLoginContactRejectsMissingOldProofAndExpiredSnapshotPostgreSQL(t *testing.T) {
	service, adapter, ctx, database := contactFixture(t)
	_, err := service.SendLoginContactCode(ctx, LoginContactInput{Kind: ContactEmail, Locale: "en-US", OldVerification: "nver_forged", Destination: "new@example.com", IdempotencyKey: "contact-no-old-0001"})
	if err == nil || adapter.deliveryCount() != 0 {
		t.Fatal("new code sent without old proof")
	}
	input := contactReady(t, service, adapter, ctx, "contact-expire-0001", "new@example.com")
	changedSession := bizctxcap.WithCurrentContext(ctx, bizctxcap.CurrentContext{UserID: 42, TenantID: 0, TokenID: "different-session"})
	if err = service.Apply(changedSession, input, func(context.Context) error { t.Fatal("wrong session reached write"); return nil }); err == nil {
		t.Fatal("cross-session proof accepted")
	}
	if _, err = database.Exec(ctx, "UPDATE contact_test_identity SET phone='13999999999' WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	if err = service.Apply(ctx, input, func(context.Context) error { t.Fatal("changed snapshot reached write"); return nil }); err == nil {
		t.Fatal("stale snapshot accepted")
	}
	if _, err = database.Exec(ctx, "UPDATE contact_test_identity SET email='',phone='' WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	_, err = service.SendLoginContactCode(ctx, LoginContactInput{Kind: ContactPhone, Locale: "en-US", IdempotencyKey: "contact-no-existing-0001"})
	if !bizerr.Is(err, CodeContactManualVerification) {
		t.Fatal("missing contacts must use existing support", err)
	}
}

// An unbound phone uses the existing email for identity and the SMS channel for
// the new number. A proof cannot be redirected, switched to email or used late.
func TestLoginContactFirstPhoneBindingAndExpiryPostgreSQL(t *testing.T) {
	s, adapter, ctx, db := contactFixture(t)
	s.adapters["smsbao_http_v1"] = adapterBinding{descriptor: channelcap.Descriptor{ProviderCode: "smsbao_http_v1", Kind: channelcap.KindSMS}, adapter: adapter}
	if _, err := db.Exec(ctx, `UPDATE contact_test_identity SET phone='' WHERE id=42;
 UPDATE plugin_idc_notify_channel SET provider_code='smsbao_http_v1',status_code='enabled',configuration_generation=1,configuration_version=1,non_secret_configuration='{}'::jsonb,credential_secret_ref='sec_contact_sms',credential_updated_at=CURRENT_TIMESTAMP WHERE channel_code='sms'`); err != nil {
		t.Fatal(err)
	}
	oldInput := LoginContactInput{Kind: ContactPhone, Locale: "en-US", IdempotencyKey: "phone-binding-old-0001"}
	old, err := s.SendLoginContactCode(ctx, oldInput)
	if err != nil {
		t.Fatal(err)
	}
	if old.MaskedDestination != "o***@example.com" {
		t.Fatalf("wrong fallback destination: %s", old.MaskedDestination)
	}
	if err = s.VerifyLoginContactCode(ctx, LoginContactVerifyInput{LoginContactInput: oldInput, VerificationNo: old.VerificationNo, Code: adapter.lastCode()}); err != nil {
		t.Fatal(err)
	}
	freshInput := LoginContactInput{Kind: ContactPhone, Locale: "en-US", OldVerification: old.VerificationNo, Destination: "13900139000", IdempotencyKey: "phone-binding-new-0001"}
	fresh, err := s.SendLoginContactCode(ctx, freshInput)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.VerifyLoginContactCode(ctx, LoginContactVerifyInput{LoginContactInput: freshInput, VerificationNo: fresh.VerificationNo, Code: adapter.lastCode()}); err != nil {
		t.Fatal(err)
	}
	proof := profilechangecap.Input{Phone: &freshInput.Destination, OldVerification: old.VerificationNo, NewVerification: fresh.VerificationNo}
	wrong := "13700137000"
	tampered := proof
	tampered.Phone = &wrong
	if err = s.Apply(ctx, tampered, func(context.Context) error { t.Fatal("redirected phone proof reached write"); return nil }); err == nil {
		t.Fatal("redirected proof accepted")
	}
	email := "wrong@example.com"
	tampered = proof
	tampered.Phone = nil
	tampered.Email = &email
	if err = s.Apply(ctx, tampered, func(context.Context) error { t.Fatal("cross-field proof reached write"); return nil }); err == nil {
		t.Fatal("cross-field proof accepted")
	}
	originalNow := s.now
	s.now = func() time.Time { return originalNow().Add(11 * time.Minute) }
	if err = s.Apply(ctx, proof, func(context.Context) error { t.Fatal("expired proof reached write"); return nil }); err == nil {
		t.Fatal("expired proof accepted")
	}
	s.now = originalNow
	if err = s.Apply(ctx, proof, func(tx context.Context) error {
		_, e := db.Exec(tx, "UPDATE contact_test_identity SET phone=? WHERE id=42", freshInput.Destination)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	phone, err := db.GetValue(ctx, "SELECT phone FROM contact_test_identity WHERE id=42")
	if err != nil || phone.String() != freshInput.Destination {
		t.Fatal("phone binding not persisted", err)
	}
}
