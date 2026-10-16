package contractautosign

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

var (
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	templateNoPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
	currencyPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
)

const (
	freshLoginWindow      = 5 * time.Minute
	allowedLoginClockSkew = 15 * time.Second
)

func (s *serviceImpl) authorize(ctx context.Context, target string) (int, error) {
	request := managementcap.AuthorizationRequest{Permission: PermissionManage, TargetNo: target}
	if s == nil || ctx == nil || ctx.Err() != nil || managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, request)
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func (s *serviceImpl) ensureRecentLogin(ctx context.Context, actor int) error {
	session, err := s.sessions.Current(ctx)
	if err != nil || session == nil || session.ID == "" || session.LoginAt == nil ||
		session.UserID != strconv.Itoa(actor) ||
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

func normalizeCreate(input CreateInput, now time.Time) (CreateInput, string, string, error) {
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ExpiresAt = input.ExpiresAt.UTC()
	if !input.Confirmed || !currencyPattern.MatchString(input.CurrencyCode) ||
		(!input.AllowRequiredOrder && !input.AllowManualOptional) || input.MaxAmountMinor <= 0 ||
		input.SuccessLimit <= 0 || len(input.Templates) == 0 || len(input.Templates) > MaxTemplates ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) || !input.ExpiresAt.After(now) ||
		input.ExpiresAt.After(now.AddDate(1, 0, 0)) {
		return CreateInput{}, "", "", bizerr.NewCode(CodeInvalid)
	}
	refs := append([]TemplateRef(nil), input.Templates...)
	for index := range refs {
		refs[index].TemplateNo = strings.TrimSpace(refs[index].TemplateNo)
		refs[index].Name = ""
		if !templateNoPattern.MatchString(refs[index].TemplateNo) || refs[index].VersionNo <= 0 {
			return CreateInput{}, "", "", bizerr.NewCode(CodeInvalid)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].TemplateNo == refs[j].TemplateNo {
			return refs[i].VersionNo < refs[j].VersionNo
		}
		return refs[i].TemplateNo < refs[j].TemplateNo
	})
	for index := 1; index < len(refs); index++ {
		if refs[index-1].TemplateNo == refs[index].TemplateNo && refs[index-1].VersionNo == refs[index].VersionNo {
			return CreateInput{}, "", "", bizerr.NewCode(CodeInvalid)
		}
	}
	input.Templates = refs
	digest := sha256Hex("create\x00" + input.IdempotencyKey)
	parts := []string{input.CurrencyCode, strconv.FormatBool(input.AllowRequiredOrder), strconv.FormatBool(input.AllowManualOptional),
		strconv.FormatInt(input.MaxAmountMinor, 10), strconv.FormatInt(input.SuccessLimit, 10), input.ExpiresAt.Format(time.RFC3339Nano)}
	for _, ref := range refs {
		parts = append(parts, ref.TemplateNo, strconv.FormatInt(ref.VersionNo, 10))
	}
	return input, digest, sha256Hex(strings.Join(parts, "\x00")), nil
}

func normalizeRevoke(input RevokeInput) (RevokeInput, string, string, error) {
	input.AuthorizationNo = strings.TrimSpace(input.AuthorizationNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.Confirmed || !templateNoPattern.MatchString(input.AuthorizationNo) || input.ExpectedVersion <= 0 ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return RevokeInput{}, "", "", bizerr.NewCode(CodeInvalid)
	}
	digest := sha256Hex("revoke\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex(fmt.Sprintf("%s\x00%d", input.AuthorizationNo, input.ExpectedVersion))
	return input, digest, fingerprint, nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func newAuthorizationNo() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "casa_" + hex.EncodeToString(buffer), nil
}
