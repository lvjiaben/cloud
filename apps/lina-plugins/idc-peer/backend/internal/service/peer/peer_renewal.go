// This file implements provider-confirmed renewal through the public channel
// protocol. The frozen order reference is resolved to exactly one upstream
// active service; ambiguity and expiry drift fail closed.

package peer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

type upstreamDelivery struct {
	DeliveryNo string            `json:"deliveryNo"`
	Status     string            `json:"status"`
	Ongoing    bool              `json:"ongoing"`
	Services   []upstreamService `json:"services"`
}
type upstreamService struct {
	ServiceNo, Status        string
	CurrencyCode, PeriodUnit string
	PeriodCount              int
	ExpiresAt                *int64 `json:"expiresAt"`
}

// Renew resolves the frozen peer order reference and submits one idempotent
// renewal operation. A lost response is result_unknown for worker recovery.
func Renew(executor confidentialoutboundcap.Service, contextID string, input renewalownercap.PeerRequest) (*renewalownercap.Task, error) {
	if renewalownercap.ValidatePeerRequest(input) != nil {
		return nil, errInvalid
	}
	c := &client{executor: executor, contextID: contextID}
	serviceNo, err := resolveRenewalService(c, input, true)
	if err != nil {
		return nil, err
	}
	var operation struct {
		OperationNo, Status, FailureCode string
		ExpiresAt                        *int64 `json:"expiresAt"`
	}
	body := struct {
		RequestID  string         `json:"requestId"`
		Parameters map[string]any `json:"parameters"`
	}{input.Request.OperationNo, map[string]any{"renewalCycles": input.PurchaseCycles}}
	if err = c.call(http.MethodPost, "/services/"+serviceNo+"/operations/renew", nil, body, []string{"operationNo", "status", "failureCode", "expiresAt"}, &operation); err != nil {
		if errors.Is(err, errRejected) {
			return failedTask(input, "upstream_rejected", c.observedAt), nil
		}
		return unknownTask(input, c.observedAt), nil
	}
	return queryRenewal(c, input, serviceNo)
}

// Query reconciles the frozen operation identifier without creating another
// operation. The core owner must pass the upstream operation number in the
// task request when a prior submit returned it.
func QueryRenewal(executor confidentialoutboundcap.Service, contextID string, input renewalownercap.PeerRequest) (*renewalownercap.Task, error) {
	if renewalownercap.ValidatePeerRequest(input) != nil {
		return nil, errInvalid
	}
	c := &client{executor: executor, contextID: contextID}
	serviceNo, err := resolveRenewalService(c, input, false)
	if err != nil {
		return nil, err
	}
	return queryRenewal(c, input, serviceNo)
}

func queryRenewal(c *client, input renewalownercap.PeerRequest, serviceNo string) (*renewalownercap.Task, error) {
	var operation struct {
		ObservedAt           int64 `json:"observedAt"`
		Found                bool
		Status, CurrencyCode string
		CurrentExpiresAt     *int64 `json:"currentExpiresAt"`
	}
	if err := c.call(http.MethodGet, "/services/"+serviceNo+"/renewals/by-request/"+input.Request.OperationNo, nil, nil, []string{"found", "status", "currencyCode", "currentExpiresAt", "observedAt"}, &operation); err != nil {
		return unknownTask(input, c.observedAt), nil
	}
	if !validObservation(operation.ObservedAt) {
		return nil, errUnavailable
	}
	c.observedAt = time.UnixMilli(operation.ObservedAt).UTC()
	if !operation.Found {
		return unknownTask(input, c.observedAt), nil
	}
	if operation.CurrencyCode != input.CurrencyCode {
		return nil, errUnavailable
	}
	if operation.Status == "succeeded" && (operation.CurrentExpiresAt == nil || *operation.CurrentExpiresAt < input.Request.ProjectedExpiresAt.UnixMilli()) {
		return nil, errUnavailable
	}
	var expiry *time.Time
	if operation.CurrentExpiresAt != nil {
		value := time.UnixMilli(*operation.CurrentExpiresAt)
		expiry = &value
	}
	return mapRenewalTask(input, operation.Status, expiry, c.observedAt), nil
}

