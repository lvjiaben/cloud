// Privacy governance validation normalizes bounded safe text and fixed business codes. Invalid or sensitive-looking input fails closed before persistence.

package privacygovernance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

var (
	businessNoPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
)

func (s *serviceImpl) authorize(ctx context.Context, permission, target string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	request := managementcap.AuthorizationRequest{Permission: permission, TargetNo: target}
	if managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, request)
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func newBusinessNo(prefix string) (string, error) {
	var content [16]byte
	if _, err := rand.Read(content[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// oneOf validates fixed retention and hold codes.
func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
