// Package localgatewayclient owns the loopback-only HTTP seam from the source
// plugin to the installation-local compliance gateway.
package localgatewayclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/localgatewayauth"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

const (
	// BaseURLEnvironment names the loopback-only gateway address setting.
	BaseURLEnvironment  = "LINAPRO_COMPLIANCE_GATEWAY_INTERNAL_BASE_URL"
	defaultBaseURL      = "http://127.0.0.1:9088"
	requestTimeout      = 5 * time.Second
	onboardingTimeout   = 45 * time.Second
	contractFlowTimeout = 35 * time.Second
	maximumResponse     = 16 << 10
)

// Client sends signed supplier-subject writes to the colocated gateway process.
type Client struct {
	baseURL              *url.URL
	httpClient           *http.Client
	onboardingHTTPClient *http.Client
	contractHTTPClient   *http.Client
	signer               *localgatewayauth.Signer
}

var _ supplierconfig.SubjectGateway = (*Client)(nil)

// LoadFromEnvironment builds one process-lifetime client from private local settings.
func LoadFromEnvironment(getenv func(string) string) (*Client, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	signer, err := localgatewayauth.LoadFromEnvironment(getenv)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(getenv(BaseURLEnvironment))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client, err := New(baseURL, signer, &http.Client{Timeout: requestTimeout})
	if err != nil {
		signer.Close()
		return nil, err
	}
	return client, nil
}

// New validates the fixed loopback endpoint and explicit runtime dependencies.
func New(baseURL string, signer *localgatewayauth.Signer, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != "http" || parsed.Host != "127.0.0.1:9088" ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		signer == nil || httpClient == nil || httpClient.Timeout <= 0 ||
		httpClient.Timeout > requestTimeout {
		return nil, fmt.Errorf("local compliance gateway client configuration is invalid")
	}
	onboardingHTTPClient := *httpClient
	onboardingHTTPClient.Timeout = onboardingTimeout
	contractHTTPClient := *httpClient
	contractHTTPClient.Timeout = contractFlowTimeout
	return &Client{
		baseURL: parsed, signer: signer, httpClient: httpClient,
		onboardingHTTPClient: &onboardingHTTPClient, contractHTTPClient: &contractHTTPClient,
	}, nil
}

// RecognizerAvailable reports the one production recognizer implemented from
// Tencent's public contract. NuoNuo remains closed behind its separate gate.
func (c *Client) RecognizerAvailable(provider string) bool {
	return c != nil && provider == supplierconfig.ProviderTencentESSBasic
}

// Bind sends one provider authorization result to the local binding route.
func (c *Client) Bind(
	ctx context.Context,
	provider string,
	authorizationResult []byte,
	idempotencyKey string,
) (supplierconfig.GatewayResult, error) {
	owned := append([]byte(nil), authorizationResult...)
	defer clearBytes(owned)
	return c.post(ctx, processauth.SupplierSubjectBindingsPath, struct {
		ProviderCode        string `json:"providerCode"`
		AuthorizationResult []byte `json:"authorizationResult"`
	}{provider, owned}, idempotencyKey)
}

// Revoke sends one local opaque binding to the local revocation route.
func (c *Client) Revoke(
	ctx context.Context,
	provider string,
	bindingNo string,
	reasonCode string,
	employeePrincipalNo string,
	evidenceSetSHA256 string,
	idempotencyKey string,
) (supplierconfig.GatewayResult, error) {
	return c.post(ctx, processauth.SupplierSubjectRevocationsPath, map[string]string{
		"providerCode": provider, "bindingNo": bindingNo, "reasonCode": reasonCode,
		"employeePrincipalNo": employeePrincipalNo, "evidenceSetSha256": evidenceSetSHA256,
	}, idempotencyKey)
}

// Reopen sends one provider authorization result to the local reopening route.
func (c *Client) Reopen(
	ctx context.Context,
	provider string,
	previousBindingNo string,
	reasonCode string,
	authorizationResult []byte,
	employeePrincipalNo string,
	evidenceSetSHA256 string,
	idempotencyKey string,
) (supplierconfig.GatewayResult, error) {
	owned := append([]byte(nil), authorizationResult...)
	defer clearBytes(owned)
	return c.post(ctx, processauth.SupplierSubjectReopeningsPath, struct {
		ProviderCode        string `json:"providerCode"`
		PreviousBindingNo   string `json:"previousBindingNo"`
		ReasonCode          string `json:"reasonCode"`
		AuthorizationResult []byte `json:"authorizationResult"`
		EmployeePrincipalNo string `json:"employeePrincipalNo"`
		EvidenceSetSHA256   string `json:"evidenceSetSha256"`
	}{provider, previousBindingNo, reasonCode, owned, employeePrincipalNo, evidenceSetSHA256}, idempotencyKey)
}

func (c *Client) post(
	ctx context.Context,
	path string,
	payload any,
	idempotencyKey string,
) (supplierconfig.GatewayResult, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.httpClient == nil ||
		ctx == nil || ctx.Err() != nil {
		return supplierconfig.GatewayResult{}, fmt.Errorf("local compliance gateway client is unavailable")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return supplierconfig.GatewayResult{}, fmt.Errorf("encode local compliance gateway request: %w", err)
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL.String()+path, bytes.NewReader(body),
	)
	if err != nil {
		return supplierconfig.GatewayResult{}, fmt.Errorf("create local compliance gateway request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if err = c.signer.SignSupplierSubjectWrite(
		request, body, processBusinessKey(idempotencyKey),
	); err != nil {
		return supplierconfig.GatewayResult{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return supplierconfig.GatewayResult{}, fmt.Errorf("call local compliance gateway: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return supplierconfig.GatewayResult{}, fmt.Errorf("read local compliance gateway response")
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		if err = json.Unmarshal(content, &failure); err != nil {
			return supplierconfig.GatewayResult{}, fmt.Errorf(
				"local compliance gateway rejected operation with status %d",
				response.StatusCode,
			)
		}
		return supplierconfig.GatewayResult{}, fmt.Errorf(
			"local compliance gateway rejected operation: %s", strings.TrimSpace(failure.Code),
		)
	}
	var result supplierconfig.GatewayResult
	if err = json.Unmarshal(content, &result); err != nil {
		return supplierconfig.GatewayResult{}, fmt.Errorf("decode local compliance gateway response")
	}
	return result, nil
}

func processBusinessKey(value string) string {
	digest := sha256.Sum256([]byte("idc-compliance/local-gateway/v1\x00" + value))
	return "lbi1_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

// Close clears copied process-auth key material and rejects later use.
func (c *Client) Close() {
	if c != nil && c.signer != nil {
		c.signer.Close()
		c.signer = nil
	}
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
