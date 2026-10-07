// This file provides the production-grade HMAC-SHA256 proof signer adapter.

package reauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// Signing keys must provide at least 256 bits and remain finitely bounded.
	minimumSigningKeyBytes = 32
	maximumSigningKeyBytes = 4096
)

// ActiveSigningKey is one atomic provider snapshot used for new proofs.
type ActiveSigningKey struct {
	KeyID string
	Key   []byte
}

// SigningKeyProvider keeps deployment-owned proof keys outside persisted
// business data. Active returns an atomic ID/key snapshot for new proofs;
// Resolve returns a detached key buffer for active or retained verification
// keys. The signer clears every returned buffer after use.
type SigningKeyProvider interface {
	// Active returns one internally consistent active key identifier and detached
	// key buffer. Providers must not read the identifier and bytes in separate,
	// independently mutable operations.
	Active(ctx context.Context) (ActiveSigningKey, error)
	// Resolve returns a detached active or retained verification-key buffer.
	// An unknown or retired key ID returns an empty buffer with no error; backend
	// failures return an error. The caller clears every non-empty returned buffer.
	Resolve(ctx context.Context, keyID string) ([]byte, error)
}

// hmacSigner authenticates proof messages without retaining provider key buffers.
type hmacSigner struct {
	keys SigningKeyProvider
}

// NewHMACSigner creates a rotating HMAC-SHA256 proof signer. Runtime
// composition must supply keys from deployment configuration or an external
// KMS-backed provider; database fallback is forbidden.
func NewHMACSigner(keys SigningKeyProvider) (Signer, error) {
	if isNilInterface(keys) {
		return nil, gerror.New("recent authentication signer requires a signing key provider")
	}
	return &hmacSigner{keys: keys}, nil
}

// ActiveKeyID validates the provider's current low-entropy key identifier.
func (s *hmacSigner) ActiveKeyID(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", gerror.New("recent authentication signing context is required")
	}
	active, err := s.keys.Active(ctx)
	defer zeroBytes(active.Key)
	if err != nil {
		return "", gerror.Wrap(err, "resolve active recent authentication key ID")
	}
	if !validKeyID(active.KeyID) ||
		len(active.Key) < minimumSigningKeyBytes || len(active.Key) > maximumSigningKeyBytes {
		return "", gerror.New("active recent authentication key ID is invalid")
	}
	return active.KeyID, nil
}

// Sign authenticates a message only if keyID remains active at signing time.
func (s *hmacSigner) Sign(ctx context.Context, keyID string, message []byte) ([]byte, error) {
	if ctx == nil || !validKeyID(keyID) || len(message) == 0 {
		return nil, gerror.New("recent authentication signing request is invalid")
	}
	active, err := s.keys.Active(ctx)
	defer zeroBytes(active.Key)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve active recent authentication signing key")
	}
	if active.KeyID != keyID || !validKeyID(active.KeyID) ||
		len(active.Key) < minimumSigningKeyBytes || len(active.Key) > maximumSigningKeyBytes {
		return nil, gerror.New("recent authentication signing key changed before signing")
	}
	mac := hmac.New(sha256.New, active.Key)
	_, _ = mac.Write(message)
	return mac.Sum(nil), nil
}

// Verify authenticates a message with an active or retained key in constant time.
func (s *hmacSigner) Verify(
	ctx context.Context,
	keyID string,
	message []byte,
	signature []byte,
) (bool, error) {
	if ctx == nil || !validKeyID(keyID) || len(message) == 0 || len(signature) != sha256.Size {
		return false, nil
	}
	key, err := s.keys.Resolve(ctx, keyID)
	if err != nil {
		zeroBytes(key)
		return false, err
	}
	defer zeroBytes(key)
	// Providers return an empty detached buffer for an unknown/retired key.
	// Treat that as an ordinary invalid proof so an untrusted key header cannot
	// turn authentication failure into a 500 response or key-presence oracle.
	if len(key) < minimumSigningKeyBytes || len(key) > maximumSigningKeyBytes {
		return false, nil
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(message)
	return hmac.Equal(mac.Sum(nil), signature), nil
}

// zeroBytes performs a best-effort in-place wipe of short-lived key buffers.
func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
