// This file verifies the real native read/quote wire protocol and confirms that
// invalid selections cannot reach get_total or any purchase-writing endpoint.
package finance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestNativeBillingTermsUsesAuthenticatedAccountQuoteOverLocalHTTP(t *testing.T) {
	data := catalogDetailFixture(t)
	data["flag"] = map[string]any{"type": 1, "bates": 85}
	data["product_pricings"].([]any)[0].(map[string]any)["currency"] = 3
	source := mustCatalogJSON(t, map[string]any{"status": 200, "data": data})
	detail, err := projectCatalogDetail(source, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil {
		t.Fatalf("invalid test source: %v", err)
	}
	var (
		mu         sync.Mutex
		quoteCount int
		paths      []string
	)
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Error(err)
			response.WriteHeader(400)
			return
		}
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/zjmf_api_login":
			if request.Form.Get("username") != "merchant-user" {
				response.WriteHeader(401)
				return
			}
			writeNativeResponse(t, response, []byte(`{"status":200,"jwt":"supplier-session"}`))
		case "/cart/get_product_config":
			if request.Header.Get("Authorization") != "Bearer supplier-session" || request.URL.Query().Get("pid") != "41" {
				response.WriteHeader(403)
				return
			}
			writeNativeResponse(t, response, source)
		case "/cart/get_total":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer supplier-session" || request.Form.Get("pid") != "41" || request.Form.Get("configoption[10]") != "20" || request.Form.Get("currencyid") != "3" || request.Form.Get("qty") != "1" {
				response.WriteHeader(422)
				return
			}
			mu.Lock()
			quoteCount++
			mu.Unlock()
			writeNativeResponse(t, response, []byte(`{"status":200,"currency":{"id":3,"code":"CNY"},"products":{"type":{"type":1,"bates":85},"signal_price":"14.84","signal_setupfee":"0.15","sale_signal_price":"12.61","sale_setupfee_total":"0.12"}}`))
		default:
			t.Errorf("quote crossed a write endpoint: %s", request.URL.Path)
			response.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	executor := &tlsConfidentialExecutor{client: server.Client(), baseURL: server.URL, secrets: map[string]string{"username": "merchant-user", "password": "merchant-password"}}
	input := channelcommercecap.ConfigurationInput{PublicSlug: "41", ConfigurationRevision: detail.ImportSnapshot.ConfigurationRevision, CurrencyCode: "CNY"}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ExecuteDynamic(executor, "native-quote", resourcecap.DynamicProviderMethodCatalogBillingTerms, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result channelcommercecap.BillingTerms
	if json.Unmarshal(raw, &result) != nil || len(result.Terms) != 1 || result.Terms[0].TotalAmount != "1273" || result.Terms[0].SubtotalAmount != "1261" || result.Terms[0].SetupAmount != "12" {
		t.Fatalf("raw prices replaced actual account quote: %s", raw)
	}
	mu.Lock()
	defer mu.Unlock()
	if quoteCount != 1 || len(paths) != 4 {
		t.Fatalf("quote reads were unbounded or repeated: %v count=%d", paths, quoteCount)
	}
}

func TestNativeConfigurationRejectsUnknownOrOutOfRangeValues(t *testing.T) {
	data := catalogDetailFixture(t)
	detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil {
		t.Fatal(err)
	}
	unknown := []channelcommercecap.PurchaseValue{{FieldToken: "config:999", Type: "single_select", OptionTokens: []string{"20"}}}
	if _, err = evaluateNativeSelections(detail.ImportSnapshot, unknown); err == nil {
		t.Fatal("unknown upstream configuration entered quote")
	}
	invalid := []channelcommercecap.PurchaseValue{{FieldToken: "config:10", Type: "single_select", OptionTokens: []string{"999"}}}
	evaluated, err := evaluateNativeSelections(detail.ImportSnapshot, invalid)
	if err != nil || evaluated.Valid || len(evaluated.Issues) != 1 {
		t.Fatalf("invalid choice became a quoteable selection: %+v %v", evaluated, err)
	}
}

func writeNativeResponse(t *testing.T, response http.ResponseWriter, raw []byte) {
	t.Helper()
	if _, err := response.Write(raw); err != nil {
		t.Error(err)
	}
}

func TestNativeTreeBillingRestoresSelectedAncestorPathOverLocalHTTPS(t *testing.T) {
	data := nativeTreeFixture(t)
	data["product_pricings"].([]any)[0].(map[string]any)["currency"] = 3
	rawDefinition := mustCatalogJSON(t, map[string]any{"status": 200, "data": data})
	detail, err := projectCatalogDetail(rawDefinition, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Error(err)
			response.WriteHeader(400)
			return
		}
		switch request.URL.Path {
		case "/zjmf_api_login":
			writeNativeResponse(t, response, []byte(`{"status":200,"jwt":"supplier-session"}`))
		case "/cart/get_product_config":
			writeNativeResponse(t, response, rawDefinition)
		case "/cart/get_total":
			if request.Method != http.MethodPost || request.Form.Get("configoption[10]") != "20" || request.Form.Get("configoption[11]") != "30" || request.Form.Get("currencyid") != "3" {
				t.Errorf("tree branch lost in real quote request %v", request.Form)
				response.WriteHeader(422)
				return
			}
			writeNativeResponse(t, response, []byte(`{"status":200,"currency":{"id":3,"code":"CNY"},"products":{"type":false,"signal_price":"18.09","signal_setupfee":"0.25"}}`))
		default:
			t.Errorf("unexpected endpoint %s", request.URL.Path)
			response.WriteHeader(404)
		}
	}))
	defer server.Close()
	executor := &tlsConfidentialExecutor{client: server.Client(), baseURL: server.URL, secrets: map[string]string{"username": "merchant-user", "password": "merchant-password"}}
	input := channelcommercecap.ConfigurationInput{PublicSlug: "41", ConfigurationRevision: detail.ImportSnapshot.ConfigurationRevision, CurrencyCode: "CNY"}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ExecuteDynamic(executor, "tree-quote", resourcecap.DynamicProviderMethodCatalogBillingTerms, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result channelcommercecap.BillingTerms
	if json.Unmarshal(raw, &result) != nil || result.Terms[0].TotalAmount != "1834" {
		t.Fatalf("source tree account quote changed %s", raw)
	}
}
