// This file validates outbox inputs, canonicalizes redacted JSON, and computes retry timing.

package outbox

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	defaultLeaseDuration       = 30 * time.Second
	defaultBaseBackoff         = time.Second
	defaultMaximumBackoff      = 15 * time.Minute
	defaultMaximumClaimBatch   = 100
	defaultMaxAttempts         = 12
	defaultMaximumAttempts     = 100
	defaultMaximumPayloadSize  = 256 * 1024
	defaultMaximumMetadataSize = 64 * 1024
	maximumLeaseDuration       = time.Hour
	maximumBackoffDuration     = 24 * time.Hour
	maximumClaimBatch          = 1000
	maximumAttempts            = 100
	maximumPayloadSize         = 4 * 1024 * 1024
	maximumMetadataSize        = 1024 * 1024
	maximumClaimEvidenceBytes  = 64 * 1024 * 1024
	maximumEventTypeLength     = 128
	maximumClaimEventTypes     = 16
	maximumTopicLength         = 128
	maximumAggregateTypeLength = 64
	maximumAggregateNoLength   = 64
	maximumCorrelationNoLength = 64
	maximumDeduplicationLength = 1024
	maximumWorkerIDLength      = 80
	maximumEventNoLength       = 40
	maximumLeaseTokenLength    = 128
	maximumErrorCodeLength     = 128
	maximumErrorSummaryLength  = 1024
	maximumTokenAttempts       = 4
	eventNumberPrefix          = "EVT_"
	leaseTokenPrefix           = "LSE_"
	expiredUnknownErrorCode    = "lease_expired_result_unknown"
	expiredUnknownErrorSummary = "Delivery lease expired before a confirmed result"
	attemptsExhaustedErrorCode = "attempts_exhausted"
	attemptsExhaustedSummary   = "Delivery attempts were exhausted before another claim"
)

var sensitiveJSONKeys = map[string]struct{}{
	"accesskey":          {},
	"accesskeyid":        {},
	"accesstoken":        {},
	"appkey":             {},
	"appsecret":          {},
	"apikey":             {},
	"apisecret":          {},
	"authorization":      {},
	"auth":               {},
	"authtoken":          {},
	"bearer":             {},
	"clientsecret":       {},
	"cookie":             {},
	"credential":         {},
	"credentials":        {},
	"idtoken":            {},
	"passphrase":         {},
	"passwd":             {},
	"password":           {},
	"privatekey":         {},
	"proxyauthorization": {},
	"refreshtoken":       {},
	"secret":             {},
	"secretid":           {},
	"secretkey":          {},
	"sessionid":          {},
	"sessiontoken":       {},
	"signature":          {},
	"setcookie":          {},
	"token":              {},
}

type normalizedEnqueueInput struct {
	EnqueueInput
	payload             json.RawMessage
	metadata            json.RawMessage
	deduplicationDigest string
	availableAtExplicit bool
	maxAttemptsExplicit bool
}

