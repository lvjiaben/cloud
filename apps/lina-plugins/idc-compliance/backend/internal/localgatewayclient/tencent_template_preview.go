// This file owns the signed local-gateway read for one Tencent template preview.

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

// TencentTemplatePreview obtains one official short-lived H5 preview URL.
func (c *Client) TencentTemplatePreview(ctx context.Context, templateRef string) (string, error) {
	templateRef = strings.TrimSpace(templateRef)
	if c == nil || c.baseURL == nil || c.signer == nil || c.onboardingHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil || templateRef == "" || len(templateRef) > 256 {
		return "", fmt.Errorf("local compliance gateway client is unavailable")
	}
	body, err := json.Marshal(struct {
		TemplateRef string `json:"templateRef"`
	}{TemplateRef: templateRef})
	if err != nil {
		return "", fmt.Errorf("encode Tencent template preview request: %w", err)
	}
	defer clearBytes(body)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL.String()+processauth.TencentTemplatePreviewsPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("create Tencent template preview request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if err = c.signer.SignTencentTemplatePreviewRead(request, body); err != nil {
		return "", err
	}
	response, err := c.onboardingHTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call local Tencent template preview gateway: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return "", fmt.Errorf("read local Tencent template preview response")
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			return "", fmt.Errorf(
				"local Tencent template preview gateway rejected request with status %d",
				response.StatusCode,
			)
		}
		return "", fmt.Errorf(
			"local Tencent template preview gateway rejected request: %s",
			strings.TrimSpace(failure.Code),
		)
	}
	var result struct {
		PreviewURL string `json:"previewUrl"`
	}
	if json.Unmarshal(content, &result) != nil || !safeTencentPreviewURL(result.PreviewURL) {
		return "", fmt.Errorf("decode local Tencent template preview response")
	}
	return strings.TrimSpace(result.PreviewURL), nil
}

func safeTencentPreviewURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.Fragment == "" && !strings.ContainsAny(parsed.Host, "\r\n\x00")
}
