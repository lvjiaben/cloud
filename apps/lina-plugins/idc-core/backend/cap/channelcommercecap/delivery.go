// This file defines customer-owned delivery facts independently of long-lived
// entitlements. One-time and multi-delivery orders remain representable.

package channelcommercecap

import (
	"context"
	"time"
)

// OrderDelivery is a safe delivery result without provider evidence or secrets.
type OrderDelivery struct {
	DeliveryNo  string            `json:"deliveryNo"`
	Status      string            `json:"status"`
	Quantity    int               `json:"quantity"`
	Ongoing     bool              `json:"ongoing"`
	StartedAt   *time.Time        `json:"startedAt"`
	CompletedAt *time.Time        `json:"completedAt"`
	Services    []DeliveryService `json:"services"`
}

// DeliveryService identifies an optional managed entitlement for a delivery.
type DeliveryService struct {
	CurrencyCode string     `json:"currencyCode"`
	PeriodUnit   string     `json:"periodUnit"`
	PeriodCount  int        `json:"periodCount"`
	ServiceNo    string     `json:"serviceNo"`
	Status       string     `json:"status"`
	RenewalMode  string     `json:"renewalMode"`
	StartedAt    *time.Time `json:"startedAt"`
	ExpiresAt    *time.Time `json:"expiresAt"`
}

// OrderDeliveries reports complete bounded order facts, never a silently
// truncated service list used to infer whether delivery succeeded.
type OrderDeliveries struct {
	OrderNo string          `json:"orderNo"`
	Items   []OrderDelivery `json:"items"`
}

type OrderDeliveryRuntime interface {
	ReadOrderDeliveries(context.Context, string, string) (*OrderDeliveries, error)
}

// RenewalRequestResult reconciles the original request after a response loss.
type RenewalRequestResult struct {
	Found            bool       `json:"found"`
	OperationNo      string     `json:"operationNo"`
	Status           string     `json:"status"`
	CurrencyCode     string     `json:"currencyCode"`
	AmountMinor      int64      `json:"amountMinor"`
	CurrentExpiresAt *time.Time `json:"currentExpiresAt"`
}

// RenewalRequestRuntime is read-only and scopes the exact confirmation digest
// to the authenticated application, payer, owner and service together.
type RenewalRequestRuntime interface {
	ReadRenewalByRequest(context.Context, string, string, string) (*RenewalRequestResult, error)
}
