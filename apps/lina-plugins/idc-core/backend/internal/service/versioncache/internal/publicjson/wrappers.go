// This file decodes finite percent, HTML-entity, base64, and hexadecimal
// wrappers into a fixed-size candidate set.

package publicjson

import (
	"encoding/base64"
	"encoding/hex"
	"html"
	"net/url"
	"strings"
	"unicode/utf8"
)

func decodedCandidates(value string) ([]string, bool) {
	var values [maximumQueueCandidates]string
	count := 0
	appendValue := func(candidate string) bool {
		if candidate == "" || candidate == value {
			return true
		}
		for index := 0; index < count; index++ {
			if values[index] == candidate {
				return true
			}
		}
		if count >= len(values) {
			return false
		}
		values[count] = candidate
		count++
		return true
	}

	entityDecoded := html.UnescapeString(value)
	if !appendValue(entityDecoded) {
		return nil, false
	}
	percentDecoded, attempted, stable := decodePercentAndPlus(value)
	if !stable {
		return nil, false
	}
	if attempted && !appendValue(percentDecoded) {
		return nil, false
	}
	tokens, tokensComplete := encodedTokens(value)
	if !tokensComplete {
		return nil, false
	}
	for _, candidate := range tokens {
		if decoded, ok := decodeBase64(candidate); ok && !appendValue(decoded) {
			return nil, false
		}
		if decoded, ok := decodeHex(candidate); ok && !appendValue(decoded) {
			return nil, false
		}
	}
	return values[:count], true
}

func decodePercentAndPlus(value string) (decoded string, attempted bool, stable bool) {
	hasEscape, malformed := percentEscapeState(value)
	if malformed && hasEscape {
		return "", true, false
	}
	if malformed && !hasEscape {
		return value, false, true
	}
	attempted = hasEscape || strings.Contains(value, "+")
	if !attempted {
		return value, false, true
	}
	decoded, err := url.QueryUnescape(value)
	if err != nil || len(decoded) > len(value) {
		return "", true, false
	}
	return decoded, true, true
}

func percentEscapeState(value string) (hasEscape bool, malformed bool) {
	for index := 0; index < len(value); index++ {
		if value[index] != '%' {
			continue
		}
		if index+2 >= len(value) || !isHexDigit(value[index+1]) || !isHexDigit(value[index+2]) {
			malformed = true
			continue
		}
		hasEscape = true
		index += 2
	}
	return hasEscape, malformed
}

func encodedTokens(value string) ([]string, bool) {
	var tokens [maximumQueueCandidates]string
	count := 0
	for index := 0; index < len(value); {
		for index < len(value) && !isEncodedByte(value[index]) {
			index++
		}
		start := index
		for index < len(value) && isEncodedByte(value[index]) {
			index++
		}
		candidate := strings.Trim(value[start:index], "-_.")
		if len(candidate) < 16 || !looksEncodedCredential(candidate) {
			continue
		}
		if count >= len(tokens) {
			return nil, false
		}
		tokens[count] = candidate
		count++
	}
	return tokens[:count], true
}

func looksEncodedCredential(value string) bool {
	if len(value) < 16 {
		return false
	}
	if len(value)%2 == 0 {
		hexOnly := true
		for index := 0; index < len(value); index++ {
			if !isHexDigit(value[index]) {
				hexOnly = false
				break
			}
		}
		if hexOnly {
			return true
		}
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '+' || character == '/' ||
			character == '-' || character == '_' || character == '=' {
			continue
		}
		return false
	}
	return true
}

func decodeBase64(value string) (string, bool) {
	if len(value) < 16 || len(value) > maximumDecodedBudget*2 {
		return "", false
	}
	encodings := [...]*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.Strict().DecodeString(value)
		if err == nil && len(decoded) > 0 && len(decoded) <= maximumDecodedBudget &&
			utf8.Valid(decoded) && string(decoded) != value {
			return string(decoded), true
		}
	}
	return "", false
}

func decodeHex(value string) (string, bool) {
	if len(value) < 16 || len(value)%2 != 0 || len(value) > maximumDecodedBudget*2 {
		return "", false
	}
	for index := 0; index < len(value); index++ {
		if !isHexDigit(value[index]) {
			return "", false
		}
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) == 0 || !utf8.Valid(decoded) || string(decoded) == value {
		return "", false
	}
	return string(decoded), true
}

func isEncodedByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || value == '+' || value == '/' ||
		value == '-' || value == '_' || value == '=' || value == '.'
}

func isHexDigit(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}
