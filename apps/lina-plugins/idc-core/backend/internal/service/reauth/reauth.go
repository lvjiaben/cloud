// Package reauth owns short-lived, operation-bound proof of a recent host login.
package reauth

import (
	"context"
	"crypto/rand"
	"io"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
)

// Purpose is one high-risk operation family that requires a fresh host login.
type Purpose string

// Supported recent-authentication purposes. Callers must additionally bind a
// proof to a digest of the exact target or immutable operation preview.
const (
	PurposeSecretAccess      Purpose = "secret.access"
	PurposePaymentConfig     Purpose = "payment.config"
	PurposeHighValueFunds    Purpose = "funds.high-value"
	PurposeBulkResource      Purpose = "resource.bulk"
	PurposeSensitiveExport   Purpose = "export.sensitive"
	PurposeSensitiveMaterial Purpose = "material.sensitive"
	PurposePermissionChange  Purpose = "permission.change"
	PurposeServerOperation   Purpose = "resource.server-operation"
)

// Config defines finite recent-login, proof, clock-skew, and token bounds.
// Zero values select safe defaults; negative values are rejected.
type Config struct {
	FreshLoginWindow time.Duration
	ProofTTL         time.Duration
	AllowedClockSkew time.Duration
	MaximumTokenSize int
}

// Signer keeps signing-key material outside this service and supports key
// rotation through an explicit key identifier. Implementations must use a
// cryptographically secure signature or MAC and constant-time verification.
type Signer interface {
	// ActiveKeyID returns the low-entropy identifier used for new proofs.
	ActiveKeyID(ctx context.Context) (string, error)
	// Sign signs a domain-separated message with the selected active key.
	Sign(ctx context.Context, keyID string, message []byte) ([]byte, error)
	// Verify authenticates a message with an active or retained verification key.
	Verify(ctx context.Context, keyID string, message []byte, signature []byte) (bool, error)
}

// ReplayClaim is the redacted, database-persisted single-use fence for one
// authenticated proof. It contains only fixed-size digests and bounded enum or
// identifier fields; raw proof, session, target, and signing-key bytes are never
// persisted.
type ReplayClaim struct {
	NonceDigest         string
	Purpose             Purpose
	TargetDigest        string
	SessionDigest       string
	UserID              int
	TenantID            int
	LoginAt             time.Time
	IssuedAt            time.Time
	ExpiresAt           time.Time
	SessionConfirmation bool
}

// ReplayStore is the database authority for proof single-use semantics.
// Claim must atomically and durably insert a unique tenant/nonce digest and
// return false when it already exists. A lossy cache implementation is
// forbidden because eviction or process restart would permit proof replay.
type ReplayStore interface {
	Claim(ctx context.Context, claim ReplayClaim) (bool, error)
}

// Binding identifies the exact dangerous operation authorized by one proof.
// TargetDigest is a lowercase SHA-256 digest of an immutable target or preview,
// never raw customer, resource, secret, or payment data.
type Binding struct {
	Purpose      Purpose
	TargetDigest string
}

// ProofContext is the opaque client-carried result of a recent login check.
type ProofContext struct {
	Token     string
	ExpiresAt time.Time
}

// Proof is the verified, single-use server-side projection passed to a high-risk operation.
type Proof struct {
	Purpose       Purpose
	TargetDigest  string
	UserID        int
	TenantID      int
	LoginAt       time.Time
	IssuedAt      time.Time
	ExpiresAt     time.Time
	SessionDigest string
}

// Service defines active-session proof issuance and single-use consumption.
type Service interface {
	// Issue binds an exact action to the current valid host session without
	// imposing an additional login-age limit.
	Issue(ctx context.Context, binding Binding) (*ProofContext, error)
	// Consume authenticates, session-binds, purpose-binds, target-binds, and
	// atomically consumes one proof. Replay-store or session failures reject the call;
	// callers must never continue a dangerous operation on degraded evidence.
	Consume(ctx context.Context, token string, expected Binding) (*Proof, error)
}

// SessionConfirmationService issues an exact, single-use confirmation from
// the current valid session without applying the recent-login time window.
type SessionConfirmationService interface {
	IssueSessionConfirmation(context.Context, Binding) (*ProofContext, error)
	Consume(context.Context, string, Binding) (*Proof, error)
}

// Ensure serviceImpl implements Service.
var _ Service = (*serviceImpl)(nil)
var _ SessionConfirmationService = (*serviceImpl)(nil)

// serviceImpl binds proofs to the shared request context, session store,
// database replay authority, signer, clock, and cryptographic randomness
// supplied at startup.
type serviceImpl struct {
	config     Config
	bizCtxSvc  bizctxcap.Service
	sessionSvc sessioncap.Service
	replay     ReplayStore
	signer     Signer
	now        func() time.Time
	random     io.Reader
}

// New creates a recent-authentication service from the exact host runtime
// dependencies shared by the current source-plugin composition.
func New(
	config Config,
	bizCtxSvc bizctxcap.Service,
	sessionSvc sessioncap.Service,
	replay ReplayStore,
	signer Signer,
) (Service, error) {
	return newService(config, bizCtxSvc, sessionSvc, replay, signer, time.Now, rand.Reader)
}

// newService exposes deterministic clock and randomness seams only to package tests.
func newService(
	config Config,
	bizCtxSvc bizctxcap.Service,
	sessionSvc sessioncap.Service,
	replay ReplayStore,
	signer Signer,
	now func() time.Time,
	random io.Reader,
) (*serviceImpl, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	dependencies := []struct {
		name  string
		value any
	}{
		{name: "host business context", value: bizCtxSvc},
		{name: "host session service", value: sessionSvc},
		{name: "database replay store", value: replay},
		{name: "proof signer", value: signer},
		{name: "clock", value: now},
		{name: "cryptographic random source", value: random},
	}
	for _, dependency := range dependencies {
		if isNilInterface(dependency.value) {
			return nil, gerror.Newf("recent authentication requires %s", dependency.name)
		}
	}
	return &serviceImpl{
		config: normalized, bizCtxSvc: bizCtxSvc, sessionSvc: sessionSvc,
		replay: replay, signer: signer, now: now, random: random,
	}, nil
}
