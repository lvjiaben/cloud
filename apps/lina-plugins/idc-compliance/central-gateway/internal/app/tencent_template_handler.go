// This file exposes a complete, safe buyer-owned Tencent template projection.

package app

import (
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/tencentess"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const (
	maximumTencentTemplateCount         = 20000
	maximumTencentComponentsPerTemplate = 512
	maximumTencentRecipientsPerTemplate = 64
	maximumTencentProjectedComponents   = 200000
	maximumTencentProjectedRecipients   = 100000
)

type tencentTemplateComponentProjection struct {
	ComponentRef string `json:"componentRef"`
	Name         string `json:"name"`
	TypeCode     string `json:"typeCode"`
	Required     bool   `json:"required"`
	RecipientRef string `json:"recipientRef,omitempty"`
}

type tencentTemplateRecipientProjection struct {
	RecipientRef       string `json:"recipientRef"`
	SequenceNo         int    `json:"sequenceNo"`
	TypeCode           string `json:"typeCode"`
	RoleName           string `json:"roleName"`
	RequireSign        bool   `json:"requireSign"`
	RoutingOrder       *int64 `json:"routingOrder,omitempty"`
	IsPromoter         bool   `json:"isPromoter"`
	SignComponentCount int    `json:"signComponentCount"`
}

type tencentTemplateSigningCounts struct {
	customerSignatures int
	customerSeals      int
	merchantSignatures int
	merchantSeals      int
}

type tencentTemplateProjection struct {
	TemplateRef            string                               `json:"templateRef"`
	Name                   string                               `json:"name"`
	Version                string                               `json:"version,omitempty"`
	ProviderState          string                               `json:"providerState"`
	ProviderTimeMS         int64                                `json:"providerTimeMs,omitempty"`
	CustomerSignatureCount int                                  `json:"customerSignatureCount"`
	CustomerSealCount      int                                  `json:"customerSealCount"`
	MerchantSignatureCount int                                  `json:"merchantSignatureCount"`
	MerchantSealCount      int                                  `json:"merchantSealCount"`
	Components             []tencentTemplateComponentProjection `json:"components"`
	Recipients             []tencentTemplateRecipientProjection `json:"recipients"`
}

type tencentTemplateListResponse struct {
	Items    []tencentTemplateProjection `json:"items"`
	SyncedAt int64                       `json:"syncedAt"`
}

type tencentTemplatePreviewRequest struct {
	TemplateRef string `json:"templateRef"`
}

type tencentTemplatePreviewResponse struct {
	PreviewURL string `json:"previewUrl"`
}

func localTencentTemplatesHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != processauth.TencentTemplatesPath ||
			!requestBodyIsEmpty(request) || runtime == nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		credentials, err := credentialsFromRequest(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{errorCodeField: processAuthRejectedCode})
			return
		}
		_, _, err = runtime.authenticateRead(
			request.Context(), processauth.TencentTemplatesReadScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeEmpty, nil, credentials,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		items, err := readAllTencentTemplates(request, runtime.tencentESS)
		if err != nil {
			writeTencentTemplateError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, tencentTemplateListResponse{
			Items: items, SyncedAt: time.Now().UTC().UnixMilli(),
		})
	}
}

func localTencentTemplatePreviewHandler(runtime *localRuntime) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != processauth.TencentTemplatePreviewsPath ||
			request.Header.Get("Content-Type") != "application/json" || runtime == nil {
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
		_, _, err = runtime.authenticateRead(
			request.Context(), processauth.TencentTemplatePreviewReadScope, request.Method,
			request.URL.RequestURI(), processauth.PayloadModeJSON, body, credentials,
		)
		if err != nil {
			writeProcessAdmissionError(writer, err)
			return
		}
		var input tencentTemplatePreviewRequest
		if err = decodeTencentOnboardingJSON(body, &input); err != nil ||
			!validTencentTemplateText(input.TemplateRef, 256) {
			writeJSON(writer, http.StatusBadRequest, map[string]string{errorCodeField: "invalid_request"})
			return
		}
		if runtime.tencentESS == nil || !runtime.tencentESS.Configured() {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_app_not_configured"})
			return
		}
		previewURL, err := runtime.tencentESS.DescribeTemplatePreview(
			request.Context(), strings.TrimSpace(input.TemplateRef),
		)
		if err != nil {
			writeTencentTemplateError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, tencentTemplatePreviewResponse{PreviewURL: previewURL})
	}
}

