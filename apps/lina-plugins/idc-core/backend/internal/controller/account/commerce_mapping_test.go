// This file verifies public commerce mapping preserves exact values and the
// published manual-processing phase without exposing private execution fields.

package account

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func TestCommerceOrderMappingPreservesExactAmountsAndMilliseconds(t *testing.T) {
	createdAt := time.UnixMilli(1_786_406_400_123)
	paymentDueAt := createdAt.Add(30 * time.Minute)
	detail := ToAPICommerceOrderDetail(&commerce.OrderDetail{
		Summary: commerce.OrderSummary{
			OrderNo: "ORD_mapping_1", CurrencyCode: "CNY", CreatedAt: createdAt,
			PaymentDueAt: &paymentDueAt, TotalAmountMinor: math.MaxInt64,
		},
		Invoice: &commerce.OrderInvoice{
			InvoiceNo: "INV_mapping_1", CurrencyCode: "CNY",
			TotalAmountMinor: math.MaxInt64, PaymentDueAt: &paymentDueAt,
		},
		Items: []commerce.OrderItemView{{
			OrderItemNo: "ITEM_1", ProductNameZhCN: "证书", ProductNameEnUS: "Certificate",
		}},
		Deliveries: []commerce.DeliveryView{{
			OrderItemNo: "ITEM_1", Quantity: 3, StatusCode: "manual_processing",
			OngoingManagement: true, StartedAt: &createdAt, UpdatedAt: createdAt,
		}},
		ServiceEntitlements: []commerce.ServiceEntitlementView{{
			EntitlementNo: "ENT_1", OrderItemNo: "ITEM_1", UnitSequence: 9,
			StatusCode: "active", StartedAt: createdAt,
		}},
		Reviews: []commerce.OrderReviewView{},
	})
	if detail == nil || detail.Summary == nil || detail.Invoice == nil {
		t.Fatal("expected complete order mapping")
	}
	if detail.Summary.TotalAmountMinor != "9223372036854775807" ||
		detail.Invoice.TotalAmountMinor != "9223372036854775807" {
		t.Fatalf("minor units lost precision: summary=%s invoice=%s",
			detail.Summary.TotalAmountMinor, detail.Invoice.TotalAmountMinor)
	}
	if detail.Summary.CreatedAt != createdAt.UnixMilli() ||
		detail.Summary.PaymentDueAt != paymentDueAt.UnixMilli() {
		t.Fatalf("timestamps are not Unix milliseconds: %+v", detail.Summary)
	}
	if len(detail.Deliveries) != 1 || detail.Deliveries[0].ProductNameZhCN != "证书" ||
		detail.Deliveries[0].Quantity != 3 || detail.Deliveries[0].StatusCode != "manual_processing" ||
		!detail.Deliveries[0].OngoingService || detail.Deliveries[0].StartedAt != createdAt.UnixMilli() ||
		detail.Deliveries[0].UpdatedAt != createdAt.UnixMilli() {
		t.Fatalf("customer-safe delivery was not mapped: %+v", detail.Deliveries)
	}
	if len(detail.ServiceEntitlements) != 1 || detail.ServiceEntitlements[0].ProductNameEnUS != "Certificate" ||
		detail.ServiceEntitlements[0].EntitlementNo != "ENT_1" ||
		detail.ServiceEntitlements[0].OrderItemNo != "ITEM_1" ||
		detail.ServiceEntitlements[0].StatusCode != "active" || detail.ServiceEntitlements[0].StartedAt != createdAt.UnixMilli() {
		t.Fatalf("customer-safe service entitlement was not mapped: %+v", detail.ServiceEntitlements)
	}
	encoded, err := json.Marshal(detail.Summary)
	if err != nil {
		t.Fatalf("marshal account order summary: %v", err)
	}
	if strings.Contains(string(encoded), "customerNo") {
		t.Fatalf("account order summary exposed staff-only customer number: %s", encoded)
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal account order detail: %v", err)
	}
	for _, forbidden := range []string{"productFulfillments", "fulfillments", "inventoryReservations", "unitSequence", "granularityCode", "plugin", "interface", "digest"} {
		if strings.Contains(string(detailJSON), forbidden) {
			t.Fatalf("account order detail exposed internal field %q: %s", forbidden, detailJSON)
		}
	}
}

func TestCommerceCartMappingPreservesServerAuthoritativeConfigurationState(t *testing.T) {
	now := time.UnixMilli(1_786_406_400_123)
	mapped := toAPICommerceCart(&commerce.Cart{
		CartNo: "CRT_mapping_1", CurrencyCode: "CNY", Version: 3,
		Items: []commerce.CartItem{{
			CartItemNo: "CTI_mapping_1", ConfigurationRequired: true,
			ConfigurationFrozen: true, ConfigurationRevision: "opaque-revision",
			Quantity: 7,
		}},
		LatestQuote: &commerce.Quote{
			QuoteNo: "QTE_mapping_1", CartNo: "CRT_mapping_1", CurrencyCode: "CNY",
			CartVersion: 3, IssuedAt: now, ExpiresAt: now.Add(time.Hour),
			Items: []commerce.QuoteItem{{
				QuoteItemNo: "QIT_mapping_1", CartItemNo: "CTI_mapping_1",
				ConfigurationRequired: true, ConfigurationFrozen: true,
				ConfigurationRevision: "opaque-revision",
				Quantity:              7,
			}},
		},
	})
	if mapped == nil || mapped.LatestQuote == nil || len(mapped.Items) != 1 ||
		len(mapped.LatestQuote.Items) != 1 {
		t.Fatalf("mapped authoritative cart=%+v", mapped)
	}
	if !mapped.Items[0].ConfigurationFrozen ||
		mapped.Items[0].ConfigurationRevision != "opaque-revision" ||
		!mapped.LatestQuote.Items[0].ConfigurationFrozen ||
		mapped.LatestQuote.Items[0].ConfigurationRevision != "opaque-revision" {
		t.Fatalf("mapped configuration authority cart=%+v quote=%+v", mapped.Items[0], mapped.LatestQuote.Items[0])
	}
	if mapped.Items[0].Quantity != 7 || mapped.LatestQuote.Items[0].Quantity != 7 {
		t.Fatalf("generic quantity was not preserved: cart=%+v quote=%+v", mapped.Items[0], mapped.LatestQuote.Items[0])
	}
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("marshal mapped cart: %v", err)
	}
	if strings.Contains(string(encoded), "serverQuantity") {
		t.Fatalf("generic cart contract exposed obsolete serverQuantity: %s", encoded)
	}
}
