// This file validates task inputs, canonicalizes redacted JSON, and computes bounded retry timing.

package asynctask

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
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
	defaultLeaseDuration       = 30 * time.Second
	defaultBaseBackoff         = time.Second
	defaultMaximumBackoff      = 15 * time.Minute
	defaultMaximumClaimBatch   = 100
	defaultMaxAttempts         = 12
	defaultMaximumAttempts     = 100
	defaultMaximumPayloadSize  = 64 * 1024
	defaultMaximumResultSize   = 64 * 1024
	maximumLeaseDuration       = time.Hour
	maximumBackoffDuration     = 24 * time.Hour
	maximumClaimBatch          = 1000
	maximumAttempts            = 100
	maximumPayloadSize         = 4 * 1024 * 1024
	maximumResultSize          = 1024 * 1024
	maximumClaimEvidenceBytes  = 32 * 1024 * 1024
	maximumTaskTypeLength      = 128
	maximumTaskNoLength        = 40
	maximumSubjectTypeLength   = 64
	maximumSubjectNoLength     = 64
	maximumCustomerNoLength    = 40
	maximumIdempotencyLength   = 512
	maximumWorkerIDLength      = 128
	maximumClaimTaskTypes      = 32
	maximumExternalRefLength   = 128
	maximumErrorCodeLength     = 128
	maximumSummaryLength       = 1024
	maximumManualReasonLength  = 1024
	maximumNumberAttempts      = 4
	minimumSmallInteger        = -32768
	maximumSmallInteger        = 32767
	taskNumberPrefix           = "TSK_"
	leaseTokenPrefix           = "LSE_"
	businessNumberTypeTask     = "async_task"
	businessNumberOwnerTask    = "async_task"
	deadlineExceededErrorCode  = "deadline_exceeded"
	deadlineExceededSummary    = "Task deadline elapsed before execution"
	leaseExpiredErrorCode      = "lease_expired_result_unknown"
	leaseExpiredErrorSummary   = "Execution lease expired before a confirmed result"
	attemptsExhaustedErrorCode = "attempts_exhausted"
	attemptsExhaustedSummary   = "Task attempt budget was exhausted before a terminal state was recorded"
)

