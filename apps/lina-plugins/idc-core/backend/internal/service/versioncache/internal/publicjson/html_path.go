// This file recognizes sensitive HTML metadata and credential-bearing path
// segments while allowing ordinary password-reset route descriptions.

package publicjson

import (
	"strings"
	"unicode"
)

type htmlAttribute struct {
	name  string
	value string
}

func containsSensitiveHTMLMetadata(value string) bool {
	if !strings.Contains(value, "<") || !strings.Contains(value, ">") {
		return false
	}
	for offset := 0; offset < len(value); {
		open := strings.IndexByte(value[offset:], '<')
		if open < 0 {
			return false
		}
		open += offset
		closingIndex := strings.IndexByte(value[open+1:], '>')
		if closingIndex < 0 {
			return false
		}
		closingIndex += open + 1
		tag, attributes, complete := parseHTMLTag(value[open+1 : closingIndex])
		if !complete {
			return true
		}
		var metadataName string
		var content string
		for _, attribute := range attributes {
			if ForbiddenKey(attribute.name) && attribute.value != "" {
				return true
			}
			switch strings.ToLower(attribute.name) {
			case "name", "property", "http-equiv":
				metadataName = attribute.value
			case "content":
				content = attribute.value
			}
		}
		if strings.EqualFold(tag, "meta") && content != "" && ForbiddenKey(metadataName) {
			return true
		}
		offset = closingIndex + 1
	}
	return false
}

func parseHTMLTag(value string) (string, []htmlAttribute, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "!--") || strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "!") || strings.HasPrefix(value, "?") {
		return "", nil, true
	}
	index := 0
	for index < len(value) && !isHTMLSpace(value[index]) {
		index++
	}
	tag := strings.Trim(value[:index], "/")
	var fixed [16]htmlAttribute
	count := 0
	for index < len(value) {
		for index < len(value) && (isHTMLSpace(value[index]) || value[index] == '/') {
			index++
		}
		if index >= len(value) {
			break
		}
		nameStart := index
		for index < len(value) && !isHTMLSpace(value[index]) && value[index] != '=' && value[index] != '/' {
			index++
		}
		name := value[nameStart:index]
		for index < len(value) && isHTMLSpace(value[index]) {
			index++
		}
		attributeValue := ""
		if index < len(value) && value[index] == '=' {
			index++
			for index < len(value) && isHTMLSpace(value[index]) {
				index++
			}
			if index < len(value) && (value[index] == '\'' || value[index] == '"') {
				quote := value[index]
				index++
				valueStart := index
				for index < len(value) && value[index] != quote {
					index++
				}
				if index >= len(value) {
					return "", nil, false
				}
				attributeValue = value[valueStart:index]
				index++
			} else {
				valueStart := index
				for index < len(value) && !isHTMLSpace(value[index]) && value[index] != '/' {
					index++
				}
				attributeValue = value[valueStart:index]
			}
		}
		if count >= len(fixed) {
			return "", nil, false
		}
		fixed[count] = htmlAttribute{name: name, value: attributeValue}
		count++
	}
	return tag, fixed[:count], true
}

func isHTMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f'
}

func containsCredentialPath(value string) bool {
	if !strings.ContainsAny(value, "/\\") {
		return false
	}
	segments := strings.FieldsFunc(value, func(character rune) bool {
		return character == '/' || character == '\\'
	})
	for index, raw := range segments {
		segment := normalizePathSegment(raw)
		if !isCredentialPathName(segment) || index+1 >= len(segments) {
			continue
		}
		next := normalizePathSegment(segments[index+1])
		if isSafeCredentialAction(next) {
			continue
		}
		if next != "" {
			return true
		}
	}
	return false
}

func normalizePathSegment(value string) string {
	if query := strings.IndexAny(value, "?# "); query >= 0 {
		value = value[:query]
	}
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return unicode.ToLower(character)
		}
		return -1
	}, value)
}

func isCredentialPathName(value string) bool {
	switch value {
	case "token", "tokens", "accesstoken", "refreshtoken", "idtoken", "sessiontoken", "authtoken",
		"password", "passwords", "passwd", "secret", "secrets", "cookie", "cookies", "setcookie",
		"auth", "authorization", "authentication", "session", "sessions", "sessionid",
		"credential", "credentials", "privatekey", "accesskey", "apikey", "bearer", "signature":
		return true
	default:
		return false
	}
}

func isSafeCredentialAction(value string) bool {
	switch value {
	case "reset", "forgot", "change", "rotate", "policy", "policies", "status", "supported",
		"login", "logout", "refresh", "verify", "verification":
		return true
	default:
		return false
	}
}
