// This file verifies fresh-login, signed binding, session binding, and one-time proof invariants.

package reauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
)

func TestSensitiveMaterialPurposeIsCataloged(t *testing.T) {
	t.Parallel()
	binding := Binding{
		Purpose:      PurposeSensitiveMaterial,
		TargetDigest: DigestTarget([]byte("enterprise-material")),
	}
	if _, err := normalizeBinding(binding); err != nil {
		t.Fatalf("normalizeBinding() error = %v", err)
	}
}

// TestIssueConsumeIsSingleUseAndOperationBound verifies the normal path and
// proves mismatched targets do not burn an otherwise valid proof.
func TestIssueConsumeIsSingleUseAndOperationBound(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	service, _, _ := newReauthTestService(t, &now, now.Add(-time.Minute))
	binding := Binding{Purpose: PurposeBulkResource, TargetDigest: DigestTarget([]byte("preview-v1"))}
	issued, err := service.Issue(t.Context(), binding)
	if err != nil {
		t.Fatalf("issue proof: %v", err)
	}
	if issued.Token == "" || !issued.ExpiresAt.Equal(now.Add(defaultProofTTL)) {
		t.Fatalf("unexpected issued proof: %+v", issued)
	}
	wrong := binding
	wrong.TargetDigest = DigestTarget([]byte("preview-v2"))
	if _, err = service.Consume(t.Context(), issued.Token, wrong); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("wrong target error=%v, want proof rejected", err)
	}
	proof, err := service.Consume(t.Context(), issued.Token, binding)
	if err != nil {
		t.Fatalf("consume proof: %v", err)
	}
	if proof.UserID != 42 || proof.TenantID != 0 || proof.Purpose != binding.Purpose ||
		proof.TargetDigest != binding.TargetDigest || !proof.LoginAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("unexpected consumed proof: %+v", proof)
	}
	if _, err = service.Consume(t.Context(), issued.Token, binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("replayed proof error=%v, want proof rejected", err)
	}
}

// TestIssueRequiresActualRecentLogin verifies an access-token refresh cannot
// turn an old session LoginAt into recent authentication evidence.
func TestIssueRequiresActualRecentLogin(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	service, _, _ := newReauthTestService(t, &now, now.Add(-defaultFreshLoginWindow-time.Second))
	binding := Binding{Purpose: PurposeSecretAccess, TargetDigest: DigestTarget([]byte("secret-operation"))}
	if _, err := service.Issue(t.Context(), binding); !bizerr.Is(err, CodeRecentLoginRequired) {
		t.Fatalf("stale LoginAt error=%v, want recent login required", err)
	}
}