// normalizeConfig applies finite defaults and rejects unsafe or contradictory bounds.
func normalizeConfig(config Config) (Config, error) {
	if config.LeaseDuration < 0 || config.BaseBackoff < 0 || config.MaximumBackoff < 0 ||
		config.MaximumClaimBatch < 0 || config.DefaultMaxAttempts < 0 || config.MaximumAttempts < 0 ||
		config.MaximumPayloadSize < 0 || config.MaximumMetadataSize < 0 {
		return Config{}, gerror.New("outbox configuration cannot contain negative bounds")
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = defaultLeaseDuration
	}
	if config.BaseBackoff == 0 {
		config.BaseBackoff = defaultBaseBackoff
	}
	if config.MaximumBackoff == 0 {
		config.MaximumBackoff = defaultMaximumBackoff
	}
	if config.MaximumClaimBatch == 0 {
		config.MaximumClaimBatch = defaultMaximumClaimBatch
	}
	if config.DefaultMaxAttempts == 0 {
		config.DefaultMaxAttempts = defaultMaxAttempts
	}
	if config.MaximumAttempts == 0 {
		config.MaximumAttempts = defaultMaximumAttempts
	}
	if config.MaximumPayloadSize == 0 {
		config.MaximumPayloadSize = defaultMaximumPayloadSize
	}
	if config.MaximumMetadataSize == 0 {
		config.MaximumMetadataSize = defaultMaximumMetadataSize
	}
	if config.LeaseDuration > maximumLeaseDuration ||
		config.BaseBackoff > maximumBackoffDuration ||
		config.MaximumBackoff > maximumBackoffDuration ||
		config.MaximumClaimBatch > maximumClaimBatch ||
		config.MaximumAttempts > maximumAttempts ||
		config.MaximumPayloadSize > maximumPayloadSize ||
		config.MaximumMetadataSize > maximumMetadataSize {
		return Config{}, gerror.New("outbox configuration exceeds a safety bound")
	}
	if config.MaximumBackoff < config.BaseBackoff {
		return Config{}, gerror.New("outbox maximum backoff must not be shorter than base backoff")
	}
	if config.DefaultMaxAttempts > config.MaximumAttempts {
		return Config{}, gerror.New("outbox default attempts exceed the configured maximum")
	}
	maximumEvidencePerEvent := config.MaximumPayloadSize + config.MaximumMetadataSize
	if config.MaximumClaimBatch > maximumClaimEvidenceBytes/maximumEvidencePerEvent {
		return Config{}, gerror.New("outbox claim batch and evidence limits exceed the total memory budget")
	}
	return config, nil
}

// normalizeEnqueueInput validates immutable fields and produces a canonical envelope.
func (s *serviceImpl) normalizeEnqueueInput(input EnqueueInput) (normalizedEnqueueInput, error) {
	input.EventType = strings.TrimSpace(input.EventType)
	input.Topic = strings.TrimSpace(input.Topic)
	input.AggregateType = strings.TrimSpace(input.AggregateType)
	input.AggregateNo = strings.TrimSpace(input.AggregateNo)
	input.CorrelationNo = strings.TrimSpace(input.CorrelationNo)
	input.DeduplicationKey = strings.TrimSpace(input.DeduplicationKey)
	if !validRequiredString(input.EventType, maximumEventTypeLength) ||
		!validRequiredString(input.Topic, maximumTopicLength) ||
		!validRequiredString(input.AggregateType, maximumAggregateTypeLength) ||
		!validRequiredString(input.AggregateNo, maximumAggregateNoLength) ||
		!validOptionalString(input.CorrelationNo, maximumCorrelationNoLength) ||
		!validRequiredString(input.DeduplicationKey, maximumDeduplicationLength) ||
		input.EventVersion < 1 {
		return normalizedEnqueueInput{}, bizerr.NewCode(CodeInvalid)
	}
	payload, err := normalizeEvidence(input.Payload, s.config.MaximumPayloadSize)
	if err != nil {
		return normalizedEnqueueInput{}, err
	}
	metadata, err := normalizeEvidence(input.Metadata, s.config.MaximumMetadataSize)
	if err != nil {
		return normalizedEnqueueInput{}, err
	}
	availableAtExplicit := !input.AvailableAt.IsZero()
	if availableAtExplicit {
		input.AvailableAt = databaseTime(input.AvailableAt)
	} else {
		input.AvailableAt = databaseTime(s.now())
	}
	maxAttemptsExplicit := input.MaxAttempts != 0
	if !maxAttemptsExplicit {
		input.MaxAttempts = s.config.DefaultMaxAttempts
	}
	if input.MaxAttempts < 1 || input.MaxAttempts > s.config.MaximumAttempts {
		return normalizedEnqueueInput{}, bizerr.NewCode(CodeInvalid)
	}
	return normalizedEnqueueInput{
		EnqueueInput:        input,
		payload:             payload,
		metadata:            metadata,
		deduplicationDigest: digest(input.DeduplicationKey),
		availableAtExplicit: availableAtExplicit,
		maxAttemptsExplicit: maxAttemptsExplicit,
	}, nil
}