// forbiddenSecretKeys contains punctuation-free, lowercase credential key forms.
var forbiddenSecretKeys = map[string]struct{}{
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

// normalizedCreateInput carries validated persistence values and replay metadata.
type normalizedCreateInput struct {
	CreateInput
	payload             json.RawMessage
	idempotencyDigest   string
	reservationScope    string
	availableAtExplicit bool
	maxAttemptsExplicit bool
	normalizedAt        time.Time
}

// normalizeConfig applies finite defaults and rejects unsafe or contradictory bounds.
func normalizeConfig(config Config) (Config, error) {
	if config.LeaseDuration < 0 || config.BaseBackoff < 0 || config.MaximumBackoff < 0 ||
		config.MaximumClaimBatch < 0 || config.DefaultMaxAttempts < 0 || config.MaximumAttempts < 0 ||
		config.MaximumPayloadSize < 0 || config.MaximumResultSize < 0 {
		return Config{}, gerror.New("async task configuration cannot contain negative bounds")
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
	if config.MaximumResultSize == 0 {
		config.MaximumResultSize = defaultMaximumResultSize
	}
	if config.LeaseDuration > maximumLeaseDuration ||
		config.BaseBackoff > maximumBackoffDuration ||
		config.MaximumBackoff > maximumBackoffDuration ||
		config.MaximumClaimBatch > maximumClaimBatch ||
		config.MaximumAttempts > maximumAttempts ||
		config.MaximumPayloadSize > maximumPayloadSize ||
		config.MaximumResultSize > maximumResultSize {
		return Config{}, gerror.New("async task configuration exceeds a safety bound")
	}
	if config.MaximumBackoff < config.BaseBackoff {
		return Config{}, gerror.New("async task maximum backoff must not be shorter than base backoff")
	}
	if config.DefaultMaxAttempts > config.MaximumAttempts {
		return Config{}, gerror.New("async task default attempts exceed the configured maximum")
	}
	maximumEvidencePerTask := 2*config.MaximumPayloadSize + config.MaximumResultSize
	if config.MaximumClaimBatch > maximumClaimEvidenceBytes/maximumEvidencePerTask {
		return Config{}, gerror.New("async task claim batch and evidence limits exceed the total memory budget")
	}
	return config, nil
}

// normalizeCreateInput validates persisted lengths, times, and redacted payload shape.
func (s *serviceImpl) normalizeCreateInput(input CreateInput) (normalizedCreateInput, error) {
	now := databaseTime(s.now())
	input.TaskType = strings.TrimSpace(input.TaskType)
	input.SubjectType = strings.TrimSpace(input.SubjectType)
	input.SubjectNo = strings.TrimSpace(input.SubjectNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validRequiredString(input.TaskType, maximumTaskTypeLength) ||
		!validRequiredString(input.SubjectType, maximumSubjectTypeLength) ||
		!validRequiredString(input.SubjectNo, maximumSubjectNoLength) ||
		!validOptionalString(input.CustomerNo, maximumCustomerNoLength) ||
		!validRequiredString(input.IdempotencyKey, maximumIdempotencyLength) ||
		input.Priority < minimumSmallInteger || input.Priority > maximumSmallInteger {
		return normalizedCreateInput{}, bizerr.NewCode(CodeInvalid)
	}
	payload, err := normalizeJSONObject(input.Payload, s.config.MaximumPayloadSize)
	if err != nil {
		return normalizedCreateInput{}, bizerr.NewCode(CodeInvalid)
	}
	availableAtExplicit := !input.AvailableAt.IsZero()
	if availableAtExplicit {
		input.AvailableAt = databaseTime(input.AvailableAt)
	} else {
		input.AvailableAt = now
	}
	if input.DeadlineAt != nil {
		deadline := databaseTime(*input.DeadlineAt)
		input.DeadlineAt = &deadline
	}
	maxAttemptsExplicit := input.MaxAttempts != 0
	if !maxAttemptsExplicit {
		input.MaxAttempts = s.config.DefaultMaxAttempts
	}
	if input.MaxAttempts < 1 || input.MaxAttempts > s.config.MaximumAttempts {
		return normalizedCreateInput{}, bizerr.NewCode(CodeInvalid)
	}
	return normalizedCreateInput{
		CreateInput:         input,
		payload:             payload,
		idempotencyDigest:   digest(input.IdempotencyKey),
		reservationScope:    digest(input.TaskType + "\x00" + input.SubjectType + "\x00" + input.SubjectNo),
		availableAtExplicit: availableAtExplicit,
		maxAttemptsExplicit: maxAttemptsExplicit,
		normalizedAt:        now,
	}, nil
}

// validateNewTaskSchedule applies time-relative rules only after an idempotent
// replay has been ruled out. Existing requests must remain readable after their deadline.
func validateNewTaskSchedule(input normalizedCreateInput) error {
	if input.DeadlineAt == nil {
		return nil
	}
	if !input.DeadlineAt.After(input.normalizedAt) ||
		!input.DeadlineAt.After(input.AvailableAt) {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

// normalizeClaimInput validates worker evidence and returns a bounded batch size.
func (s *serviceImpl) normalizeClaimInput(input ClaimInput) (ClaimInput, error) {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if !validRequiredString(input.WorkerID, maximumWorkerIDLength) || input.Limit < 0 {
		return ClaimInput{}, bizerr.NewCode(CodeInvalid)
	}
	if input.Limit == 0 {
		input.Limit = s.config.MaximumClaimBatch
	}
	if input.Limit > s.config.MaximumClaimBatch {
		return ClaimInput{}, bizerr.NewCode(CodeInvalid)
	}
	if len(input.TaskTypes) > maximumClaimTaskTypes {
		return ClaimInput{}, bizerr.NewCode(CodeInvalid)
	}
	seen := make(map[string]struct{}, len(input.TaskTypes))
	for index, taskType := range input.TaskTypes {
		taskType = strings.TrimSpace(taskType)
		if !validRequiredString(taskType, maximumTaskTypeLength) {
			return ClaimInput{}, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := seen[taskType]; duplicate {
			return ClaimInput{}, bizerr.NewCode(CodeInvalid)
		}
		seen[taskType] = struct{}{}
		input.TaskTypes[index] = taskType
	}
	return input, nil
}

// validateLeaseRef validates only syntax; persistence performs the complete live-lease fence.
func validateLeaseRef(lease LeaseRef) error {
	if lease.TenantID != idccap.DefaultTenantID ||
		!validRequiredString(strings.TrimSpace(lease.TaskNo), maximumTaskNoLength) ||
		!validRequiredString(strings.TrimSpace(lease.LeaseToken), maximumWorkerIDLength) ||
		lease.AttemptNo < 1 {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

// normalizeJSONObject returns canonical JSON for stable idempotency comparisons.
func normalizeJSONObject(raw json.RawMessage, maximumSize int) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("{}"), nil
	}
	if len(trimmed) > maximumSize || trimmed[0] != '{' {
		return nil, gerror.New("JSON evidence must be a bounded object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, gerror.New("JSON evidence must be a non-null object")
	}
	if err := ensureJSONDocumentEnd(decoder); err != nil {
		return nil, gerror.New("JSON evidence must contain exactly one object")
	}
	if err := rejectSecretKeys(object); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return nil, gerror.Wrap(err, "canonicalize JSON evidence")
	}
	if len(canonical) > maximumSize {
		return nil, gerror.New("canonical JSON evidence exceeds size bound")
	}
	return canonical, nil
}

// ensureJSONDocumentEnd rejects any second JSON value after the evidence object.
func ensureJSONDocumentEnd(decoder *json.Decoder) error {
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

// retryBackoff returns capped exponential delay with deterministic task/attempt
// jitter. Stable jitter prevents synchronized retry storms without making tests
// or recovery behavior depend on process-local randomness.
func (s *serviceImpl) retryBackoff(taskNo string, attemptNo int) time.Duration {
	delay := s.config.BaseBackoff
	for current := 1; current < attemptNo && delay < s.config.MaximumBackoff; current++ {
		if delay > s.config.MaximumBackoff/2 {
			delay = s.config.MaximumBackoff
			break
		}
		delay *= 2
	}
	if delay >= s.config.MaximumBackoff {
		return s.config.MaximumBackoff
	}
	jitterWindow := delay / 4
	if jitterWindow <= 0 {
		return delay
	}
	digestBytes := sha256.Sum256([]byte(taskNo + "\x00" + strconv.Itoa(attemptNo)))
	jitter := time.Duration(binary.BigEndian.Uint64(digestBytes[:8]) % uint64(jitterWindow+1))
	if delay > s.config.MaximumBackoff-jitter {
		return s.config.MaximumBackoff
	}
	return delay + jitter
}

// newOpaqueToken returns a cryptographically random 128-bit opaque identifier.
func newOpaqueToken(random io.Reader, prefix string) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate asynchronous task token")
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

// digest returns the lowercase SHA-256 digest used by persisted idempotency scopes.
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

// rejectSecretKeys recursively prevents task snapshots from becoming an
// accidental plaintext secret store. Explicit reference keys remain allowed.
func rejectSecretKeys(value any) error {
	switch typed := value.(type) {
	case map[string]json.RawMessage:
		for key, raw := range typed {
			if isForbiddenSecretKey(key) {
				return gerror.New("JSON evidence contains a forbidden plaintext secret key")
			}
			var nested any
			if err := json.Unmarshal(raw, &nested); err != nil {
				return gerror.Wrap(err, "inspect JSON evidence")
			}
			if err := rejectSecretKeys(nested); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, nested := range typed {
			if isForbiddenSecretKey(key) {
				return gerror.New("JSON evidence contains a forbidden plaintext secret key")
			}
			if err := rejectSecretKeys(nested); err != nil {
				return err
			}
		}
	case []any:
		for _, nested := range typed {
			if err := rejectSecretKeys(nested); err != nil {
				return err
			}
		}
	}
	return nil
}

// isForbiddenSecretKey detects punctuation and case variants while permitting reference fields.
func isForbiddenSecretKey(key string) bool {
	normalized := normalizedJSONKey(key)
	if strings.HasSuffix(normalized, "ref") || strings.HasSuffix(normalized, "reference") {
		return false
	}
	for forbidden := range forbiddenSecretKeys {
		if normalized == forbidden || strings.HasSuffix(normalized, forbidden) {
			return true
		}
	}
	return false
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

// normalizedJSONKey removes punctuation so common key variants share one policy entry.
func normalizedJSONKey(value string) string {
	return strings.Map(func(item rune) rune {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			return unicode.ToLower(item)
		}
		return -1
	}, value)
}
