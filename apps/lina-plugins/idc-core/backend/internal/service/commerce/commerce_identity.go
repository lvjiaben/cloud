package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func (s *serviceImpl) currentCustomerNo(ctx context.Context) (string, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	return audience.CustomerNo, nil
}

func newOpaqueNumber(random io.Reader, prefix string) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate commerce business number")
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

func newGuestToken(random io.Reader) (string, string, error) {
	var content [32]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", "", gerror.Wrap(err, "generate guest cart token")
	}
	raw := "gcart_" + base64.RawURLEncoding.EncodeToString(content[:])
	return raw, digestGuestToken(raw), nil
}

func digestGuestToken(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func normalizeGuestToken(raw string) (string, error) {
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, "gcart_") || len(raw) != 49 {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "gcart_"))
	if err != nil || len(decoded) != 32 {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	return digestGuestToken(raw), nil
}

func normalizeCurrency(currency string) (string, error) {
	if currency != strings.TrimSpace(currency) || !currencyPattern.MatchString(currency) {
		return "", bizerr.NewCode(CodeInvalid)
	}
	return currency, nil
}