func TestServerOperationSessionConfirmationAcceptsOldActiveSession(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	service, bizCtx, sessions := newReauthTestService(t, &now, now.Add(-30*24*time.Hour))
	binding := Binding{Purpose: PurposeServerOperation,
		TargetDigest: DigestTarget([]byte("customer-server-operation"))}
	issued, err := service.IssueSessionConfirmation(t.Context(), binding)
	if err != nil || issued == nil || issued.Token == "" || !issued.ExpiresAt.Equal(now.Add(defaultProofTTL)) {
		t.Fatalf("session confirmation=%+v err=%v", issued, err)
	}
	if _, err = service.Consume(t.Context(), issued.Token, binding); err != nil {
		t.Fatalf("consume old-session confirmation: %v", err)
	}
	if _, err = service.Consume(t.Context(), issued.Token, binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("replayed session confirmation error=%v", err)
	}
	issued, err = service.IssueSessionConfirmation(t.Context(), binding)
	if err != nil {
		t.Fatalf("issue second session confirmation: %v", err)
	}
	bizCtx.current.TokenID = "replacement-session"
	sessions.current.ID = "replacement-session"
	if _, err = service.Consume(t.Context(), issued.Token, binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("cross-session confirmation error=%v", err)
	}
	if _, err = service.IssueSessionConfirmation(t.Context(), Binding{
		Purpose: PurposeSecretAccess, TargetDigest: binding.TargetDigest,
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("non-server session confirmation error=%v", err)
	}
}

// TestProofRejectsTamperingSessionChangeAndExpiry verifies neither a modified
// wire token nor a different newly logged-in session can reuse an old proof.
func TestProofRejectsTamperingSessionChangeAndExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	service, bizCtx, sessions := newReauthTestService(t, &now, now.Add(-time.Minute))
	binding := Binding{Purpose: PurposeSensitiveExport, TargetDigest: DigestTarget([]byte("export-42"))}
	issued, err := service.Issue(t.Context(), binding)
	if err != nil {
		t.Fatalf("issue proof: %v", err)
	}
	tampered := issued.Token[:len(issued.Token)-1] + "A"
	if tampered == issued.Token {
		tampered = issued.Token[:len(issued.Token)-1] + "B"
	}
	if _, err = service.Consume(t.Context(), tampered, binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("tampered proof error=%v, want rejected", err)
	}

	newLogin := now
	bizCtx.current.TokenID = "session-new"
	sessions.current.ID = "session-new"
	sessions.current.LoginAt = &newLogin
	if _, err = service.Consume(t.Context(), issued.Token, binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("different session error=%v, want rejected", err)
	}

	bizCtx.current.TokenID = "session-original"
	sessions.current.ID = "session-original"
	oldLogin := now.Add(-time.Minute)
	sessions.current.LoginAt = &oldLogin
	now = issued.ExpiresAt
	if _, err = service.Consume(t.Context(), issued.Token, binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("expired proof error=%v, want rejected", err)
	}
}

