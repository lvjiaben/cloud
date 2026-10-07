// This file calls exact signed local-gateway routes for Tencent contract flows and read-side actions.

package localgatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

var localWriteRoutePattern = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)

// TencentContractFlowError is a secret-free local/provider failure.
type TencentContractFlowError struct {
	Code          string
	ResultUnknown bool
}

func (e *TencentContractFlowError) Error() string {
	if e == nil || e.Code == "" {
		return "tencent_contract_flow_unavailable"
	}
	return e.Code
}

// IsResultUnknown reports that Tencent may have accepted the write.
func (e *TencentContractFlowError) IsResultUnknown() bool {
	return e != nil && e.ResultUnknown
}

// CreateTencentContractFlow submits one already-frozen flow request.
func (c *Client) CreateTencentContractFlow(
	ctx context.Context,
	input processauth.TencentContractFlowRequest,
	idempotencyKey string,
) (processauth.TencentContractFlowResponse, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.contractHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || strings.TrimSpace(idempotencyKey) == "" {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "local_gateway_unavailable",
		}
	}
	body, err := json.Marshal(input)
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearBytes(body)
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "invalid_contract_flow_request",
		}
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL.String()+processauth.TencentContractFlowsPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "local_gateway_unavailable",
		}
	}
	request.Header.Set("Content-Type", "application/json")
	if err = c.signer.SignTencentContractFlowCreate(
		request, body, processBusinessKey(idempotencyKey),
	); err != nil {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "local_gateway_auth_unavailable",
		}
	}
	response, err := c.contractHTTPClient.Do(request)
	if err != nil {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "tencent_result_unknown", ResultUnknown: true,
		}
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "tencent_result_unknown", ResultUnknown: true,
		}
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code    string `json:"code"`
			RouteNo string `json:"routeNo"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
				Code: "tencent_result_unknown", ResultUnknown: true,
			}
		}
		code := strings.TrimSpace(failure.Code)
		result := processauth.TencentContractFlowResponse{}
		if localWriteRoutePattern.MatchString(failure.RouteNo) {
			result.RouteNo = failure.RouteNo
		}
		return result, &TencentContractFlowError{
			Code: code, ResultUnknown: code == "tencent_result_unknown",
		}
	}
	var result processauth.TencentContractFlowResponse
	if err = json.Unmarshal(content, &result); err != nil || !localWriteRoutePattern.MatchString(result.RouteNo) ||
		result.Status != "signing" {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "tencent_result_unknown", ResultUnknown: true,
		}
	}
	return result, nil
}

// CreateTencentContractTermination submits one frozen release agreement.
func (c *Client) CreateTencentContractTermination(
	ctx context.Context,
	input processauth.TencentContractTerminationRequest,
	idempotencyKey string,
) (processauth.TencentContractFlowResponse, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.contractHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || strings.TrimSpace(idempotencyKey) == "" {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{Code: "local_gateway_unavailable"}
	}
	body, err := json.Marshal(input)
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearBytes(body)
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{Code: "invalid_contract_flow_request"}
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL.String()+processauth.TencentContractTerminationsPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{Code: "local_gateway_unavailable"}
	}
	request.Header.Set("Content-Type", "application/json")
	if err = c.signer.SignTencentContractTerminationCreate(
		request, body, processBusinessKey(idempotencyKey),
	); err != nil {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{Code: "local_gateway_auth_unavailable"}
	}
	response, err := c.contractHTTPClient.Do(request)
	if err != nil {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "tencent_result_unknown", ResultUnknown: true,
		}
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "tencent_result_unknown", ResultUnknown: true,
		}
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code    string `json:"code"`
			RouteNo string `json:"routeNo"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
				Code: "tencent_result_unknown", ResultUnknown: true,
			}
		}
		code := strings.TrimSpace(failure.Code)
		result := processauth.TencentContractFlowResponse{}
		if localWriteRoutePattern.MatchString(failure.RouteNo) {
			result.RouteNo = failure.RouteNo
		}
		return result, &TencentContractFlowError{Code: code, ResultUnknown: code == "tencent_result_unknown"}
	}
	var result processauth.TencentContractFlowResponse
	if json.Unmarshal(content, &result) != nil || !localWriteRoutePattern.MatchString(result.RouteNo) ||
		result.Status != "signing" {
		return processauth.TencentContractFlowResponse{}, &TencentContractFlowError{
			Code: "tencent_result_unknown", ResultUnknown: true,
		}
	}
	return result, nil
}

