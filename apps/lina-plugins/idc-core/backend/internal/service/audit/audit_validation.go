// This file validates finite audit inputs and recursively redacts versioned JSON evidence.

package audit

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html"
	"io"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	defaultMaximumSnapshotBytes = 64 * 1024
	defaultMaximumSnapshotDepth = 16
	defaultMaximumSnapshotNodes = 4096
	defaultMaximumPageSize      = 25
	defaultMaximumResponseBytes = 1024 * 1024

	absoluteMaximumSnapshotBytes = 256 * 1024
	absoluteMaximumSnapshotDepth = 64
	absoluteMaximumSnapshotNodes = 100000
	absoluteMaximumPageSize      = 100
	absoluteMaximumResponseBytes = 8 * 1024 * 1024
	minimumEventScalarAllowance  = 32 * 1024
	maximumDigestKeyBytes        = 4096
	minimumDigestKeyBytes        = 32
	maximumRetainedReplayKeys    = 8
	maximumRetainedIntegrityKeys = 32
	maximumIntegrityKeyIDRunes   = 64
	maximumJSONKeyRunes          = 256
	maximumIdentifierRunes       = 128
	maximumIdempotencyKeyRunes   = 512
	maximumPermissionRunes       = 256
	maximumUserAgentBytes        = 4096
	maximumJSONBIntegralDigits   = 131072
	maximumJSONBFractionalDigits = 16383
	maximumCredentialScanDepth   = 8
	maximumCredentialCandidates  = 16384
	minimumCredentialScanBytes   = 64 * 1024
	maximumCredentialScanBytes   = 4 * 1024 * 1024
	maximumCredentialFragments   = 16
	snapshotSchemaVersion        = 1
	redactedValue                = "[REDACTED]"
)

var forbiddenSnapshotKeys = map[string]struct{}{
	"accesskey": {}, "accesskeyid": {}, "accesstoken": {},
	"apikey": {}, "apisecret": {}, "appkey": {}, "appsecret": {},
	"authorization": {}, "authtoken": {}, "bearer": {},
	"clientsecret": {}, "cookie": {}, "credential": {}, "credentials": {},
	"idtoken": {}, "passphrase": {}, "passwd": {}, "password": {},
	"privatekey": {}, "proxyauthorization": {}, "refreshtoken": {},
	"secret": {}, "secretid": {}, "secretkey": {}, "sessionid": {},
	"sessiontoken": {}, "setcookie": {}, "signature": {}, "token": {},
}

type normalizedObject struct {
	original       json.RawMessage
	originalObject map[string]any
	redacted       map[string]any
}

type credentialScanBudget struct {
	remainingBytes      int
	remainingCandidates int
}

