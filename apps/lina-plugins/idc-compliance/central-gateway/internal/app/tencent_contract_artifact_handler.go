// This file serves completed Tencent PDFs without exposing supplier URLs.
package app

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/tencentartifact"
	"lina-platform/idc-compliance-gateway/internal/tencentess"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

func localTencentContractFinalPDFHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		body, ok := authenticateTencentContractRead(
			writer, request, runtime, processauth.TencentContractFinalPDFsPath,
			processauth.TencentContractFinalPDFReadScope,
		)
		if !ok {
			return
		}
		defer clearRuntimeBytes(body)
		var input processauth.TencentContractArtifactRequest
		if decodeTencentOnboardingJSON(body, &input) != nil ||
			!gatewayRoutePattern.MatchString(input.RouteNo) {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentArtifact == nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		document, err := runtime.tencentArtifact.FinalPDF(request.Context(), input.RouteNo)
		if err != nil {
			writeTencentArtifactError(writer, err)
			return
		}
		defer clearRuntimeBytes(document.Content)
		writeTencentArtifactPDF(writer, processauth.TencentArtifactKindFinalPDF, document)
	}
}

func localTencentContractSignReportHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != processauth.TencentContractSignReportsPath ||
			request.Header.Get("Content-Type") != "application/json" || runtime == nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		body, err := readProcessJSONBody(request)
		if err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		defer clearRuntimeBytes(body)
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		_, admission, _, err := runtime.authenticateRequest(
			request.Context(), processauth.TencentContractSignReportScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeJSON, body, credentials, true,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		var input processauth.TencentContractArtifactRequest
		if decodeTencentOnboardingJSON(body, &input) != nil ||
			!gatewayRoutePattern.MatchString(input.RouteNo) {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentArtifact == nil {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		result, err := runtime.tencentArtifact.EnsureSignReport(
			request.Context(), input.RouteNo, admission.requestNo,
		)
		if err != nil {
			writeTencentArtifactError(writer, err)
			return
		}
		if result.State == tencentartifact.StateReady && result.Document != nil {
			defer clearRuntimeBytes(result.Document.Content)
			writeTencentArtifactPDF(writer, processauth.TencentArtifactKindSignReport, result.Document)
			return
		}
		status := http.StatusOK
		if result.State == tencentartifact.StatePending {
			status = http.StatusAccepted
		} else if result.State == tencentartifact.StateResultUnknown {
			status = http.StatusConflict
		}
		response := processauth.TencentContractArtifactPendingResponse{
			State: string(result.State), Code: result.Code,
		}
		if !result.NextCheckAt.IsZero() {
			response.NextCheckUnix = result.NextCheckAt.UTC().Unix()
		}
		writeJSON(writer, status, response)
	}
}

func readProcessJSONBody(request *http.Request) ([]byte, error) {
	if request == nil || request.Body == nil {
		return nil, errors.New("request body is unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, processauth.MaximumPayloadBytes+1))
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearRuntimeBytes(body)
		return nil, errors.New("request body is invalid")
	}
	return body, nil
}

func writeTencentArtifactPDF(
	writer http.ResponseWriter,
	kind string,
	document *tencentess.ArtifactDocument,
) {
	if document == nil || len(document.Content) == 0 || document.SHA256 == "" || document.ObservedAt.IsZero() {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "artifact_unavailable"})
		return
	}
	writer.Header().Set("Content-Type", "application/pdf")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Length", strconv.Itoa(len(document.Content)))
	writer.Header().Set(processauth.TencentArtifactStateHeader, processauth.TencentArtifactStateReady)
	writer.Header().Set(processauth.TencentArtifactKindHeader, kind)
	writer.Header().Set(processauth.TencentArtifactSHA256Header, document.SHA256)
	writer.Header().Set(
		processauth.TencentArtifactObservedUnixHeader,
		strconv.FormatInt(document.ObservedAt.UTC().Unix(), 10),
	)
	writer.WriteHeader(http.StatusOK)
	if _, err := writer.Write(document.Content); err != nil {
		return
	}
}

func writeTencentArtifactError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tencentartifact.ErrInvalid):
		writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
	case errors.Is(err, tencentartifact.ErrConflict), errors.Is(err, suppliercallback.ErrConflict):
		writeJSON(writer, http.StatusConflict, map[string]string{errorCodeField: "contract_artifact_not_ready"})
	default:
		writeTencentContractFlowError(writer, err, "")
	}
}
