// Package aliyunface implements the fixed Aliyun financial-grade Alipay H5
// FACE contract without exposing credentials, identity plaintext, or raw
// provider payloads outside the bounded synchronous call.
package aliyunface

import (
	"context"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	// Endpoint is the only production endpoint documented for Alipay H5 FACE.
	Endpoint = "https://saf.cn-shanghai.aliyuncs.com"
	// ProviderCode is the stable local provider identity.
	ProviderCode = "aliyun_face"
	// CertTypeIdentityCard covers the provider's current supported identity-card family.
	CertTypeIdentityCard = "IDENTITY_CARD"
	// CertificationLifetime is the provider-documented one-time URL lifetime.
	CertificationLifetime = 30 * time.Minute
	// MaximumHTTPTimeout bounds one synchronous provider call in production.
	MaximumHTTPTimeout = 30 * time.Second
)

var outerOrderNumberPattern = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)

// QueryState is the bounded authoritative result projection.
type QueryState string

const (
	// QueryStatePassed means the provider returned passed=T.
	QueryStatePassed QueryState = "passed"
	// QueryStateNotPassed deliberately covers both failure and not-yet-submitted.
	QueryStateNotPassed QueryState = "not_passed"
	// QueryStateExpired means the provider explicitly reported an expired certification.
	QueryStateExpired QueryState = "expired"
)

// ProbeOutcome is the bounded result of one read-only synthetic query.
type ProbeOutcome string

const (
	// ProbeOutcomeReachable means the signed request reached the configured FACE
	// product and returned either a success envelope or an expected certify ID result.
	ProbeOutcomeReachable ProbeOutcome = "reachable"
	// ProbeOutcomeRejected means Aliyun authoritatively rejected the credential,
	// product entitlement, or scene configuration.
	ProbeOutcomeRejected ProbeOutcome = "rejected"
	// ProbeOutcomeUnavailable means transport or provider availability was ambiguous.
	ProbeOutcomeUnavailable ProbeOutcome = "unavailable"
)

// StableErrorCode is a non-secret provider failure classification.
type StableErrorCode string

const (
	// ErrorInvalidRequest reports a locally rejected adapter request.
	ErrorInvalidRequest StableErrorCode = "invalid_request"
	// ErrorCredentialAccess reports that the exact secret scope could not be accessed.
	ErrorCredentialAccess StableErrorCode = "credential_access_failed"
	// ErrorProviderRejected reports a bounded authoritative provider rejection.
	ErrorProviderRejected StableErrorCode = "provider_rejected"
	// ErrorResultUnknown reports a transport, HTTP, or ambiguous provider failure.
	ErrorResultUnknown StableErrorCode = "result_unknown"
	// ErrorInvalidResponse reports a malformed success response.
	ErrorInvalidResponse StableErrorCode = "invalid_response"
)

// CallError contains only a stable code and bounded provider response code.
// It never retains raw response bodies, provider messages, credentials, or
// identity material. Unknown is true when an initialization call must not be
// automatically replayed.
type CallError struct {
	Code         StableErrorCode
	ProviderCode string
	Unknown      bool
}

// Error returns a stable diagnostic without raw provider material.
func (e *CallError) Error() string {
	if e == nil {
		return "aliyun face call failed"
	}
	return "aliyun face call failed: " + string(e.Code)
}

// InitiateRequest is the minimal identity input used only during one call.
// Callers must not persist or log CertName or CertNo.
type InitiateRequest struct {
	SceneID      string
	OuterOrderNo string
	CertType     string
	CertName     string
	CertNo       string
	ReturnURL    string
}

// Initiation is the non-secret provider-issued browser continuation.
type Initiation struct {
	CertifyID  string
	CertifyURL string
}

// QueryRequest binds an authoritative query to its original scene.
type QueryRequest struct {
	SceneID   string
	CertifyID string
}

// QueryResult contains only the provider's bounded final-state projection.
type QueryResult struct {
	State QueryState
}

// Client initiates and actively queries the fixed Alipay H5 FACE contract.
type Client interface {
	// Initiate performs one non-replayable external initialization. Transport
	// ambiguity is returned as CallError.Unknown and must not be auto-replayed.
	Initiate(ctx context.Context, request InitiateRequest) (Initiation, error)
	// Query actively obtains the authoritative provider result. A browser return
	// or callback value is never accepted as an authentication decision.
	Query(ctx context.Context, request QueryRequest) (QueryResult, error)
	// Probe performs one read-only query with a synthetic, non-persisted certify
	// ID. It never initializes an authentication flow or creates billable identity data.
	Probe(ctx context.Context, sceneID string) ProbeOutcome
}

// New constructs the production client with fixed endpoint, callback-only
// credentials, caller-supplied bounded HTTP transport, clock, and nonce source.
func New(
	accessor secretcap.Accessor,
	credential secretcap.Handle,
	httpClient *http.Client,
	clock func() time.Time,
	nonce func() (string, error),
) (Client, error) {
	if httpClient == nil || httpClient.Timeout <= 0 || httpClient.Timeout > MaximumHTTPTimeout {
		return nil, gerror.New("Aliyun face HTTP client must have a bounded timeout")
	}
	boundedClient := *httpClient
	boundedClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return newNativeClient(Endpoint, accessor, credential, &boundedClient, clock, nonce)
}

func validateInitiateRequest(request InitiateRequest) error {
	request.SceneID = strings.TrimSpace(request.SceneID)
	request.CertName = strings.TrimSpace(request.CertName)
	request.CertNo = strings.TrimSpace(request.CertNo)
	request.ReturnURL = strings.TrimSpace(request.ReturnURL)
	if !validSceneID(request.SceneID) || !outerOrderNumberPattern.MatchString(request.OuterOrderNo) ||
		request.CertType != CertTypeIdentityCard || request.CertName == "" || len(request.CertName) > 128 ||
		request.CertNo == "" || len(request.CertNo) > 64 || !validHTTPSURL(request.ReturnURL) {
		return &CallError{Code: ErrorInvalidRequest}
	}
	return nil
}

func validateQueryRequest(request QueryRequest) error {
	if !validSceneID(strings.TrimSpace(request.SceneID)) ||
		request.CertifyID == "" || request.CertifyID != strings.TrimSpace(request.CertifyID) ||
		len(request.CertifyID) > 128 {
		return &CallError{Code: ErrorInvalidRequest}
	}
	return nil
}

func validSceneID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateDependency(value any) error {
	if value == nil {
		return gerror.New("Aliyun face client dependency is unavailable")
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflected.IsNil() {
			return gerror.New("Aliyun face client dependency is unavailable")
		}
	}
	return nil
}
