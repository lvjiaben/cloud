// This file centralizes bounded input normalization and authorization failure handling.

package secret

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	defaultMaximumPlaintextSize = 64 * 1024
	defaultMaximumRotationBatch = 100
	defaultMaximumKeyWraps      = 64
	maximumRotationBatch        = 500
	maximumPlaintextSize        = 1024 * 1024
	maximumIdempotencyKeyBytes  = 256
	maximumOwnerPluginIDBytes   = 128
	maximumCallerScopeBytes     = 128
	maximumKeyIDBytes           = 128
	maximumNumberAttempts       = 8
	sha256DigestSize            = 32
	secretNumberPrefix          = "SEC_"
	secretNumberType            = "secret"
	secretNumberOwnerType       = "secret"
	statusConfigured            = "configured"
	statusRevoked               = "revoked"
	requestOperationCreate      = "create"
	requestOperationGenerate256 = "generate_u256"
	requestOperationUpdate      = "update"
	rotationStatusRotating      = "rotating"
	rotationStatusStable        = "stable"
)

// normalizedSecretInput is an owned plaintext copy and its stable request digests.
type normalizedSecretInput struct {
	scope             Scope
	idempotencyDigest string
	reservationScope  string
	plaintext         []byte
}

// normalizeGeneratedCreateInput binds the fixed generation policy to a
// distinct authenticated request operation without materializing plaintext.
func (s *serviceImpl) normalizeGeneratedCreateInput(
	input CreateGeneratedInput,
) (normalizedSecretInput, error) {
	scope, err := normalizeScope(input.Scope)
	if err != nil || input.Policy != secretcap.GeneratedSecretPolicyURLSafe256 {
		return normalizedSecretInput{}, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest, err := normalizeIdempotencyKey(input.IdempotencyKey)
	if err != nil {
		return normalizedSecretInput{}, bizerr.NewCode(CodeInvalid)
	}
	return normalizedSecretInput{
		scope:             scope,
		idempotencyDigest: idempotencyDigest,
		reservationScope:  digestString(scope.OwnerPluginID + "\x00" + scope.CallerScope),
	}, nil
}

// requestFence is the complete authenticated replay context for one write.
// expectedUpdatedAt is zero only for create.
type requestFence struct {
	operation         string
	idempotencyDigest string
	parentVersion     int
	expectedUpdatedAt time.Time
}

// normalizeConfig applies bounded defaults without allowing negative bypasses.
func normalizeConfig(config Config) (Config, error) {
	if config.MaximumPlaintextSize < 0 || config.MaximumRotationBatch < 0 || config.MaximumKeyWraps < 0 {
		return Config{}, bizerr.NewCode(CodeInvalid)
	}
	if config.MaximumPlaintextSize == 0 {
		config.MaximumPlaintextSize = defaultMaximumPlaintextSize
	}
	if config.MaximumRotationBatch == 0 {
		config.MaximumRotationBatch = defaultMaximumRotationBatch
	}
	if config.MaximumKeyWraps == 0 {
		config.MaximumKeyWraps = defaultMaximumKeyWraps
	}
	if config.MaximumPlaintextSize > maximumPlaintextSize ||
		config.MaximumRotationBatch > maximumRotationBatch ||
		config.MaximumKeyWraps < 2 || config.MaximumKeyWraps > defaultMaximumKeyWraps {
		return Config{}, bizerr.NewCode(CodeInvalid)
	}
	return config, nil
}

// normalizeScope accepts only the single-merchant tenant and stable low-entropy codes.
func normalizeScope(scope Scope) (Scope, error) {
	if scope.TenantID != idccap.DefaultTenantID ||
		!validOwnerPluginID(scope.OwnerPluginID) ||
		!validScopeCode(scope.CallerScope) {
		return Scope{}, bizerr.NewCode(CodeInvalid)
	}
	return scope, nil
}

func validOwnerPluginID(value string) bool {
	if len(value) < 2 || len(value) > maximumOwnerPluginIDBytes || value != strings.TrimSpace(value) {
		return false
	}
	for index, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		if character != '-' || index == 0 || index == len(value)-1 || value[index-1] == '-' {
			return false
		}
	}
	return true
}