// TestProofWireHidesRawSessionAndBindsKeyID verifies the client-carried token
// contains only a digest of the host session and authenticates its key header.
func TestProofWireHidesRawSessionAndBindsKeyID(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	service, _, _ := newReauthTestService(t, &now, now.Add(-time.Minute))
	binding := Binding{Purpose: PurposeSecretAccess, TargetDigest: DigestTarget([]byte("secret-42"))}
	issued, err := service.Issue(t.Context(), binding)
	if err != nil {
		t.Fatalf("issue proof: %v", err)
	}
	if strings.Contains(issued.Token, "session-original") {
		t.Fatal("proof wire exposed the raw host session identifier")
	}
	parts := strings.Split(issued.Token, ".")
	if len(parts) != 4 {
		t.Fatalf("proof segments=%d, want 4", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode proof claims: %v", err)
	}
	if strings.Contains(string(payload), "session-original") ||
		!strings.Contains(string(payload), digestSessionID("session-original")) {
		t.Fatalf("proof claims did not contain only the session digest: %s", payload)
	}
	parts[1] = "test-key-v2"
	if _, err = service.Consume(t.Context(), strings.Join(parts, "."), binding); !bizerr.Is(err, CodeProofRejected) {
		t.Fatalf("tampered key ID error=%v, want proof rejected", err)
	}
}

// TestKeyIDExcludesWireDelimiter ensures configuration cannot mint a token
// whose key identifier changes the fixed four-segment wire structure.
func TestKeyIDExcludesWireDelimiter(t *testing.T) {
	if validKeyID("key.v1") {
		t.Fatal("key ID containing the proof delimiter must be rejected")
	}
	for _, value := range []string{"key-v1", "key_v1", "key:v1"} {
		if !validKeyID(value) {
			t.Fatalf("expected key ID %q to be valid", value)
		}
	}
}

// TestIssueRejectsDelegatedIdentityAndMismatchedSession verifies assistance,
// impersonation, and context/session drift cannot mint proofs.
func TestIssueRejectsDelegatedIdentityAndMismatchedSession(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	binding := Binding{Purpose: PurposePermissionChange, TargetDigest: DigestTarget([]byte("role-7"))}
	tests := []struct {
		name   string
		mutate func(*reauthTestBizCtx, *reauthTestSessions)
	}{
		{name: "impersonation", mutate: func(b *reauthTestBizCtx, _ *reauthTestSessions) { b.current.IsImpersonation = true }},
		{name: "acting user", mutate: func(b *reauthTestBizCtx, _ *reauthTestSessions) { b.current.ActingUserID = 99 }},
		{name: "session user mismatch", mutate: func(_ *reauthTestBizCtx, s *reauthTestSessions) { s.current.UserID = "43" }},
		{name: "session tenant mismatch", mutate: func(_ *reauthTestBizCtx, s *reauthTestSessions) { s.current.TenantID = "8" }},
		{name: "session token mismatch", mutate: func(_ *reauthTestBizCtx, s *reauthTestSessions) { s.current.ID = "other" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, bizCtx, sessions := newReauthTestService(t, &now, now.Add(-time.Minute))
			test.mutate(bizCtx, sessions)
			if _, err := service.Issue(t.Context(), binding); !bizerr.Is(err, CodeRecentLoginRequired) {
				t.Fatalf("error=%v, want recent login required", err)
			}
		})
	}
}

// TestConsumeFailsClosedOnSignerSessionAndReplayStoreFailures verifies no dependency
// outage degrades into permission to continue a dangerous operation.
func TestConsumeFailsClosedOnSignerSessionAndReplayStoreFailures(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	binding := Binding{Purpose: PurposePaymentConfig, TargetDigest: DigestTarget([]byte("channel-1"))}
	tests := []struct {
		name   string
		mutate func(*reauthTestSigner, *reauthTestSessions, *reauthTestReplayStore)
	}{
		{name: "signer", mutate: func(s *reauthTestSigner, _ *reauthTestSessions, _ *reauthTestReplayStore) {
			s.verifyErr = errors.New("kms unavailable")
		}},
		{name: "session", mutate: func(_ *reauthTestSigner, s *reauthTestSessions, _ *reauthTestReplayStore) {
			s.err = errors.New("session unavailable")
		}},
		{name: "replay store", mutate: func(_ *reauthTestSigner, _ *reauthTestSessions, store *reauthTestReplayStore) {
			store.err = errors.New("database unavailable")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _, sessions := newReauthTestService(t, &now, now.Add(-time.Minute))
			issued, err := service.Issue(t.Context(), binding)
			if err != nil {
				t.Fatalf("issue proof: %v", err)
			}
			test.mutate(service.signer.(*reauthTestSigner), sessions, service.replay.(*reauthTestReplayStore))
			if _, err = service.Consume(t.Context(), issued.Token, binding); !bizerr.Is(err, CodeUnavailable) {
				t.Fatalf("dependency failure error=%v, want unavailable", err)
			}
		})
	}
}

// TestConcurrentConsumeAllowsExactlyOne verifies the database claim is the
// cluster-safe replay fence rather than a process-local boolean.
func TestConcurrentConsumeAllowsExactlyOne(t *testing.T) {
	now := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	service, _, _ := newReauthTestService(t, &now, now.Add(-time.Minute))
	binding := Binding{Purpose: PurposeHighValueFunds, TargetDigest: DigestTarget([]byte("refund-preview"))}
	issued, err := service.Issue(t.Context(), binding)
	if err != nil {
		t.Fatalf("issue proof: %v", err)
	}
	const workers = 32
	var successes atomic.Int32
	errorsOut := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, consumeErr := service.Consume(context.Background(), issued.Token, binding)
			if consumeErr == nil {
				successes.Add(1)
				return
			}
			if !bizerr.Is(consumeErr, CodeProofRejected) {
				errorsOut <- consumeErr
			}
		}()
	}
	wait.Wait()
	close(errorsOut)
	for consumeErr := range errorsOut {
		t.Errorf("unexpected concurrent consume error: %v", consumeErr)
	}
	if got := successes.Load(); got != 1 {
		t.Fatalf("concurrent successes=%d, want exactly 1", got)
	}
}

// TestConstructorRejectsTypedNilDependency verifies runtime DI fails before a
// high-risk operation can retain an incomplete capability graph.
func TestConstructorRejectsTypedNilDependency(t *testing.T) {
	var sessions *reauthTestSessions
	_, err := New(Config{}, &reauthTestBizCtx{}, sessions, newReauthTestReplayStore(), newReauthTestSigner())
	if err == nil {
		t.Fatal("expected typed-nil session dependency to fail")
	}
}