func readAllTencentTemplates(
	request *http.Request,
	client *tencentess.Client,
) ([]tencentTemplateProjection, error) {
	if request == nil || client == nil {
		return nil, errors.New("tencent template reader is unavailable")
	}
	items := make([]tencentTemplateProjection, 0, 100)
	totalComponents := 0
	totalRecipients := 0
	for offset := uint64(0); ; offset += 100 {
		response, err := client.DescribeTemplates(request.Context(), 100, offset)
		if err != nil {
			return nil, err
		}
		if response == nil || response.Response == nil || response.Response.TotalCount == nil ||
			*response.Response.TotalCount < 0 || *response.Response.TotalCount > maximumTencentTemplateCount {
			return nil, errors.New("tencent template response is malformed")
		}
		for _, source := range response.Response.Templates {
			if source == nil || source.TemplateId == nil || source.TemplateName == nil ||
				!validTencentTemplateText(*source.TemplateId, 256) ||
				!validTencentTemplateText(*source.TemplateName, 200) {
				return nil, errors.New("tencent template response is malformed")
			}
			state := "disabled"
			if source.Available != nil && *source.Available == 1 {
				state = "active"
			} else if source.Available == nil || *source.Available != 2 {
				return nil, errors.New("tencent template response is malformed")
			}
			item := tencentTemplateProjection{
				TemplateRef: strings.TrimSpace(*source.TemplateId),
				Name:        strings.TrimSpace(*source.TemplateName), ProviderState: state,
			}
			components, recipients, customerSignatures, customerSeals,
				merchantSignatures, merchantSeals, countErr := tencentTemplateMappingProjection(source)
			if countErr != nil {
				return nil, countErr
			}
			totalComponents += len(components)
			totalRecipients += len(recipients)
			if totalComponents > maximumTencentProjectedComponents ||
				totalRecipients > maximumTencentProjectedRecipients {
				return nil, errors.New("tencent template mapping projection exceeds the local bound")
			}
			item.CustomerSignatureCount = customerSignatures
			item.CustomerSealCount = customerSeals
			item.MerchantSignatureCount = merchantSignatures
			item.MerchantSealCount = merchantSeals
			item.Components = components
			item.Recipients = recipients
			if source.TemplateVersion != nil {
				item.Version = strings.TrimSpace(*source.TemplateVersion)
				if item.Version != "" && !validTencentTemplateText(item.Version, 32) {
					return nil, errors.New("tencent template response is malformed")
				}
			}
			if source.CreatedOn != nil && *source.CreatedOn > 0 {
				item.ProviderTimeMS = *source.CreatedOn * 1000
			}
			items = append(items, item)
			if len(items) > maximumTencentTemplateCount {
				return nil, errors.New("tencent template response exceeds the local bound")
			}
		}
		if uint64(len(items)) >= uint64(*response.Response.TotalCount) {
			break
		}
		if len(response.Response.Templates) == 0 || offset+100 > maximumTencentTemplateCount {
			return nil, errors.New("tencent template pagination is incomplete")
		}
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].TemplateRef < items[right].TemplateRef
	})
	for index := 1; index < len(items); index++ {
		if items[index-1].TemplateRef == items[index].TemplateRef {
			return nil, errors.New("tencent template response contains duplicates")
		}
	}
	return items, nil
}

