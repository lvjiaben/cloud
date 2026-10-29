package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

var (
	stableCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	variablePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	digestPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

func (s *serviceImpl) authorize(ctx context.Context, target string) (int, error) {
	actor, err := s.management.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionChannelManage, TargetNo: target,
	})
	if err != nil || actor <= 0 {
		return 0, errorOrCode(err, CodeNotAccessible)
	}
	return actor, nil
}

func (s *serviceImpl) requireRecentLogin(ctx context.Context, actor int) error {
	current, err := s.sessions.Current(ctx)
	if err != nil || current == nil || current.LoginAt == nil || current.UserID != strconv.Itoa(actor) {
		return errorOrCode(err, CodeRecentAuthenticationRequired)
	}
	now := s.now().UTC()
	loginAt := current.LoginAt.UTC()
	if loginAt.After(now.Add(15 * time.Second)) {
		return bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	return nil
}

func digestText(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(strconv.Itoa(len(part))))
		_, _ = hash.Write([]byte{':'})
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func stableNumber(prefix string, parts ...string) string {
	digest := digestText(parts...)
	return prefix + digest[:32]
}

func validateIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' && character != '.' && character != ':' {
			return false
		}
	}
	return true
}

func isDigest(value string) bool { return digestPattern.MatchString(value) }

func jsonObject(value map[string]string) (string, error) {
	if value == nil {
		return "{}", nil
	}
	if len(value) > MaximumVariables {
		return "", bizerr.NewCode(CodeInvalid)
	}
	for key, item := range value {
		if !stableCodePattern.MatchString(key) || len(item) > 4096 {
			return "", bizerr.NewCode(CodeInvalid)
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", bizerr.WrapCode(err, CodeInvalid)
	}
	return string(encoded), nil
}

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}

func publicError(err error) error {
	if err == nil {
		return nil
	}
	for _, code := range []*bizerr.Code{
		CodeInvalid, CodeNotAccessible, CodePasswordChanged, CodeRecentAuthenticationRequired, CodeConflict, CodeContactManualVerification,
		CodeChannelUnavailable, CodeHumanVerificationUnavailable, CodeVerificationInvalid,
		CodeVerificationExpired, CodeVerificationRateLimited, CodeVerificationResultUnknown,
	} {
		if bizerr.Is(err, code) {
			return err
		}
	}
	return bizerr.WrapCode(gerror.Wrap(err, "notification operation failed"), CodeUnavailable)
}

func errorOrCode(err error, code *bizerr.Code) error {
	if err == nil {
		return bizerr.NewCode(code)
	}
	return bizerr.WrapCode(err, code)
}
