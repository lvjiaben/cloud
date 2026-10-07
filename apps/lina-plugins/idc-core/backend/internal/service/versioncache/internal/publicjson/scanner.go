// This file performs allocation-bounded semantic credential detection.

package publicjson

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maximumQueueCandidates = 24
	maximumFieldScanBytes  = 256
	maximumDecodedBudget   = 4 * 1024 * 1024
	maximumCandidateBudget = 4096
)

type credentialScanner struct {
	remainingBytes      int
	remainingCandidates int
}

func newScanner(maximumBytes int, maximumNodes int) *credentialScanner {
	decodedBudget := maximumBytes * 4
	if decodedBudget > maximumDecodedBudget {
		decodedBudget = maximumDecodedBudget
	}
	candidateBudget := maximumNodes
	if candidateBudget > maximumCandidateBudget {
		candidateBudget = maximumCandidateBudget
	}
	return &credentialScanner{
		remainingBytes: decodedBudget, remainingCandidates: candidateBudget,
	}
}

// ContainsCredentialText exposes the bounded value scanner for key and test
// callers that do not need full JSON canonicalization.
func ContainsCredentialText(value string) bool {
	maximumBytes := len(value)
	if maximumBytes < 64 {
		maximumBytes = 64
	}
	return newScanner(maximumBytes, 64).contains(value)
}

func (s *credentialScanner) contains(value string) bool {
	var queue [maximumQueueCandidates]string
	queue[0] = value
	queued := 1
	for index := 0; index < queued; index++ {
		current := queue[index]
		if !s.consumeBytes(len(current)) || semanticCredential(current) {
			return true
		}
		candidates, complete := decodedCandidates(current)
		if !complete {
			return true
		}
		for _, candidate := range candidates {
			if candidate == "" || candidate == current || duplicateCandidate(queue[:queued], candidate) {
				continue
			}
			if queued >= len(queue) || !s.consumeCandidate() {
				return true
			}
			queue[queued] = candidate
			queued++
		}
	}
	return false
}

func (s *credentialScanner) consumeBytes(size int) bool {
	if size < 0 || size > s.remainingBytes {
		return false
	}
	s.remainingBytes -= size
	return true
}

func (s *credentialScanner) consumeCandidate() bool {
	if s.remainingCandidates <= 0 {
		return false
	}
	s.remainingCandidates--
	return true
}

func duplicateCandidate(existing []string, candidate string) bool {
	for _, value := range existing {
		if value == candidate {
			return true
		}
	}
	return false
}

func semanticCredential(value string) bool {
	if value == "" {
		return false
	}
	if containsCredentialAssignment(value) || containsURIUserInfo(value) ||
		containsCredentialScheme(value) || containsCredentialPath(value) ||
		containsSensitiveHTMLMetadata(value) {
		return true
	}
	compact, spaced, changed := normalizeFormatRunes(value)
	if !changed {
		return false
	}
	return containsCredentialAssignment(compact) || containsCredentialAssignment(spaced) ||
		containsURIUserInfo(compact) || containsURIUserInfo(spaced) ||
		containsCredentialScheme(compact) || containsCredentialScheme(spaced) ||
		containsCredentialPath(compact) || containsCredentialPath(spaced) ||
		containsSensitiveHTMLMetadata(compact) || containsSensitiveHTMLMetadata(spaced)
}

func containsCredentialAssignment(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] != ':' && value[index] != '=' {
			continue
		}
		if ForbiddenKey(fieldNameBeforeSeparator(value, index)) {
			return true
		}
	}
	return false
}

func fieldNameBeforeSeparator(value string, separator int) string {
	start := separator - maximumFieldScanBytes
	if start < 0 {
		start = 0
	}
	for index := separator - 1; index >= start; index-- {
		switch value[index] {
		case '&', ';', '?', '\r', '\n', '<', '>':
			start = index + 1
			index = -1
		}
	}
	return strings.TrimSpace(value[start:separator])
}

func containsURIUserInfo(value string) bool {
	searchFrom := 0
	for {
		relative := strings.Index(value[searchFrom:], "://")
		if relative < 0 {
			return false
		}
		marker := searchFrom + relative
		start := marker - 1
		for start >= 0 && isSchemeByte(value[start]) {
			start--
		}
		start++
		end := marker + 3
		for end < len(value) {
			character, size := utf8.DecodeRuneInString(value[end:])
			if unicode.IsSpace(character) || strings.ContainsRune("\"'`<>(){}[]", character) {
				break
			}
			end += size
		}
		candidate := strings.TrimRight(value[start:end], ",.;")
		parsed, err := url.Parse(candidate)
		if err == nil && parsed.IsAbs() && parsed.User != nil {
			username := parsed.User.Username()
			_, hasPassword := parsed.User.Password()
			if username != "" || hasPassword {
				return true
			}
		}
		searchFrom = marker + 3
	}
}

func isSchemeByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || value == '+' || value == '-' || value == '.'
}

func containsCredentialScheme(value string) bool {
	lowered := strings.ToLower(value)
	for _, marker := range []string{"-----begin private key", "-----begin rsa private key", "-----begin ec private key"} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	if schemePayload(lowered, "bearer", false) {
		return true
	}
	return schemePayload(lowered, "basic", true)
}

func schemePayload(value string, scheme string, basic bool) bool {
	searchFrom := 0
	for {
		relative := strings.Index(value[searchFrom:], scheme)
		if relative < 0 {
			return false
		}
		start := searchFrom + relative
		end := start + len(scheme)
		if (start == 0 || !isASCIIWord(value[start-1])) && end < len(value) &&
			isSecuritySeparator(value[end]) {
			payload := strings.TrimLeftFunc(value[end:], isSecuritySeparatorRune)
			payload = credentialToken(payload)
			if payload != "" && (!basic || looksBasicAuthorization(payload)) {
				return true
			}
		}
		searchFrom = end
	}
}

func looksBasicAuthorization(value string) bool {
	if len(value) < 4 || len(value) > maximumFieldScanBytes*4 {
		return false
	}
	for _, encoding := range [...]*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		decoded, err := encoding.Strict().DecodeString(value)
		if err == nil && bytes.ContainsRune(decoded, ':') {
			return true
		}
	}
	return false
}

func credentialToken(value string) string {
	for index, character := range value {
		if unicode.IsSpace(character) || strings.ContainsRune("\"'`<>(){}[],;", character) {
			return value[:index]
		}
	}
	return value
}

func isASCIIWord(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || value == '_'
}

func isSecuritySeparator(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == ':'
}

func isSecuritySeparatorRune(value rune) bool {
	return unicode.IsSpace(value) || value == ':' || unicode.In(value, unicode.Cf)
}

func normalizeFormatRunes(value string) (compact string, spaced string, changed bool) {
	for _, character := range value {
		if unicode.In(character, unicode.Cf) || (unicode.IsSpace(character) && character != ' ') {
			changed = true
			break
		}
	}
	if !changed {
		return value, value, false
	}
	var compactBuilder strings.Builder
	var spacedBuilder strings.Builder
	compactBuilder.Grow(len(value))
	spacedBuilder.Grow(len(value))
	for _, character := range value {
		switch {
		case unicode.In(character, unicode.Cf):
			spacedBuilder.WriteByte(' ')
		case unicode.IsSpace(character):
			compactBuilder.WriteByte(' ')
			spacedBuilder.WriteByte(' ')
		default:
			compactBuilder.WriteRune(character)
			spacedBuilder.WriteRune(character)
		}
	}
	return compactBuilder.String(), spacedBuilder.String(), true
}
