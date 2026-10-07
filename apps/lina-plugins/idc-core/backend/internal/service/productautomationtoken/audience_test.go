// This file verifies the production provider-selection audience security boundary.

package productautomationtoken

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type audienceBizCtxStub struct {
	current bizctxcap.CurrentContext
	calls   *atomic.Int64
}

func (s audienceBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return s.current
}

type audiencePrincipalStub struct {
	audience *principal.Audience
	err      error
	calls    *atomic.Int64
	after    func()
}

func (s audiencePrincipalStub) Current(context.Context) (*principal.Audience, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	if s.after != nil {
		s.after()
	}
	return s.audience, s.err
}

type audienceSessionStub struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
	err     error
	calls   *atomic.Int64
	after   func()
}

func (s audienceSessionStub) Current(context.Context) (*sessioncap.SessionInfo, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	if s.after != nil {
		s.after()
	}
	return s.current, s.err
}

func TestProductionAudienceReaderReturnsStablePurposeIsolatedDigests(t *testing.T) {
	key := bytes.Repeat([]byte{0x51}, 32)
	reader, err := NewProductionAudienceReader(key, validAudienceBizCtx(), validAudiencePrincipal(), validAudienceSession())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)
	if !bytes.Equal(key, make([]byte, len(key))) {
		t.Fatal("constructor did not clear the caller's temporary key")
	}

	first, err := reader.ReadProviderSelectionAudience(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := reader.ReadProviderSelectionAudience(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.TenantID != 0 || !first.DirectStaff || first.Proxied || first.ActingTenant {
		t.Fatalf("unexpected stable audience: %+v / %+v", first, second)
	}
	if first.ActorDigest == first.SessionDigest {
		t.Fatal("actor and session digest domains were not isolated")
	}
	for _, raw := range []string{"42", "session-42", "admin@example.test"} {
		if strings.Contains(first.ActorDigest, raw) || strings.Contains(first.SessionDigest, raw) {
			t.Fatalf("audience leaked raw value %q", raw)
		}
	}

	otherKey := bytes.Repeat([]byte{0x52}, 32)
	other, err := NewProductionAudienceReader(otherKey, validAudienceBizCtx(), validAudiencePrincipal(), validAudienceSession())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.Close)
	otherAudience, err := other.ReadProviderSelectionAudience(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.ActorDigest == otherAudience.ActorDigest || first.SessionDigest == otherAudience.SessionDigest {
		t.Fatal("audience digests were not bound to the installation key")
	}
}

func TestProductionAudienceReaderFailsClosed(t *testing.T) {
	tests := map[string]struct {
		mutate       func(*bizctxcap.CurrentContext, **principal.Audience, **sessioncap.SessionInfo)
		principalErr error
		sessionErr   error
	}{
		"customer": {mutate: func(_ *bizctxcap.CurrentContext, audience **principal.Audience, _ **sessioncap.SessionInfo) {
			(*audience).Code = principal.AudienceCustomer
		}},
		"revoked": {mutate: func(_ *bizctxcap.CurrentContext, audience **principal.Audience, _ **sessioncap.SessionInfo) {
			(*audience).Status = principal.AudienceStatusRevoked
		}},
		"missing audience": {mutate: func(_ *bizctxcap.CurrentContext, audience **principal.Audience, _ **sessioncap.SessionInfo) {
			*audience = nil
		}},
		"mismatched session id": {mutate: func(_ *bizctxcap.CurrentContext, _ **principal.Audience, session **sessioncap.SessionInfo) {
			(*session).ID = "other"
		}},
		"mismatched session user": {mutate: func(_ *bizctxcap.CurrentContext, _ **principal.Audience, session **sessioncap.SessionInfo) {
			(*session).UserID = "41"
		}},
		"missing session": {mutate: func(_ *bizctxcap.CurrentContext, _ **principal.Audience, session **sessioncap.SessionInfo) {
			*session = nil
		}},
		"proxy": {mutate: func(current *bizctxcap.CurrentContext, _ **principal.Audience, _ **sessioncap.SessionInfo) {
			current.IsImpersonation = true
		}},
		"acting user": {mutate: func(current *bizctxcap.CurrentContext, _ **principal.Audience, _ **sessioncap.SessionInfo) {
			current.ActingUserID = 7
		}},
		"acting tenant": {mutate: func(current *bizctxcap.CurrentContext, _ **principal.Audience, _ **sessioncap.SessionInfo) {
			current.ActingAsTenant = true
		}},
		"foreign tenant": {mutate: func(current *bizctxcap.CurrentContext, _ **principal.Audience, _ **sessioncap.SessionInfo) {
			current.TenantID = 9
		}},
		"foreign session tenant": {mutate: func(_ *bizctxcap.CurrentContext, _ **principal.Audience, session **sessioncap.SessionInfo) {
			(*session).TenantID = capmodel.DomainID("9")
		}},
		"principal unavailable": {principalErr: errors.New("database unavailable")},
		"session unavailable":   {sessionErr: errors.New("session unavailable")},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			current := validAudienceBizCtx().current
			audience := validAudiencePrincipal().audience
			session := validAudienceSession().current
			if test.mutate != nil {
				test.mutate(&current, &audience, &session)
			}
			key := bytes.Repeat([]byte{0x61}, 32)
			reader, err := NewProductionAudienceReader(
				key,
				audienceBizCtxStub{current: current},
				audiencePrincipalStub{audience: audience, err: test.principalErr},
				audienceSessionStub{current: session, err: test.sessionErr},
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reader.Close)
			if value, readErr := reader.ReadProviderSelectionAudience(context.Background()); readErr == nil || value != (Audience{}) {
				t.Fatalf("unsafe audience was accepted: value=%+v err=%v", value, readErr)
			} else if current.TokenID != "" && strings.Contains(readErr.Error(), current.TokenID) {
				t.Fatalf("failure leaked the raw session token: %v", readErr)
			}
		})
	}
}

