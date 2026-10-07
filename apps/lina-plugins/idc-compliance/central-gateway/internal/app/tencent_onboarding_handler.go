// This file exposes the buyer-owned Tencent enterprise onboarding boundary.
package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
	"lina-platform/idc-compliance-gateway/internal/tencentess"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const (
	tencentOnboardingNotConfigured = "not_configured"
	tencentOnboardingReady         = "ready"
	tencentOnboardingPending       = "enterprise_pending"
	tencentOnboardingOperator      = "operator_pending"
	tencentOnboardingActive        = "active"
)

type tencentOnboardingLinkRequest struct {
	LegalName string `json:"legalName"`
}

type tencentOnboardingCapability struct {
	Code       string `json:"code"`
	State      string `json:"state"`
	ReasonCode string `json:"reasonCode,omitempty"`
	CheckedAt  int64  `json:"checkedAt,omitempty"`
}

type tencentOnboardingResponse struct {
	State             string                        `json:"state"`
	ReasonCode        string                        `json:"reasonCode,omitempty"`
	ConsoleURL        string                        `json:"consoleUrl,omitempty"`
	OrganizationReady bool                          `json:"organizationReady"`
	OperatorReady     bool                          `json:"operatorReady"`
	BindingNo         string                        `json:"bindingNo,omitempty"`
	OperationNo       string                        `json:"operationNo,omitempty"`
	Capabilities      []tencentOnboardingCapability `json:"capabilities"`
}

func localTencentOnboardingStatusHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != processauth.TencentOnboardingPath ||
			!requestBodyIsEmpty(request) {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		_, _, err = runtime.authenticateRead(
			request.Context(), processauth.TencentOnboardingReadScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeEmpty, nil, credentials,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() {
			writeJSON(writer, http.StatusOK, tencentOnboardingResponse{
				State: tencentOnboardingNotConfigured, ReasonCode: "tencent_app_not_configured",
				Capabilities: []tencentOnboardingCapability{},
			})
			return
		}
		state := tencentOnboardingReady
		if runtime.tencentESS.SubjectAvailable(request.Context()) {
			state = tencentOnboardingActive
		}
		writeJSON(writer, http.StatusOK, tencentOnboardingResponse{
			State: state, OrganizationReady: state == tencentOnboardingActive,
			OperatorReady: state == tencentOnboardingActive,
			Capabilities:  []tencentOnboardingCapability{},
		})
	}
}

func localTencentOnboardingLinkHandler(runtime *localRuntime) http.HandlerFunc {
	return localTencentOnboardingMutationHandler(
		runtime, processauth.TencentOnboardingLinksPath,
		processauth.TencentOnboardingLinkScope, true,
	)
}

func localTencentOnboardingRecheckHandler(runtime *localRuntime) http.HandlerFunc {
	return localTencentOnboardingMutationHandler(
		runtime, processauth.TencentOnboardingRechecksPath,
		processauth.TencentOnboardingRecheckScope, false,
	)
}

func localTencentOnboardingMutationHandler(
	runtime *localRuntime,
	path string,
	scope string,
	includeURL bool,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != path ||
			request.Header.Get("Content-Type") != "application/json" || runtime == nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, processauth.MaximumPayloadBytes+1))
		if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
			clearRuntimeBytes(body)
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		defer clearRuntimeBytes(body)
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		verified, admission, _, err := runtime.authenticateRequest(
			request.Context(), scope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeJSON, body, credentials, true,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		// A recovered business request has already crossed this handler's
		// provider-call boundary. Never create a second one-use URL or repeat an
		// authoritative Tencent recheck with the same business key.
		if admission.recovered {
			writeJSON(writer, http.StatusConflict, map[string]string{errorCodeField: processAuthReplayCode})
			return
		}
		var input tencentOnboardingLinkRequest
		if err = decodeTencentOnboardingJSON(body, &input); err != nil ||
			strings.TrimSpace(input.LegalName) == "" {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		result, err := runtime.tencentESS.CreateOrganizationConsoleURL(
			request.Context(), runtime.installationID, input.LegalName,
		)
		if err != nil {
			writeTencentOnboardingError(writer, err)
			return
		}
		response := tencentOnboardingResponse{
			State:             tencentOnboardingPending,
			OrganizationReady: result.Activated, OperatorReady: result.OperatorVerified,
			Capabilities: []tencentOnboardingCapability{},
		}
		if includeURL {
			response.ConsoleURL = result.ConsoleURL
		}
		if result.Activated {
			response.State = tencentOnboardingOperator
		}
		if result.Activated && result.OperatorVerified {
			authorization := result.ConsumeAuthorization()
			defer clearRuntimeBytes(authorization)
			bindingNo, found, bindingErr := runtime.supplierIdentity.FindActiveBinding(
				request.Context(), supplieridentity.ProviderTencentESSBasic,
				append([]byte(nil), authorization...),
			)
			if bindingErr != nil {
				writeSupplierIdentityError(writer, bindingErr)
				return
			}
			if !found {
				key := append([]byte(nil), verified.IdempotencyHMAC[:]...)
				bound, bindErr := runtime.supplierIdentity.Bind(
					request.Context(), supplieridentity.BindInput{
						Provider: supplieridentity.ProviderTencentESSBasic,
						Subject:  append([]byte(nil), authorization...), IdempotencyKey: key,
					},
				)
				clearRuntimeBytes(key)
				if bindErr != nil {
					writeSupplierIdentityError(writer, bindErr)
					return
				}
				bindingNo = bound.BindingNo
				response.OperationNo = bound.OperationNo
			}
			response.State = tencentOnboardingActive
			response.BindingNo = bindingNo
			response.Capabilities = projectTencentOnboardingCapabilities(
				runtime.tencentESS.CheckSealAndDownload(request.Context()),
			)
		}
		writeJSON(writer, http.StatusOK, response)
	}
}

func decodeTencentOnboardingJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func projectTencentOnboardingCapabilities(
	statuses []tencentess.CapabilityStatus,
) []tencentOnboardingCapability {
	result := make([]tencentOnboardingCapability, 0, len(statuses))
	for _, status := range statuses {
		item := tencentOnboardingCapability{
			Code: string(status.Capability), State: string(status.State), ReasonCode: string(status.Code),
		}
		if !status.CheckedAt.IsZero() {
			item.CheckedAt = status.CheckedAt.UTC().UnixMilli()
		}
		result = append(result, item)
	}
	return result
}

func writeTencentOnboardingError(writer http.ResponseWriter, err error) {
	var safe *tencentess.Error
	if !errors.As(err, &safe) {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_temporarily_unavailable"})
		return
	}
	status := http.StatusServiceUnavailable
	code := string(safe.Code)
	if safe.Code == tencentess.ErrorInvalidInput {
		status = http.StatusBadRequest
	}
	writeJSON(writer, status, map[string]string{errorCodeField: code})
}