// normalizeConfig applies safe finite defaults, copies both versioned keyrings,
// rejects cross-purpose key reuse, and parses trusted proxy prefixes.
func normalizeConfig(
	config Config,
) (Config, []netip.Prefix, map[string][]byte, map[string][]byte, error) {
	if config.MaximumSnapshotBytes < 0 || config.MaximumSnapshotDepth < 0 ||
		config.MaximumSnapshotNodes < 0 || config.MaximumPageSize < 0 ||
		config.MaximumResponseBytes < 0 {
		return Config{}, nil, nil, nil, gerror.New("audit configuration cannot contain negative bounds")
	}
	if config.MaximumSnapshotBytes == 0 {
		config.MaximumSnapshotBytes = defaultMaximumSnapshotBytes
	}
	if config.MaximumSnapshotDepth == 0 {
		config.MaximumSnapshotDepth = defaultMaximumSnapshotDepth
	}
	if config.MaximumSnapshotNodes == 0 {
		config.MaximumSnapshotNodes = defaultMaximumSnapshotNodes
	}
	if config.MaximumPageSize == 0 {
		config.MaximumPageSize = defaultMaximumPageSize
	}
	if config.MaximumResponseBytes == 0 {
		config.MaximumResponseBytes = defaultMaximumResponseBytes
	}
	if config.MaximumSnapshotBytes > absoluteMaximumSnapshotBytes ||
		config.MaximumSnapshotDepth > absoluteMaximumSnapshotDepth ||
		config.MaximumSnapshotNodes > absoluteMaximumSnapshotNodes ||
		config.MaximumPageSize > absoluteMaximumPageSize ||
		config.MaximumResponseBytes > absoluteMaximumResponseBytes {
		return Config{}, nil, nil, nil, gerror.New("audit configuration exceeds an absolute safety bound")
	}
	minimumResponseBytes := config.MaximumSnapshotBytes*3 + minimumEventScalarAllowance
	if config.MaximumResponseBytes < minimumResponseBytes {
		return Config{}, nil, nil, nil, gerror.New("audit response bound cannot contain one maximum-sized event")
	}
	replayActive, replayRetained, replayKeys, err := normalizeHMACKeyring(
		config.ActiveReplayDigestKey,
		config.RetainedReplayDigestKeys,
		maximumRetainedReplayKeys,
		"replay digest",
	)
	if err != nil {
		return Config{}, nil, nil, nil, err
	}
	reservationLookup, err := normalizeHMACKey(config.ReservationLookupKey, "reservation lookup")
	if err != nil {
		return Config{}, nil, nil, nil, err
	}
	integrityActive, integrityRetained, integrityKeys, err := normalizeHMACKeyring(
		config.ActiveIntegrityKey,
		config.RetainedIntegrityKeys,
		maximumRetainedIntegrityKeys,
		"row integrity",
	)
	if err != nil {
		return Config{}, nil, nil, nil, err
	}
	for _, replaySecret := range replayKeys {
		if bytes.Equal(replaySecret, reservationLookup.Secret) {
			return Config{}, nil, nil, nil, gerror.New(
				"audit reservation-lookup and replay keys must be distinct",
			)
		}
		for _, integritySecret := range integrityKeys {
			if bytes.Equal(replaySecret, integritySecret) {
				return Config{}, nil, nil, nil, gerror.New(
					"audit replay and row-integrity keys must be distinct",
				)
			}
		}
	}
	for _, integritySecret := range integrityKeys {
		if bytes.Equal(integritySecret, reservationLookup.Secret) {
			return Config{}, nil, nil, nil, gerror.New(
				"audit reservation-lookup and row-integrity keys must be distinct",
			)
		}
	}
	config.ReservationLookupKey = reservationLookup
	config.ActiveReplayDigestKey = replayActive
	config.RetainedReplayDigestKeys = replayRetained
	config.ActiveIntegrityKey = integrityActive
	config.RetainedIntegrityKeys = integrityRetained
	config.TrustedProxyCIDRs = append([]string(nil), config.TrustedProxyCIDRs...)
	proxies := make([]netip.Prefix, 0, len(config.TrustedProxyCIDRs))
	seen := make(map[netip.Prefix]struct{}, len(config.TrustedProxyCIDRs))
	for _, raw := range config.TrustedProxyCIDRs {
		if raw != strings.TrimSpace(raw) || raw == "" {
			return Config{}, nil, nil, nil, gerror.New(
				"audit trusted proxy CIDR must be non-empty and normalized",
			)
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return Config{}, nil, nil, nil, gerror.Wrap(err, "parse audit trusted proxy CIDR")
		}
		prefix = prefix.Masked()
		if _, exists := seen[prefix]; exists {
			continue
		}
		seen[prefix] = struct{}{}
		proxies = append(proxies, prefix)
	}
	return config, proxies, replayKeys, integrityKeys, nil
}

// normalizeHMACKeyring validates one bounded active/retained keyring and owns
// private copies of all secret bytes.
func normalizeHMACKeyring(
	active IntegrityKey,
	retained []IntegrityKey,
	maximumRetained int,
	label string,
) (IntegrityKey, []IntegrityKey, map[string][]byte, error) {
	if len(retained) > maximumRetained {
		return IntegrityKey{}, nil, nil, gerror.Newf(
			"audit %s keyring exceeds its retained-key bound",
			label,
		)
	}
	active, err := normalizeHMACKey(active, label)
	if err != nil {
		return IntegrityKey{}, nil, nil, err
	}
	keys := map[string][]byte{active.ID: append([]byte(nil), active.Secret...)}
	normalizedRetained := make([]IntegrityKey, 0, len(retained))
	for _, candidate := range retained {
		key, keyErr := normalizeHMACKey(candidate, label)
		if keyErr != nil {
			return IntegrityKey{}, nil, nil, keyErr
		}
		if _, exists := keys[key.ID]; exists {
			return IntegrityKey{}, nil, nil, gerror.Newf(
				"audit %s key IDs must be unique",
				label,
			)
		}
		for _, secret := range keys {
			if bytes.Equal(secret, key.Secret) {
				return IntegrityKey{}, nil, nil, gerror.Newf(
					"audit %s key secrets must be unique",
					label,
				)
			}
		}
		keys[key.ID] = append([]byte(nil), key.Secret...)
		normalizedRetained = append(normalizedRetained, key)
	}
	return active, normalizedRetained, keys, nil
}

// normalizeHMACKey validates one normalized, versioned HMAC key and owns a
// private copy of its secret bytes.
func normalizeHMACKey(key IntegrityKey, label string) (IntegrityKey, error) {
	if key.ID != strings.TrimSpace(key.ID) ||
		!validPersistedStableCode(key.ID, maximumIntegrityKeyIDRunes, true) {
		return IntegrityKey{}, gerror.Newf("audit %s key ID is invalid", label)
	}
	if len(key.Secret) < minimumDigestKeyBytes || len(key.Secret) > maximumDigestKeyBytes {
		return IntegrityKey{}, gerror.Newf(
			"audit %s key must contain 32 to 4096 bytes",
			label,
		)
	}
	key.Secret = append([]byte(nil), key.Secret...)
	return key, nil
}

