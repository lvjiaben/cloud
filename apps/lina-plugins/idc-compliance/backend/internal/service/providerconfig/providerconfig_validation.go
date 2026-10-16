// This file bounds authorization, idempotency, scene identifiers, and credential lifetime.
package providerconfig

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

var (
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	lowerHex64Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (s *serviceImpl) authorize(ctx context.Context, permission string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	request := managementcap.AuthorizationRequest{Permission: permission, TargetNo: configurationNo}
	if managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, request)
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func (s *serviceImpl) ensureRecentLogin(ctx context.Context, actorUserID int) error {
	if s == nil || nilDependency(s.sessions) || s.now == nil || actorUserID <= 0 {
		return bizerr.NewCode(CodeUnavailable)
	}
	session, err := s.sessions.Current(ctx)
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if session == nil || session.ID == "" || session.LoginAt == nil ||
		session.UserID != strconv.Itoa(actorUserID) ||
		session.TenantID != capmodel.DomainID(strconv.Itoa(idccap.DefaultTenantID)) {
		return bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	now := s.now().UTC()
	loginAt := session.LoginAt.UTC()
	if loginAt.After(now.Add(allowedLoginClockSkew)) {
		return bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	return nil
}

func normalizeSave(input SaveInput) (SaveInput, string, error) {
	input.SceneID = strings.TrimSpace(input.SceneID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.ExpectedVersion < 0 || !validSceneID(input.SceneID) ||
		!idempotencyKeyPattern.MatchString(input.IdempotencyKey) {
		return SaveInput{}, "", bizerr.NewCode(CodeInvalid)
	}
	digest := sha256.Sum256([]byte(input.IdempotencyKey))
	return input, hex.EncodeToString(digest[:]), nil
}

func validSceneID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func newMutationOperationKey() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func clearCredentialInput(input *SaveInput) {
	if input == nil {
		return
	}
	for index := range input.AccessKeyID {
		input.AccessKeyID[index] = 0
	}
	for index := range input.AccessKeySecret {
		input.AccessKeySecret[index] = 0
	}
	input.AccessKeyID = nil
	input.AccessKeySecret = nil
}

func clearRecoveryInput(input *RecoveryInput) {
	if input == nil {
		return
	}
	for index := range input.AccessKeyID {
		input.AccessKeyID[index] = 0
	}
	for index := range input.AccessKeySecret {
		input.AccessKeySecret[index] = 0
	}
	input.AccessKeyID = nil
	input.AccessKeySecret = nil
}

func nilDependency(value any) bool {
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