func tencentTemplateMappingProjection(source *essbasic.TemplateInfo) (
	[]tencentTemplateComponentProjection,
	[]tencentTemplateRecipientProjection,
	int,
	int,
	int,
	int,
	error,
) {
	if source == nil {
		return nil, nil, 0, 0, 0, 0, errors.New("tencent template signing positions are malformed")
	}
	if len(source.Components) > maximumTencentComponentsPerTemplate ||
		len(source.SignComponents) > maximumTencentComponentsPerTemplate ||
		len(source.Components)+len(source.SignComponents) > maximumTencentComponentsPerTemplate ||
		len(source.Recipients) > maximumTencentRecipientsPerTemplate {
		return nil, nil, 0, 0, 0, 0, errors.New("tencent template mapping projection exceeds the per-template bound")
	}
	recipients, recipientItems, err := projectTencentTemplateRecipients(source.Recipients)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	componentItems := make([]tencentTemplateComponentProjection, 0, len(source.Components)+len(source.SignComponents))
	componentRefs := make(map[string]struct{}, len(source.Components)+len(source.SignComponents))
	for _, component := range source.Components {
		projected, projectionErr := projectTencentTemplateComponent(component, false, recipients, componentRefs)
		if projectionErr != nil {
			return nil, nil, 0, 0, 0, 0, projectionErr
		}
		componentItems = append(componentItems, projected)
	}
	componentItems, signingCounts, err := projectTencentTemplateSigningComponents(
		source.SignComponents, recipients, recipientItems, componentRefs, componentItems,
	)
	if err != nil {
		return nil, nil, 0, 0, 0, 0, err
	}
	sort.Slice(componentItems, func(left, right int) bool {
		return componentItems[left].ComponentRef < componentItems[right].ComponentRef
	})
	sort.Slice(recipientItems, func(left, right int) bool {
		return recipientItems[left].RecipientRef < recipientItems[right].RecipientRef
	})
	return componentItems, recipientItems, signingCounts.customerSignatures, signingCounts.customerSeals,
		signingCounts.merchantSignatures, signingCounts.merchantSeals, nil
}

func projectTencentTemplateRecipients(
	source []*essbasic.Recipient,
) (map[string]int, []tencentTemplateRecipientProjection, error) {
	recipients := make(map[string]int, len(source))
	items := make([]tencentTemplateRecipientProjection, 0, len(source))
	for _, recipient := range source {
		if recipient == nil || recipient.RecipientId == nil ||
			!validTencentTemplateText(*recipient.RecipientId, 128) || recipient.RecipientType == nil ||
			recipient.RoleName == nil || !validTencentTemplateText(*recipient.RoleName, 100) {
			return nil, nil, errors.New("tencent template recipient is malformed")
		}
		id := strings.TrimSpace(*recipient.RecipientId)
		if _, exists := recipients[id]; exists {
			return nil, nil, errors.New("tencent template recipient is duplicated")
		}
		typeCode := strings.TrimSpace(*recipient.RecipientType)
		if typeCode != "ENTERPRISE" && typeCode != "INDIVIDUAL" && typeCode != "PROMOTER" {
			return nil, nil, errors.New("tencent template recipient type is unsupported")
		}
		if recipient.RoutingOrder != nil && (*recipient.RoutingOrder < 0 || *recipient.RoutingOrder > 1000000) {
			return nil, nil, errors.New("tencent template recipient routing order is malformed")
		}
		item := tencentTemplateRecipientProjection{
			RecipientRef: id, TypeCode: typeCode, RoleName: strings.TrimSpace(*recipient.RoleName),
			SequenceNo:   len(items) + 1,
			RequireSign:  recipient.RequireSign != nil && *recipient.RequireSign,
			RoutingOrder: recipient.RoutingOrder,
			IsPromoter:   typeCode == "PROMOTER" || recipient.IsPromoter != nil && *recipient.IsPromoter,
		}
		recipients[id] = len(items)
		items = append(items, item)
	}
	return recipients, items, nil
}

