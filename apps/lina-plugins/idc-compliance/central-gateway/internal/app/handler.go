// This file exposes liveness, readiness, the authenticated installation
// projection, and the reviewed supplier-subject route boundary.

package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/buildinfo"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// Internal response codes, path, and scope define the installation read route.
const (
	errorCodeField              = "code"
	localRuntimeUnavailableCode = "local_runtime_unavailable"
	processAuthRejectedCode     = "process_auth_rejected"
	processAuthReplayCode       = "process_auth_replay"
	localInstallationPath       = "/internal/v1/installation"
	localInstallationReadScope  = "installation.read"
)

// healthResponse is the non-secret operational health projection.
type healthResponse struct {
	Status string         `json:"status"`
	Code   string         `json:"code,omitempty"`
	Build  buildinfo.Info `json:"build"`
}

// installationResponse exposes only the safe local installation state.
type installationResponse struct {
	Scope                  string `json:"scope"`
	ActiveKeyID            string `json:"activeKeyId"`
	SupplierClientsEnabled bool   `json:"supplierClientsEnabled"`
	BusinessRoutesEnabled  bool   `json:"businessRoutesEnabled"`
}

// handlerConfig carries the single startup-owned optional runtime.
type handlerConfig struct {
	runtime *localRuntime
}

// HandlerOption installs one reviewed optional route boundary.
type HandlerOption func(*handlerConfig) error

// WithLocalRuntime installs the current installation's database and process identity.
func WithLocalRuntime(runtime *localRuntime) HandlerOption {
	return func(config *handlerConfig) error {
		if runtime == nil || runtime.database == nil || runtime.keyring == nil ||
			runtime.installationID == "" || runtime.processAuthWindow <= 0 ||
			runtime.processAuthWindow > processauth.MaximumAuthenticationWindow ||
			config.runtime != nil {
			return errors.New("local runtime is unavailable or duplicated")
		}
		config.runtime = runtime
		return nil
	}
}

// NewHandler returns operational routes and the authenticated local boundary.
func NewHandler(info buildinfo.Info, options ...HandlerOption) http.Handler {
	config := handlerConfig{}
	for _, option := range options {
		if option == nil || option(&config) != nil {
			return unavailableHandler()
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", methodGET(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, healthResponse{Status: "alive", Build: info})
	}))
	mux.HandleFunc("/health/ready", methodGET(func(writer http.ResponseWriter, request *http.Request) {
		if config.runtime == nil || !config.runtime.Ready(request.Context()) {
			writeJSON(writer, http.StatusServiceUnavailable, healthResponse{
				Status: "blocked", Code: localRuntimeUnavailableCode, Build: info,
			})
			return
		}
		writeJSON(writer, http.StatusOK, healthResponse{Status: "ready", Build: info})
	}))
	if config.runtime != nil {
		mux.HandleFunc(localInstallationPath, methodGET(localInstallationHandler(config.runtime)))
		mux.HandleFunc(
			processauth.SupplierSubjectBindingsPath,
			methodPOST(localSupplierSubjectBindingHandler(config.runtime)),
		)
		mux.HandleFunc(
			processauth.SupplierSubjectRevocationsPath,
			methodPOST(localSupplierSubjectRevocationHandler(config.runtime)),
		)
		mux.HandleFunc(
			processauth.SupplierSubjectReopeningsPath,
			methodPOST(localSupplierSubjectReopeningHandler(config.runtime)),
		)
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		setSecurityHeaders(writer)
		mux.ServeHTTP(writer, request)
	})
}

// localInstallationHandler authenticates the exact installation read route.
func localInstallationHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != localInstallationPath || !requestBodyIsEmpty(request) {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		_, _, err = runtime.authenticateRead(
			request.Context(), localInstallationReadScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeEmpty, nil, credentials,
		)
		if err != nil {
			code := processAuthRejectedCode
			status := http.StatusUnauthorized
			if errors.Is(err, errProcessNonceConflict) {
				code = processAuthReplayCode
				status = http.StatusConflict
			}
			writeJSON(writer, status, map[string]string{errorCodeField: code})
			return
		}
		writeJSON(writer, http.StatusOK, installationResponse{
			Scope: "local_installation", ActiveKeyID: runtime.keyring.ActiveKeyID(),
			SupplierClientsEnabled: false,
			BusinessRoutesEnabled:  true,
		})
	}
}