// normalizeFields validates and canonicalizes business identifiers without actor input.
func normalizeFields(fields Fields) (Fields, error) {
	fields.IdempotencyScope = strings.TrimSpace(fields.IdempotencyScope)
	fields.IdempotencyKey = strings.TrimSpace(fields.IdempotencyKey)
	fields.OperationCode = strings.TrimSpace(fields.OperationCode)
	fields.TargetType = strings.TrimSpace(fields.TargetType)
	fields.TargetNo = strings.TrimSpace(fields.TargetNo)
	fields.CustomerNo = strings.TrimSpace(fields.CustomerNo)
	fields.ReasonCode = strings.TrimSpace(fields.ReasonCode)
	fields.RequiredPermission = strings.TrimSpace(fields.RequiredPermission)
	fields.CorrelationNo = strings.TrimSpace(fields.CorrelationNo)
	if fields.Risk == "" {
		fields.Risk = RiskLow
	}
	if err := validateNormalizedFields(fields); err != nil {
		return Fields{}, err
	}
	return fields, nil
}

// validateNormalizedFields is the final pre-persistence scalar gate. It is
// deliberately reusable after trusted authority resolution mutates a field.
func validateNormalizedFields(fields Fields) error {
	if !validStableCode(fields.IdempotencyScope, maximumIdentifierRunes, true) ||
		!validRequiredText(fields.IdempotencyKey, maximumIdempotencyKeyRunes) ||
		!validPersistedStableCode(fields.OperationCode, maximumIdentifierRunes, true) ||
		!validPersistedStableCode(fields.TargetType, 64, true) ||
		!validReferenceText(fields.TargetNo, 64, true) ||
		!validReferenceText(fields.CustomerNo, 40, false) ||
		!validPersistedStableCode(fields.ReasonCode, maximumIdentifierRunes, false) ||
		!validAuditPermission(fields.RequiredPermission) ||
		!validReferenceText(fields.CorrelationNo, 64, false) ||
		!validResult(fields.Result) {
		return bizerr.NewCode(CodeInvalid)
	}
	if !validRisk(fields.Risk) {
		return bizerr.NewCode(CodeInvalid)
	}
	if fields.Result != ResultSucceeded && fields.ReasonCode == "" {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

// normalizeSearchInput validates indexed filters and a finite keyset page.
func (s *serviceImpl) normalizeSearchInput(input SearchInput) (SearchInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.OperationCode = strings.TrimSpace(input.OperationCode)
	input.TargetType = strings.TrimSpace(input.TargetType)
	input.TargetNo = strings.TrimSpace(input.TargetNo)
	input.CorrelationNo = strings.TrimSpace(input.CorrelationNo)
	if input.ActorUserID < 0 || input.ActingUserID < 0 || input.Limit < 0 ||
		!validReferenceText(input.CustomerNo, 40, false) ||
		!validPersistedStableCode(input.OperationCode, maximumIdentifierRunes, false) ||
		!validPersistedStableCode(input.TargetType, 64, false) ||
		!validReferenceText(input.TargetNo, 64, false) ||
		!validReferenceText(input.CorrelationNo, 64, false) ||
		(input.Result != "" && !validResult(input.Result)) ||
		(input.Risk != "" && !validRisk(input.Risk)) {
		return SearchInput{}, bizerr.NewCode(CodeInvalid)
	}
	if (input.TargetType == "") != (input.TargetNo == "") {
		return SearchInput{}, bizerr.NewCode(CodeInvalid)
	}
	if input.Limit == 0 {
		input.Limit = min(50, s.config.MaximumPageSize)
	}
	if input.Limit > s.config.MaximumPageSize {
		return SearchInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.OccurredFrom = normalizeOptionalTime(input.OccurredFrom)
	input.OccurredUntil = normalizeOptionalTime(input.OccurredUntil)
	if input.OccurredFrom != nil && input.OccurredUntil != nil &&
		!input.OccurredFrom.Before(*input.OccurredUntil) {
		return SearchInput{}, bizerr.NewCode(CodeInvalid)
	}
	if input.Cursor != nil {
		if input.Cursor.InternalID <= 0 || input.Cursor.OccurredAt.IsZero() {
			return SearchInput{}, bizerr.NewCode(CodeInvalid)
		}
		copyCursor := *input.Cursor
		copyCursor.OccurredAt = databaseTime(copyCursor.OccurredAt)
		input.Cursor = &copyCursor
	}
	return input, nil
}

// normalizeObject streams exactly one bounded JSON object, rejects duplicate
// keys at every depth, and keeps a canonical original only in request memory.
func (s *serviceImpl) normalizeObject(value any) (normalizedObject, error) {
	raw, err := marshalSnapshotInput(value)
	if err != nil {
		return normalizedObject{}, err
	}
	if len(raw) > s.config.MaximumSnapshotBytes {
		return normalizedObject{}, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	if !utf8.Valid(raw) {
		return normalizedObject{}, bizerr.NewCode(CodeInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var (
		nodes            int
		credentialBudget = newCredentialScanBudget(len(raw))
	)
	decoded, err := s.decodeJSONValue(decoder, 0, &nodes, &credentialBudget)
	if err != nil {
		return normalizedObject{}, err
	}
	originalObject, originalOK := decoded.original.(map[string]any)
	redactedObject, redactedOK := decoded.redacted.(map[string]any)
	if !originalOK || !redactedOK || originalObject == nil || redactedObject == nil {
		return normalizedObject{}, bizerr.NewCode(CodeInvalid)
	}
	if err = ensureJSONEnd(decoder); err != nil {
		return normalizedObject{}, bizerr.NewCode(CodeInvalid)
	}
	original, err := json.Marshal(originalObject)
	if err != nil {
		return normalizedObject{}, gerror.Wrap(err, "canonicalize audit snapshot")
	}
	return normalizedObject{
		original:       original,
		originalObject: originalObject,
		redacted:       redactedObject,
	}, nil
}

type decodedJSONValue struct {
	original any
	redacted any
}

// canonicalNumberValue retains the original JSON number token for strict
// metadata type checks while marshaling only its JSONB-stable representation.
type canonicalNumberValue struct {
	lexical   string
	canonical json.Number
}

func (value canonicalNumberValue) MarshalJSON() ([]byte, error) {
	return []byte(value.canonical.String()), nil
}

// decodeJSONValue recursively consumes one JSON value without materializing an
// object through encoding/json's duplicate-key last-write-wins behavior.
func (s *serviceImpl) decodeJSONValue(
	decoder *json.Decoder,
	depth int,
	nodes *int,
	credentialBudget *credentialScanBudget,
) (decodedJSONValue, error) {
	if depth > s.config.MaximumSnapshotDepth {
		return decodedJSONValue{}, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	(*nodes)++
	if *nodes > s.config.MaximumSnapshotNodes {
		return decodedJSONValue{}, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	token, err := decoder.Token()
	if err != nil {
		return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
	}
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			return s.decodeJSONObject(decoder, depth, nodes, credentialBudget)
		case '[':
			return s.decodeJSONArray(decoder, depth, nodes, credentialBudget)
		default:
			return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
		}
	case string:
		if len(typed) > s.config.MaximumSnapshotBytes || !utf8.ValidString(typed) {
			return decodedJSONValue{}, bizerr.NewCode(CodeEvidenceTooLarge)
		}
		redacted := any(typed)
		if containsCredentialBearingTextWithBudget(typed, credentialBudget) {
			redacted = redactedValue
		}
		return decodedJSONValue{original: typed, redacted: redacted}, nil
	case json.Number:
		canonical, canonicalErr := canonicalJSONNumber(typed, s.config.MaximumSnapshotBytes)
		if canonicalErr != nil {
			return decodedJSONValue{}, canonicalErr
		}
		number := canonicalNumberValue{lexical: typed.String(), canonical: canonical}
		return decodedJSONValue{original: number, redacted: number}, nil
	case bool, nil:
		return decodedJSONValue{original: typed, redacted: typed}, nil
	default:
		return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
	}
}

// canonicalJSONNumber expands exponent notation before persistence so the
// authenticated JSON bytes remain stable after PostgreSQL JSONB normalization.
// It also rejects values outside PostgreSQL numeric's storage range.
func canonicalJSONNumber(value json.Number, maximumBytes int) (json.Number, error) {
	raw := value.String()
	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = raw[1:]
	}
	significand := raw
	exponent := int64(0)
	if exponentIndex := strings.IndexAny(raw, "eE"); exponentIndex >= 0 {
		significand = raw[:exponentIndex]
		parsed, err := strconv.ParseInt(raw[exponentIndex+1:], 10, 64)
		if err != nil {
			return "", bizerr.NewCode(CodeEvidenceTooLarge)
		}
		if parsed > maximumJSONBIntegralDigits || parsed < -maximumJSONBFractionalDigits {
			return "", bizerr.NewCode(CodeEvidenceTooLarge)
		}
		exponent = parsed
	}
	integerPart := significand
	fractionalPart := ""
	if decimalIndex := strings.IndexByte(significand, '.'); decimalIndex >= 0 {
		integerPart = significand[:decimalIndex]
		fractionalPart = significand[decimalIndex+1:]
	}
	digits := strings.TrimLeft(integerPart+fractionalPart, "0")
	scale := int64(len(fractionalPart)) - exponent
	if digits == "" {
		return canonicalJSONZero(scale, maximumBytes)
	}
	if !validJSONBNumericShape(int64(len(digits)), scale) {
		return "", bizerr.NewCode(CodeEvidenceTooLarge)
	}
	var encoded string
	if scale <= 0 {
		zeroCount := -scale
		if int64(len(digits))+zeroCount > int64(maximumBytes) {
			return "", bizerr.NewCode(CodeEvidenceTooLarge)
		}
		encoded = digits + strings.Repeat("0", int(zeroCount))
	} else if int64(len(digits)) > scale {
		decimalIndex := int64(len(digits)) - scale
		encoded = digits[:decimalIndex] + "." + digits[decimalIndex:]
	} else {
		leadingZeros := scale - int64(len(digits))
		if 2+leadingZeros+int64(len(digits)) > int64(maximumBytes) {
			return "", bizerr.NewCode(CodeEvidenceTooLarge)
		}
		encoded = "0." + strings.Repeat("0", int(leadingZeros)) + digits
	}
	if negative {
		encoded = "-" + encoded
	}
	if len(encoded) > maximumBytes {
		return "", bizerr.NewCode(CodeEvidenceTooLarge)
	}
	return json.Number(encoded), nil
}

// canonicalJSONZero removes a negative-zero sign while preserving the scale
// PostgreSQL numeric retains for a zero represented with a fractional scale.
func canonicalJSONZero(scale int64, maximumBytes int) (json.Number, error) {
	if scale <= 0 {
		return json.Number("0"), nil
	}
	if scale > maximumJSONBFractionalDigits || scale+2 > int64(maximumBytes) {
		return "", bizerr.NewCode(CodeEvidenceTooLarge)
	}
	return json.Number("0." + strings.Repeat("0", int(scale))), nil
}

// validJSONBNumericShape mirrors PostgreSQL numeric's integral and fractional
// digit limits before a JSONB insert can fail outside the audit error contract.
func validJSONBNumericShape(significantDigits int64, scale int64) bool {
	fractionalDigits := max(scale, int64(0))
	integralDigits := significantDigits - scale
	if integralDigits < 1 {
		integralDigits = 1
	}
	return integralDigits <= maximumJSONBIntegralDigits &&
		fractionalDigits <= maximumJSONBFractionalDigits
}

// decodeJSONObject consumes one object and rejects an exact duplicate key
// before any redaction or canonicalization can hide the ambiguity.
func (s *serviceImpl) decodeJSONObject(
	decoder *json.Decoder,
	depth int,
	nodes *int,
	credentialBudget *credentialScanBudget,
) (decodedJSONValue, error) {
	original := make(map[string]any)
	redacted := make(map[string]any)
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
		}
		key, ok := keyToken.(string)
		if !ok || key == "" || !validOptionalText(key, maximumJSONKeyRunes) {
			return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := seen[key]; duplicate {
			return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
		}
		if containsCredentialBearingTextWithBudget(key, credentialBudget) {
			return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
		}
		seen[key] = struct{}{}
		value, err := s.decodeJSONValue(decoder, depth+1, nodes, credentialBudget)
		if err != nil {
			return decodedJSONValue{}, err
		}
		original[key] = value.original
		if isForbiddenSnapshotKey(key) {
			redacted[key] = redactedValue
		} else {
			redacted[key] = value.redacted
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
	}
	return decodedJSONValue{original: original, redacted: redacted}, nil
}

// decodeJSONArray consumes one array while applying the same recursive bounds
// and string-value credential scan as object fields.
func (s *serviceImpl) decodeJSONArray(
	decoder *json.Decoder,
	depth int,
	nodes *int,
	credentialBudget *credentialScanBudget,
) (decodedJSONValue, error) {
	original := make([]any, 0)
	redacted := make([]any, 0)
	for decoder.More() {
		value, err := s.decodeJSONValue(decoder, depth+1, nodes, credentialBudget)
		if err != nil {
			return decodedJSONValue{}, err
		}
		original = append(original, value.original)
		redacted = append(redacted, value.redacted)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return decodedJSONValue{}, bizerr.NewCode(CodeInvalid)
	}
	return decodedJSONValue{original: original, redacted: redacted}, nil
}

// versionedSnapshot wraps redacted state with an explicit schema version.
func (s *serviceImpl) versionedSnapshot(object map[string]any) (json.RawMessage, error) {
	encoded, err := json.Marshal(stateEnvelope{
		SchemaVersion: snapshotSchemaVersion,
		Data:          object,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "encode versioned audit snapshot")
	}
	if len(encoded) > s.config.MaximumSnapshotBytes {
		return nil, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	return encoded, nil
}

// marshalSnapshotInput accepts structs or raw JSON while preserving one-object semantics.
func marshalSnapshotInput(value any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	switch typed := value.(type) {
	case json.RawMessage:
		if len(bytes.TrimSpace(typed)) == 0 {
			return []byte("{}"), nil
		}
		return append([]byte(nil), typed...), nil
	case []byte:
		if len(bytes.TrimSpace(typed)) == 0 {
			return []byte("{}"), nil
		}
		return append([]byte(nil), typed...), nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeInvalid)
		}
		return encoded, nil
	}
}

// ensureJSONEnd rejects trailing JSON values after one complete object.
func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return gerror.New("audit snapshot contains more than one JSON value")
}

// isForbiddenSnapshotKey treats every key containing a credential marker as
// sensitive. This deliberately over-redacts decorated forms such as
// providerApiKeyValueArchive instead of relying on a finite suffix allowlist.
func isForbiddenSnapshotKey(key string) bool {
	normalized := normalizeJSONKey(key)
	for forbidden := range forbiddenSnapshotKeys {
		if strings.Contains(normalized, forbidden) {
			return true
		}
	}
	return false
}

// normalizeJSONKey removes punctuation so common key variants share one policy entry.
func normalizeJSONKey(value string) string {
	return strings.Map(func(item rune) rune {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			return unicode.ToLower(item)
		}
		return -1
	}, value)
}

// validResult reports whether value is one supported persisted result.
func validResult(value ResultCode) bool {
	switch value {
	case ResultSucceeded, ResultDenied, ResultFailed, ResultResultUnknown:
		return true
	default:
		return false
	}
}

// validRisk reports whether value is one supported persisted risk classification.
func validRisk(value RiskCode) bool {
	switch value {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return true
	default:
		return false
	}
}

// validRequiredText adds non-emptiness to bounded, printable UTF-8 validation.
func validRequiredText(value string, maximumRunes int) bool {
	return value != "" && validOptionalText(value, maximumRunes)
}

// validOptionalText rejects invalid UTF-8, control characters, and oversized values.
func validOptionalText(value string, maximumRunes int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximumRunes {
		return false
	}
	for _, item := range value {
		if unicode.IsControl(item) {
			return false
		}
	}
	return true
}

// validStableCode accepts only low-entropy ASCII business-code characters.
func validStableCode(value string, maximumRunes int, required bool) bool {
	if value == "" {
		return !required
	}
	if !validOptionalText(value, maximumRunes) {
		return false
	}
	for index, item := range value {
		if (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') {
			continue
		}
		if index > 0 && (item == '.' || item == '_' || item == ':' || item == '-') {
			continue
		}
		return false
	}
	return true
}

// validPersistedStableCode keeps the stable-code alphabet while rejecting
// reversible credential encodings before they reach independent columns or
// security evidence.
func validPersistedStableCode(value string, maximumRunes int, required bool) bool {
	return validStableCode(value, maximumRunes, required) &&
		!containsCredentialBearingText(value)
}

// Only exact compiled permission identifiers may bypass credential-text
// scanning: channel-secret:manage is a permission, not a secret assignment.
// Unknown codes and every other independent audit column keep the scanner.
func validAuditPermission(value string) bool {
	return validStableCode(value, maximumPermissionRunes, false) &&
		(idccap.IsCatalogedAdminPermission(value) || !containsCredentialBearingText(value))
}

// validReferenceText prevents credential-bearing prose from being smuggled
// into independently persisted business-number or trace columns.
func validReferenceText(value string, maximumRunes int, required bool) bool {
	if value == "" {
		return !required
	}
	return validOptionalText(value, maximumRunes) && !containsCredentialBearingText(value)
}

// newCredentialScanBudget gives one snapshot or scalar a strict aggregate work
// budget. Exhaustion is treated as sensitive so adversarial encodings cannot
// trade CPU amplification for a redaction bypass.
func newCredentialScanBudget(inputBytes int) credentialScanBudget {
	byteBudget := inputBytes * 16
	if byteBudget < minimumCredentialScanBytes {
		byteBudget = minimumCredentialScanBytes
	}
	if byteBudget > maximumCredentialScanBytes {
		byteBudget = maximumCredentialScanBytes
	}
	return credentialScanBudget{
		remainingBytes:      byteBudget,
		remainingCandidates: maximumCredentialCandidates,
	}
}

// spend accounts for every candidate inspection or decoding attempt.
func (budget *credentialScanBudget) spend(byteCount int, candidate bool) bool {
	if budget == nil || byteCount < 0 || byteCount > budget.remainingBytes {
		return false
	}
	if candidate {
		if budget.remainingCandidates <= 0 {
			return false
		}
		budget.remainingCandidates--
	}
	budget.remainingBytes -= byteCount
	return true
}

// containsCredentialBearingText detects credential material in headers,
// assignments, paths, wrappers, and a fixed point of bounded encoding layers.
func containsCredentialBearingText(value string) bool {
	budget := newCredentialScanBudget(len(value))
	return containsCredentialBearingTextWithBudget(value, &budget)
}

// containsCredentialBearingTextWithBudget shares one aggregate scan budget
// across every string in a snapshot and fails closed when it is exhausted.
func containsCredentialBearingTextWithBudget(
	value string,
	budget *credentialScanBudget,
) bool {
	type candidate struct {
		text  string
		depth int
	}
	queue := []candidate{{text: value}}
	seen := make(map[string]struct{})
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := seen[current.text]; exists {
			continue
		}
		if !budget.spend(len(current.text), true) {
			return true
		}
		seen[current.text] = struct{}{}
		if directlyContainsCredentialText(current.text) {
			return true
		}
		if current.depth >= maximumCredentialScanDepth {
			return true
		}
		fragments, exhausted := credentialEncodingFragments(current.text, budget)
		if exhausted {
			return true
		}
		for _, fragment := range fragments {
			if len(queue) >= budget.remainingCandidates {
				return true
			}
			queue = append(queue, candidate{text: fragment, depth: current.depth + 1})
		}
		variants, exhausted := decodeCredentialTextVariants(current.text, budget)
		if exhausted {
			return true
		}
		for _, decoded := range variants {
			if decoded != current.text {
				if len(queue) >= budget.remainingCandidates {
					return true
				}
				queue = append(queue, candidate{text: decoded, depth: current.depth + 1})
			}
		}
	}
	return false
}

// directlyContainsCredentialText recognizes unencoded private keys, auth
// schemes, key/value labels, request headers, and path segments.
func directlyContainsCredentialText(value string) bool {
	lower := normalizeCredentialText(value)
	whitespaceNormalized := strings.Join(strings.Fields(lower), " ")
	for _, marker := range []string{
		"-----begin private key-----", "-----begin rsa private key-----",
		"-----begin ec private key-----", "-----begin openssh private key-----",
	} {
		if strings.Contains(whitespaceNormalized, marker) {
			return true
		}
	}
	if containsBearerCredential(lower) {
		return true
	}
	if containsURIUserinfo(strings.TrimSpace(value)) {
		return true
	}
	canonical := strings.Map(func(item rune) rune {
		if unicode.IsLetter(item) || unicode.IsDigit(item) ||
			item == ':' || item == '=' || item == '/' || item == '\\' ||
			item == '?' || item == '&' {
			return unicode.ToLower(item)
		}
		return -1
	}, lower)
	for index, item := range canonical {
		if item != ':' && item != '=' {
			continue
		}
		label := credentialLabelBefore(canonical, index)
		if label != "" && isForbiddenSnapshotKey(label) {
			return true
		}
	}
	segments := strings.FieldsFunc(canonical, func(item rune) bool {
		return item == '/' || item == '\\' || item == '?' || item == '&'
	})
	for index := 0; index+1 < len(segments); index++ {
		if isForbiddenSnapshotKey(segments[index]) && segments[index+1] != "" {
			return true
		}
	}
	return false
}

// normalizeCredentialText collapses Unicode whitespace and invisible format
// controls before token matching. They are never allowed to split a credential
// label or authentication scheme into a scanner bypass.
func normalizeCredentialText(value string) string {
	return strings.Map(func(item rune) rune {
		if unicode.IsSpace(item) || unicode.In(item, unicode.Cf) {
			return ' '
		}
		return unicode.ToLower(item)
	}, value)
}

// containsURIUserinfo rejects credential-bearing endpoint and DSN forms. URL
// parsing avoids treating an ordinary at-sign in free text as authentication.
func containsURIUserinfo(value string) bool {
	if value == "" || (!strings.Contains(value, "://") && !strings.HasPrefix(value, "//")) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.User != nil && parsed.User.String() != ""
}

// containsBearerCredential recognizes the strong bearer authentication scheme
// only at a token boundary. Ordinary prose containing words such as "basic"
// is left alone unless an authorization header or credential label is present.
func containsBearerCredential(value string) bool {
	const marker = "bearer"
	for offset := 0; offset < len(value); {
		index := strings.Index(value[offset:], marker)
		if index < 0 {
			return false
		}
		index += offset
		beforeBoundary := index == 0 || !isASCIILetterOrDigit(value[index-1])
		after := index + len(marker)
		separator, separatorBytes := utf8.DecodeRuneInString(value[after:])
		if beforeBoundary && separatorBytes > 0 && unicode.IsSpace(separator) {
			if fields := strings.Fields(value[after+separatorBytes:]); len(fields) > 0 && fields[0] != "" {
				return true
			}
		}
		offset = index + len(marker)
	}
	return false
}

// isASCIILetterOrDigit supports boundary checks around ASCII auth schemes.
func isASCIILetterOrDigit(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9')
}

// credentialLabelBefore returns the alphanumeric label immediately preceding
// an assignment or header delimiter.
func credentialLabelBefore(value string, delimiterIndex int) string {
	start := delimiterIndex
	for start > 0 {
		item := value[start-1]
		if (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') {
			start--
			continue
		}
		break
	}
	return value[start:delimiterIndex]
}

// credentialEncodingFragments extracts bounded wrapper values such as
// payload=<base64>. Padding '=' remains part of the candidate.
func credentialEncodingFragments(
	value string,
	budget *credentialScanBudget,
) ([]string, bool) {
	if !budget.spend(len(value), false) {
		return nil, true
	}
	fragments := make([]string, 0, 2)
	seen := make(map[string]struct{})
	for index := 0; index < len(value); index++ {
		if value[index] != '=' && value[index] != ':' {
			continue
		}
		start := index + 1
		for start < len(value) && (unicode.IsSpace(rune(value[start])) || value[start] == '\'' || value[start] == '"') {
			start++
		}
		end := start
		for end < len(value) && !isCredentialFragmentDelimiter(value[end]) {
			end++
		}
		fragment := strings.TrimSpace(value[start:end])
		if len(fragment) < 8 || fragment == value {
			continue
		}
		if _, exists := seen[fragment]; exists {
			continue
		}
		if len(fragments) >= maximumCredentialFragments {
			return nil, true
		}
		seen[fragment] = struct{}{}
		fragments = append(fragments, fragment)
	}
	return fragments, false
}

// isCredentialFragmentDelimiter terminates one wrapper value without treating
// base64 padding '=' as a delimiter.
func isCredentialFragmentDelimiter(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', '&', ';', ',', '\'', '"', '{', '}', '[', ']', '(', ')', '<', '>':
		return true
	default:
		return false
	}
}

