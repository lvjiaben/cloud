package inboxcap

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

var stableValuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func ValidatePublishRequest(input PublishRequest) error {
	if input.RecipientUserID <= 0 {
		return gerror.New("inboxcap: recipient user is invalid")
	}
	for label, value := range map[string]string{
		"source type":     input.SourceType,
		"source number":   input.SourceNo,
		"idempotency key": input.IdempotencyKey,
	} {
		if value != strings.TrimSpace(value) || !stableValuePattern.MatchString(value) {
			return gerror.Newf("inboxcap: %s is invalid", label)
		}
	}
	if input.Title != strings.TrimSpace(input.Title) || input.Title == "" || utf8.RuneCountInString(input.Title) > 200 {
		return gerror.New("inboxcap: title is invalid")
	}
	if input.Body != strings.TrimSpace(input.Body) || input.Body == "" || utf8.RuneCountInString(input.Body) > 4000 {
		return gerror.New("inboxcap: body is invalid")
	}
	if input.Category != strings.TrimSpace(input.Category) || !stableValuePattern.MatchString(input.Category) || utf8.RuneCountInString(input.Category) > 64 {
		return gerror.New("inboxcap: category is invalid")
	}
	if !validActionURL(input.ActionURL) {
		return gerror.New("inboxcap: action URL must be a same-site relative path")
	}
	return nil
}

func validActionURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil &&
		value == strings.TrimSpace(value) &&
		len(value) <= 2048 &&
		strings.HasPrefix(value, "/") &&
		!strings.HasPrefix(value, "//") &&
		!strings.Contains(value, "\\") &&
		strings.IndexFunc(value, unicode.IsControl) < 0 &&
		!parsed.IsAbs() &&
		parsed.Host == "" &&
		parsed.User == nil
}