func projectTencentTemplateSigningComponents(
	source []*essbasic.Component,
	recipients map[string]int,
	recipientItems []tencentTemplateRecipientProjection,
	componentRefs map[string]struct{},
	componentItems []tencentTemplateComponentProjection,
) ([]tencentTemplateComponentProjection, tencentTemplateSigningCounts, error) {
	counts := tencentTemplateSigningCounts{}
	for _, component := range source {
		if component == nil || component.ComponentType == nil || component.ComponentRecipientId == nil ||
			!validTencentTemplateText(*component.ComponentRecipientId, 128) {
			return nil, counts, errors.New("tencent template signing component is malformed")
		}
		typeCode := strings.TrimSpace(*component.ComponentType)
		recipientIndex, exists := recipients[strings.TrimSpace(*component.ComponentRecipientId)]
		if !exists {
			return nil, counts, errors.New("tencent template signing component recipient is unknown")
		}
		recipientItems[recipientIndex].SignComponentCount++
		counts.add(recipientItems[recipientIndex].IsPromoter, typeCode)
		if typeCode != "SIGN_DATE" {
			continue
		}
		projected, err := projectTencentTemplateComponent(component, true, recipients, componentRefs)
		if err != nil {
			return nil, counts, err
		}
		componentItems = append(componentItems, projected)
	}
	return componentItems, counts, nil
}

func (counts *tencentTemplateSigningCounts) add(merchant bool, typeCode string) {
	switch {
	case merchant && (typeCode == "SIGN_SIGNATURE" || typeCode == "SIGN_PAGING_SIGNATURE"):
		counts.merchantSignatures++
	case merchant && (typeCode == "SIGN_SEAL" || typeCode == "SIGN_PAGING_SEAL" || typeCode == "SIGN_LEGAL_PERSON_SEAL"):
		counts.merchantSeals++
	case !merchant && (typeCode == "SIGN_SIGNATURE" || typeCode == "SIGN_PAGING_SIGNATURE"):
		counts.customerSignatures++
	case !merchant && (typeCode == "SIGN_SEAL" || typeCode == "SIGN_PAGING_SEAL" || typeCode == "SIGN_LEGAL_PERSON_SEAL"):
		counts.customerSeals++
	}
}

func projectTencentTemplateComponent(
	component *essbasic.Component,
	signingDate bool,
	recipients map[string]int,
	seen map[string]struct{},
) (tencentTemplateComponentProjection, error) {
	if component == nil || component.ComponentId == nil || component.ComponentName == nil ||
		component.ComponentType == nil || !validTencentTemplateText(*component.ComponentId, 128) ||
		!validTencentTemplateText(*component.ComponentName, 100) ||
		!validTencentTemplateText(*component.ComponentType, 64) {
		return tencentTemplateComponentProjection{}, errors.New("tencent template component is malformed")
	}
	ref := strings.TrimSpace(*component.ComponentId)
	if _, exists := seen[ref]; exists {
		return tencentTemplateComponentProjection{}, errors.New("tencent template component is duplicated")
	}
	seen[ref] = struct{}{}
	recipientRef := ""
	if component.ComponentRecipientId != nil {
		recipientRef = strings.TrimSpace(*component.ComponentRecipientId)
		if _, exists := recipients[recipientRef]; !validTencentTemplateText(recipientRef, 128) || !exists {
			return tencentTemplateComponentProjection{}, errors.New("tencent template component recipient is unknown")
		}
	}
	return tencentTemplateComponentProjection{
		ComponentRef: ref, Name: strings.TrimSpace(*component.ComponentName),
		TypeCode:     strings.TrimSpace(*component.ComponentType),
		Required:     signingDate || component.ComponentRequired != nil && *component.ComponentRequired,
		RecipientRef: recipientRef,
	}, nil
}

func validTencentTemplateText(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}

func writeTencentTemplateError(writer http.ResponseWriter, err error) {
	var safe *tencentess.Error
	if errors.As(err, &safe) {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: string(safe.Code)})
		return
	}
	writeJSON(writer, http.StatusServiceUnavailable, map[string]string{errorCodeField: "tencent_templates_unavailable"})
}
