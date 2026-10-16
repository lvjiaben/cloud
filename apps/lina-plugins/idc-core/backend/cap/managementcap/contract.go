// Package managementcap defines the narrow core-owned administration runtime supplied to IDC source plugins.
package managementcap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

var targetNumberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// AuthorizationRequest asks core to authorize one exact cataloged IDC permission before target access.
type AuthorizationRequest struct {
	Permission string
	TargetNo   string
}

// Authorizer verifies direct staff audience, single-merchant scope, exact permission, and data scope.
type Authorizer interface {
	Authorize(ctx context.Context, request AuthorizationRequest) (actorUserID int, err error)
}

// SensitiveAccessBinding binds one short-lived proof to an immutable digest of
// the exact private material about to be accessed. Raw target values are never
// carried by the core proof service.
type SensitiveAccessBinding struct {
	TargetDigest string
}

// SensitiveAccessContext is the opaque, short-lived context returned only
// after the current direct staff session has been verified as active.
type SensitiveAccessContext struct {
	Token     string
	ExpiresAt time.Time
}

// SensitiveAccessProof is the trusted, single-use result consumed immediately
// before a plugin releases private material.
type SensitiveAccessProof struct {
	ActorUserID int
	ExpiresAt   time.Time
}

// SensitiveAccessAuthenticator exposes the core-owned active-session and durable
// replay fence without leaking the core implementation into source plugins.
type SensitiveAccessAuthenticator interface {
	IssueSensitiveAccess(
		ctx context.Context,
		binding SensitiveAccessBinding,
	) (*SensitiveAccessContext, error)
	ConsumeSensitiveAccess(
		ctx context.Context,
		token string,
		binding SensitiveAccessBinding,
	) (*SensitiveAccessProof, error)
}

// Runtime is the complete narrow administration seam supplied by core.
type Runtime struct {
	Authorizer      Authorizer
	Audit           idccap.BusinessAuditWriter
	SensitiveAccess SensitiveAccessAuthenticator
	Renewals        StaffRenewalManager
}

// DigestSensitiveAccessTarget creates the canonical opaque target binding used
// by the recent-authentication proof. Callers must provide an immutable,
// domain-separated byte representation.
func DigestSensitiveAccessTarget(canonical []byte) string {
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

// ValidateSensitiveAccessBinding rejects malformed or non-canonical digests.
func ValidateSensitiveAccessBinding(binding SensitiveAccessBinding) error {
	if len(binding.TargetDigest) != sha256.Size*2 {
		return gerror.New("IDC sensitive access binding is invalid")
	}
	for _, character := range []byte(binding.TargetDigest) {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return gerror.New("IDC sensitive access binding is invalid")
		}
	}
	return nil
}

// ValidateAuthorizationRequest rejects uncataloged permissions and malformed opaque targets.
func ValidateAuthorizationRequest(request AuthorizationRequest) error {
	if !idccap.IsCatalogedAdminPermission(request.Permission) ||
		!targetNumberPattern.MatchString(request.TargetNo) {
		return gerror.New("IDC management authorization request is invalid")
	}
	return nil
}

// ValidateRuntime rejects absent or typed-nil administration dependencies.
func ValidateRuntime(runtime Runtime) error {
	if nilInterface(runtime.Authorizer) || nilInterface(runtime.Audit) {
		return gerror.New("IDC management runtime is incomplete")
	}
	return nil
}

func nilInterface(value any) bool {
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