// CreateTencentContractSigningLink creates one fresh link for an existing flow.
func (c *Client) CreateTencentContractSigningLink(
	ctx context.Context,
	input processauth.TencentContractSigningLinkRequest,
) (processauth.TencentContractSigningLinkResponse, error) {
	var result processauth.TencentContractSigningLinkResponse
	err := c.doTencentContractRead(
		ctx, processauth.TencentContractSigningLinksPath, input, &result,
		func(request *http.Request, body []byte) error {
			return c.signer.SignTencentContractSigningLinkCreate(request, body)
		},
	)
	if err != nil {
		return processauth.TencentContractSigningLinkResponse{}, err
	}
	if !strings.HasPrefix(result.URL, "https://") || result.ExpiresUnix <= 0 {
		return processauth.TencentContractSigningLinkResponse{}, &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
	}
	return result, nil
}

// RecheckTencentContractFlow queries one existing flow through the local gateway.
func (c *Client) RecheckTencentContractFlow(
	ctx context.Context,
	input processauth.TencentContractFlowRecheckRequest,
) (processauth.TencentContractFlowRecheckResponse, error) {
	var result processauth.TencentContractFlowRecheckResponse
	err := c.doTencentContractRead(
		ctx, processauth.TencentContractFlowRechecksPath, input, &result,
		func(request *http.Request, body []byte) error {
			return c.signer.SignTencentContractFlowRecheck(request, body)
		},
	)
	if err != nil {
		return processauth.TencentContractFlowRecheckResponse{}, err
	}
	if !localWriteRoutePattern.MatchString(result.RouteNo) || result.RouteVersion < 1 ||
		(result.Status != "running" && result.Status != "succeeded" && result.Status != "failed" && result.Status != "result_unknown") ||
		!validTencentContractFlowProjection(result) {
		return processauth.TencentContractFlowRecheckResponse{}, &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
	}
	return result, nil
}

func validTencentContractFlowProjection(result processauth.TencentContractFlowRecheckResponse) bool {
	if result.DecisionCode == "query_failed" {
		return result.SupplierObservedUnix == 0 &&
			(result.ProviderStatus == "" || validTencentContractProviderStatus(result.ProviderStatus, result.Status))
	}
	if result.DecisionCode != "applied" && result.DecisionCode != "stale" && result.DecisionCode != "conflict" {
		return false
	}
	return result.SupplierObservedUnix > 0 &&
		validTencentContractProviderStatus(result.ProviderStatus, result.Status)
}

func validTencentContractProviderStatus(providerStatus string, projectedStatus string) bool {
	switch providerStatus {
	case "INIT", "PART", "WILLEXPIRE":
		return projectedStatus == "running" || projectedStatus == "succeeded" || projectedStatus == "failed"
	case "ALL", "RELIEVED":
		return projectedStatus == "succeeded" || projectedStatus == "failed"
	case "REJECT", "DEADLINE", "CANCEL", "INVALID", "EXCEPTION":
		return projectedStatus == "failed" || projectedStatus == "succeeded"
	default:
		return false
	}
}

type tencentContractRequestSigner func(*http.Request, []byte) error

func (c *Client) doTencentContractRead(
	ctx context.Context,
	path string,
	input any,
	destination any,
	sign tencentContractRequestSigner,
) error {
	if c == nil || c.baseURL == nil || c.signer == nil || c.contractHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || sign == nil {
		return &TencentContractFlowError{Code: "local_gateway_unavailable"}
	}
	body, err := json.Marshal(input)
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearBytes(body)
		return &TencentContractFlowError{Code: "invalid_contract_flow_request"}
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL.String()+path, bytes.NewReader(body))
	if err != nil {
		return &TencentContractFlowError{Code: "local_gateway_unavailable"}
	}
	request.Header.Set("Content-Type", "application/json")
	if err = sign(request, body); err != nil {
		return &TencentContractFlowError{Code: "local_gateway_auth_unavailable"}
	}
	response, err := c.contractHTTPClient.Do(request)
	if err != nil {
		return &TencentContractFlowError{Code: "tencent_temporarily_unavailable"}
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return &TencentContractFlowError{Code: "tencent_temporarily_unavailable"}
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			failure.Code = "tencent_temporarily_unavailable"
		}
		return &TencentContractFlowError{Code: strings.TrimSpace(failure.Code)}
	}
	if json.Unmarshal(content, destination) != nil {
		return &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
	}
	return nil
}
