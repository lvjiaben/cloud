// This file proves recovery proof isolation, rollback, replay rejection and exact
// old-session denial using a real PostgreSQL transaction with scoped capability doubles.
package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/internal/service/loginprofile"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recoveryDirectory struct {
	customerlogincap.Directory
	database gdb.DB
}

func (d recoveryDirectory) Resolve(ctx context.Context, kind customerlogincap.Kind, value string) (*customerlogincap.Identity, error) {
	var p loginprofile.Snapshot
	if err := d.database.GetScan(ctx, &p, "SELECT id,email,phone FROM contact_test_identity WHERE id=42"); err != nil {
		return nil, err
	}
	item := &customerlogincap.Identity{UserID: p.ID, CustomerNo: "CUS_contact_test", Username: "test-customer", Email: p.Email, Phone: p.Phone}
	if !sameLoginContact(kind, value, item) {
		return nil, bizerr.NewCode(CodeVerificationInvalid)
	}
	return item, nil
}

type recoveryAuth struct {
	loginprofile.AuthClient
	enabled bool
}

func (a recoveryAuth) RecoveryEnabled(context.Context) error {
	if !a.enabled {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

type recoverySessions struct {
	sessioncap.Service
	mu    sync.Mutex
	items map[sessioncap.SessionID]*sessioncap.SessionInfo
	fail  bool
}

func (s *recoverySessions) List(_ context.Context, in sessioncap.ListInput) (*capmodel.PageResult[*sessioncap.SessionInfo], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]*sessioncap.SessionInfo, 0, len(s.items))
	for _, v := range s.items {
		items = append(items, v)
	}
	return &capmodel.PageResult[*sessioncap.SessionInfo]{Items: items, Total: len(items)}, nil
}
func (s *recoverySessions) BatchGet(_ context.Context, ids []sessioncap.SessionID) (*capmodel.BatchResult[*sessioncap.SessionInfo, sessioncap.SessionID], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &capmodel.BatchResult[*sessioncap.SessionInfo, sessioncap.SessionID]{Items: map[sessioncap.SessionID]*sessioncap.SessionInfo{}}
	for _, id := range ids {
		if v := s.items[id]; v != nil {
			r.Items[id] = v
		} else {
			r.MissingIDs = append(r.MissingIDs, id)
		}
	}
	return r, nil
}
func (s *recoverySessions) RevokeMany(ctx context.Context, ids []sessioncap.SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("temporary host revocation failure")
	}
	uid := strconv.Itoa(bizctxcap.CurrentFromContext(ctx).UserID)
	for _, id := range ids {
		if s.items[id] != nil && s.items[id].UserID != uid {
			return errors.New("foreign session")
		}
	}
	for _, id := range ids {
		delete(s.items, id)
	}
	return nil
}
func recoveryFixture(t *testing.T) (*serviceImpl, *presalesAdapterStub, context.Context, gdb.DB, *recoverySessions) {
	t.Helper()
	s, adapter, ctx, db := contactFixture(t)
	_, file, _, _ := runtime.Caller(0)
	sql, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/011-customer-password-recovery.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "ALTER TABLE contact_test_identity ADD COLUMN password TEXT NOT NULL DEFAULT 'old-password'"); err != nil {
		t.Fatal(err)
	}
	s.loginDirectory = recoveryDirectory{database: db}
	s.loginAuth = recoveryAuth{enabled: true}
	now := time.Now().UTC()
	sessions := &recoverySessions{items: map[sessioncap.SessionID]*sessioncap.SessionInfo{
		"session-contact-42": {ID: "session-contact-42", UserID: "42", TenantID: "0", LoginAt: &now},
		"old-other-device":   {ID: "old-other-device", UserID: "42", TenantID: "0", LoginAt: &now},
		"foreign-session":    {ID: "foreign-session", UserID: "43", TenantID: "0", LoginAt: &now},
	}}
	s.sessions = sessions
	return s, adapter, ctx, db, sessions
}
func recoveryReady(t *testing.T, s *serviceImpl, a *presalesAdapterStub, ctx context.Context, key string) (RecoveryInput, string, string) {
	t.Helper()
	input := RecoveryInput{Kind: customerlogincap.Email, Account: "old@example.com", BrowserNonce: "browser-recovery-0123456789", Locale: "en-US"}
	sent, err := s.SendRecoveryCode(ctx, RecoveryCodeInput{RecoveryInput: input, ProofToken: "proof-test", IdempotencyKey: key, ClientIP: "192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	code := a.lastCode()
	if err = s.verifyBoundContactCode(ctx, 42, sent.VerificationNo, code, func(tx context.Context, revision int64) (*loginContactBinding, error) {
		return s.recoveryBinding(tx, input, revision)
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(recoveryProof{RecoveryInput: input, VerificationNo: sent.VerificationNo, CleanupToken: strings.Repeat("c", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return input, sent.VerificationNo, base64.RawURLEncoding.EncodeToString(raw)
}
func TestRecoveryRollbackReplayAndExactOldSessionsPostgreSQL(t *testing.T) {
	s, a, ctx, db, sessions := recoveryFixture(t)
	_, number, proof := recoveryReady(t, s, a, ctx, "recovery-rollback-0001")
	write := func(tx context.Context) error {
		_, err := db.Exec(tx, "UPDATE contact_test_identity SET password='new-password' WHERE id=42")
		return err
	}
	if err := s.ApplyRecovery(ctx, proof, func(tx context.Context) error {
		if err := write(tx); err != nil {
			return err
		}
		return errors.New("after original host write")
	}); err == nil {
		t.Fatal("accepted failed write")
	}
	password, err := db.GetValue(ctx, "SELECT password FROM contact_test_identity WHERE id=42")
	if err != nil || password.String() != "old-password" {
		t.Fatal("password not rolled back", err)
	}
	status, err := db.GetValue(ctx, "SELECT status_code FROM plugin_idc_notify_contact_verification WHERE verification_no=?", number)
	if err != nil || status.String() != contactVerified {
		t.Fatal("proof consumed on failed write", err)
	}
	blocked, err := db.GetValue(ctx, "SELECT count(*) FROM plugin_idc_notify_revoked_login_session")
	if err != nil || blocked.Int() != 0 {
		t.Fatal("denial escaped rolled-back write", err)
	}
	if err = s.ApplyRecovery(ctx, proof, write); err != nil {
		t.Fatal(err)
	}
	if err = s.ApplyRecovery(ctx, proof, write); err == nil {
		t.Fatal("replayed reset")
	}
	sessions.fail = true
	if pending, err := s.CleanupRecoverySessions(ctx, number); err == nil || !pending {
		t.Fatal("hidden cleanup failure")
	}
	if err = s.CheckRecoverySession(ctx); !bizerr.Is(err, CodePasswordChanged) {
		t.Fatal("old session still usable", err)
	}
	// Timestamp equality cannot mistake a newly issued exact session ID for old.
	sessions.mu.Lock()
	at := sessions.items["session-contact-42"].LoginAt
	sessions.items["new-same-time"] = &sessioncap.SessionInfo{ID: "new-same-time", UserID: "42", TenantID: "0", LoginAt: at}
	sessions.mu.Unlock()
	fresh := bizctxcap.WithCurrentContext(ctx, bizctxcap.CurrentContext{UserID: 42, TenantID: 0, TokenID: "new-same-time"})
	if err = s.CheckRecoverySession(fresh); err != nil {
		t.Fatal("new session denied at same timestamp", err)
	}
	sessions.fail = false
	if pending, err := s.CleanupRecoverySessions(fresh, number); err != nil || pending {
		t.Fatal("cleanup retry failed", err)
	}
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if len(sessions.items) != 2 || sessions.items["new-same-time"] == nil || sessions.items["foreign-session"] == nil {
		t.Fatal("cleanup affected newer/foreign session")
	}
}
func TestRecoveryConcurrentSingleConsumptionPostgreSQL(t *testing.T) {
	s, a, ctx, db, _ := recoveryFixture(t)
	_, _, proof := recoveryReady(t, s, a, ctx, "recovery-race-00001")
	var writes, success atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := s.ApplyRecovery(ctx, proof, func(tx context.Context) error {
				writes.Add(1)
				_, err := db.Exec(tx, "UPDATE contact_test_identity SET password='new-password' WHERE id=42")
				return err
			}); err == nil {
				success.Add(1)
			}
		})
	}
	wg.Wait()
	if writes.Load() != 1 || success.Load() != 1 {
		t.Fatalf("writes=%d success=%d", writes.Load(), success.Load())
	}
}
func TestRecoveryWrongCodeLockoutAndDisabledPostgreSQL(t *testing.T) {
	s, a, ctx, db, _ := recoveryFixture(t)
	input := RecoveryInput{Kind: customerlogincap.Email, Account: "old@example.com", BrowserNonce: "browser-recovery-negative-01", Locale: "en-US"}
	sent, err := s.SendRecoveryCode(ctx, RecoveryCodeInput{RecoveryInput: input, ProofToken: "proof-test", IdempotencyKey: "recovery-negative-01", ClientIP: "192.0.2.11"})
	if err != nil {
		t.Fatal(err)
	}
	actual := a.lastCode()
	resolver := func(tx context.Context, revision int64) (*loginContactBinding, error) {
		return s.recoveryBinding(tx, input, revision)
	}
	wrong := "000000"
	if actual == wrong {
		wrong = "111111"
	}
	for range 5 {
		if err = s.verifyBoundContactCode(ctx, 42, sent.VerificationNo, wrong, resolver); err == nil {
			t.Fatal("accepted wrong code")
		}
	}
	failed, err := db.GetValue(ctx, "SELECT failed_attempts FROM plugin_idc_notify_contact_verification WHERE verification_no=?", sent.VerificationNo)
	if err != nil || failed.Int() != 5 {
		t.Fatal("wrong attempts rolled back", err)
	}
	if err = s.verifyBoundContactCode(ctx, 42, sent.VerificationNo, actual, resolver); err == nil {
		t.Fatal("unlocked code after five failures")
	}
	s.loginAuth = recoveryAuth{enabled: false}
	if _, err = s.SendRecoveryCode(ctx, RecoveryCodeInput{RecoveryInput: input, ProofToken: "proof-test", IdempotencyKey: "recovery-disabled-01", ClientIP: "192.0.2.11"}); err == nil {
		t.Fatal("recovery bypassed disabled switch")
	}
}

func TestRecoveryBindingExpiryAndOriginalRouteGuardPostgreSQL(t *testing.T) {
	for _, variant := range []string{"wrong_browser", "changed_contact", "expired", "original_guard"} {
		t.Run(variant, func(t *testing.T) {
			s, a, ctx, db, _ := recoveryFixture(t)
			_, _, proof := recoveryReady(t, s, a, ctx, "recovery-binding-"+variant)
			switch variant {
			case "wrong_browser":
				raw, err := base64.RawURLEncoding.DecodeString(proof)
				if err != nil {
					t.Fatal(err)
				}
				var p recoveryProof
				if err = json.Unmarshal(raw, &p); err != nil {
					t.Fatal(err)
				}
				p.BrowserNonce = "other-browser-0123456789"
				raw, err = json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				proof = base64.RawURLEncoding.EncodeToString(raw)
			case "changed_contact":
				if _, err := db.Exec(ctx, "UPDATE contact_test_identity SET email='changed@example.com' WHERE id=42"); err != nil {
					t.Fatal(err)
				}
			case "expired":
				now := s.now()
				s.now = func() time.Time { return now.Add(11 * time.Minute) }
			}
			writes := 0
			err := s.ApplyRecovery(ctx, proof, func(tx context.Context) error { writes++; return s.CheckRecoverySession(tx) })
			if variant == "original_guard" {
				if err != nil || writes != 1 {
					t.Fatal("private verified transaction could not reach original route", err)
				}
				if err = s.CheckRecoverySession(ctx); !bizerr.Is(err, CodePasswordChanged) {
					t.Fatal("verified-route exception leaked outside transaction", err)
				}
			} else if err == nil || writes != 0 {
				t.Fatal("invalid recovery reached password write", variant, err)
			}
		})
	}
}
