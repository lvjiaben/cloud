// Package credentialscan implements bounded credential detection for
// resourcecap payloads without exposing normalization internals to consumers.
package credentialscan

import (
	"encoding/base64"
	"encoding/hex"
	"html"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maximumCredentialDecodePasses caps defensive normalization work.
	maximumCredentialDecodePasses = 64
	// maximumCredentialNameScanBytes bounds reverse inspection before a separator.
	maximumCredentialNameScanBytes = 256
	// maximumCredentialMaterialBytes bounds one caller-controlled value.
	maximumCredentialMaterialBytes = 1 << 20
	// maximumEncodedDecodeRounds bounds nested Base64 and hexadecimal wrappers.
	maximumEncodedDecodeRounds = 4
	// maximumEncodedCandidateCount bounds candidate decoding across one value.
	maximumEncodedCandidateCount = 64
	// maximumDecodedCandidateBytes bounds aggregate decoded candidate material.
	maximumDecodedCandidateBytes = 2 << 20
	// minimumEncodedCandidateBytes avoids decoding ordinary short words.
	minimumEncodedCandidateBytes = 8
)

// Contains reports whether value contains credential-like material or cannot
// be inspected safely within the scanner's fixed budgets.
func Contains(value string) bool {
	return containsCredentialMaterial(value)
}

// SensitiveDiagnosticKey reports whether a log or metric key is sensitive.
func SensitiveDiagnosticKey(key string) bool {
	return sensitiveDiagnosticKey(key)
}

// sensitiveJSONKey reports whether a decoded JSON key can carry credential
// material. Only an explicit set of non-secret reference names bypasses the
// reference-name branch; credential words always take precedence.
func sensitiveJSONKey(key string) bool {
	decoded, stable := normalizeCredentialMaterial(key)
	if !stable {
		return true
	}
	// "defaults" is a common structural object in connector-owned selectors.
	// Its values are still scanned recursively; only the harmless container
	// name skips speculative Base64 decoding of this ordinary word.
	if normalizeKey(decoded) == "defaults" {
		return false
	}
	return containsCredentialCandidateGraph(decoded, sensitiveJSONKeyDirect)
}

// sensitiveJSONKeyDirect classifies one decoded JSON key without recursively
// traversing additional encoded wrappers.
func sensitiveJSONKeyDirect(decoded string) bool {
	decoded = strings.ToLower(decoded)
	normalized, complete := normalizeCredentialName(decoded)
	if !complete {
		return true
	}
	if normalized == "" {
		return false
	}
	if containsSensitiveKeyMarker(normalized) {
		return true
	}
	if isJSONReferenceName(decoded) {
		return !isSafeJSONReferenceKey(normalized)
	}
	return false
}