// requestBodyIsEmpty proves the route's empty payload instead of trusting length metadata.
func requestBodyIsEmpty(request *http.Request) bool {
	if request == nil || request.ContentLength > 0 {
		return false
	}
	if request.Body == nil || request.Body == http.NoBody {
		return true
	}
	var probe [1]byte
	read, err := request.Body.Read(probe[:])
	return read == 0 && errors.Is(err, io.EOF)
}

// credentialsFromRequest parses one canonical value for every authentication header.
func credentialsFromRequest(request *http.Request) (processauth.Credentials, error) {
	timestampValue, err := singleAuthenticationHeader(request, processauth.HeaderTimestampMS)
	if err != nil {
		return processauth.Credentials{}, processauth.ErrInvalid
	}
	timestamp, err := strconv.ParseInt(timestampValue, 10, 64)
	if err != nil {
		return processauth.Credentials{}, processauth.ErrInvalid
	}
	expiresValue, err := singleAuthenticationHeader(request, processauth.HeaderExpiresMS)
	if err != nil {
		return processauth.Credentials{}, processauth.ErrInvalid
	}
	expires, err := strconv.ParseInt(expiresValue, 10, 64)
	if err != nil {
		return processauth.Credentials{}, processauth.ErrInvalid
	}
	credentials := processauth.Credentials{
		TimestampMS: timestamp, ExpiresMS: expires,
	}
	for _, item := range []struct {
		name   string
		target *string
	}{
		{name: processauth.HeaderProtocol, target: &credentials.Protocol},
		{name: processauth.HeaderInstallationID, target: &credentials.InstallationID},
		{name: processauth.HeaderOperationScope, target: &credentials.OperationScope},
		{name: processauth.HeaderKeyID, target: &credentials.KeyID},
		{name: processauth.HeaderNonce, target: &credentials.Nonce},
		{name: processauth.HeaderBusinessIdempotency, target: &credentials.BusinessIdempotencyKey},
		{name: processauth.HeaderContentSHA, target: &credentials.ContentSHA256},
		{name: processauth.HeaderBusinessContentSHA, target: &credentials.BusinessContentSHA256},
		{name: processauth.HeaderSignature, target: &credentials.Signature},
	} {
		*item.target, err = singleAuthenticationHeader(request, item.name)
		if err != nil {
			return processauth.Credentials{}, processauth.ErrInvalid
		}
	}
	return credentials, nil
}

// singleAuthenticationHeader rejects missing, repeated, joined, or unsafe values.
func singleAuthenticationHeader(request *http.Request, name string) (string, error) {
	values := request.Header.Values(name)
	if len(values) != 1 || values[0] == "" || strings.ContainsAny(values[0], "\r\n,") {
		return "", processauth.ErrInvalid
	}
	return values[0], nil
}

// unavailableHandler fails closed when startup runtime assembly is invalid.
func unavailableHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		setSecurityHeaders(writer)
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: localRuntimeUnavailableCode})
	})
}

// methodGET applies the read-only method boundary to one handler.
func methodGET(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{errorCodeField: "method_not_allowed"})
			return
		}
		next(writer, request)
	}
}

func methodPOST(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{errorCodeField: "method_not_allowed"})
			return
		}
		next(writer, request)
	}
}

// setSecurityHeaders prevents caching or embedding internal responses.
func setSecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
}

// writeJSON emits one bounded JSON response with a terminating newline.
func writeJSON(writer http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		http.Error(writer, "response_encoding_failed", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	payload = append(payload, '\n')
	if _, err := writer.Write(payload); err != nil {
		return
	}
}
