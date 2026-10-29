package accountnative

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

type exactMoneyCommerce struct {
	nativeCommerce
	serverNo string
}

func (s exactMoneyCommerce) Quote(_ context.Context, command customeroperations.CommercialQuoteCommand) (*servercommercialcap.Quote, error) {
	if command.ServerNo != s.serverNo {
		return nil, customeroperations.ErrUnavailable
	}
	return &servercommercialcap.Quote{TotalAmountMinor: math.MaxInt64, TaxAmountMinor: 9007199254740993, CurrentRecurringMinor: 9007199254740995, TargetRecurringMinor: math.MaxInt64, ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (s exactMoneyCommerce) Checkout(_ context.Context, command customeroperations.CommercialCheckoutCommand) (*servercommercialcap.Purchase, error) {
	if command.ServerNo != s.serverNo {
		return nil, customeroperations.ErrUnavailable
	}
	return &servercommercialcap.Purchase{TotalAmountMinor: math.MaxInt64}, nil
}
func (s exactMoneyCommerce) TrafficPackages(_ context.Context, serverNo string) ([]servercommercialcap.TrafficPackage, error) {
	if serverNo != s.serverNo {
		return nil, customeroperations.ErrUnavailable
	}
	return []servercommercialcap.TrafficPackage{{AmountMinor: math.MaxInt64, TrafficGB: 100}}, nil
}

// Both history and object billing keep exact money and missing dates distinct.
func (s exactMoneyCommerce) CommercialOperations(_ context.Context, q customeroperations.CommercialHistoryQuery) (*servercommercialcap.OperationPage, error) {
	if q.ServerNo != s.serverNo || q.Limit != 20 {
		return nil, customeroperations.ErrUnavailable
	}
	return &servercommercialcap.OperationPage{NextCursor: "next-page", Items: []servercommercialcap.OperationHistory{{OperationNo: "historical", TotalAmountMinor: "9223372036854775807", Status: "manual_review", CreatedAt: time.UnixMilli(1792382400123), UpdatedAt: time.UnixMilli(1792382400456)}}}, nil
}
func (s exactMoneyCommerce) AccessoryBilling(_ context.Context, q customeroperations.AccessoryBillingQuery) (*servercommercialcap.AccessoryBillingPage, error) {
	if q.ServerNo != s.serverNo || q.Limit != 20 {
		return nil, customeroperations.ErrUnavailable
	}
	amount := "9007199254740993"
	start := time.UnixMilli(1792382400123)
	return &servercommercialcap.AccessoryBillingPage{Items: []servercommercialcap.AccessoryBilling{{ObjectNo: "bound-paid", BillingMode: "paid", PendingAmountMinor: &amount, CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &start}, {ObjectNo: "bound-free", BillingMode: "free"}}}, nil
}
func TestNativeCommercialHTTPMoneyNeverPassesThroughJSONNumber(t *testing.T) {
	number := "SVR-" + strings.Repeat("A", 32)
	controller := &ControllerV1{commercial: exactMoneyCommerce{serverNo: number}}
	server := ghttp.GetServer(guid.S())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.SetAccessLogEnabled(false)
	server.Group("/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(controller.NativeServerCommercialQuote, controller.NativeServerCommercialCheckout, controller.NativeServerTrafficPackages, controller.NativeServerCommercialOperations, controller.NativeServerAccessoryBilling)
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	cases := []struct {
		route, method, body string
		expected            map[string]string
	}{
		{"commercial-quotes", http.MethodPost, `{"action":"traffic_add","idempotencyKey":"quote-exact-money"}`, map[string]string{"totalAmountMinor": "9223372036854775807", "taxAmountMinor": "9007199254740993", "currentRecurringMinor": "9007199254740995", "targetRecurringMinor": "9223372036854775807"}},
		{"commercial-purchases", http.MethodPost, `{"quoteNo":"quote-1","idempotencyKey":"purchase-exact-money","confirmed":true}`, map[string]string{"totalAmountMinor": "9223372036854775807"}},
		{"traffic-packages", http.MethodGet, "", map[string]string{"amountMinor": "9223372036854775807"}},
		{"commercial-operations", http.MethodGet, "", map[string]string{"totalAmountMinor": "9223372036854775807"}},
		{"accessory-billing", http.MethodGet, "", map[string]string{"pendingAmountMinor": "9007199254740993"}},
	}
	for _, test := range cases {
		t.Run(test.route, func(t *testing.T) {
			req, err := http.NewRequest(test.method, fmt.Sprintf("http://127.0.0.1:%d/api/v1/account/native-servers/%s/%s", server.GetListenedPort(), number, test.route), strings.NewReader(test.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			var envelope struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Code != 0 {
				t.Fatalf("HTTP code=%d", envelope.Code)
			}
			for key, want := range test.expected {
				if !bytes.Contains(envelope.Data, []byte(fmt.Sprintf(`"%s":"%s"`, key, want))) {
					t.Fatalf("amount lost string precision: %s=%s in %s", key, want, envelope.Data)
				}
			}
			if test.route == "commercial-operations" && (!bytes.Contains(envelope.Data, []byte(`"completedAt":null`)) || !bytes.Contains(envelope.Data, []byte(`"createdAt":1792382400123`)) || !bytes.Contains(envelope.Data, []byte(`"status":"manual_review"`)) || !bytes.Contains(envelope.Data, []byte(`"nextCursor":"next-page"`))) {
				t.Fatal("history time, unknown state or cursor changed")
			}
			if test.route == "accessory-billing" && (!bytes.Contains(envelope.Data, []byte(`"currentPeriodStartAt":1792382400123`)) || !bytes.Contains(envelope.Data, []byte(`"currentPeriodEndAt":null`)) || !bytes.Contains(envelope.Data, []byte(`"pendingAmountMinor":null`))) {
				t.Fatal("free object acquired an invented bill or expiry")
			}
			if test.route == "traffic-packages" && !bytes.Contains(envelope.Data, []byte(`"trafficGb":100`)) {
				t.Fatal("count incorrectly converted to money string")
			}
		})
	}
}