func resolveRenewalService(c *client, input renewalownercap.PeerRequest, checkOriginalExpiry bool) (string, error) {
	const prefix = "peer-order:"
	if !strings.HasPrefix(input.ExternalReference, prefix) {
		return "", errInvalid
	}
	orderID, err := strconv.ParseInt(strings.TrimPrefix(input.ExternalReference, prefix), 10, 64)
	if err != nil || orderID < 1 {
		return "", errInvalid
	}
	var response struct {
		ObservedAt int64              `json:"observedAt"`
		OrderNo    string             `json:"orderNo"`
		Items      []upstreamDelivery `json:"items"`
	}
	fields := []string{"orderNo", "items", "deliveryNo", "status", "ongoing", "services", "serviceNo", "expiresAt", "currencyCode", "periodUnit", "periodCount", "observedAt"}
	if err = c.call(http.MethodGet, "/orders/"+strconv.FormatInt(orderID, 10)+"/deliveries", nil, nil, fields, &response); err != nil {
		return "", err
	}
	if !validObservation(response.ObservedAt) {
		return "", errUnavailable
	}
	c.observedAt = time.UnixMilli(response.ObservedAt).UTC()
	var matches []upstreamService
	for _, delivery := range response.Items {
		if delivery.Status != "completed" || !delivery.Ongoing {
			continue
		}
		for _, service := range delivery.Services {
			if service.ServiceNo == input.UpstreamServiceNo && (service.Status == "active" || service.Status == "expired") {
				matches = append(matches, service)
			}
		}
	}
	if len(matches) != 1 || matches[0].ServiceNo == "" {
		return "", gerror.New("frozen upstream renewal service is unavailable")
	}
	if matches[0].CurrencyCode != input.CurrencyCode || matches[0].PeriodUnit != input.PeriodUnitCode || matches[0].PeriodCount != input.PeriodCount {
		return "", gerror.New("upstream renewal billing interval mismatch")
	}
	if checkOriginalExpiry && (matches[0].ExpiresAt == nil || *matches[0].ExpiresAt != input.Request.OriginalExpiresAt.UnixMilli()) {
		return "", gerror.New("upstream renewal expiry changed")
	}
	return matches[0].ServiceNo, nil
}

func mapRenewalTask(input renewalownercap.PeerRequest, status string, expiry *time.Time, observedAt time.Time) *renewalownercap.Task {
	result := &renewalownercap.Task{TaskNo: input.Request.OperationNo, Status: renewalownercap.TaskStatusRunning, ObservedAt: observedAt}
	switch strings.ToLower(status) {
	case "succeeded":
		result.Status = renewalownercap.TaskStatusSucceeded
	case "failed", "cancelled":
		result.Status = renewalownercap.TaskStatusFailed
		result.FailureCode = "upstream_failed"
	case "ready", "executing", "quoted", "awaiting_financial_settlement":
	default:
		result.Status = renewalownercap.TaskStatusRunning
	}
	if expiry != nil {
		result.EvidenceDigest = digestRenewalExpiry(*expiry)
	} else {
		result.EvidenceDigest = digestRenewalExpiry(result.ObservedAt)
	}
	return result
}
func failedTask(input renewalownercap.PeerRequest, code string, observedAt time.Time) *renewalownercap.Task {
	result := mapRenewalTask(input, "failed", nil, observedAt)
	result.FailureCode = code
	return result
}
func unknownTask(input renewalownercap.PeerRequest, observedAt time.Time) *renewalownercap.Task {
	result := mapRenewalTask(input, "executing", nil, observedAt)
	result.Status = renewalownercap.TaskStatusResultUnknown
	result.FailureCode = "upstream_result_unknown"
	return result
}
func digestRenewalExpiry(value time.Time) string {
	sum := sha256.Sum256([]byte(value.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])
}
