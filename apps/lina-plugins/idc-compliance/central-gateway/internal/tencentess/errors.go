// This file maps SDK, network, timeout, and write-ambiguity failures into the
// adapter's closed, secret-safe error vocabulary.

package tencentess

import (
	"context"
	"errors"
	"net"
	"strings"

	tencenterrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"

	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

// ErrorCode is the closed, secret-safe Tencent failure vocabulary.
type ErrorCode string

const (
	// ErrorInvalidInput rejects invalid local inputs before a provider call.
	ErrorInvalidInput ErrorCode = "tencent_invalid_input"
	// ErrorNotConfigured means the local buyer-owned application is unavailable.
	ErrorNotConfigured ErrorCode = "tencent_not_configured"
	// ErrorCredentialRejected means Tencent rejected the local application credential.
	ErrorCredentialRejected ErrorCode = "tencent_credential_rejected"
	// ErrorPermissionDenied means the buyer account lacks the required permission.
	ErrorPermissionDenied ErrorCode = "tencent_permission_denied"
	// ErrorSubjectUnavailable means no usable locally authorized Tencent subject exists.
	ErrorSubjectUnavailable ErrorCode = "tencent_subject_unavailable"
	// ErrorResourceNotFound means the requested Tencent resource was not found.
	ErrorResourceNotFound ErrorCode = "tencent_resource_not_found"
	// ErrorRateLimited means Tencent applied a request-rate limit.
	ErrorRateLimited ErrorCode = "tencent_rate_limited"
	// ErrorServiceUnavailable means Tencent or its network path is unavailable.
	ErrorServiceUnavailable ErrorCode = "tencent_service_unavailable"
	// ErrorTimeout means the bounded Tencent call exceeded its deadline.
	ErrorTimeout ErrorCode = "tencent_timeout"
	// ErrorMalformedResponse means Tencent returned an unsafe or inconsistent response.
	ErrorMalformedResponse ErrorCode = "tencent_malformed_response"
	// ErrorResultUnknown means a write outcome cannot safely be classified.
	ErrorResultUnknown ErrorCode = "tencent_result_unknown"
	// ErrorRejected is the stable fallback for a definitive provider rejection.
	ErrorRejected ErrorCode = "tencent_rejected"
)

// Error exposes only a stable code and an optional Tencent request identifier.
type Error struct {
	// Code is the stable local error code safe for logs and management projections.
	Code ErrorCode
	// RequestID is the optional Tencent request identifier for support correlation.
	RequestID string
	cause     error
}

func (e *Error) Error() string {
	if e == nil || e.Code == "" {
		return string(ErrorRejected)
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func safeError(err error, write bool) *Error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: chooseWriteCode(write, ErrorTimeout), cause: err}
	}
	if errors.Is(err, context.Canceled) {
		return &Error{Code: chooseWriteCode(write, ErrorServiceUnavailable), cause: err}
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return &Error{Code: chooseWriteCode(write, ErrorServiceUnavailable), cause: err}
	}
	var providerError *tencenterrors.TencentCloudSDKError
	if !errors.As(err, &providerError) {
		return &Error{Code: chooseWriteCode(write, ErrorServiceUnavailable), cause: err}
	}
	code := providerError.GetCode()
	result := &Error{Code: ErrorRejected, RequestID: boundedRequestID(providerError.GetRequestId()), cause: err}
	switch {
	case strings.HasPrefix(code, "AuthFailure"):
		result.Code = ErrorCredentialRejected
	case strings.HasPrefix(code, "UnauthorizedOperation"):
		result.Code = ErrorPermissionDenied
	case strings.HasPrefix(code, "InvalidParameter") || strings.HasPrefix(code, "MissingParameter"):
		result.Code = ErrorInvalidInput
	case strings.HasPrefix(code, "ResourceNotFound"):
		result.Code = ErrorResourceNotFound
	case strings.HasPrefix(code, "RequestLimitExceeded"):
		result.Code = ErrorRateLimited
	case strings.HasPrefix(code, "ResourceUnavailable") || strings.HasPrefix(code, "InternalError"):
		result.Code = ErrorServiceUnavailable
	case strings.Contains(strings.ToLower(code), "organization") ||
		strings.Contains(strings.ToLower(code), "notactivated"):
		result.Code = ErrorSubjectUnavailable
	}
	return result
}

func chooseWriteCode(write bool, readCode ErrorCode) ErrorCode {
	if write {
		return ErrorResultUnknown
	}
	return readCode
}

func boundedRequestID(value string) string {
	if !validOpaque(value, 1, 128) {
		return ""
	}
	return value
}

func writeResultClass(err *Error) supplierguard.ResultClass {
	if err == nil {
		return supplierguard.ResultSucceeded
	}
	switch err.Code {
	case ErrorResultUnknown:
		return supplierguard.ResultUnknown
	case ErrorRateLimited, ErrorServiceUnavailable, ErrorTimeout:
		return supplierguard.ResultAvailabilityFailed
	default:
		return supplierguard.ResultDefinitiveFailure
	}
}
