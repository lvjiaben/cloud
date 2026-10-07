// This file owns credential-shaped JSON field-name classification.

package publicjson

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var forbiddenKeys = map[string]struct{}{
	"accesskey": {}, "accesskeyid": {}, "accesstoken": {}, "apikey": {},
	"apisecret": {}, "appsecret": {}, "authorization": {}, "authtoken": {},
	"clientsecret": {}, "cookie": {}, "credential": {}, "credentialref": {},
	"idtoken": {}, "passwd": {}, "password": {}, "privatekey": {},
	"refreshtoken": {}, "secret": {}, "secretid": {}, "secretkey": {},
	"secretref": {}, "sessionid": {}, "sessiontoken": {}, "signature": {},
	"token": {},
}

var forbiddenTerms = [...]string{
	"password", "passwd", "secret", "token", "privatekey", "credential", "apikey", "accesskey",
	"authorization", "authentication", "auth", "cookie", "session", "signature", "bearer",
}

var forbiddenSuffixes = [...]string{
	"", "value", "ref", "reference", "bytes", "data", "digest", "hash", "hint",
	"fingerprint", "header", "id", "key", "secret", "token",
}

// ForbiddenKey reports whether a decoded field name can carry credentials.
func ForbiddenKey(value string) bool {
	var queue [maximumQueueCandidates]string
	queue[0] = value
	queued := 1
	for index := 0; index < queued; index++ {
		candidate := queue[index]
		if forbiddenPlainKey(candidate) {
			return true
		}
		decoded, complete := decodedCandidates(candidate)
		if !complete {
			return true
		}
		for _, next := range decoded {
			if next == "" || next == candidate || duplicateCandidate(queue[:queued], next) {
				continue
			}
			if queued >= len(queue) {
				return true
			}
			queue[queued] = next
			queued++
		}
	}
	return false
}

func forbiddenPlainKey(value string) bool {
	normalized := normalizeFieldName(value)
	if _, forbidden := forbiddenKeys[normalized]; forbidden {
		return true
	}
	for _, term := range forbiddenTerms {
		for _, suffix := range forbiddenSuffixes {
			if strings.HasSuffix(normalized, term+suffix) {
				return true
			}
		}
	}
	return false
}

func normalizeFieldName(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); {
		character, size := utf8.DecodeRuneInString(value[index:])
		if character == '[' {
			closing := strings.IndexByte(value[index+size:], ']')
			if closing >= 0 {
				content := value[index+size : index+size+closing]
				if isASCIIInteger(content) {
					index += size + closing + 1
					continue
				}
			}
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(unicode.ToLower(character))
		}
		index += size
	}
	return builder.String()
}

func isASCIIInteger(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
