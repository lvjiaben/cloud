// This file owns the signed local-gateway client for Tencent onboarding.

package localgatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// TencentOnboardingCapability is a secret-free runtime capability projection.
type TencentOnboardingCapability struct {
	Code       string `json:"code"`
	State      string `json:"state"`
	ReasonCode string `json:"reasonCode"`
	CheckedAt  int64  `json:"checkedAt"`
}

// TencentOnboardingResult never contains credentials or supplier subject identifiers.
type TencentOnboardingResult struct {
	State             string                        `json:"state"`
	ReasonCode        string                        `json:"reasonCode"`
	ConsoleURL        string                        `json:"consoleUrl"`
	OrganizationReady bool                          `json:"organizationReady"`
	OperatorReady     bool                          `json:"operatorReady"`
	BindingNo         string                        `json:"bindingNo"`
	OperationNo       string                        `json:"operationNo"`
	Capabilities      []TencentOnboardingCapability `json:"capabilities"`
}

// TencentOnboardingStatus reads only installation-local activation state.
func (c *Client) TencentOnboardingStatus(ctx context.Context) (TencentOnboardingResult, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.onboardingHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil {
		return TencentOnboardingResult{}, fmt.Errorf("local compliance gateway client is unavailable")
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.baseURL.String()+processauth.TencentOnboardingPath, nil,
	)
	if err != nil {
		return TencentOnboardingResult{}, fmt.Errorf("create Tencent onboarding status request: %w", err)
	}
	if err = c.signer.SignTencentOnboardingRead(request); err != nil {
		return TencentOnboardingResult{}, err
	}
	return c.doTencentOnboarding(request)
}

// CreateTencentOnboardingLink asks the local gateway for one official one-use link.
func (c *Client) CreateTencentOnboardingLink(
	ctx context.Context,
	legalName string,
	idempotencyKey string,
) (TencentOnboardingResult, error) {
	return c.mutateTencentOnboarding(
		ctx, legalName, idempotencyKey, processauth.TencentOnboardingLinksPath, false,
	)
}

// RecheckTencentOnboarding queries Tencent's authoritative activation flags
// and never exposes the freshly generated one-use URL to the management API.
func (c *Client) RecheckTencentOnboarding(
	ctx context.Context,
	legalName string,
	idempotencyKey string,
) (TencentOnboardingResult, error) {
	return c.mutateTencentOnboarding(
		ctx, legalName, idempotencyKey, processauth.TencentOnboardingRechecksPath, true,
	)
}

func (c *Client) mutateTencentOnboarding(
	ctx context.Context,
	legalName string,
	idempotencyKey string,
	path string,
	recheck bool,
) (TencentOnboardingResult, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.onboardingHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || strings.TrimSpace(legalName) == "" ||
		strings.TrimSpace(idempotencyKey) == "" {
		return TencentOnboardingResult{}, fmt.Errorf("local compliance gateway client is unavailable")
	}
	body, err := json.Marshal(struct {
		LegalName string `json:"legalName"`
	}{LegalName: strings.TrimSpace(legalName)})
	if err != nil {
		return TencentOnboardingResult{}, fmt.Errorf("encode Tencent onboarding request: %w", err)
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL.String()+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return TencentOnboardingResult{}, fmt.Errorf("create Tencent onboarding request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if recheck {
		err = c.signer.SignTencentOnboardingRecheck(
			request, body, processBusinessKey(idempotencyKey),
		)
	} else {
		err = c.signer.SignTencentOnboardingLink(
			request, body, processBusinessKey(idempotencyKey),
		)
	}
	if err != nil {
		return TencentOnboardingResult{}, err
	}
	result, err := c.doTencentOnboarding(request)
	if recheck {
		result.ConsoleURL = ""
	}
	return result, err
}

func (c *Client) doTencentOnboarding(request *http.Request) (TencentOnboardingResult, error) {
	response, err := c.onboardingHTTPClient.Do(request)
	if err != nil {
		return TencentOnboardingResult{}, fmt.Errorf("call local Tencent onboarding gateway: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return TencentOnboardingResult{}, fmt.Errorf("read local Tencent onboarding response")
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			return TencentOnboardingResult{}, fmt.Errorf(
				"local Tencent onboarding gateway rejected request with status %d", response.StatusCode,
			)
		}
		return TencentOnboardingResult{}, fmt.Errorf("local Tencent onboarding gateway rejected request: %s", failure.Code)
	}
	var result TencentOnboardingResult
	if err = json.Unmarshal(content, &result); err != nil || !validTencentOnboardingResult(result) {
		return TencentOnboardingResult{}, fmt.Errorf("decode local Tencent onboarding response")
	}
	return result, nil
}

func validTencentOnboardingResult(result TencentOnboardingResult) bool {
	switch result.State {
	case "not_configured", "ready", "enterprise_pending", "operator_pending", "active":
	default:
		return false
	}
	if result.ConsoleURL != "" {
		parsed, err := url.Parse(result.ConsoleURL)
		if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" ||
			(parsed.Hostname() != "tencent.com" && !strings.HasSuffix(parsed.Hostname(), ".tencent.com")) {
			return false
		}
	}
	return len(result.Capabilities) <= 8
}