func TestProductionAudienceReaderRejectsInvalidDependenciesAndClosedKey(t *testing.T) {
	short := []byte("short")
	if _, err := NewProductionAudienceReader(short, validAudienceBizCtx(), validAudiencePrincipal(), validAudienceSession()); err == nil {
		t.Fatal("short key was accepted")
	}
	if !bytes.Equal(short, make([]byte, len(short))) {
		t.Fatal("rejected constructor did not clear the caller's temporary key")
	}
	key := bytes.Repeat([]byte{0x71}, 32)
	reader, err := NewProductionAudienceReader(key, validAudienceBizCtx(), validAudiencePrincipal(), validAudienceSession())
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if value, readErr := reader.ReadProviderSelectionAudience(context.Background()); readErr == nil || value != (Audience{}) {
		t.Fatalf("closed reader did not fail closed: value=%+v err=%v", value, readErr)
	}
}

func TestProductionAudienceReaderRejectsTypedNilDependencies(t *testing.T) {
	var (
		nilBizCtx    *audienceBizCtxStub
		nilPrincipal *audiencePrincipalStub
		nilSession   *audienceSessionStub
	)
	tests := map[string]struct {
		bizCtx    bizctxcap.Service
		principal principal.Reader
		sessions  sessioncap.Service
	}{
		"bizctx":    {bizCtx: nilBizCtx, principal: validAudiencePrincipal(), sessions: validAudienceSession()},
		"principal": {bizCtx: validAudienceBizCtx(), principal: nilPrincipal, sessions: validAudienceSession()},
		"session":   {bizCtx: validAudienceBizCtx(), principal: validAudiencePrincipal(), sessions: nilSession},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			key := bytes.Repeat([]byte{0x72}, 32)
			if _, err := NewProductionAudienceReader(key, test.bizCtx, test.principal, test.sessions); err == nil {
				t.Fatal("typed-nil dependency was accepted")
			}
			if !bytes.Equal(key, make([]byte, len(key))) {
				t.Fatal("typed-nil rejection did not clear the caller key")
			}
		})
	}
}

func TestProductionAudienceReaderRejectsCanceledContextsAtEveryBoundary(t *testing.T) {
	t.Run("already canceled", func(t *testing.T) {
		var bizCtxCalls atomic.Int64
		bizCtx := validAudienceBizCtx()
		bizCtx.calls = &bizCtxCalls
		reader := mustProductionAudienceReader(t, bizCtx, validAudiencePrincipal(), validAudienceSession())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if value, err := reader.ReadProviderSelectionAudience(ctx); err == nil || value != (Audience{}) {
			t.Fatalf("canceled context was accepted: value=%+v err=%v", value, err)
		}
		if bizCtxCalls.Load() != 0 {
			t.Fatal("already-canceled context called an identity dependency")
		}
	})
	t.Run("expired deadline", func(t *testing.T) {
		var bizCtxCalls atomic.Int64
		bizCtx := validAudienceBizCtx()
		bizCtx.calls = &bizCtxCalls
		reader := mustProductionAudienceReader(t, bizCtx, validAudiencePrincipal(), validAudienceSession())
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		if value, err := reader.ReadProviderSelectionAudience(ctx); err == nil || value != (Audience{}) {
			t.Fatalf("expired context was accepted: value=%+v err=%v", value, err)
		}
		if bizCtxCalls.Load() != 0 {
			t.Fatal("expired context called an identity dependency")
		}
	})
	t.Run("canceled after principal", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var sessionCalls atomic.Int64
		principalReader := validAudiencePrincipal()
		principalReader.after = cancel
		sessions := validAudienceSession()
		sessions.calls = &sessionCalls
		reader := mustProductionAudienceReader(t, validAudienceBizCtx(), principalReader, sessions)
		if value, err := reader.ReadProviderSelectionAudience(ctx); err == nil || value != (Audience{}) {
			t.Fatalf("principal cancellation was accepted: value=%+v err=%v", value, err)
		}
		if sessionCalls.Load() != 0 {
			t.Fatal("principal cancellation still called the session dependency")
		}
	})
	t.Run("canceled after session", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		sessions := validAudienceSession()
		sessions.after = cancel
		reader := mustProductionAudienceReader(t, validAudienceBizCtx(), validAudiencePrincipal(), sessions)
		if value, err := reader.ReadProviderSelectionAudience(ctx); err == nil || value != (Audience{}) {
			t.Fatalf("session cancellation was accepted: value=%+v err=%v", value, err)
		}
	})
}

