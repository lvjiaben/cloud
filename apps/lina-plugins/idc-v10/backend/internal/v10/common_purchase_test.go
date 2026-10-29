// These tests verify generic native mappings and query-first retry behavior on
// isolated local HTTPS endpoints, without performing any supplier transaction.
package v10

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestCommonNativePurchaseKeepsCycleRepeatedQuantityCustomFieldsAndRetry(t *testing.T) {
	quantity := "7"
	zero := "0"
	purpose := "Use / purpose & value"
	plan := resourcecap.NativePurchasePlan{Protocol: productfulfillmentcap.SaleImportProtocolNativeV10CommonV1, Version: resourcecap.NativePurchasePlanVersion, Purchase: channelcommercecap.Purchase{PublicSlug: "81", ConfigurationRevision: strings.Repeat("a", 64), BillingTermToken: "3", CurrencyCode: "USD", PurchaseCycles: 1, Quantity: 1, Values: []channelcommercecap.PurchaseValue{{FieldToken: "config:10:0", Type: "integer", IntegerValue: &quantity}, {FieldToken: "config:10:1", Type: "integer", IntegerValue: &zero}, {FieldToken: "config:11", Type: "multi_select", OptionTokens: []string{"31", "32"}}, {FieldToken: "custom:5", Type: "single_line", TextValue: &purpose}}}}
	parameters, err := BuildNativePurchaseParameters(plan)
	if err != nil {
		t.Fatal(err)
	}
	var frozen commonPurchaseParameters
	if json.Unmarshal(parameters, &frozen) != nil || frozen.Workflow != "console_cart_v1" {
		t.Fatal("native task cannot satisfy real capability constraints")
	}
	writes, payments := 0, 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Error(err)
			response.WriteHeader(400)
			return
		}
		if request.Method == http.MethodDelete {
			raw, readErr := io.ReadAll(request.Body)
			if readErr != nil {
				t.Error(readErr)
			}
			form, parseErr := url.ParseQuery(string(raw))
			if parseErr != nil {
				t.Error(parseErr)
			}
			request.Form = form
		}
		body := `{"status":200}`
		switch request.URL.Path {
		case "/api/v1/auth":
		case "/console/v1/cart":
			if request.Method == http.MethodDelete {
				if request.Form.Get("downstream_host_id") != "55" {
					t.Errorf("missing correlation: %v", request.Form)
				}
				if writes > 0 {
					body = `{"status":200,"data":{"order_id":71}}`
				}
			} else {
				writes++
				for key, want := range map[string]string{"product_id": "81", "qty": "1", "config_options[cycle]": "3", "config_options[configoption][10][0]": "7", "config_options[configoption][10][1]": "0", "config_options[configoption][11][0]": "31", "config_options[configoption][11][1]": "32", "self_defined_field[5]": purpose, "is_downstream": "1"} {
					if request.Form.Get(key) != want {
						t.Errorf("frozen mapping %s got %q want %q", key, request.Form.Get(key), want)
					}
				}
				if request.Form.Get("downstream_client_id") != "" {
					t.Error("native product invented a supplier customer")
				}
			}
		case "/console/v1/cart/settle":
			body = `{"status":200,"data":{"order_id":71,"host_ids":[91],"amount":"12.00"}}`
		case "/console/v1/pay":
			payments++
			body = `{"status":200,"data":{"host_ids":[91]}}`
		default:
			t.Errorf("unexpected endpoint %s", request.URL.Path)
			response.WriteHeader(404)
			return
		}
		if _, err := fmt.Fprint(response, body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	executor := &commonHTTPExecutor{client: server.Client(), baseURL: server.URL}
	request := resourcecap.DynamicOperationRequest{RequestID: "request-native-common", Capability: resourcecap.CapabilityProvision, Parameters: parameters, ProviderCorrelation: &resourcecap.ProviderCorrelation{DownstreamURL: "https://merchant.example/api/plugins/idc-resource/provider-callback", DownstreamID: 55}, ConfidentialInputNames: []string{"downstream_token"}}
	for index := 0; index < 2; index++ {
		raw, executeErr := ExecuteDynamic(executor, fmt.Sprint("native-", index), resourcecap.DynamicProviderMethodExecute, commonJSON(t, request))
		if executeErr != nil {
			t.Fatal(executeErr)
		}
		var result resourcecap.OperationResult
		if json.Unmarshal(raw, &result) != nil || result.ExternalResourceRef != "91" {
			t.Fatalf("generic native result lost %s", raw)
		}
	}
	if writes != 1 || payments != 2 {
		t.Fatalf("retry recreated cart instead of converging existing order: cart=%d pay=%d", writes, payments)
	}
}
func TestCommonNativeBuilderRejectsMixedValueArmsAndParameterInjection(t *testing.T) {
	number := "1"
	text := "x"
	for _, value := range []channelcommercecap.PurchaseValue{{FieldToken: "config:10", Type: "integer", IntegerValue: &number, TextValue: &text}, {FieldToken: "config:10][evil", Type: "integer", IntegerValue: &number}, {FieldToken: "custom:5:1", Type: "single_line", TextValue: &text}, {FieldToken: "config:10:32", Type: "integer", IntegerValue: &number}} {
		if _, err := commonConfigurationForm([]channelcommercecap.PurchaseValue{value}); err == nil {
			t.Fatalf("unsafe form mapping accepted: %+v", value)
		}
	}
}
