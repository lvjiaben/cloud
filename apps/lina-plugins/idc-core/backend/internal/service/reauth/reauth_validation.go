// This file centralizes bounded recent-authentication configuration and input validation.

package reauth

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
)

const (
	// Default bounds keep recent-login evidence substantially shorter-lived than a session.
	defaultFreshLoginWindow = 5 * time.Minute
	defaultProofTTL         = 2 * time.Minute
	defaultAllowedClockSkew = 15 * time.Second
	defaultMaximumTokenSize = 4096

	absoluteFreshLoginWindow = 15 * time.Minute
	absoluteProofTTL         = 5 * time.Minute
	absoluteAllowedClockSkew = time.Minute
	absoluteMaximumTokenSize = 16 * 1024

	minimumSignatureBytes = 32
	maximumSignatureBytes = 512
	maximumKeyIDBytes     = 128
	maximumSessionIDBytes = 4096
	nonceBytes            = 24
)

// normalizeConfig applies finite defaults and rejects security-weakening bounds.
func normalizeConfig(config Config) (Config, error) {
	if config.FreshLoginWindow < 0 || config.ProofTTL < 0 ||
		config.AllowedClockSkew < 0 || config.MaximumTokenSize < 0 {
		return Config{}, bizerr.NewCode(CodeInvalid)
	}
	if config.FreshLoginWindow == 0 {
		config.FreshLoginWindow = defaultFreshLoginWindow
	}
	if config.ProofTTL == 0 {
		config.ProofTTL = defaultProofTTL
	}
	if config.AllowedClockSkew == 0 {
		config.AllowedClockSkew = defaultAllowedClockSkew
	}
	if config.MaximumTokenSize == 0 {
		config.MaximumTokenSize = defaultMaximumTokenSize
	}
	if config.FreshLoginWindow > absoluteFreshLoginWindow ||
		config.ProofTTL > absoluteProofTTL ||
		config.ProofTTL > config.FreshLoginWindow ||
		config.AllowedClockSkew > absoluteAllowedClockSkew ||
		config.MaximumTokenSize > absoluteMaximumTokenSize ||
		config.MaximumTokenSize < 512 {
		return Config{}, bizerr.NewCode(CodeInvalid)
	}
	return config, nil
}

// normalizeBinding accepts only a known purpose and a canonical opaque digest.
func normalizeBinding(binding Binding) (Binding, error) {
	if !validPurpose(binding.Purpose) || !isLowerHexDigest(binding.TargetDigest) {
		return Binding{}, bizerr.NewCode(CodeInvalid)
	}
	return binding, nil
}

func validReplayClaim(claim ReplayClaim) bool {
	if !isLowerHexDigest(claim.NonceDigest) || !validPurpose(claim.Purpose) ||
		!isLowerHexDigest(claim.TargetDigest) || !isLowerHexDigest(claim.SessionDigest) ||
		claim.UserID <= 0 || claim.TenantID != 0 || claim.LoginAt.IsZero() ||
		claim.IssuedAt.IsZero() || claim.ExpiresAt.IsZero() {
		return false
	}
	loginAt := databaseTime(claim.LoginAt)
	issuedAt := databaseTime(claim.IssuedAt)
	expiresAt := databaseTime(claim.ExpiresAt)
	if loginAt.After(issuedAt.Add(absoluteAllowedClockSkew)) || !expiresAt.After(issuedAt) ||
		expiresAt.After(issuedAt.Add(absoluteProofTTL)) {
		return false
	}
	if claim.SessionConfirmation {
		return claim.Purpose == PurposeServerOperation
	}
	return !issuedAt.After(loginAt.Add(absoluteFreshLoginWindow))
}

// validPurpose reports whether a purpose is part of the closed high-risk catalog.
func validPurpose(value Purpose) bool {
	switch value {
	case PurposeSecretAccess, PurposePaymentConfig, PurposeHighValueFunds,
		PurposeBulkResource, PurposeSensitiveExport, PurposeSensitiveMaterial,
		PurposePermissionChange, PurposeServerOperation:
		return true
	default:
		return false
	}
}

// validKeyID accepts only bounded, low-entropy key identifiers safe for token headers.
func validKeyID(value string) bool {
	if value == "" || len(value) > maximumKeyIDBytes || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

// isLowerHexDigest verifies one canonical SHA-256 text encoding.
func isLowerHexDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range []byte(value) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// DigestTarget returns the stable lowercase SHA-256 binding for an immutable,
// already-canonical operation target or preview.
func DigestTarget(canonical []byte) string {
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

// isNilInterface rejects nil and typed-nil runtime dependencies at construction.
func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// databaseTime normalizes signed timestamps to the proof wire precision.
func databaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Millisecond)
}
