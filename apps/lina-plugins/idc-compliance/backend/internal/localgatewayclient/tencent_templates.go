// This file owns the signed local-gateway client for explicit Tencent template synchronization.

package localgatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const maximumTemplateResponse = 16 << 20

// TencentTemplate is the supplier-safe template metadata returned by the local gateway.
type TencentTemplate struct {
	TemplateRef            string                     `json:"templateRef"`
	Name                   string                     `json:"name"`
	Version                string                     `json:"version"`
	ProviderState          string                     `json:"providerState"`
	ProviderTimeMS         int64                      `json:"providerTimeMs"`
	CustomerSignatureCount int                        `json:"customerSignatureCount"`
	CustomerSealCount      int                        `json:"customerSealCount"`
	MerchantSignatureCount int                        `json:"merchantSignatureCount"`
	MerchantSealCount      int                        `json:"merchantSealCount"`
	Components             []TencentTemplateComponent `json:"components"`
	Recipients             []TencentTemplateRecipient `json:"recipients"`
}

// TencentTemplateComponent is one safe provider fill or signing-date position.
type TencentTemplateComponent struct {
	ComponentRef string `json:"componentRef"`
	Name         string `json:"name"`
	TypeCode     string `json:"typeCode"`
	Required     bool   `json:"required"`
	RecipientRef string `json:"recipientRef"`
}

// TencentTemplateRecipient is one safe provider signing-role projection.
type TencentTemplateRecipient struct {
	RecipientRef       string `json:"recipientRef"`
	SequenceNo         int    `json:"sequenceNo"`
	TypeCode           string `json:"typeCode"`
	RoleName           string `json:"roleName"`
	RequireSign        bool   `json:"requireSign"`
	RoutingOrder       *int64 `json:"routingOrder"`
	IsPromoter         bool   `json:"isPromoter"`
	SignComponentCount int    `json:"signComponentCount"`
}

// TencentTemplateList is one complete provider snapshot.
type TencentTemplateList struct {
	Items    []TencentTemplate `json:"items"`
	SyncedAt int64             `json:"syncedAt"`
}

// TencentTemplates performs one explicit, signed read through the installation-local gateway.
func (c *Client) TencentTemplates(ctx context.Context) (TencentTemplateList, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.onboardingHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil {
		return TencentTemplateList{}, fmt.Errorf("local compliance gateway client is unavailable")
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.baseURL.String()+processauth.TencentTemplatesPath, nil,
	)
	if err != nil {
		return TencentTemplateList{}, fmt.Errorf("create Tencent templates request: %w", err)
	}
	if err = c.signer.SignTencentTemplatesRead(request); err != nil {
		return TencentTemplateList{}, err
	}
	response, err := c.onboardingHTTPClient.Do(request)
	if err != nil {
		return TencentTemplateList{}, fmt.Errorf("call local Tencent templates gateway: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumTemplateResponse+1))
	if err != nil || len(content) > maximumTemplateResponse {
		clearBytes(content)
		return TencentTemplateList{}, fmt.Errorf("read local Tencent templates response")
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			return TencentTemplateList{}, fmt.Errorf(
				"local Tencent templates gateway rejected request with status %d", response.StatusCode,
			)
		}
		return TencentTemplateList{}, fmt.Errorf("local Tencent templates gateway rejected request: %s", failure.Code)
	}
	var result TencentTemplateList
	if err = json.Unmarshal(content, &result); err != nil || !validTencentTemplateList(result) {
		return TencentTemplateList{}, fmt.Errorf("decode local Tencent templates response")
	}
	return result, nil
}

func validTencentTemplateList(result TencentTemplateList) bool {
	if result.SyncedAt <= 0 || len(result.Items) > 20000 {
		return false
	}
	seen := make(map[string]struct{}, len(result.Items))
	for _, item := range result.Items {
		if strings.TrimSpace(item.TemplateRef) == "" || len(item.TemplateRef) > 256 ||
			strings.TrimSpace(item.Name) == "" || len(item.Name) > 200 || len(item.Version) > 32 ||
			(item.ProviderState != "active" && item.ProviderState != "disabled") || item.ProviderTimeMS < 0 ||
			item.CustomerSignatureCount < 0 || item.CustomerSealCount < 0 ||
			item.MerchantSignatureCount < 0 || item.MerchantSealCount < 0 {
			return false
		}
		if len(item.Components) > 512 || len(item.Recipients) > 64 ||
			!validTencentTemplateComponents(item.Components, item.Recipients) {
			return false
		}
		if _, exists := seen[item.TemplateRef]; exists {
			return false
		}
		seen[item.TemplateRef] = struct{}{}
	}
	return true
}

func validTencentTemplateComponents(
	components []TencentTemplateComponent,
	recipients []TencentTemplateRecipient,
) bool {
	recipientRefs := make(map[string]struct{}, len(recipients))
	sequenceNos := make(map[int]struct{}, len(recipients))
	for _, recipient := range recipients {
		if !validProjectionText(recipient.RecipientRef, 128) ||
			!validProjectionText(recipient.RoleName, 100) ||
			(recipient.TypeCode != "ENTERPRISE" && recipient.TypeCode != "INDIVIDUAL" &&
				recipient.TypeCode != "PROMOTER") || recipient.SequenceNo < 1 ||
			recipient.SequenceNo > len(recipients) || recipient.SignComponentCount < 0 ||
			recipient.RoutingOrder != nil && (*recipient.RoutingOrder < 0 || *recipient.RoutingOrder > 1000000) {
			return false
		}
		if _, exists := recipientRefs[recipient.RecipientRef]; exists {
			return false
		}
		recipientRefs[recipient.RecipientRef] = struct{}{}
		if _, exists := sequenceNos[recipient.SequenceNo]; exists {
			return false
		}
		sequenceNos[recipient.SequenceNo] = struct{}{}
	}
	componentRefs := make(map[string]struct{}, len(components))
	for _, component := range components {
		if !validProjectionText(component.ComponentRef, 128) ||
			!validProjectionText(component.Name, 100) || !validProjectionText(component.TypeCode, 64) {
			return false
		}
		if component.RecipientRef != "" {
			if _, exists := recipientRefs[component.RecipientRef]; !exists {
				return false
			}
		}
		if _, exists := componentRefs[component.ComponentRef]; exists {
			return false
		}
		componentRefs[component.ComponentRef] = struct{}{}
	}
	return true
}

func validProjectionText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}
