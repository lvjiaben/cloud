// This file exposes the one buyer-owned public Tencent eSign callback route.

package app

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
)

// TencentCallbackPath is the exact public callback path configured in the
// current buyer installation's Tencent application.
const TencentCallbackPath = "/callbacks/v1/tencent-essbasic"

func localTencentCallbackHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if runtime == nil || runtime.supplierCallback == nil || runtime.tencentCallback == nil ||
			request.URL.RequestURI() != TencentCallbackPath ||
			request.Header.Get("Content-Type") != "application/json" ||
			request.ContentLength > suppliercallback.MaximumRawBodyBytes {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "callback_rejected"})
			return
		}
		signatures := request.Header.Values("Content-Signature")
		if len(signatures) != 1 || signatures[0] == "" || strings.ContainsAny(signatures[0], "\r\n,") {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: "callback_rejected"})
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, suppliercallback.MaximumRawBodyBytes+1))
		if err != nil || len(body) == 0 || len(body) > suppliercallback.MaximumRawBodyBytes {
			clearRuntimeBytes(body)
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "callback_rejected"})
			return
		}
		result, err := runtime.supplierCallback.Accept(request.Context(), suppliercallback.RawCallback{
			Provider: suppliercallback.ProviderTencentESSBasic,
			Body:     body, Signature: []byte(signatures[0]),
		})
		switch {
		case err == nil:
			writeJSON(writer, http.StatusOK, map[string]any{
				"status": "accepted", "replayed": result.Replayed,
			})
		case errors.Is(err, suppliercallback.ErrConflict):
			writeJSON(writer, http.StatusConflict, map[string]string{errorCodeField: "callback_conflict"})
		default:
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: "callback_rejected"})
		}
	}
}