func TestProductionAudienceReaderConcurrentReadAndClose(t *testing.T) {
	var (
		bizCtxCalls    atomic.Int64
		principalCalls atomic.Int64
		sessionCalls   atomic.Int64
		once           sync.Once
	)
	entered := make(chan struct{})
	release := make(chan struct{})
	bizCtx := validAudienceBizCtx()
	bizCtx.calls = &bizCtxCalls
	principalReader := validAudiencePrincipal()
	principalReader.calls = &principalCalls
	principalReader.after = func() {
		once.Do(func() { close(entered) })
		<-release
	}
	sessions := validAudienceSession()
	sessions.calls = &sessionCalls
	reader := mustProductionAudienceReader(t, bizCtx, principalReader, sessions)

	type result struct {
		audience Audience
		err      error
	}
	const workers = 64
	results := make(chan result, workers)
	start := make(chan struct{})
	var workersDone sync.WaitGroup
	for index := 0; index < workers; index++ {
		workersDone.Add(1)
		go func() {
			defer workersDone.Done()
			<-start
			value, err := reader.ReadProviderSelectionAudience(context.Background())
			results <- result{audience: value, err: err}
		}()
	}
	close(start)
	<-entered
	closeDone := make(chan struct{})
	go func() {
		reader.Close()
		close(closeDone)
	}()
	close(release)
	workersDone.Wait()
	<-closeDone
	close(results)
	for result := range results {
		if result.err == nil {
			if !result.audience.DirectStaff || result.audience.TenantID != 0 {
				t.Fatalf("concurrent read returned an invalid success: %+v", result.audience)
			}
			continue
		}
		if result.audience != (Audience{}) || strings.Contains(result.err.Error(), "session-42") {
			t.Fatalf("concurrent read returned an unsafe failure: value=%+v err=%v", result.audience, result.err)
		}
	}

	before := [3]int64{bizCtxCalls.Load(), principalCalls.Load(), sessionCalls.Load()}
	for index := 0; index < workers; index++ {
		if value, err := reader.ReadProviderSelectionAudience(context.Background()); err == nil || value != (Audience{}) {
			t.Fatalf("closed reader accepted request %d: value=%+v err=%v", index, value, err)
		}
	}
	after := [3]int64{bizCtxCalls.Load(), principalCalls.Load(), sessionCalls.Load()}
	if before != after {
		t.Fatalf("closed reader called identity dependencies: before=%v after=%v", before, after)
	}
}

func mustProductionAudienceReader(
	t *testing.T,
	bizCtx bizctxcap.Service,
	principalReader principal.Reader,
	sessions sessioncap.Service,
) *ProductionAudienceReader {
	t.Helper()
	key := bytes.Repeat([]byte{0x73}, 32)
	reader, err := NewProductionAudienceReader(key, bizCtx, principalReader, sessions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)
	return reader
}

func validAudienceBizCtx() audienceBizCtxStub {
	return audienceBizCtxStub{current: bizctxcap.CurrentContext{
		TokenID: "session-42", UserID: 42, Username: "admin@example.test", TenantID: 0, PlatformBypass: true,
	}}
}

func validAudiencePrincipal() audiencePrincipalStub {
	return audiencePrincipalStub{audience: &principal.Audience{
		UserID: 42, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 1,
	}}
}

func validAudienceSession() audienceSessionStub {
	return audienceSessionStub{current: &sessioncap.SessionInfo{
		ID: "session-42", TenantID: capmodel.DomainID("0"), UserID: "42", Username: "admin@example.test",
	}}
}