// normalizeClaimInput validates worker evidence and returns a bounded batch size.
func (s *serviceImpl) normalizeClaimInput(input ClaimInput) (ClaimInput, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if !validRequiredString(input.WorkerID, maximumWorkerIDLength) ||
		input.Limit < 0 || len(input.EventTypes) > maximumClaimEventTypes {
		return ClaimInput{}, bizerr.NewCode(CodeInvalid)
	}
	if input.Limit == 0 {
		input.Limit = s.config.MaximumClaimBatch
	}
	if input.Limit > s.config.MaximumClaimBatch {
		return ClaimInput{}, bizerr.NewCode(CodeInvalid)
	}
	if len(input.EventTypes) > 0 {
		unique := make(map[string]struct{}, len(input.EventTypes))
		for _, eventType := range input.EventTypes {
			eventType = strings.TrimSpace(eventType)
			if !validRequiredString(eventType, maximumEventTypeLength) {
				return ClaimInput{}, bizerr.NewCode(CodeInvalid)
			}
			unique[eventType] = struct{}{}
		}
		input.EventTypes = make([]string, 0, len(unique))
		for eventType := range unique {
			input.EventTypes = append(input.EventTypes, eventType)
		}
		sort.Strings(input.EventTypes)
	}
	return input, nil
}

// normalizeLeaseRef validates syntax; persistence performs the complete live-lease fence.
func normalizeLeaseRef(lease LeaseRef) (LeaseRef, error) {
	lease.EventNo = strings.TrimSpace(lease.EventNo)
	lease.LeaseToken = strings.TrimSpace(lease.LeaseToken)
	if !validRequiredString(lease.EventNo, maximumEventNoLength) ||
		!validRequiredString(lease.LeaseToken, maximumLeaseTokenLength) || lease.AttemptNo < 1 {
		return LeaseRef{}, bizerr.NewCode(CodeInvalid)
	}
	return lease, nil
}

// normalizeFailureInput validates and trims redacted failure evidence.
func normalizeFailureInput(input FailureInput) (FailureInput, error) {
	lease, err := normalizeLeaseRef(input.Lease)
	if err != nil {
		return FailureInput{}, err
	}
	input.Lease = lease
	input.ErrorCode = strings.TrimSpace(input.ErrorCode)
	input.ErrorSummary = strings.TrimSpace(input.ErrorSummary)
	if !validRequiredString(input.ErrorCode, maximumErrorCodeLength) ||
		!validRequiredRedactedText(input.ErrorSummary, maximumErrorSummaryLength) {
		return FailureInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

// normalizeUnknownInput validates and trims redacted unknown-result evidence.
func normalizeUnknownInput(input UnknownInput) (UnknownInput, error) {
	lease, err := normalizeLeaseRef(input.Lease)
	if err != nil {
		return UnknownInput{}, err
	}
	input.Lease = lease
	input.ErrorCode = strings.TrimSpace(input.ErrorCode)
	input.ErrorSummary = strings.TrimSpace(input.ErrorSummary)
	if !validRequiredString(input.ErrorCode, maximumErrorCodeLength) ||
		!validRequiredRedactedText(input.ErrorSummary, maximumErrorSummaryLength) {
		return UnknownInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

// normalizeEvidence returns canonical, bounded JSON and rejects common secret-bearing keys.
func normalizeEvidence(raw json.RawMessage, maximumSize int) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("{}"), nil
	}
	if len(trimmed) > maximumSize {
		return nil, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, bizerr.NewCode(CodeEvidenceInvalid)
	}
	object, ok := decoded.(map[string]any)
	if !ok || object == nil {
		return nil, bizerr.NewCode(CodeEvidenceInvalid)
	}
	if err := ensureJSONEOF(decoder); err != nil || containsSensitiveJSONKey(object) {
		return nil, bizerr.NewCode(CodeEvidenceInvalid)
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(canonical) > maximumSize {
		return nil, bizerr.NewCode(CodeEvidenceTooLarge)
	}
	return canonical, nil
}

// ensureJSONEOF rejects a second JSON value after the event object.
func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return gerror.New("JSON evidence contains more than one value")
}

// containsSensitiveJSONKey recursively detects common credential field names.
func containsSensitiveJSONKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isSensitiveJSONKey(key) {
				return true
			}
			if containsSensitiveJSONKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsSensitiveJSONKey(child) {
				return true
			}
		}
	}
	return false
}