func validScopeCode(value string) bool {
	if len(value) < 3 || len(value) > maximumCallerScopeBytes || value != strings.TrimSpace(value) {
		return false
	}
	segments := strings.Split(value, ".")
	if len(segments) < 2 {
		return false
	}
	for _, segment := range segments {
		if !validScopeSegment(segment) {
			return false
		}
	}
	return true
}

func validScopeSegment(value string) bool {
	if value == "" || value[0] < 'a' || value[0] > 'z' || value[len(value)-1] == '-' {
		return false
	}
	previousHyphen := false
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
		previousHyphen = false
	}
	return true
}

// normalizeCreateInput owns a copy so the caller cannot mutate plaintext during encryption.
func (s *serviceImpl) normalizeCreateInput(input CreateInput) (normalizedSecretInput, error) {
	scope, err := normalizeScope(input.Scope)
	if err != nil {
		return normalizedSecretInput{}, err
	}
	idempotencyDigest, err := normalizeIdempotencyKey(input.IdempotencyKey)
	if err != nil || len(input.Plaintext) == 0 || len(input.Plaintext) > s.config.MaximumPlaintextSize {
		return normalizedSecretInput{}, bizerr.NewCode(CodeInvalid)
	}
	return normalizedSecretInput{
		scope:             scope,
		idempotencyDigest: idempotencyDigest,
		reservationScope:  digestString(scope.OwnerPluginID + "\x00" + scope.CallerScope),
		plaintext:         append([]byte(nil), input.Plaintext...),
	}, nil
}

// normalizeUpdateInput validates optimistic state and returns an owned plaintext copy.
func (s *serviceImpl) normalizeUpdateInput(input UpdateInput) (normalizedSecretInput, string, time.Time, error) {
	normalized, err := s.normalizeCreateInput(CreateInput{
		Scope:          input.Scope,
		IdempotencyKey: input.IdempotencyKey,
		Plaintext:      input.Plaintext,
	})
	if err != nil || input.Handle.IsZero() || !validSecretRef(input.Handle.OpaqueRef()) || input.ExpectedUpdatedAt.IsZero() {
		zeroBytes(normalized.plaintext)
		return normalizedSecretInput{}, "", time.Time{}, bizerr.NewCode(CodeInvalid)
	}
	return normalized, input.Handle.OpaqueRef(), databaseTime(input.ExpectedUpdatedAt), nil
}

func validSecretRef(value string) bool {
	if len(value) != len(secretNumberPrefix)+32 || !strings.HasPrefix(value, secretNumberPrefix) {
		return false
	}
	for _, character := range []byte(value[len(secretNumberPrefix):]) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func normalizeIdempotencyKey(value string) (string, error) {
	if value == "" || len(value) > maximumIdempotencyKeyBytes || value != strings.TrimSpace(value) {
		return "", bizerr.NewCode(CodeInvalid)
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return "", bizerr.NewCode(CodeInvalid)
		}
	}
	return digestString(value), nil
}

func validKeyID(value string) bool {
	if value == "" || len(value) > maximumKeyIDBytes || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

// authorize collapses all implementation errors so neither credential-bearing
// provider messages nor secret existence can escape this boundary.
func (s *serviceImpl) authorize(
	ctx context.Context,
	scope Scope,
	action Action,
	handle secretcap.Handle,
	purpose secretcap.Purpose,
) error {
	if ctx == nil {
		return bizerr.NewCode(CodeAccessDenied)
	}
	if err := s.authorizer.Authorize(ctx, AuthorizationRequest{
		Scope: scope, Action: action, Handle: handle, Purpose: purpose,
	}); err != nil {
		return bizerr.NewCode(CodeAccessDenied)
	}
	return nil
}

func digestString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func databaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

// maskedTail exposes at most four trailing printable ASCII bytes and never a
// complete short secret.
func maskedTail(plaintext []byte) string {
	if len(plaintext) <= 4 {
		return "****"
	}
	tail := plaintext[len(plaintext)-4:]
	for _, character := range tail {
		if character < 0x21 || character > 0x7e {
			return "****"
		}
	}
	return "****" + string(tail)
}

func validMaskedTail(value string) bool {
	if value == "****" {
		return true
	}
	if len(value) != 8 || value[:4] != "****" {
		return false
	}
	for _, character := range []byte(value[4:]) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

// zeroBytes performs a best-effort in-place wipe of short-lived sensitive buffers.
func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
