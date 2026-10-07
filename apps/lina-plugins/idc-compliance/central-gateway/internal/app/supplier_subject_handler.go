// This file is the only HTTP boundary allowed to pass provider authorization
// results into the installation-local supplier identity service.
package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/datalifecycle"
	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

type supplierSubjectBindingRequest struct {
	ProviderCode        string `json:"providerCode"`
	AuthorizationResult []byte `json:"authorizationResult"`
}

type supplierSubjectRevocationRequest struct {
	ProviderCode        string `json:"providerCode"`
	BindingNo           string `json:"bindingNo"`
	ReasonCode          string `json:"reasonCode"`
	EmployeePrincipalNo string `json:"employeePrincipalNo"`
	EvidenceSetSHA256   string `json:"evidenceSetSha256"`
}

type supplierSubjectReopeningRequest struct {
	ProviderCode        string `json:"providerCode"`
	PreviousBindingNo   string `json:"previousBindingNo"`
	ReasonCode          string `json:"reasonCode"`
	AuthorizationResult []byte `json:"authorizationResult"`
	EmployeePrincipalNo string `json:"employeePrincipalNo"`
	EvidenceSetSHA256   string `json:"evidenceSetSha256"`
}

type supplierSubjectResponse struct {
	OperationNo string `json:"operationNo"`
	BindingNo   string `json:"bindingNo"`
	Replayed    bool   `json:"replayed"`
}

func localSupplierSubjectBindingHandler(runtime *localRuntime) http.HandlerFunc {
	return supplierSubjectWriteHandler(
		runtime, processauth.SupplierSubjectBindingsPath, processauth.SupplierSubjectBindScope,
		func(ctx context.Context, body []byte, key []byte) (supplieridentity.Result, error) {
			var input supplierSubjectBindingRequest
			if err := decodeSupplierSubjectJSON(body, &input); err != nil {
				clearRuntimeBytes(input.AuthorizationResult)
				return supplieridentity.Result{}, err
			}
			defer clearRuntimeBytes(input.AuthorizationResult)
			if len(input.AuthorizationResult) == 0 || len(input.AuthorizationResult) > 4096 {
				clearRuntimeBytes(input.AuthorizationResult)
				return supplieridentity.Result{}, supplieridentity.ErrInvalid
			}
			return runtime.supplierIdentity.Bind(ctx, supplieridentity.BindInput{
				Provider: strings.TrimSpace(input.ProviderCode), Subject: input.AuthorizationResult,
				IdempotencyKey: key,
			})
		},
	)
}

func localSupplierSubjectRevocationHandler(runtime *localRuntime) http.HandlerFunc {
	return supplierSubjectWriteHandler(
		runtime, processauth.SupplierSubjectRevocationsPath, processauth.SupplierSubjectRevokeScope,
		func(ctx context.Context, body []byte, key []byte) (supplieridentity.Result, error) {
			var input supplierSubjectRevocationRequest
			if err := decodeSupplierSubjectJSON(body, &input); err != nil {
				return supplieridentity.Result{}, err
			}
			if strings.TrimSpace(input.ReasonCode) == supplieridentity.ReasonAuthorizationLost {
				digest, err := hex.DecodeString(strings.TrimSpace(input.EvidenceSetSHA256))
				if err != nil || len(digest) != sha256.Size || runtime.dataLifecycle == nil {
					clearRuntimeBytes(digest)
					return supplieridentity.Result{}, datalifecycle.ErrInvalid
				}
				defer clearRuntimeBytes(digest)
				var evidenceDigest [sha256.Size]byte
				copy(evidenceDigest[:], digest)
				return runtime.dataLifecycle.RevokeSupplierAuthorization(
					ctx, datalifecycle.SupplierAuthorizationRevocation{
						Provider:            strings.TrimSpace(input.ProviderCode),
						BindingNo:           strings.TrimSpace(input.BindingNo),
						EmployeePrincipalNo: strings.TrimSpace(input.EmployeePrincipalNo),
						EvidenceSetSHA256:   evidenceDigest, IdempotencyKey: key,
					},
				)
			}
			return runtime.supplierIdentity.Revoke(ctx, supplieridentity.RevokeInput{
				BindingNo:  strings.TrimSpace(input.BindingNo),
				ReasonCode: strings.TrimSpace(input.ReasonCode), IdempotencyKey: key,
			})
		},
	)
}

