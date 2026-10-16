// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package peer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

type order struct {
	ID                 int64  `json:"id"`
	CoreOrderNo        string `json:"coreOrderNo"`
	Status             string `json:"status"`
	FinanciallySettled bool   `json:"financiallySettled"`
}

func requestID(kind, key string) string {
	sum := sha256.Sum256([]byte(key))
	return "peer-" + kind + "-" + hex.EncodeToString(sum[:])
}

// Purchase submits at most one frozen quote and one idempotent order per
// invocation. An uncertain order response is reconciled by QueryPurchase.
func Purchase(executor confidentialoutboundcap.Service, contextID, key string, purchase channelcommercecap.Purchase) (*productfulfillmentcap.Result, error) {
	return purchaseWithClient(&client{executor: executor, contextID: contextID}, key, purchase)
}

func purchaseWithClient(c *client, key string, purchase channelcommercecap.Purchase) (*productfulfillmentcap.Result, error) {
	if key == "" || purchase.PublicSlug == "" || purchase.ConfigurationRevision == "" || purchase.BillingTermToken == "" || purchase.CurrencyCode == "" || purchase.Quantity < 1 || purchase.Quantity > 1000 || purchase.PurchaseCycles < 1 || purchase.PurchaseCycles > 120 {
		return nil, errInvalid
	}
	var quote struct {
		QuoteNo          string `json:"quoteNo"`
		Currency         string `json:"currency"`
		TotalAmountMinor int64  `json:"totalAmountMinor"`
	}
	err := c.call(http.MethodPost, "/quotes", nil, struct {
		RequestID string                      `json:"requestId"`
		Purchase  channelcommercecap.Purchase `json:"purchase"`
	}{requestID("quote", key), purchase}, []string{"quoteNo", "currency", "totalAmountMinor"}, &quote)
	if err != nil {
		return nil, err
	}
	if quote.QuoteNo == "" || quote.Currency != purchase.CurrencyCode || quote.TotalAmountMinor < 0 {
		return nil, errUnavailable
	}
	var created struct {
		ID       int64  `json:"id"`
		Status   string `json:"status"`
		Replayed bool   `json:"replayed"`
	}
	err = c.call(http.MethodPost, "/orders", nil, struct {
		RequestID string `json:"requestId"`
		QuoteNo   string `json:"quoteNo"`
	}{requestID("order", key), quote.QuoteNo}, []string{"id", "status", "replayed"}, &created)
	if err != nil {
		if errors.Is(err, errRejected) {
			return fulfillmentResult(productfulfillmentcap.StatusNotAccepted, "", nil), nil
		}
		return fulfillmentResult(productfulfillmentcap.StatusResultUnknown, "", nil), nil
	}
	if created.ID <= 0 {
		return fulfillmentResult(productfulfillmentcap.StatusResultUnknown, "", nil), nil
	}
	return fulfillmentResult(productfulfillmentcap.StatusAccepted, "peer-order:"+strconv.FormatInt(created.ID, 10), nil), nil
}

// QueryPurchase is read-only. A lookup by the original idempotency key also
// recovers an order whose successful submission response was lost.
func QueryPurchase(executor confidentialoutboundcap.Service, contextID, key string) (*productfulfillmentcap.Result, error) {
	if key == "" {
		return nil, errInvalid
	}
	c := &client{executor: executor, contextID: contextID}
	var lookup struct {
		Found bool   `json:"found"`
		Order *order `json:"order"`
	}
	if err := c.call(http.MethodGet, "/orders/by-request/"+url.PathEscape(requestID("order", key)), nil, nil, []string{"found", "order", "id", "coreOrderNo", "status", "financiallySettled"}, &lookup); err != nil {
		return nil, err
	}
	if !lookup.Found {
		return fulfillmentResult(productfulfillmentcap.StatusNotFound, "", nil), nil
	}
	if lookup.Order == nil || lookup.Order.ID <= 0 || lookup.Order.CoreOrderNo == "" || !lookup.Order.FinanciallySettled {
		return nil, errUnavailable
	}
	reference := "peer-order:" + strconv.FormatInt(lookup.Order.ID, 10)
	var deliveries peerOrderDeliveries
	if err := c.call(http.MethodGet, "/orders/"+strconv.FormatInt(lookup.Order.ID, 10)+"/deliveries", nil, nil,
		[]string{"orderNo", "items", "deliveryNo", "status", "quantity", "ongoing", "completedAt", "startedAt", "services", "serviceNo", "renewalMode", "expiresAt"}, &deliveries); err != nil {
		return nil, err
	}
	if deliveries.OrderNo != lookup.Order.CoreOrderNo || len(deliveries.Items) > 1000 {
		return nil, errUnavailable
	}
	if len(deliveries.Items) == 0 {
		return fulfillmentResult(productfulfillmentcap.StatusRunning, reference, nil), nil
	}
	complete := true
	for _, delivery := range deliveries.Items {
		if delivery.DeliveryNo == "" || delivery.Quantity < 1 {
			return nil, errUnavailable
		}
		if delivery.Status != "completed" {
			complete = false
		}
		if delivery.Ongoing && len(delivery.Services) == 0 {
			complete = false
		}
		for _, service := range delivery.Services {
			if service.ServiceNo == "" {
				return nil, errUnavailable
			}
			if service.Status != "active" {
				complete = false
			}
		}
	}
	evidence, err := json.Marshal(struct {
		OrderID    int64               `json:"orderId"`
		OrderNo    string              `json:"orderNo"`
		Deliveries []peerOrderDelivery `json:"deliveries"`
	}{lookup.Order.ID, lookup.Order.CoreOrderNo, deliveries.Items})
	if err != nil {
		return nil, errUnavailable
	}
	if complete {
		result := fulfillmentResult(productfulfillmentcap.StatusSucceeded, reference, evidence)
		for _, delivery := range deliveries.Items {
			if len(delivery.Services) == 0 {
				if delivery.StartedAt == nil {
					return nil, errUnavailable
				}
				result.Deliveries = append(result.Deliveries, productfulfillmentcap.Delivery{UpstreamDeliveryNo: delivery.DeliveryNo, StartedAt: *delivery.StartedAt})
			}
			for _, service := range delivery.Services {
				if service.StartedAt == nil {
					return nil, errUnavailable
				}
				row := productfulfillmentcap.Delivery{UpstreamDeliveryNo: delivery.DeliveryNo, UpstreamServiceNo: service.ServiceNo, StartedAt: *service.StartedAt}
				if service.ExpiresAt != nil {
					row.ExpiresAt = *service.ExpiresAt
				}
				result.Deliveries = append(result.Deliveries, row)
			}
		}
		return result, nil
	}
	return fulfillmentResult(productfulfillmentcap.StatusRunning, reference, evidence), nil
}

func fulfillmentResult(status productfulfillmentcap.ResultStatus, reference string, evidence json.RawMessage) *productfulfillmentcap.Result {
	return &productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: status, ExternalReference: reference, Evidence: evidence}
}
