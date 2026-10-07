package supportdesk

import (
	"context"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const verifiedAnonymousTestToken = "0123456789abcdef0123456789abcdef0123456789a"

type verifiedAnonymousTokenManagerStub struct {
	secretcap.GeneratedManager
	mu     sync.Mutex
	handle secretcap.Handle
	calls  int
}

func (s *verifiedAnonymousTokenManagerStub) CreateGenerated(
	_ context.Context,
	idempotencyKey string,
	policy string,
) (secretcap.CreateResult, error) {
	if idempotencyKey == "" || policy != secretcap.GeneratedSecretPolicyURLSafe256 {
		return secretcap.CreateResult{}, gerror.New("invalid generated token request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return secretcap.CreateResult{Handle: s.handle, Reused: s.calls > 1}, nil
}

type verifiedAnonymousAccessorStub struct {
	mu             sync.Mutex
	handle         secretcap.Handle
	token          string
	failuresBefore int
}

func (s *verifiedAnonymousAccessorStub) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if !request.Handle.Equal(s.handle) || request.Purpose != anonymousAccessSecretPurpose || consume == nil {
		return gerror.New("invalid token access request")
	}
	s.mu.Lock()
	if s.failuresBefore > 0 {
		s.failuresBefore--
		s.mu.Unlock()
		return gerror.New("simulated process interruption")
	}
	token := []byte(s.token)
	s.mu.Unlock()
	defer clear(token)
	return consume(token)
}

func newVerifiedAnonymousTokenStubs(t *testing.T) (*verifiedAnonymousTokenManagerStub, *verifiedAnonymousAccessorStub) {
	t.Helper()
	handle, err := secretcap.NewHandle("SEC_verified_anonymous_test")
	if err != nil {
		t.Fatal(err)
	}
	return &verifiedAnonymousTokenManagerStub{handle: handle}, &verifiedAnonymousAccessorStub{
		handle: handle, token: verifiedAnonymousTestToken,
	}
}

func TestDeterministicAnonymousNumbersUseSeparatePurposes(t *testing.T) {
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	ticket := deterministicAnonymousNo("TKT_", digest)
	message := deterministicAnonymousNo("MSG_", digest)
	if ticket != deterministicAnonymousNo("TKT_", digest) || ticket == message || len(ticket) != 36 || len(message) != 36 {
		t.Fatalf("unexpected deterministic numbers: ticket=%q message=%q", ticket, message)
	}
}