func localSupplierSubjectReopeningHandler(runtime *localRuntime) http.HandlerFunc {
	return supplierSubjectWriteHandler(
		runtime, processauth.SupplierSubjectReopeningsPath, processauth.SupplierSubjectReopenScope,
		func(ctx context.Context, body []byte, key []byte) (supplieridentity.Result, error) {
			var input supplierSubjectReopeningRequest
			if err := decodeSupplierSubjectJSON(body, &input); err != nil {
				clearRuntimeBytes(input.AuthorizationResult)
				return supplieridentity.Result{}, err
			}
			defer clearRuntimeBytes(input.AuthorizationResult)
			digest, err := hex.DecodeString(strings.TrimSpace(input.EvidenceSetSHA256))
			if len(input.AuthorizationResult) == 0 || len(input.AuthorizationResult) > 4096 ||
				err != nil || len(digest) != sha256.Size || runtime.dataLifecycle == nil {
				clearRuntimeBytes(input.AuthorizationResult)
				clearRuntimeBytes(digest)
				return supplieridentity.Result{}, datalifecycle.ErrInvalid
			}
			defer clearRuntimeBytes(digest)
			var evidenceDigest [sha256.Size]byte
			copy(evidenceDigest[:], digest)
			result, err := runtime.dataLifecycle.ReopenSupplierAuthorization(
				ctx, datalifecycle.SupplierAuthorizationReopening{
					PreviousBindingNo: strings.TrimSpace(input.PreviousBindingNo),
					Provider:          strings.TrimSpace(input.ProviderCode), Subject: input.AuthorizationResult,
					ReasonCode:          strings.TrimSpace(input.ReasonCode),
					EmployeePrincipalNo: strings.TrimSpace(input.EmployeePrincipalNo),
					EvidenceSetSHA256:   evidenceDigest, IdempotencyKey: key,
				},
			)
			if err != nil {
				return supplieridentity.Result{}, err
			}
			if runtime.supplierUsageCollection == nil {
				return supplieridentity.Result{}, datalifecycle.ErrUnavailable
			}
			if err = runtime.supplierUsageCollection.EnsureReady(ctx); err != nil {
				return supplieridentity.Result{}, datalifecycle.ErrUnavailable
			}
			return result, nil
		},
	)
}

type supplierSubjectOperation func(
	context.Context,
	[]byte,
	[]byte,
) (supplieridentity.Result, error)

func supplierSubjectWriteHandler(
	runtime *localRuntime,
	exactPath string,
	scope string,
	operation supplierSubjectOperation,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != exactPath ||
			request.Header.Get("Content-Type") != "application/json" || runtime == nil ||
			runtime.supplierIdentity == nil || operation == nil {
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
		verified, _, _, err := runtime.authenticateRequest(
			request.Context(), scope, request.Method, request.URL.RequestURI(),
			processauth.PayloadModeJSON, body, credentials, true,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		key := append([]byte(nil), verified.IdempotencyHMAC[:]...)
		result, err := operation(request.Context(), body, key)
		clearRuntimeBytes(key)
		if err != nil {
			writeSupplierIdentityError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, supplierSubjectResponse{
			OperationNo: result.OperationNo, BindingNo: result.BindingNo, Replayed: result.Replayed,
		})
	}
}

func decodeSupplierSubjectJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return supplieridentity.ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return supplieridentity.ErrInvalid
	}
	return nil
}

func writeProcessAdmissionError(writer http.ResponseWriter, err error) {
	status, code := http.StatusUnauthorized, processAuthRejectedCode
	if errors.Is(err, errProcessNonceConflict) || errors.Is(err, errBusinessConflict) {
		status, code = http.StatusConflict, processAuthReplayCode
	}
	writeJSON(writer, status, map[string]string{errorCodeField: code})
}

func writeSupplierIdentityError(writer http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "supplier_identity_unavailable"
	switch {
	case errors.Is(err, datalifecycle.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, datalifecycle.ErrConflict):
		status, code = http.StatusConflict, "supplier_subject_conflict"
	case errors.Is(err, datalifecycle.ErrUnavailable):
		status, code = http.StatusServiceUnavailable, "supplier_identity_unavailable"
	case errors.Is(err, supplieridentity.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, supplieridentity.ErrCanonicalizerOff):
		status, code = http.StatusConflict, "external_confirmation_required"
	case errors.Is(err, supplieridentity.ErrConflict), errors.Is(err, supplieridentity.ErrReopenRequired),
		errors.Is(err, supplieridentity.ErrInstanceRevoked):
		status, code = http.StatusConflict, "supplier_subject_conflict"
	case errors.Is(err, supplieridentity.ErrNotFound):
		status, code = http.StatusNotFound, "supplier_subject_not_found"
	}
	writeJSON(writer, status, map[string]string{errorCodeField: code})
}
