// This file verifies one selected discounted purchase across the complete local
// HTTPS source chain. It uses the source's real term and typed native plan; no
// server hardware plan or peer quote token is manufactured.
package finance

import (
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestNativeDiscountedPurchaseReadEvaluateTermBuildAndExecuteLocalHTTPS(t *testing.T) {
	data := catalogDetailFixture(t)
	data["flag"] = map[string]any{"type": 1, "bates": 85}
	data["products"].(map[string]any)["id"] = 42
	data["product_pricings"].([]any)[0].(map[string]any)["currency"] = 1
	option := data["config_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)
	option["id"] = 11
	option["sub"].([]any)[0].(map[string]any)["id"] = 102
	data["customfields"] = []any{map[string]any{"id": 51, "fieldname": "Purpose", "fieldtype": "textarea", "required": 1, "regexpr": ""}}
	supplier := newNativeFinanceSupplier(t)
	supplier.catalogue = mustCatalogJSON(t, map[string]any{"status": 200, "data": data})
	supplier.quote = json.RawMessage(`{"status":200,"currency":{"id":1,"code":"CNY"},"products":{"type":{"type":1,"bates":85},"signal_price":"14.84","signal_setupfee":"0.15","sale_signal_price":"12.61","sale_setupfee_total":"0.12"}}`)
	executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, secrets: map[string]string{"username": "merchant-user", "password": "merchant-password", "downstream_token": testDownstreamToken}}
	read, err := ExecuteDynamic(executor, "discounted-read", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"42"}`))
	if err != nil {
		t.Fatal(err)
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if json.Unmarshal(read, &detail) != nil || detail.ImportSnapshot == nil || detail.PriceScheduleComplete || detail.ImportSnapshot.PricingMode != channelcommercecap.ImportPricingRemoteEvaluation || len(detail.ImportSnapshot.Prices[0].Base) != 0 {
		t.Fatalf("discounted account became unavailable or raw prices authoritative %s", read)
	}
	text := "Customer purpose & value\nSecond line"
	input := channelcommercecap.ConfigurationInput{PublicSlug: "42", ConfigurationRevision: detail.ImportSnapshot.ConfigurationRevision, CurrencyCode: "CNY", Values: []channelcommercecap.PurchaseValue{{FieldToken: "config:11", Type: "single_select", OptionTokens: []string{"102"}}, {FieldToken: "custom:51", Type: "multi_line", TextValue: &text}}}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	evaluated, err := ExecuteDynamic(executor, "discounted-evaluate", resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, raw)
	if err != nil {
		t.Fatal(err)
	}
	var view channelcommercecap.ConfigurationEvaluation
	if json.Unmarshal(evaluated, &view) != nil || !view.Valid {
		t.Fatalf("selected source input invalid %s", evaluated)
	}
	quoted, err := ExecuteDynamic(executor, "discounted-billing", resourcecap.DynamicProviderMethodCatalogBillingTerms, raw)
	if err != nil {
		t.Fatal(err)
	}
	var terms channelcommercecap.BillingTerms
	if json.Unmarshal(quoted, &terms) != nil || terms.Terms[0].TotalAmount != "1273" {
		t.Fatalf("account quote lost %s", quoted)
	}
	plan := resourcecap.NativePurchasePlan{Version: resourcecap.NativePurchasePlanVersion, Protocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Purchase: channelcommercecap.Purchase{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, BillingTermToken: terms.Terms[0].BillingTermToken, CurrencyCode: terms.CurrencyCode, Quantity: 1, PurchaseCycles: 1, Values: view.Values}}
	payload, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := ExecuteDynamic(executor, "discounted-build", resourcecap.DynamicProviderMethodBuildProvisionParameters, payload)
	if err != nil {
		t.Fatal(err)
	}
	request := provisionDynamicRequest()
	request.Parameters = parameters
	payload, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	resultRaw, err := ExecuteDynamic(executor, "discounted-execute", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result resourcecap.OperationResult
	if json.Unmarshal(resultRaw, &result) != nil || result.Status != resourcecap.TaskStatusSucceeded || result.ExternalResourceRef != "91" {
		t.Fatalf("same selected purchase did not converge %s", resultRaw)
	}
	supplier.mu.Lock()
	defer supplier.mu.Unlock()
	if supplier.receivedQuote.Get("configoption[11]") != "102" || supplier.receivedQuote.Get("customfield[51]") != text || supplier.receivedAdd.Get("customfield[51]") != text || supplier.receivedAdd.Get("billingcycle") != plan.Purchase.BillingTermToken || supplier.receivedSettle.Get("cart_data[customfield][51]") != text || supplier.addCount != 1 || supplier.settleCount != 1 || supplier.creditCount != 1 || supplier.quoteCount != 1 {
		t.Fatal("source selections or actual term changed between quote and execution")
	}
}

func TestNativeCatalogPreviewQuotesOnlyValidExplicitStaffDefaultsOverLocalHTTPS(t *testing.T) {
	for _, scenario := range []string{"frozen read", "staff preview", "required customer input"} {
		t.Run(scenario, func(t *testing.T) {
			data := catalogDetailFixture(t)
			data["product_pricings"].([]any)[0].(map[string]any)["currency"] = 1
			if scenario == "required customer input" {
				data["customfields"] = []any{map[string]any{"id": 51, "fieldname": "Purpose", "fieldtype": "text", "required": 1, "regexpr": ""}}
			}
			supplier := newNativeFinanceSupplier(t)
			supplier.catalogue = mustCatalogJSON(t, map[string]any{"status": 200, "data": data})
			supplier.quote = json.RawMessage(`{"status":200,"currency":{"id":1,"code":"CNY"},"products":{"type":false,"signal_price":"45.67","signal_setupfee":"0.12"}}`)
			executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, secrets: map[string]string{"username": "merchant-user", "password": "merchant-password"}}
			query := resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41", IncludePricePreview: scenario != "frozen read"}
			payload, _ := json.Marshal(query)
			raw, err := ExecuteDynamic(executor, "preview", resourcecap.DynamicProviderMethodReadCatalogProduct, payload)
			if err != nil {
				t.Fatal(err)
			}
			var detail resourceinterfaceprojectioncap.CatalogDetail
			if json.Unmarshal(raw, &detail) != nil {
				t.Fatal("invalid detail")
			}
			assertNativeRemoteSchedule(t, &detail)
			supplier.mu.Lock()
			defer supplier.mu.Unlock()
			if supplier.addCount != 0 || supplier.settleCount != 0 || supplier.creditCount != 0 {
				t.Fatal("preview wrote supplier state")
			}
			if scenario == "staff preview" {
				if supplier.quoteCount != 1 || detail.PeerPurchase == nil || !detail.PeerPurchase.PricingReady || detail.PeerPurchase.BillingTerms[0].Terms[0].TotalAmount != "4579" {
					t.Fatalf("preview was repeated or raw instead of account quote: %s count=%d", raw, supplier.quoteCount)
				}
			} else if supplier.quoteCount != 0 {
				t.Fatal("unrequested or invalid default was quoted")
			} else if scenario == "frozen read" && detail.PeerPurchase != nil {
				t.Fatal("frozen read included a sample")
			} else if scenario == "required customer input" && (detail.PeerPurchase == nil || detail.PeerPurchase.PricingReady || len(detail.PeerPurchase.BillingTerms) > 0) {
				t.Fatal("missing customer input fabricated a zero price")
			}
		})
	}
}
