// This file consumes bounded contract PDFs from the loopback-only gateway.
package localgatewayclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const maximumContractArtifactBytes int64 = 32 << 20

// TencentContractArtifact is one verified PDF copied from the local gateway.
type TencentContractArtifact struct {
	Kind               string
	Body               []byte
	SHA256             string
	SupplierObservedAt time.Time
}

// TencentContractArtifactState is a safe non-ready report result.
type TencentContractArtifactState struct {
	State       string
	Code        string
	NextCheckAt time.Time
}

// FetchTencentContractFinalPDF immediately copies one terminal-flow PDF.
func (c *Client) FetchTencentContractFinalPDF(
	ctx context.Context,
	routeNo string,
) (*TencentContractArtifact, error) {
	return c.doTencentContractArtifact(
		ctx, processauth.TencentContractFinalPDFsPath,
		processauth.TencentContractArtifactRequest{RouteNo: routeNo},
		processauth.TencentArtifactKindFinalPDF,
		func(request *http.Request, body []byte) error {
			return c.signer.SignTencentContractFinalPDFRead(request, body)
		},
	)
}

// EnsureTencentContractSignReport creates or queries one buyer-paid report.
func (c *Client) EnsureTencentContractSignReport(
	ctx context.Context,
	routeNo string,
	idempotencyKey string,
) (*TencentContractArtifact, TencentContractArtifactState, error) {
	artifact, state, err := c.doTencentContractReport(
		ctx, processauth.TencentContractArtifactRequest{RouteNo: routeNo}, idempotencyKey,
	)
	return artifact, state, err
}

func (c *Client) doTencentContractReport(
	ctx context.Context,
	input processauth.TencentContractArtifactRequest,
	idempotencyKey string,
) (*TencentContractArtifact, TencentContractArtifactState, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.contractHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || idempotencyKey == "" {
		return nil, TencentContractArtifactState{}, contractArtifactError("local_gateway_unavailable")
	}
	body, err := json.Marshal(input)
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearBytes(body)
		return nil, TencentContractArtifactState{}, contractArtifactError("invalid_contract_artifact_request")
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL.String()+processauth.TencentContractSignReportsPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, TencentContractArtifactState{}, contractArtifactError("local_gateway_unavailable")
	}
	request.Header.Set("Content-Type", "application/json")
	if err = c.signer.SignTencentContractSignReport(request, body, idempotencyKey); err != nil {
		return nil, TencentContractArtifactState{}, contractArtifactError("local_gateway_auth_unavailable")
	}
	response, err := c.contractHTTPClient.Do(request)
	if err != nil {
		return nil, TencentContractArtifactState{}, contractArtifactError("tencent_temporarily_unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK && response.Header.Get("Content-Type") == "application/pdf" {
		artifact, readErr := readTencentArtifact(response, processauth.TencentArtifactKindSignReport)
		return artifact, TencentContractArtifactState{}, readErr
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return nil, TencentContractArtifactState{}, contractArtifactError("tencent_temporarily_unavailable")
	}
	defer clearBytes(content)
	var result processauth.TencentContractArtifactPendingResponse
	if json.Unmarshal(content, &result) != nil || !validArtifactState(result.State, response.StatusCode) ||
		strings.TrimSpace(result.Code) == "" {
		return nil, TencentContractArtifactState{}, contractArtifactError("tencent_result_unknown")
	}
	state := TencentContractArtifactState{State: result.State, Code: result.Code}
	if result.NextCheckUnix > 0 {
		state.NextCheckAt = time.Unix(result.NextCheckUnix, 0).UTC()
	}
	return nil, state, nil
}

func (c *Client) doTencentContractArtifact(
	ctx context.Context,
	path string,
	input any,
	expectedKind string,
	sign tencentContractRequestSigner,
) (*TencentContractArtifact, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.contractHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || sign == nil {
		return nil, contractArtifactError("local_gateway_unavailable")
	}
	body, err := json.Marshal(input)
	if err != nil || len(body) == 0 || len(body) > processauth.MaximumPayloadBytes {
		clearBytes(body)
		return nil, contractArtifactError("invalid_contract_artifact_request")
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL.String()+path, bytes.NewReader(body))
	if err != nil {
		return nil, contractArtifactError("local_gateway_unavailable")
	}
	request.Header.Set("Content-Type", "application/json")
	if err = sign(request, body); err != nil {
		return nil, contractArtifactError("local_gateway_auth_unavailable")
	}
	response, err := c.contractHTTPClient.Do(request)
	if err != nil {
		return nil, contractArtifactError("tencent_temporarily_unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/pdf" {
		return nil, readTencentArtifactFailure(response)
	}
	return readTencentArtifact(response, expectedKind)
}

func readTencentArtifact(response *http.Response, expectedKind string) (*TencentContractArtifact, error) {
	if response == nil || response.Body == nil ||
		response.Header.Get(processauth.TencentArtifactStateHeader) != processauth.TencentArtifactStateReady ||
		response.Header.Get(processauth.TencentArtifactKindHeader) != expectedKind {
		return nil, contractArtifactError("tencent_result_unknown")
	}
	expectedSHA := strings.TrimSpace(response.Header.Get(processauth.TencentArtifactSHA256Header))
	observedUnix, observedErr := strconv.ParseInt(
		response.Header.Get(processauth.TencentArtifactObservedUnixHeader), 10, 64,
	)
	if len(expectedSHA) != sha256.Size*2 || observedErr != nil || observedUnix <= 0 ||
		response.ContentLength < 5 || response.ContentLength > maximumContractArtifactBytes {
		return nil, contractArtifactError("tencent_result_unknown")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumContractArtifactBytes+1))
	if err != nil || int64(len(content)) != response.ContentLength || len(content) < 5 ||
		int64(len(content)) > maximumContractArtifactBytes || string(content[:5]) != "%PDF-" {
		clearBytes(content)
		return nil, contractArtifactError("tencent_result_unknown")
	}
	digest := sha256.Sum256(content)
	actualSHA := hex.EncodeToString(digest[:])
	if actualSHA != expectedSHA {
		clearBytes(content)
		return nil, contractArtifactError("tencent_result_unknown")
	}
	return &TencentContractArtifact{
		Kind: expectedKind, Body: content, SHA256: actualSHA,
		SupplierObservedAt: time.Unix(observedUnix, 0).UTC(),
	}, nil
}

func readTencentArtifactFailure(response *http.Response) error {
	if response == nil || response.Body == nil {
		return contractArtifactError("tencent_temporarily_unavailable")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return contractArtifactError("tencent_temporarily_unavailable")
	}
	defer clearBytes(content)
	var failure struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
		failure.Code = "tencent_temporarily_unavailable"
	}
	return contractArtifactError(strings.TrimSpace(failure.Code))
}

func validArtifactState(state string, status int) bool {
	switch state {
	case processauth.TencentArtifactStatePending:
		return status == http.StatusAccepted
	case processauth.TencentArtifactStateSkipped:
		return status == http.StatusOK
	case processauth.TencentArtifactStateResultUnknown:
		return status == http.StatusConflict
	default:
		return false
	}
}

func contractArtifactError(code string) error {
	return &TencentContractFlowError{Code: code, ResultUnknown: code == "tencent_result_unknown"}
}
