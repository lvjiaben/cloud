// This file validates the single RFC 3986 request-target representation that
// both local processes must sign and verify without URL alias normalization.

package processauth

import (
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

// queryParameter contains one already canonical query name and value.
type queryParameter struct {
	name  string
	value string
}

// canonicalRequestTarget accepts only the protocol's one wire representation.
func canonicalRequestTarget(requestTarget string) (string, error) {
	if len(requestTarget) == 0 || len(requestTarget) > MaximumRequestTargetBytes ||
		strings.ContainsAny(requestTarget, "\r\n\x00#") {
		return "", ErrInvalid
	}
	path, rawQuery, hasQuery := strings.Cut(requestTarget, "?")
	if !validCanonicalPath(path) || hasQuery && rawQuery == "" ||
		strings.Contains(rawQuery, "?") || len(rawQuery) > MaximumQueryBytes {
		return "", ErrInvalid
	}
	if !hasQuery {
		return path, nil
	}
	canonicalQuery, err := canonicalQuery(rawQuery)
	if err != nil || canonicalQuery != rawQuery {
		return "", ErrInvalid
	}
	return path + "?" + canonicalQuery, nil
}

// validCanonicalPath rejects routing aliases before HTTP authorization.
func validCanonicalPath(path string) bool {
	if !strings.HasPrefix(path, "/internal/v1/") || strings.Contains(path, "%") ||
		strings.Contains(path, "//") || !utf8.ValidString(path) {
		return false
	}
	for _, value := range []byte(path) {
		if value > 0x7f || !isPathByte(value) {
			return false
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// isPathByte reports whether one ASCII byte is allowed in the canonical path.
func isPathByte(value byte) bool {
	return isUnreserved(value) || strings.ContainsRune("/!$&'()*+,;=:@", rune(value))
}

// canonicalQuery validates and rebuilds a bounded, uniquely named query.
func canonicalQuery(rawQuery string) (string, error) {
	parts := strings.Split(rawQuery, "&")
	if len(parts) == 0 || len(parts) > MaximumQueryKeys {
		return "", ErrInvalid
	}
	parameters := make([]queryParameter, 0, len(parts))
	names := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		namePart, valuePart, found := strings.Cut(part, "=")
		if !found || namePart == "" || strings.Contains(valuePart, "=") {
			return "", ErrInvalid
		}
		name, err := url.PathUnescape(namePart)
		if err != nil || name == "" || !utf8.ValidString(name) {
			return "", ErrInvalid
		}
		value, err := url.PathUnescape(valuePart)
		if err != nil || !utf8.ValidString(value) {
			return "", ErrInvalid
		}
		canonicalName := percentEncode(name)
		canonicalValue := percentEncode(value)
		if canonicalName != namePart || canonicalValue != valuePart {
			return "", ErrInvalid
		}
		if _, exists := names[canonicalName]; exists {
			return "", ErrInvalid
		}
		names[canonicalName] = struct{}{}
		parameters = append(parameters, queryParameter{name: canonicalName, value: canonicalValue})
	}
	sort.Slice(parameters, func(left, right int) bool {
		return parameters[left].name < parameters[right].name
	})
	var builder strings.Builder
	builder.Grow(len(rawQuery))
	for index, parameter := range parameters {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(parameter.name)
		builder.WriteByte('=')
		builder.WriteString(parameter.value)
	}
	return builder.String(), nil
}

// percentEncode emits the canonical uppercase RFC 3986 representation.
func percentEncode(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		current := value[index]
		if isUnreserved(current) {
			builder.WriteByte(current)
			continue
		}
		const hexadecimal = "0123456789ABCDEF"
		builder.WriteByte('%')
		builder.WriteByte(hexadecimal[current>>4])
		builder.WriteByte(hexadecimal[current&0x0f])
	}
	return builder.String()
}

// isUnreserved reports whether a byte may remain unescaped in RFC 3986.
func isUnreserved(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || strings.ContainsRune("-._~", rune(value))
}
