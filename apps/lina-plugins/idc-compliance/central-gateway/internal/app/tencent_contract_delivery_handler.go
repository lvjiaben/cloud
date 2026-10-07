package app

import (
	"io"
	"net/http"
	"strconv"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

func localTencentContractDeliveryPageHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		ok := authenticateTencentContractDeliveryPageRead(writer, request, runtime)
		if !ok {
			return
		}
		limit, err := strconv.Atoi(request.URL.Query().Get(processauth.TencentContractDeliveryPageLimitQuery))
		if err != nil || limit < 1 || limit > suppliercallback.MaximumDeliveryBatchSize ||
			request.URL.RawQuery != processauth.TencentContractDeliveryPageLimitQuery+"="+strconv.Itoa(limit) ||
			runtime.supplierReconciliation == nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		items, err := runtime.supplierReconciliation.PendingDeliveries(
			request.Context(), suppliercallback.ProviderTencentESSBasic,
			tencentContractOperationScope, limit,
		)
		if err != nil {
			writeTencentContractReadError(writer, err)
			return
		}
		response := processauth.TencentContractDeliveryPageResponse{
			Items: make([]processauth.TencentContractDeliveryItem, 0, len(items)),
		}
		for _, item := range items {
			supplierObservedUnix := int64(0)
			if !item.SupplierObservedAt.IsZero() {
				supplierObservedUnix = item.SupplierObservedAt.Unix()
			}
			response.Items = append(response.Items, processauth.TencentContractDeliveryItem{
				DeliveryNo: item.DeliveryNo, RouteNo: item.RouteNo,
				RouteVersion: item.RouteVersion, Status: string(item.ProjectedStatus),
				ProviderStatus:       item.ProviderStatus,
				ReconciliationStatus: item.ReconciliationStatus,
				SourceKind:           item.SourceKind, DecisionCode: item.DecisionCode,
				SupplierObservedUnix: supplierObservedUnix,
				ObservedUnix:         item.ObservedAt.Unix(),
			})
		}
		writeJSON(writer, http.StatusOK, response)
	}
}

func authenticateTencentContractDeliveryPageRead(
	writer http.ResponseWriter,
	request *http.Request,
	runtime *localRuntime,
) bool {
	if request.URL.Path != processauth.TencentContractDeliveryPagesPath ||
		request.ContentLength > 0 || len(request.TransferEncoding) > 0 || runtime == nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
		return false
	}
	credentials, err := credentialsFromRequest(request)
	if err != nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
		return false
	}
	_, _, err = runtime.authenticateRead(
		request.Context(), processauth.TencentContractDeliveryReadScope,
		request.Method, request.URL.RequestURI(), processauth.PayloadModeEmpty, nil, credentials,
	)
	if err != nil {
		writeProcessAdmissionError(writer, err)
		return false
	}
	return true
}

func localTencentContractDeliveryAcknowledgementHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		body, ok := authenticateTencentContractDeliveryWrite(writer, request, runtime)
		if !ok {
			return
		}
		defer clearRuntimeBytes(body)
		var input processauth.TencentContractDeliveryAcknowledgementRequest
		if decodeTencentOnboardingJSON(body, &input) != nil || runtime.supplierReconciliation == nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if err := runtime.supplierReconciliation.AcknowledgeDeliveries(
			request.Context(), suppliercallback.ProviderTencentESSBasic,
			tencentContractOperationScope, input.DeliveryNos,
		); err != nil {
			writeTencentContractReadError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, processauth.TencentContractDeliveryAcknowledgementResponse{
			Acknowledged: len(input.DeliveryNos),
		})
	}
}

func authenticateTencentContractDeliveryWrite(
	writer http.ResponseWriter,
	request *http.Request,
	runtime *localRuntime,
) ([]byte, bool) {
	if request.URL.RequestURI() != processauth.TencentContractDeliveryAcknowledgementsPath ||
		request.Header.Get("Content-Type") != "application/json" || runtime == nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, processauth.MaximumPayloadBytes+1))
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearRuntimeBytes(body)
		writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
		return nil, false
	}
	credentials, err := credentialsFromRequest(request)
	if err != nil {
		clearRuntimeBytes(body)
		writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
		return nil, false
	}
	authenticatedInstallation, admission, businessFence, err := runtime.authenticateRequest(
		request.Context(), processauth.TencentContractDeliveryAcknowledgeScope,
		request.Method, request.URL.RequestURI(), processauth.PayloadModeJSON,
		body, credentials, true,
	)
	_ = authenticatedInstallation
	_ = admission
	_ = businessFence
	if err != nil {
		clearRuntimeBytes(body)
		writeProcessAdmissionError(writer, err)
		return nil, false
	}
	return body, true
}
