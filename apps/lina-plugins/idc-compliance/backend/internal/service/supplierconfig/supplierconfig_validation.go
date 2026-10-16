// This file normalizes and validates supplier configuration commands and state values.

package supplierconfig

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

var (
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	bindingNoPattern      = regexp.MustCompile(`^lsb_[0-9a-f]{32}$`)
	operationNoPattern    = regexp.MustCompile(`^lsi_[0-9a-f]{32}$`)
	lowerHex64Pattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func (s *serviceImpl) authorize(ctx context.Context, permission, provider string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !validProvider(provider) {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	request := managementcap.AuthorizationRequest{
		Permission: permission, TargetNo: configurationNo(provider),
	}
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

func normalizeCredentialSave(input CredentialSaveInput) (CredentialSaveInput, string, error) {
	input.ProviderCode = strings.TrimSpace(input.ProviderCode)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validProvider(input.ProviderCode) || input.ExpectedVersion < 0 ||
		!idempotencyKeyPattern.MatchString(input.IdempotencyKey) ||
		len(input.CredentialEnvelope) < 16 || len(input.CredentialEnvelope) > 16<<10 {
		return CredentialSaveInput{}, "", bizerr.NewCode(CodeInvalid)
	}
	// Credential bytes are intentionally excluded from the persisted digest.
	// A completed replay returns the original generation and never re-consumes a
	// changed envelope under the same operator idempotency key.
	digest := sha256.Sum256([]byte(
		input.ProviderCode + "\x00" + strconv.FormatInt(input.ExpectedVersion, 10) +
			"\x00" + input.IdempotencyKey,
	))
	return input, hex.EncodeToString(digest[:]), nil
}

func newMutationOperationKey() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	defer clearBytes(value)
	return hex.EncodeToString(value), nil
}

func credentialProducer(value []byte) (secretcap.PlaintextProducer, func()) {
	owned := append([]byte(nil), value...)
	cleanup := func() { clearBytes(owned) }
	return func(consume secretcap.Consumer) error {
		defer clearBytes(owned)
		if consume == nil || len(owned) < 16 || len(owned) > 16<<10 {
			return bizerr.NewCode(CodeInvalid)
		}
		return consume(owned)
	}, cleanup
}

func clearCredentialSaveInput(input *CredentialSaveInput) {
	if input == nil {
		return
	}
	clearBytes(input.CredentialEnvelope)
	input.CredentialEnvelope = nil
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