// containsSensitiveKeyMarker detects credential-bearing key-name components.
func containsSensitiveKeyMarker(normalized string) bool {
	for _, marker := range []string{
		"password", "passwd", "secret", "token", "cookie", "auth", "session",
		"credential", "privatekey", "accesskey", "apikey", "bearer", "signature",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// isJSONReferenceName recognizes explicit ref/reference machine naming forms
// without treating ordinary words such as "preference" as references.
func isJSONReferenceName(key string) bool {
	lower := strings.ToLower(key)
	if lower == "ref" || lower == "reference" {
		return true
	}
	if strings.HasSuffix(lower, "ref") {
		return true
	}
	for _, suffix := range []string{"_ref", "-ref", ".ref", "_reference", "-reference", ".reference"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return strings.HasSuffix(key, "Ref") || strings.HasSuffix(key, "Reference")
}

// isSafeJSONReferenceKey lists the non-secret reference names currently
// published by the normalized resource contract. New names require review.
func isSafeJSONReferenceKey(normalized string) bool {
	switch normalized {
	case "channelref",
		"providerref",
		"resourceref",
		"externalresourceref",
		"taskref",
		"externaltaskref",
		"requestref",
		"operationref",
		"regionref",
		"lineref",
		"planref",
		"imageref",
		"stockref",
		"costref",
		"balanceref",
		"expiryref",
		"poolref",
		"productref",
		"templateref",
		"diskref",
		"ipref",
		"networkref",
		"publickeyref",
		"sshkeyref",
		"snapshotref",
		"backupref",
		"securitygroupref",
		"natref",
		"ticketref",
		"parentref",
		"sourceref",
		"targetref":
		return true
	default:
		return false
	}
}

// sensitiveDiagnosticKey detects unsafe log or metric label keys.
func sensitiveDiagnosticKey(key string) bool {
	decoded, stable := normalizeCredentialMaterial(key)
	if !stable {
		return true
	}
	return containsCredentialCandidateGraph(decoded, func(value string) bool {
		return containsSensitiveKeyMarker(normalizeKey(value))
	})
}

// normalizeKey reduces a machine key to lowercase ASCII alphanumerics for
// stable security-marker comparison.
func normalizeKey(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

// containsCredentialMaterial detects inline credential encodings in
// diagnostics, references, patterns, URLs, paths, and JSON string values.
// Interleaved percent and HTML representations are normalized under a fixed
// pass ceiling; malformed, oversized, or non-stable input fails closed.
func containsCredentialMaterial(value string) bool {
	decoded, stable := normalizeCredentialMaterial(value)
	if !stable {
		return true
	}
	return containsCredentialCandidates(decoded)
}

// credentialCandidate is one normalized value plus its wrapper depth.
type credentialCandidate struct {
	value string
	round int
}

// containsCredentialCandidates scans direct text and a bounded breadth-first
// set of encoded candidates. Every byte is tokenized once per bounded round;
// padding runs are consumed as a unit so repeated '=' input remains linear.
func containsCredentialCandidates(value string) bool {
	return containsCredentialCandidateGraph(value, containsDirectCredential)
}

// containsCredentialCandidateGraph applies one direct inspector to the raw and
// Cf-stripped view of every candidate, then traverses a bounded mixed
// Base64/hex graph. Invalid UTF-8 decoded bytes are inspected through both
// replacement and removal views rather than being silently skipped.
func containsCredentialCandidateGraph(value string, inspect func(string) bool) bool {
	var (
		queue        = []credentialCandidate{{value: value}}
		seen         = map[string]struct{}{value: {}}
		candidateCnt int
		decodedBytes int
	)
	for len(queue) > 0 {
		candidate := queue[0]
		queue = queue[1:]
		views := []string{candidate.value}
		if withoutFormat, changed := removeCredentialFormatCharacters(candidate.value); changed {
			views = append(views, withoutFormat)
		}
		for _, view := range views {
			if inspect(view) {
				return true
			}
			encoded, exhausted := decodedCredentialCandidates(view)
			if exhausted {
				return true
			}
			if candidate.round >= maximumEncodedDecodeRounds {
				if len(encoded) > 0 {
					return true
				}
				continue
			}
			for _, decoded := range encoded {
				candidateCnt++
				decodedBytes += len(decoded)
				if candidateCnt > maximumEncodedCandidateCount || decodedBytes > maximumDecodedCandidateBytes {
					return true
				}
				normalizedViews, stable := normalizeDecodedCredentialViews(decoded)
				if !stable {
					return true
				}
				for _, normalized := range normalizedViews {
					if _, exists := seen[normalized]; exists {
						continue
					}
					seen[normalized] = struct{}{}
					queue = append(queue, credentialCandidate{
						value: normalized, round: candidate.round + 1,
					})
				}
			}
		}
	}
	return false
}

// containsDirectCredential checks one normalized decoded text view.
func containsDirectCredential(value string) bool {
	return containsDecodedCredentialMaterial(strings.ToLower(value))
}

// normalizeDecodedCredentialViews returns bounded normalized interpretations
// of one decoded byte slice. Invalid UTF-8 bytes are replaced in one view and
// removed in another so they cannot hide adjacent credential syntax.
func normalizeDecodedCredentialViews(decoded []byte) ([]string, bool) {
	raw := string(decoded)
	values := []string{raw}
	if !utf8.Valid(decoded) {
		values = []string{
			strings.ToValidUTF8(raw, "\uFFFD"),
			strings.ToValidUTF8(raw, ""),
		}
	}
	views := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized, stable := normalizeCredentialMaterial(value)
		if !stable {
			return nil, false
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		views = append(views, normalized)
	}
	return views, true
}

// decodedCredentialCandidates extracts Base64/URL-safe Base64 and pure hex
// tokens in one pass. It reports exhausted only when a fixed candidate budget
// is exceeded; malformed tokens are ignored as ordinary diagnostics.
func decodedCredentialCandidates(value string) ([][]byte, bool) {
	decoded := make([][]byte, 0, 4)
	candidateCount := 0
	for index := 0; index < len(value); {
		if !isEncodedCoreByte(value[index]) {
			index++
			continue
		}
		start := index
		for index < len(value) && isEncodedCoreByte(value[index]) {
			index++
		}
		coreEnd := index
		for index < len(value) && value[index] == '=' {
			index++
		}
		if coreEnd-start < minimumEncodedCandidateBytes {
			continue
		}
		candidateCount++
		if candidateCount > maximumEncodedCandidateCount {
			return nil, true
		}
		token := value[start:index]
		if base64Value, ok := decodeBase64Candidate(token); ok {
			decoded = append(decoded, base64Value)
		}
		core := value[start:coreEnd]
		if hexValue, ok := decodeHexCandidate(core); ok {
			decoded = append(decoded, hexValue)
		}
	}
	return decoded, false
}

// decodeBase64Candidate decodes one strict standard or URL-safe token with or
// without padding. Excess padding is discarded only for a strict raw decode,
// preventing it from splitting into quadratically many retry candidates.
func decodeBase64Candidate(value string) ([]byte, bool) {
	core := strings.TrimRight(value, "=")
	padding := len(value) - len(core)
	if len(core) < minimumEncodedCandidateBytes || len(core)%4 == 1 {
		return nil, false
	}
	encoding := base64.RawStdEncoding.Strict()
	if strings.ContainsAny(core, "-_") {
		encoding = base64.RawURLEncoding.Strict()
	}
	if padding > 0 && padding <= 2 {
		encoding = base64.StdEncoding.Strict()
		if strings.ContainsAny(core, "-_") {
			encoding = base64.URLEncoding.Strict()
		}
		core = value
	}
	decoded, err := encoding.DecodeString(core)
	return decoded, err == nil && len(decoded) <= maximumCredentialMaterialBytes
}

// decodeHexCandidate decodes one even-length pure hexadecimal token.
func decodeHexCandidate(value string) ([]byte, bool) {
	if len(value) < minimumEncodedCandidateBytes || len(value)%2 != 0 {
		return nil, false
	}
	for index := 0; index < len(value); index++ {
		if !isHexDigit(value[index]) {
			return nil, false
		}
	}
	decoded, err := hex.DecodeString(value)
	return decoded, err == nil && len(decoded) <= maximumCredentialMaterialBytes
}

// isEncodedCoreByte reports whether value belongs to either supported Base64
// alphabet. Padding is handled separately as one trailing run.
func isEncodedCoreByte(value byte) bool {
	return isASCIIAlpha(value) || value >= '0' && value <= '9' ||
		value == '+' || value == '/' || value == '-' || value == '_'
}

// removeCredentialFormatCharacters provides a second bounded interpretation
// for format runes inserted inside a credential word. The primary scan keeps
// those runes so the same characters can also act as scheme separators.
func removeCredentialFormatCharacters(value string) (string, bool) {
	var (
		builder strings.Builder
		changed bool
	)
	builder.Grow(len(value))
	for _, character := range value {
		if unicode.Is(unicode.Cf, character) {
			changed = true
			continue
		}
		builder.WriteRune(character)
	}
	if !changed {
		return value, false
	}
	return builder.String(), true
}

// normalizeCredentialMaterial resolves interleaved percent encodings, HTML
// entities, markup, Unicode whitespace, and format characters under one
// length-derived pass budget. Any oversized or non-stable input fails closed.
func normalizeCredentialMaterial(value string) (string, bool) {
	if len(value) > maximumCredentialMaterialBytes {
		return "", false
	}
	var (
		current        = value
		passLimit      = credentialDecodePassLimit(len(value))
		percentDecoded string
		entityDecoded  string
		markupStripped string
		percentChanged bool
		markupChanged  bool
		complete       bool
	)
	for pass := 0; pass < passLimit; pass++ {
		percentDecoded, percentChanged = decodePercentLayer(current)
		if !utf8.ValidString(percentDecoded) {
			return "", false
		}
		entityDecoded = html.UnescapeString(percentDecoded)
		markupStripped, markupChanged, complete = stripCredentialHTMLMarkup(entityDecoded)
		if !complete {
			return "", false
		}
		entityChanged := entityDecoded != percentDecoded
		if !percentChanged && !entityChanged && !markupChanged {
			return canonicalizeCredentialText(markupStripped), true
		}
		current = markupStripped
	}
	return "", false
}

// canonicalizeCredentialText maps every Unicode whitespace rune to ASCII space
// while preserving case for case-sensitive Base64 decoding. Direct credential
// scans lowercase their own view after encoded candidates have been extracted.
func canonicalizeCredentialText(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, character := range value {
		switch {
		case unicode.IsSpace(character):
			builder.WriteByte(' ')
		default:
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

// stripCredentialHTMLMarkup removes complete HTML delimiters so markup cannot
// split a credential word. Comment bodies and tag attributes remain scannable,
// while sensitive tag names become assignments so their bodies fail closed.
func stripCredentialHTMLMarkup(value string) (string, bool, bool) {
	var (
		builder   strings.Builder
		metadata  strings.Builder
		component string
		end       int
		changed   bool
		markup    bool
		complete  bool
	)
	builder.Grow(len(value))
	metadata.Grow(len(value) / 4)
	for index := 0; index < len(value); {
		if value[index] != '<' {
			builder.WriteByte(value[index])
			index++
			continue
		}
		end, component, markup, complete = credentialHTMLMarkupAt(value, index)
		if !complete {
			return "", false, false
		}
		if !markup {
			builder.WriteByte(value[index])
			index++
			continue
		}
		changed = true
		if component != "" {
			metadata.WriteByte(' ')
			metadata.WriteString(component)
		}
		index = end
	}
	if !changed {
		return value, false, true
	}
	builder.WriteString(metadata.String())
	return builder.String(), true, true
}

// credentialHTMLMarkupAt parses one bounded HTML comment or tag beginning at
// start. It never accepts an unterminated recognized construct.
func credentialHTMLMarkupAt(value string, start int) (int, string, bool, bool) {
	if strings.HasPrefix(value[start:], "<!--") {
		endOffset := strings.Index(value[start+4:], "-->")
		if endOffset < 0 {
			return 0, "", true, false
		}
		end := start + 4 + endOffset
		return end + 3, value[start+4 : end], true, true
	}

	var (
		cursor     = start + 1
		nameStart  int
		end        int
		name       string
		preserved  string
		normalized string
		complete   bool
	)
	if cursor >= len(value) {
		return 0, "", false, true
	}
	if value[cursor] == '/' {
		cursor++
	}
	if cursor >= len(value) || !isASCIIAlpha(value[cursor]) {
		return 0, "", false, true
	}
	nameStart = cursor
	for cursor < len(value) && isHTMLTagNameByte(value[cursor]) {
		cursor++
	}
	name = value[nameStart:cursor]
	end, complete = credentialHTMLTagEnd(value, cursor)
	if !complete {
		return 0, "", true, false
	}
	// Preserve attributes on both opening and closing forms. Closing-tag
	// attributes are invalid HTML but remain attacker-controlled input and must
	// not disappear from the security interpretation.
	preserved = value[cursor : end-1]
	normalized = normalizeKey(name)
	if normalized == "key" || isCredentialKeyQualifier(normalized) {
		preserved = name + "=" + preserved
	}
	return end, preserved, true, true
}

// credentialHTMLTagEnd locates a tag closer without treating a greater-than
// sign inside a quoted attribute as the end of the tag.
func credentialHTMLTagEnd(value string, cursor int) (int, bool) {
	quote := byte(0)
	for cursor < len(value) {
		character := value[cursor]
		switch {
		case quote != 0 && character == quote:
			quote = 0
		case quote == 0 && (character == '\'' || character == '"'):
			quote = character
		case quote == 0 && character == '>':
			return cursor + 1, true
		}
		cursor++
	}
	return 0, false
}

// isASCIIAlpha reports whether value can start an HTML tag name.
func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

// isHTMLTagNameByte reports whether value can continue an HTML tag name.
func isHTMLTagNameByte(value byte) bool {
	return isASCIIAlpha(value) || value >= '0' && value <= '9' || value == '-' || value == ':'
}

// decodePercentLayer decodes every complete percent triplet in one pass and
// leaves standalone or incomplete percent text untouched.
func decodePercentLayer(value string) (string, bool) {
	firstEscape := -1
	for index := 0; index+2 < len(value); index++ {
		if value[index] == '%' && isHexDigit(value[index+1]) && isHexDigit(value[index+2]) {
			firstEscape = index
			break
		}
	}
	if firstEscape < 0 {
		return value, false
	}

	var builder strings.Builder
	builder.Grow(len(value) - 2)
	builder.WriteString(value[:firstEscape])
	for index := firstEscape; index < len(value); {
		if index+2 < len(value) && value[index] == '%' &&
			isHexDigit(value[index+1]) && isHexDigit(value[index+2]) {
			builder.WriteByte(hexValue(value[index+1])<<4 | hexValue(value[index+2]))
			index += 3
			continue
		}
		builder.WriteByte(value[index])
		index++
	}
	return builder.String(), true
}

// isHexDigit reports whether value is one ASCII hexadecimal digit.
func isHexDigit(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

// hexValue converts one validated ASCII hexadecimal digit to its value.
func hexValue(value byte) byte {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10
	default:
		return value - 'A' + 10
	}
}

// credentialDecodePassLimit derives the maximum useful nesting depth from the
// encoded input length while retaining a fixed CPU ceiling for hostile input.
func credentialDecodePassLimit(inputBytes int) int {
	theoretical := inputBytes/2 + 2
	if theoretical > maximumCredentialDecodePasses {
		return maximumCredentialDecodePasses
	}
	return theoretical
}

// containsDecodedCredentialMaterial recognizes credential assignments,
// request headers, authorization schemes, private-key blocks, and path names.
func containsDecodedCredentialMaterial(value string) bool {
	if containsCredentialSchemePrefix(value, "-----begin") ||
		containsCredentialSchemePrefix(value, "bearer") {
		return true
	}
	for index := 0; index < len(value); index++ {
		if value[index] != ':' && value[index] != '=' {
			continue
		}
		if index > 0 && (value[index-1] == ':' || value[index-1] == '=') {
			continue
		}
		name, complete := credentialNameBeforeSeparator(value, index)
		if !complete || containsSensitiveCredentialName(name) {
			return true
		}
	}
	return containsCredentialURL(value) || containsCredentialPathSegment(value)
}

// containsCredentialSchemePrefix recognizes the mandatory separator after a
// credential scheme even when an invisible Unicode format rune replaces
// ordinary whitespace.
func containsCredentialSchemePrefix(value string, prefix string) bool {
	for searchStart := 0; searchStart < len(value); {
		offset := strings.Index(value[searchStart:], prefix)
		if offset < 0 {
			return false
		}
		separator := searchStart + offset + len(prefix)
		if separator < len(value) {
			character, size := utf8.DecodeRuneInString(value[separator:])
			if character != utf8.RuneError || size != 1 {
				if unicode.IsSpace(character) || unicode.Is(unicode.Cf, character) {
					return true
				}
			}
		}
		searchStart = separator
	}
	return false
}

// containsSensitiveCredentialName evaluates the complete structured key while
// ignoring numeric bracket indexes that do not change its credential meaning.
func containsSensitiveCredentialName(value string) bool {
	normalized, complete := normalizeCredentialName(value)
	return !complete || containsSensitiveKeyMarker(normalized)
}

// normalizeCredentialName preserves base and property names while omitting
// numeric-only bracket indexes. Malformed or mismatched brackets fail closed.
func normalizeCredentialName(value string) (string, bool) {
	var (
		builder        strings.Builder
		bracket        strings.Builder
		bracketClosers []rune
		bracketDigits  bool
	)
	builder.Grow(len(value))
	bracket.Grow(len(value))
	for _, character := range value {
		if closing, opening := credentialBracketClosing(character); opening {
			if len(bracketClosers) == 0 {
				bracket.Reset()
				bracketDigits = true
			}
			bracketClosers = append(bracketClosers, closing)
			continue
		}
		if isCredentialBracketClosing(character) {
			if len(bracketClosers) == 0 {
				return "", false
			}
			last := len(bracketClosers) - 1
			if bracketClosers[last] != character {
				return "", false
			}
			bracketClosers = bracketClosers[:last]
			if len(bracketClosers) == 0 && !bracketDigits {
				builder.WriteString(bracket.String())
			}
			continue
		}
		if character < '0' || character > '9' {
			bracketDigits = false
		}
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			if len(bracketClosers) == 0 {
				builder.WriteRune(character)
			} else {
				bracket.WriteRune(character)
			}
		}
	}
	if len(bracketClosers) != 0 {
		return "", false
	}
	return builder.String(), true
}

// credentialNameBeforeSeparator extracts one complete field or header key
// expression, including base, bracket, property, wrapper, and form-plus
// syntax. Reverse work is capped; an exhausted window fails closed.
func credentialNameBeforeSeparator(value string, separatorIndex int) (string, bool) {
	if separatorIndex < 0 || separatorIndex > len(value) || !utf8.ValidString(value[:separatorIndex]) {
		return "", false
	}
	if separatorIndex == 0 {
		return "", true
	}

	lowerBound := separatorIndex - maximumCredentialNameScanBytes
	if lowerBound < 0 {
		lowerBound = 0
	}
	for lowerBound < separatorIndex && !utf8.RuneStart(value[lowerBound]) {
		lowerBound++
	}

	end, complete := trimCredentialWhitespaceBackward(value, lowerBound, separatorIndex)
	if !complete {
		return "", false
	}
	start, complete := scanCredentialNameExpressionBackward(value, lowerBound, end)
	if !complete {
		return "", false
	}

	var (
		previousEnd   int
		previousStart int
		joined        bool
	)

	// A percent-decoded form key may separate its base from one or more
	// bracket/property expressions with spaces. Prepend every such component
	// once so the caller evaluates the complete semantic key.
	for isCredentialBracketLedExpression(value[start:end]) {
		previousEnd, joined, complete = trimCredentialWordJoinBackward(value, lowerBound, start)
		if !complete {
			return "", false
		}
		if !joined {
			break
		}
		previousStart, complete = scanCredentialNameExpressionBackward(value, lowerBound, previousEnd)
		if !complete {
			return "", false
		}
		if previousStart == previousEnd {
			break
		}
		start = previousStart
	}

	candidate := value[start:end]
	if normalizeKey(credentialNameBase(candidate)) != "key" {
		return candidate, true
	}
	for {
		previousEnd, joined, complete = trimCredentialWordJoinBackward(value, lowerBound, start)
		if !complete {
			return "", false
		}
		if !joined {
			return candidate, true
		}
		previousStart, complete = scanCredentialNameExpressionBackward(value, lowerBound, previousEnd)
		if !complete {
			return "", false
		}
		if previousStart == previousEnd {
			return candidate, true
		}
		previous := value[previousStart:previousEnd]
		if isCredentialKeyQualifier(normalizeKey(previous)) {
			return value[previousStart:end], true
		}
		if !isCredentialIndexExpression(previous) {
			return candidate, true
		}
		start = previousStart
	}
}

// scanCredentialNameExpressionBackward scans one contiguous form-style key
// expression and reports whether its beginning was observed inside the bound.
func scanCredentialNameExpressionBackward(value string, lowerBound int, cursor int) (int, bool) {
	for cursor > lowerBound {
		character, size := utf8.DecodeLastRuneInString(value[lowerBound:cursor])
		if character == utf8.RuneError && size == 1 {
			return 0, false
		}
		if !isCredentialNameExpressionCharacter(character) {
			return cursor, true
		}
		cursor -= size
	}
	return cursor, lowerBound == 0
}

// trimCredentialWhitespaceBackward removes trailing Unicode whitespace
// without crossing the fixed reverse-scan boundary.
func trimCredentialWhitespaceBackward(value string, lowerBound int, cursor int) (int, bool) {
	for cursor > lowerBound {
		character, size := utf8.DecodeLastRuneInString(value[lowerBound:cursor])
		if character == utf8.RuneError && size == 1 {
			return 0, false
		}
		if !unicode.IsSpace(character) {
			return cursor, true
		}
		cursor -= size
	}
	return cursor, lowerBound == 0
}

// trimCredentialWordJoinBackward consumes form-urlencoded plus signs and
// Unicode whitespace joining two adjacent key components.
func trimCredentialWordJoinBackward(value string, lowerBound int, cursor int) (int, bool, bool) {
	joined := false
	for cursor > lowerBound {
		character, size := utf8.DecodeLastRuneInString(value[lowerBound:cursor])
		if character == utf8.RuneError && size == 1 {
			return 0, false, false
		}
		if !unicode.IsSpace(character) && character != '+' {
			return cursor, joined, true
		}
		joined = true
		cursor -= size
	}
	return cursor, joined, lowerBound == 0
}

// isCredentialKeyQualifier reports whether a word joined with "key" forms a
// credential-bearing multi-word field name.
func isCredentialKeyQualifier(value string) bool {
	return value == "api" || value == "private" || value == "access" || containsSensitiveKeyMarker(value)
}

// isCredentialIndexExpression reports whether a joined expression consists
// only of one or more numeric bracket indexes.
func isCredentialIndexExpression(value string) bool {
	if !isCredentialBracketLedExpression(value) {
		return false
	}
	normalized, complete := normalizeCredentialName(value)
	return complete && normalized == ""
}

// credentialNameBase returns the leading base name before bracket or property
// syntax. Wrappers remain part of the slice and are removed by normalization.
func credentialNameBase(value string) string {
	for index, character := range value {
		if character == '.' || isCredentialBracketOpening(character) {
			return value[:index]
		}
	}
	return value
}

// isCredentialBracketLedExpression reports whether a contiguous expression
// starts, after optional wrappers, with bracket syntax. Treating a malformed
// opening as bracket-led is deliberately conservative.
func isCredentialBracketLedExpression(value string) bool {
	for value != "" {
		character, size := utf8.DecodeRuneInString(value)
		if isCredentialBracketOpening(character) {
			return true
		}
		if !isCredentialNameWrapper(character) {
			return false
		}
		value = value[size:]
	}
	return false
}

// isCredentialBracketOpening reports whether value starts supported form-key
// bracket syntax.
func isCredentialBracketOpening(value rune) bool {
	switch value {
	case '[', '\uff3b', '\u3010', '\u3014', '\u3016', '\u3018', '\u301a':
		return true
	default:
		return false
	}
}

// credentialBracketClosing returns the supported closer for one opening rune.
func credentialBracketClosing(value rune) (rune, bool) {
	switch value {
	case '[':
		return ']', true
	case '\uff3b':
		return '\uff3d', true
	case '\u3010':
		return '\u3011', true
	case '\u3014':
		return '\u3015', true
	case '\u3016':
		return '\u3017', true
	case '\u3018':
		return '\u3019', true
	case '\u301a':
		return '\u301b', true
	default:
		return 0, false
	}
}

// isCredentialBracketClosing reports whether value closes supported form-key
// bracket syntax.
func isCredentialBracketClosing(value rune) bool {
	switch value {
	case ']', '\uff3d', '\u3011', '\u3015', '\u3017', '\u3019', '\u301b':
		return true
	default:
		return false
	}
}

// isCredentialNameCharacter reports whether a rune may occur inside a common
// header, assignment, or human-readable credential field name.
func isCredentialNameCharacter(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsNumber(value) || unicode.IsMark(value) ||
		unicode.Is(unicode.Cf, value) || unicode.Is(unicode.Pc, value) || unicode.Is(unicode.Pd, value) ||
		value == '.'
}

// isCredentialNameExpressionCharacter reports whether a rune connects parts
// of one form-style key expression without crossing into ordinary prose.
func isCredentialNameExpressionCharacter(value rune) bool {
	return isCredentialNameCharacter(value) || isCredentialNameWrapper(value)
}

// isCredentialNameWrapper reports whether a rune may wrap a diagnostic field
// name without changing its credential-bearing meaning.
func isCredentialNameWrapper(value rune) bool {
	switch value {
	case '\'', '"', '`',
		'(', ')', '[', ']', '{', '}', '<', '>',
		'\u2018', '\u2019', '\u201c', '\u201d', '\u00ab', '\u00bb', '\u2039', '\u203a',
		'\uff08', '\uff09', '\uff1c', '\uff1e', '\uff3b', '\uff3d', '\uff5b', '\uff5d',
		'\u3008', '\u3009', '\u300a', '\u300b', '\u300c', '\u300d', '\u300e', '\u300f',
		'\u3010', '\u3011', '\u3014', '\u3015', '\u3016', '\u3017', '\u3018', '\u3019', '\u301a', '\u301b':
		return true
	default:
		return false
	}
}

// containsCredentialURL recognizes userinfo in arbitrary schemes and
// protocol-relative references using one monotonic authority scan. Query and
// path credentials are handled by the shared assignment and path scanners.
func containsCredentialURL(value string) bool {
	for index := 0; index+1 < len(value); {
		if value[index] != '/' || value[index+1] != '/' {
			index++
			continue
		}
		authorityStart := index + 2
		authorityEnd := credentialAuthorityEnd(value, authorityStart)
		if credentialAuthorityCarriesUserinfo(value[authorityStart:authorityEnd]) {
			return true
		}
		index = authorityEnd
	}
	return false
}

// credentialAuthorityCarriesUserinfo conservatively recognizes an authority
// containing userinfo. Colons without an authority marker are never URLs.
func credentialAuthorityCarriesUserinfo(value string) bool {
	at := strings.LastIndexByte(value, '@')
	return at >= 0 && at+1 < len(value)
}

// credentialAuthorityEnd finds the end of one authority. When a slash ends an
// empty authority, the caller resumes at that slash, so repeated "//" input
// advances monotonically without rescanning a suffix.
func credentialAuthorityEnd(value string, start int) int {
	for index := start; index < len(value); {
		character, size := utf8.DecodeRuneInString(value[index:])
		if unicode.IsSpace(character) || strings.ContainsRune("/?#'\"`<>{}", character) {
			return index
		}
		index += size
	}
	return len(value)
}

// containsCredentialPathSegment recognizes path fields that carry a value
// after a credential-name segment. Route names alone are safe diagnostics.
func containsCredentialPathSegment(value string) bool {
	if !strings.Contains(value, "/") {
		return false
	}
	for _, field := range strings.Fields(value) {
		if !strings.Contains(field, "/") {
			continue
		}
		path := field
		if queryStart := strings.IndexAny(path, "?#"); queryStart >= 0 {
			path = path[:queryStart]
		}
		if credentialPathCarriesSecret(path) {
			return true
		}
	}
	return false
}

// credentialPathCarriesSecret distinguishes a route name from a path that
// places an apparent secret value immediately after that name.
func credentialPathCarriesSecret(value string) bool {
	var (
		rawSegments = strings.Split(value, "/")
		segments    = make([]string, 0, len(rawSegments))
		normalized  string
	)
	for _, segment := range rawSegments {
		normalized = normalizeKey(segment)
		if normalized != "" {
			segments = append(segments, normalized)
		}
	}
	for index, segment := range segments {
		if isCredentialPathSegment(segment) && index+1 < len(segments) {
			if segment == "password" && segments[index+1] == "reset" && index+2 == len(segments) {
				continue
			}
			return true
		}
		if (segment == "private" || segment == "access") && index+2 < len(segments) &&
			segments[index+1] == "key" {
			return true
		}
	}
	return false
}

// isCredentialPathSegment reports whether one normalized path component names
// credential material rather than an ordinary resource.
func isCredentialPathSegment(value string) bool {
	switch value {
	case "token", "tokens", "accesstoken", "refreshtoken", "idtoken", "sessiontoken", "authtoken", "xauthtoken",
		"password", "passwords", "passwd", "secret", "secrets", "cookie", "cookies", "setcookie",
		"auth", "authorization", "authentication", "session", "sessions", "sessionid",
		"credential", "credentials", "privatekey", "accesskey", "apikey", "xapikey", "bearer", "signature":
		return true
	default:
		return false
	}
}
