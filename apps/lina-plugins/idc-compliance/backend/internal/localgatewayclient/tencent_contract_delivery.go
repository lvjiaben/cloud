package localgatewayclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const maximumTencentContractDeliveryBatch = 50

var localDeliveryNoPattern = regexp.MustCompile(`^ldv_[0-9a-f]{32}$`)

func (c *Client) ListTencentContractDeliveries(
	ctx context.Context,
	limit int,
) ([]processauth.TencentContractDeliveryItem, error) {
	if limit < 1 || limit > maximumTencentContractDeliveryBatch {
		return nil, &TencentContractFlowError{Code: "invalid_contract_delivery_request"}
	}
	result, err := c.listTencentContractDeliveryPage(ctx, limit)
	if err != nil {
		return nil, err
	}
	if len(result.Items) > limit {
		return nil, &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
	}
	seen := make(map[string]struct{}, len(result.Items))
	for _, item := range result.Items {
		if !validTencentContractDelivery(item) {
			return nil, &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
		}
		if _, duplicate := seen[item.DeliveryNo]; duplicate {
			return nil, &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
		}
		seen[item.DeliveryNo] = struct{}{}
	}
	return result.Items, nil
}

func (c *Client) listTencentContractDeliveryPage(
	ctx context.Context,
	limit int,
) (processauth.TencentContractDeliveryPageResponse, error) {
	if c == nil || c.baseURL == nil || c.signer == nil || c.contractHTTPClient == nil ||
		ctx == nil || ctx.Err() != nil {
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: "local_gateway_unavailable"}
	}
	target := c.baseURL.String() + processauth.TencentContractDeliveryPagesPath + "?" +
		processauth.TencentContractDeliveryPageLimitQuery + "=" + strconv.Itoa(limit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: "local_gateway_unavailable"}
	}
	if err = c.signer.SignTencentContractDeliveryPage(request); err != nil {
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: "local_gateway_auth_unavailable"}
	}
	response, err := c.contractHTTPClient.Do(request)
	if err != nil {
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: "tencent_temporarily_unavailable"}
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil || len(content) > maximumResponse {
		clearBytes(content)
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: "tencent_temporarily_unavailable"}
	}
	defer clearBytes(content)
	if response.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(content, &failure) != nil || strings.TrimSpace(failure.Code) == "" {
			failure.Code = "tencent_temporarily_unavailable"
		}
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: strings.TrimSpace(failure.Code)}
	}
	var result processauth.TencentContractDeliveryPageResponse
	if json.Unmarshal(content, &result) != nil {
		return processauth.TencentContractDeliveryPageResponse{},
			&TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
	}
	return result, nil
}

func (c *Client) AcknowledgeTencentContractDeliveries(
	ctx context.Context,
	deliveryNos []string,
	idempotencyKey string,
) error {
	if len(deliveryNos) < 1 || len(deliveryNos) > maximumTencentContractDeliveryBatch ||
		strings.TrimSpace(idempotencyKey) == "" {
		return &TencentContractFlowError{Code: "invalid_contract_delivery_request"}
	}
	numbers := append([]string(nil), deliveryNos...)
	sort.Strings(numbers)
	for index, number := range numbers {
		if !localDeliveryNoPattern.MatchString(number) || index > 0 && number == numbers[index-1] {
			return &TencentContractFlowError{Code: "invalid_contract_delivery_request"}
		}
	}
	var result processauth.TencentContractDeliveryAcknowledgementResponse
	err := c.doTencentContractRead(
		ctx, processauth.TencentContractDeliveryAcknowledgementsPath,
		processauth.TencentContractDeliveryAcknowledgementRequest{DeliveryNos: numbers}, &result,
		func(request *http.Request, body []byte) error {
			return c.signer.SignTencentContractDeliveryAcknowledgement(
				request, body, processBusinessKey(idempotencyKey),
			)
		},
	)
	if err != nil {
		return err
	}
	if result.Acknowledged != len(numbers) {
		return &TencentContractFlowError{Code: "tencent_result_unknown", ResultUnknown: true}
	}
	return nil
}

func validTencentContractDelivery(item processauth.TencentContractDeliveryItem) bool {
	if !localDeliveryNoPattern.MatchString(item.DeliveryNo) ||
		!localWriteRoutePattern.MatchString(item.RouteNo) || item.RouteVersion < 2 ||
		item.ObservedUnix < 1 || item.SupplierObservedUnix < 1 ||
		!validTencentContractProviderStatus(item.ProviderStatus, item.Status) {
		return false
	}
	if item.SourceKind != "callback" && item.SourceKind != "active_query" {
		return false
	}
	if item.DecisionCode != "applied" && item.DecisionCode != "stale" && item.DecisionCode != "conflict" {
		return false
	}
	return item.ReconciliationStatus == "none" || item.ReconciliationStatus == "query_required" ||
		item.ReconciliationStatus == "reconciliation_required"
}
