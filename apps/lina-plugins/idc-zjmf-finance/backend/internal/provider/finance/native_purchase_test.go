// This file checks arbitrary native selections and text fields through the
// builder and the real local HTTPS cart/settlement transport, including retries.
package finance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestNativePurchaseRestoresIDsZeroQuantitiesAndCustomText(t *testing.T) {
	purchase := nativePurchaseFixture()
	plan := resourcecap.NativePurchasePlan{Protocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Version: resourcecap.NativePurchasePlanVersion, Purchase: purchase}
	payload, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ExecuteDynamic(nil, "builder", resourcecap.DynamicProviderMethodBuildProvisionParameters, payload)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := decodePurchaseParameters(raw)
	if err != nil || parameters.Workflow != "serialized_cart_v1" || parameters.ProductID != "42" || parameters.ConfigOptions["11"] != "102" || parameters.ConfigOptions["12"] != "0" || parameters.CustomFields["51"] != "客户备注 & + = 多行\n第二行" {
		t.Fatalf("native frozen values were lost: %+v %v", parameters, err)
	}
	intent := validProvisionIntent()
	intent.Parameters = raw
	add, _, err := BuildAddToCartCall(intent, "3")
	if err != nil || add.Form.Get("customfield[51]") != parameters.CustomFields["51"] {
		t.Fatalf("cart custom field lost: %+v %v", add, err)
	}
	settle, _, err := BuildSettleCall(intent)
	if err != nil || settle.Form.Get("cart_data[customfield][51]") != parameters.CustomFields["51"] {
		t.Fatalf("settlement custom field lost: %+v %v", settle, err)
	}
}

func TestNativePurchaseSupportsProductsWithoutConfigurationOptions(t *testing.T) {
	purchase := nativePurchaseFixture()
	purchase.Values = nil
	raw, err := BuildNativePurchaseParameters(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}
	intent := validProvisionIntent()
	intent.Parameters = raw
	add, _, err := BuildAddToCartCall(intent, "3")
	if err != nil || add.Form.Get("pid") != "42" {
		t.Fatalf("option-free native product cannot be ordered: %+v %v", add, err)
	}
}

func TestNativePurchaseRejectsMixedOrInjectedFieldValues(t *testing.T) {
	for _, scenario := range []string{"field injection", "mixed arms", "wrong config kind", "multiple options", "negative quantity", "duplicate field", "purchase cycles"} {
		t.Run(scenario, func(t *testing.T) {
			purchase := nativePurchaseFixture()
			switch scenario {
			case "field injection":
				purchase.Values[0].FieldToken = "config:11][pid"
			case "mixed arms":
				text := "102"
				purchase.Values[0].TextValue = &text
			case "wrong config kind":
				purchase.Values[0].Type = capmodel.ConfigurationFieldSingleLine
			case "multiple options":
				purchase.Values[0].OptionTokens = []string{"102", "103"}
			case "negative quantity":
				negative := "-1"
				purchase.Values[1].IntegerValue = &negative
			case "duplicate field":
				purchase.Values = append(purchase.Values, purchase.Values[0])
			case "purchase cycles":
				purchase.PurchaseCycles = 2
			}
			if _, err := BuildNativePurchaseParameters(context.Background(), purchase); err == nil {
				t.Fatal("unrepresentable native purchase was accepted")
			}
		})
	}
}

func TestNativePurchaseLocalHTTPPreservesFieldsAndDoesNotResettle(t *testing.T) {
	supplier := newNativeFinanceSupplier(t)
	executor := &tlsConfidentialExecutor{client: supplier.server.Client(), baseURL: supplier.server.URL, secrets: map[string]string{"username": "merchant-user", "password": "merchant-password", "downstream_token": testDownstreamToken}}
	raw, err := BuildNativePurchaseParameters(context.Background(), nativePurchaseFixture())
	if err != nil {
		t.Fatal(err)
	}
	request := provisionDynamicRequest()
	request.Parameters = raw
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ExecuteDynamic(executor, "native-http", resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result resourcecap.OperationResult
	if json.Unmarshal(first, &result) != nil || result.Status != resourcecap.TaskStatusSucceeded || supplier.receivedAdd.Get("configoption[12]") != "0" || supplier.receivedAdd.Get("customfield[51]") != "客户备注 & + = 多行\n第二行" || supplier.receivedSettle.Get("cart_data[customfield][51]") != "客户备注 & + = 多行\n第二行" {
		t.Fatalf("actual HTTP discarded the native purchase: %+v add=%v settle=%v", result, supplier.receivedAdd, supplier.receivedSettle)
	}
	if _, err = ExecuteDynamic(executor, "native-http-retry", resourcecap.DynamicProviderMethodExecute, payload); err != nil {
		t.Fatal(err)
	}
	if supplier.addCount != 1 || supplier.settleCount != 1 || supplier.creditCount != 1 {
		t.Fatalf("native retry purchased twice: add=%d settle=%d credit=%d", supplier.addCount, supplier.settleCount, supplier.creditCount)
	}
}

func nativePurchaseFixture() channelcommercecap.Purchase {
	zero := "0"
	text := "客户备注 & + = 多行\n第二行"
	return channelcommercecap.Purchase{PublicSlug: "42", ConfigurationRevision: strings.Repeat("a", 64), BillingTermToken: "monthly", CurrencyCode: "CNY", PurchaseCycles: 1, Quantity: 1, Values: []channelcommercecap.PurchaseValue{{FieldToken: "config:11", Type: capmodel.ConfigurationFieldSingleSelect, OptionTokens: []string{"102"}}, {FieldToken: "config:12", Type: capmodel.ConfigurationFieldInteger, IntegerValue: &zero}, {FieldToken: "custom:51", Type: capmodel.ConfigurationFieldMultiLine, TextValue: &text}}}
}