// isSensitiveJSONKey rejects credential keys even when wrapped in common
// transport prefixes such as X-API-Key, while explicit references stay safe.
func isSensitiveJSONKey(key string) bool {
	normalized := normalizedJSONKey(key)
	if strings.HasSuffix(normalized, "ref") || strings.HasSuffix(normalized, "reference") {
		return false
	}
	for sensitive := range sensitiveJSONKeys {
		if normalized == sensitive || strings.HasSuffix(normalized, sensitive) {
			return true
		}
	}
	return false
}

// normalizedJSONKey removes punctuation so common key variants share one policy entry.
func normalizedJSONKey(value string) string {
	return strings.Map(func(item rune) rune {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			return unicode.ToLower(item)
		}
		return -1
	}, value)
}

// retryBackoff returns deterministic equal-jitter backoff in [cap/2, cap].
func (s *serviceImpl) retryBackoff(eventNo string, attemptNo int) time.Duration {
	ceiling := s.config.BaseBackoff
	for current := 1; current < attemptNo && ceiling < s.config.MaximumBackoff; current++ {
		if ceiling > s.config.MaximumBackoff/2 {
			ceiling = s.config.MaximumBackoff
			break
		}
		ceiling *= 2
	}
	if ceiling > s.config.MaximumBackoff {
		ceiling = s.config.MaximumBackoff
	}
	if ceiling <= 1 {
		return ceiling
	}
	floor := ceiling / 2
	window := ceiling - floor
	seed := sha256.Sum256([]byte(eventNo + "\x00" + strconv.Itoa(attemptNo)))
	jitter := time.Duration(binary.BigEndian.Uint64(seed[:8]) % uint64(window+1))
	return floor + jitter
}

// newOpaqueToken returns a cryptographically random 128-bit opaque identifier.
func newOpaqueToken(random io.Reader, prefix string) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate outbox token")
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

// newLeaseToken binds an opaque invocation token to the validated worker identity.
func newLeaseToken(random io.Reader, workerID string) (string, error) {
	token, err := newOpaqueToken(random, leaseTokenPrefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", workerID, token), nil
}

// digest returns the lowercase SHA-256 digest used by the persisted deduplication scope.
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// databaseTime normalizes comparisons to PostgreSQL timestamp precision.
func databaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

// validRequiredString checks UTF-8, length, non-emptiness, and control characters.
func validRequiredString(value string, maximum int) bool {
	return value != "" && validOptionalString(value, maximum)
}

// validOptionalString checks UTF-8, length, and disallows control characters.
func validOptionalString(value string, maximum int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, item := range value {
		if unicode.IsControl(item) {
			return false
		}
	}
	return true
}

// validRequiredRedactedText adds a defensive credential-marker check to the
// caller-owned requirement that diagnostic prose is already redacted.
func validRequiredRedactedText(value string, maximum int) bool {
	if !validRequiredString(value, maximum) {
		return false
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"authorization:", "proxy-authorization:", "bearer ",
		"token=", "token:", "api_key=", "api-key=", "apikey=",
		"secret=", "password=", "passwd=", "private_key=", "private-key=",
		"cookie:", "set-cookie:", "-----begin ",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}