// decodeCredentialTextVariants returns only bounded, valid UTF-8 candidates;
// every attempted transformation is charged to the shared total work budget.
func decodeCredentialTextVariants(
	value string,
	budget *credentialScanBudget,
) ([]string, bool) {
	variants := make([]string, 0, 7)
	appendVariant := func(decoded []byte, err error) {
		if err == nil && len(decoded) > 0 && len(decoded) <= absoluteMaximumSnapshotBytes &&
			utf8.Valid(decoded) && validDecodedCredentialText(decoded) {
			variants = append(variants, string(decoded))
		}
	}
	if !budget.spend(len(value), false) {
		return variants, true
	}
	if decoded := html.UnescapeString(value); decoded != value {
		appendVariant([]byte(decoded), nil)
	}
	if !budget.spend(len(value), false) {
		return variants, true
	}
	if decoded, err := url.QueryUnescape(value); err == nil && decoded != value {
		appendVariant([]byte(decoded), nil)
	}
	if !budget.spend(len(value), false) {
		return variants, true
	}
	if decoded, err := url.PathUnescape(value); err == nil && decoded != value {
		appendVariant([]byte(decoded), nil)
	}
	compact := strings.TrimSpace(value)
	if compact != "" {
		for _, encoding := range []*base64.Encoding{
			base64.StdEncoding,
			base64.RawStdEncoding,
			base64.URLEncoding,
			base64.RawURLEncoding,
		} {
			if !budget.spend(len(compact), false) {
				return variants, true
			}
			decoded, err := encoding.DecodeString(compact)
			appendVariant(decoded, err)
		}
	}
	if compact != "" && len(compact)%2 == 0 {
		if !budget.spend(len(compact), false) {
			return variants, true
		}
		decoded, err := hex.DecodeString(compact)
		appendVariant(decoded, err)
	}
	if len(compact) >= 2 && compact[0] == '"' && compact[len(compact)-1] == '"' {
		if !budget.spend(len(compact), false) {
			return variants, true
		}
		var decoded string
		if err := json.Unmarshal([]byte(compact), &decoded); err == nil {
			appendVariant([]byte(decoded), nil)
		}
	}
	return variants, false
}

// validDecodedCredentialText prevents arbitrary binary decodes from creating
// noisy false positives while retaining normal header, path, and JSON text.
func validDecodedCredentialText(value []byte) bool {
	for _, item := range string(value) {
		if unicode.IsControl(item) && item != '\r' && item != '\n' && item != '\t' {
			return false
		}
	}
	return true
}

// normalizeOptionalTime copies one timestamp at PostgreSQL precision.
func normalizeOptionalTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := databaseTime(*value)
	return &copyValue
}

// databaseTime normalizes comparisons to PostgreSQL timestamp precision.
func databaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}