func newReauthTestService(
	t *testing.T,
	now *time.Time,
	loginAt time.Time,
) (*serviceImpl, *reauthTestBizCtx, *reauthTestSessions) {
	t.Helper()
	bizCtx := &reauthTestBizCtx{current: bizctxcap.CurrentContext{
		TokenID: "session-original", UserID: 42, TenantID: 0,
	}}
	sessions := &reauthTestSessions{current: &sessioncap.SessionInfo{
		ID: "session-original", TenantID: "0", UserID: "42", LoginAt: &loginAt,
	}}
	service, err := newService(
		Config{}, bizCtx, sessions, newReauthTestReplayStore(), newReauthTestSigner(),
		func() time.Time { return *now }, newIncrementingReader(),
	)
	if err != nil {
		t.Fatalf("create recent authentication service: %v", err)
	}
	return service, bizCtx, sessions
}

type reauthTestBizCtx struct{ current bizctxcap.CurrentContext }

func (b *reauthTestBizCtx) Current(context.Context) bizctxcap.CurrentContext { return b.current }

type reauthTestSessions struct {
	current *sessioncap.SessionInfo
	err     error
}

func (s *reauthTestSessions) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return s.current, s.err
}
func (s *reauthTestSessions) Get(context.Context, sessioncap.SessionID) (*sessioncap.SessionInfo, error) {
	return nil, nil
}
func (s *reauthTestSessions) List(context.Context, sessioncap.ListInput) (*capmodel.PageResult[*sessioncap.SessionInfo], error) {
	return nil, nil
}
func (s *reauthTestSessions) BatchGet(context.Context, []sessioncap.SessionID) (*capmodel.BatchResult[*sessioncap.SessionInfo, sessioncap.SessionID], error) {
	return nil, nil
}
func (s *reauthTestSessions) BatchGetUserOnlineStatus(context.Context, []string) (*capmodel.BatchResult[*sessioncap.UserOnlineStatus, string], error) {
	return nil, nil
}
func (s *reauthTestSessions) EnsureVisible(context.Context, []sessioncap.SessionID) error { return nil }
func (s *reauthTestSessions) Revoke(context.Context, sessioncap.SessionID) error          { return nil }
func (s *reauthTestSessions) RevokeMany(context.Context, []sessioncap.SessionID) error    { return nil }

type reauthTestSigner struct {
	key       []byte
	verifyErr error
}

func newReauthTestSigner() *reauthTestSigner {
	return &reauthTestSigner{key: []byte("0123456789abcdef0123456789abcdef")}
}

func (s *reauthTestSigner) ActiveKeyID(context.Context) (string, error) { return "test-key-v1", nil }

func (s *reauthTestSigner) Sign(_ context.Context, _ string, message []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(message)
	return mac.Sum(nil), nil
}

func (s *reauthTestSigner) Verify(_ context.Context, _ string, message []byte, signature []byte) (bool, error) {
	if s.verifyErr != nil {
		return false, s.verifyErr
	}
	expected, _ := s.Sign(context.Background(), "test-key-v1", message)
	return hmac.Equal(expected, signature), nil
}

type reauthTestReplayStore struct {
	mutex  sync.Mutex
	values map[string]ReplayClaim
	err    error
}

func newReauthTestReplayStore() *reauthTestReplayStore {
	return &reauthTestReplayStore{values: make(map[string]ReplayClaim)}
}

func (s *reauthTestReplayStore) Claim(_ context.Context, claim ReplayClaim) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if !validReplayClaim(claim) {
		return false, errors.New("invalid replay claim")
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.values[claim.NonceDigest]; exists {
		return false, nil
	}
	s.values[claim.NonceDigest] = claim
	return true, nil
}

type incrementingReader struct {
	mutex sync.Mutex
	next  byte
}

func newIncrementingReader() *incrementingReader { return &incrementingReader{next: 1} }

func (r *incrementingReader) Read(value []byte) (int, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for index := range value {
		value[index] = r.next
		r.next++
	}
	return len(value), nil
}
