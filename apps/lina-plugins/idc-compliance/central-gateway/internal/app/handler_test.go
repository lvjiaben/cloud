// This file verifies the local gateway's operational and authenticated HTTP
// route boundaries.

package app

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"lina-platform/idc-compliance-gateway/internal/buildinfo"
	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

func TestOperationalHandlerFailsReadinessWithoutLocalRuntime(t *testing.T) {
	handler := NewHandler(buildinfo.Info{Service: "gateway", Version: "test"})
	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/health/live", wantStatus: http.StatusOK, wantBody: `"status":"alive"`},
		{path: "/health/ready", wantStatus: http.StatusServiceUnavailable, wantBody: localRuntimeUnavailableCode},
		{path: "/v1/contracts", wantStatus: http.StatusNotFound, wantBody: "404 page not found"},
		{path: localInstallationPath, wantStatus: http.StatusNotFound, wantBody: "404 page not found"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
			t.Fatalf("path %s got status=%d body=%q", test.path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" ||
			response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("path %s is missing security headers", test.path)
		}
	}
}

func TestOperationalHandlerRejectsWrites(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/health/live", strings.NewReader("ignored"))
	response := httptest.NewRecorder()
	NewHandler(buildinfo.Current()).ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("unexpected response: status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}
}



func TestInstallationRouteUsesFixedScopeAndReadNonceFence(t *testing.T) {
	runtime, mock := newAdmissionTestRuntime(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	credentials := signAdmissionTestFrame(
		t, runtime, localInstallationReadScope, http.MethodGet, localInstallationPath,
		processauth.PayloadModeEmpty, nil, admissionBusinessKey(t, 41), now, 42,
	)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT CURRENT_TIMESTAMP`).WillReturnRows(
		sqlmock.NewRows([]string{"current_timestamp"}).AddRow(now),
	)
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_process_nonce`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	request := httptest.NewRequest(http.MethodGet, localInstallationPath, nil)
	setAuthenticationHeaders(request, credentials)
	response := httptest.NewRecorder()
	NewHandler(buildinfo.Info{Service: "gateway", Version: "test"}, WithLocalRuntime(runtime)).
		ServeHTTP(response, request)
	if response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"supplierClientsEnabled":false`) ||
		!strings.Contains(response.Body.String(), `"businessRoutesEnabled":true`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	assertAdmissionExpectations(t, runtime, mock)
}




func TestInstallationRouteRejectsQueryAndUnsignedBodyBeforeDatabase(t *testing.T) {
	runtime, mock := newAdmissionTestRuntime(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	tests := []struct {
		name        string
		target      string
		requestBody string
	}{
		{name: "query", target: localInstallationPath + "?detail=safe"},
		{name: "body", target: localInstallationPath, requestBody: `{}`},
	}
	for index, test := range tests {
		credentials := signAdmissionTestFrame(
			t, runtime, localInstallationReadScope, http.MethodGet, test.target,
			processauth.PayloadModeEmpty, nil, admissionBusinessKey(t, byte(50+index)),
			now, byte(60+index),
		)
		request := httptest.NewRequest(
			http.MethodGet, test.target, strings.NewReader(test.requestBody),
		)
		setAuthenticationHeaders(request, credentials)
		response := httptest.NewRecorder()
		NewHandler(buildinfo.Info{Service: "gateway", Version: "test"}, WithLocalRuntime(runtime)).
			ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized ||
			!strings.Contains(response.Body.String(), processAuthRejectedCode) {
			t.Fatalf("%s status=%d body=%s", test.name, response.Code, response.Body.String())
		}
	}
	assertAdmissionExpectations(t, runtime, mock)
}

func TestSupplierSubjectBindingFailsClosedWithoutReviewedRecognizer(t *testing.T) {
	runtime, mock := newAdmissionTestRuntime(t)
	attachDisabledSupplierIdentity(t, runtime)
	now := time.Unix(1_800_000_000, 0).UTC()
	body := []byte(`{"providerCode":"tencent_essbasic","authorizationResult":"` +
		base64.StdEncoding.EncodeToString([]byte("opaque-provider-result")) + `"}`)
	credentials := signAdmissionTestFrame(
		t, runtime, processauth.SupplierSubjectBindScope, http.MethodPost,
		processauth.SupplierSubjectBindingsPath, processauth.PayloadModeJSON, body,
		admissionBusinessKey(t, 71), now, 72,
	)
	expectBusinessInsert(mock, 1)
	expectNonceInsert(mock, credentials.KeyID, credentialsExpiry(credentials))
	mock.ExpectCommit()
	request := httptest.NewRequest(http.MethodPost, processauth.SupplierSubjectBindingsPath, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	setAuthenticationHeaders(request, credentials)
	response := httptest.NewRecorder()
	NewHandler(buildinfo.Info{Service: "gateway", Version: "test"}, WithLocalRuntime(runtime)).
		ServeHTTP(response, request)
	if response.Code != http.StatusConflict ||
		!strings.Contains(response.Body.String(), "external_confirmation_required") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	assertAdmissionExpectations(t, runtime, mock)
}

func attachDisabledSupplierIdentity(t *testing.T, runtime *localRuntime) {
	t.Helper()
	file, err := supplieridentity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate supplier identity keyring: %v", err)
	}
	ring, err := supplieridentity.New(file)
	if err != nil {
		t.Fatalf("new supplier identity keyring: %v", err)
	}
	t.Cleanup(ring.Clear)
	service, err := supplieridentity.NewService(
		runtime.database, runtime.installationID, ring, nil,
	)
	if err != nil {
		t.Fatalf("new supplier identity service: %v", err)
	}
	runtime.supplierIdentityKeyring = ring
	runtime.supplierIdentity = service
}

func TestCredentialsRejectDuplicateAuthenticationHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, localInstallationPath, nil)
	request.Header.Add(processauth.HeaderTimestampMS, "1")
	request.Header.Add(processauth.HeaderTimestampMS, "2")
	if _, err := credentialsFromRequest(request); err == nil {
		t.Fatal("duplicate process-auth header was accepted")
	}
}

func setAuthenticationHeaders(request *http.Request, credentials processauth.Credentials) {
	request.Header.Set(processauth.HeaderProtocol, credentials.Protocol)
	request.Header.Set(processauth.HeaderInstallationID, credentials.InstallationID)
	request.Header.Set(processauth.HeaderOperationScope, credentials.OperationScope)
	request.Header.Set(processauth.HeaderKeyID, credentials.KeyID)
	request.Header.Set(processauth.HeaderTimestampMS, strconv.FormatInt(credentials.TimestampMS, 10))
	request.Header.Set(processauth.HeaderExpiresMS, strconv.FormatInt(credentials.ExpiresMS, 10))
	request.Header.Set(processauth.HeaderNonce, credentials.Nonce)
	request.Header.Set(processauth.HeaderBusinessIdempotency, credentials.BusinessIdempotencyKey)
	request.Header.Set(processauth.HeaderContentSHA, credentials.ContentSHA256)
	request.Header.Set(processauth.HeaderBusinessContentSHA, credentials.BusinessContentSHA256)
	request.Header.Set(processauth.HeaderSignature, credentials.Signature)
}
