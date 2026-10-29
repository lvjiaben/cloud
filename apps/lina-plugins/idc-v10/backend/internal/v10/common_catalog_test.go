// These tests use a real local HTTPS server and source-shaped public responses.
// They verify protocol behavior, not a live third-party supplier deployment.
package v10

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const commonSource = `{"status":200,"data":{"common_product":{"pay_type":"recurring_prepayment"},"configoptions":[{"id":10,"option_name":"Disk","option_type":"quantity","qty_min":0,"qty_max":100,"allow_repeat":1,"max_repeat":2,"subs":[{"id":21,"option_name":"Disk","qty_min":0,"qty_max":100}]},{"id":11,"option_name":"Extras","option_type":"multi_select","subs":[{"id":31,"option_name":"A"},{"id":32,"option_name":"B"}]},{"id":12,"option_name":"OS","option_type":"os","subs":[{"os":"Linux","version":[{"id":41,"option_name":"Stable"}]}]}],"cycles":{},"custom_cycles":[{"id":3,"name":"Month","cycle_time":1,"cycle_unit":"month","cycle_amount":"50.00"}]}}`
const commonProductSource = `{"status":200,"data":{"product":{"id":81,"name":"Generic service","description":"Service","price":"0.01","password":"must-not-leak"},"self_defined_field":[{"id":5,"field_name":"Purpose","field_type":"text","is_required":1}]}}`

type commonHTTPExecutor struct {
	client  *http.Client
	baseURL string
}

func (e *commonHTTPExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ConfidentialHTTPRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	values := url.Values{}
	for key, value := range request.Form {
		values.Set(key, value)
	}
	endpoint := e.baseURL + request.Path
	query := url.Values(request.Query)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequest(request.Method, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if request.UseAuthentication {
		req.Header.Set("Authorization", "Bearer local-session")
	}
	response, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resourcecap.ConfidentialHTTPResponse{StatusCode: response.StatusCode, Authenticated: request.Kind == resourcecap.ConfidentialHTTPKindAuthenticate && response.StatusCode == 200, Data: raw})
}
func commonJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestCommonCatalogEvaluationAndAccountQuoteOverLocalHTTPS(t *testing.T) {
	var mu sync.Mutex
	calculations := 0
	purchases := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Error(err)
			response.WriteHeader(400)
			return
		}
		body := `{"status":200}`
		switch request.URL.Path {
		case "/api/v1/auth":
		case "/api/v1/product/81":
			body = commonProductSource
		case "/api/v1/product/81/resource":
			body = `{"status":200,"data":{"module":"idcsmart_common","url":"https://untrusted.invalid/module.zip","version":"1.0"}}`
		case "/api/v1/product":
			body = `{"status":200,"data":{"currency_code":"USD","list":[]}}`
		case "/console/v1/reidcsmart_common/product/81/configoption":
			body = commonSource
		case "/console/v1/reidcsmart_common/product/81/configoption/calculate":
			if request.Method != "POST" || request.Form.Get("is_downstream") != "1" || request.Form.Get("config_options[configoption][12]") != "41" {
				t.Errorf("invalid source account quote: %v", request.Form)
				response.WriteHeader(422)
				return
			}
			mu.Lock()
			calculations++
			mu.Unlock()
			body = `{"status":200,"data":{"custom_cycles":[{"id":3,"cycle_amount":"47.50","cycle_amount_client_level_discount":"2.50"}],"cycles":{}}}`
		case "/console/v1/cart":
			if request.Method == http.MethodPost {
				mu.Lock()
				purchases++
				mu.Unlock()
				if request.Form.Get("config_options[cycle]") != "3" || request.Form.Get("config_options[configoption][10][0]") != "7" || request.Form.Get("self_defined_field[5]") != "Example purpose" {
					t.Errorf("quoted selection changed in native cart %v", request.Form)
					response.WriteHeader(422)
					return
				}
			}
		case "/console/v1/cart/settle":
			body = `{"status":200,"data":{"order_id":71,"host_ids":[91],"amount":"45.00"}}`
		case "/console/v1/pay":
			body = `{"status":200,"data":{"host_ids":[91]}}`
		default:
			t.Errorf("unexpected supplier endpoint %s", request.URL.Path)
			response.WriteHeader(404)
			return
		}
		if request.URL.Path != "/api/v1/auth" && request.Header.Get("Authorization") != "Bearer local-session" {
			response.WriteHeader(401)
			return
		}
		if _, err := fmt.Fprint(response, body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	executor := &commonHTTPExecutor{client: server.Client(), baseURL: server.URL}
	raw, err := ExecuteDynamic(executor, "common-catalog", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"81"}`))
	if err != nil {
		t.Fatal(err)
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if json.Unmarshal(raw, &detail) != nil || detail.PeerPurchase == nil || detail.ImportSnapshot != nil || detail.PriceScheduleComplete {
		t.Fatalf("module price promoted to static schedule: %s", raw)
	}
	snapshot := detail.PeerPurchase
	if snapshot.Validate() != nil {
		t.Fatalf("invalid public snapshot: %s", raw)
	}
	if snapshot.Currencies[0].CurrencyCode != "USD" || snapshot.BillingTerms[0].Terms[0].TotalAmount != "4500" || len(snapshot.Configuration.Fields) != 5 {
		t.Fatalf("wrong evaluated snapshot: %s", raw)
	}
	for _, forbidden := range []string{"must-not-leak", "module.zip", "0.01"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("source private/starting price leaked: %s", raw)
		}
	}
	number := "7"
	text := "Example purpose"
	values := []channelcommercecap.PurchaseValue{{FieldToken: "config:10:0", Type: "integer", IntegerValue: &number}, {FieldToken: "config:11", Type: "multi_select", OptionTokens: []string{"31", "32"}}, {FieldToken: "config:12", Type: "single_select", OptionTokens: []string{"41"}}, {FieldToken: "custom:5", Type: "single_line", TextValue: &text}}
	input := channelcommercecap.ConfigurationInput{PublicSlug: "81", ConfigurationRevision: snapshot.Configuration.ConfigurationRevision, CurrencyCode: "USD", Values: values}
	evaluated, err := ExecuteDynamic(executor, "common-evaluate", resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, commonJSON(t, input))
	if err != nil {
		t.Fatal(err)
	}
	var evaluation channelcommercecap.ConfigurationEvaluation
	if json.Unmarshal(evaluated, &evaluation) != nil || !evaluation.Valid {
		t.Fatalf("source selection rejected %s", evaluated)
	}
	quote, err := ExecuteDynamic(executor, "common-billing", resourcecap.DynamicProviderMethodCatalogBillingTerms, commonJSON(t, input))
	if err != nil {
		t.Fatal(err)
	}
	var terms channelcommercecap.BillingTerms
	if json.Unmarshal(quote, &terms) != nil || terms.Validate() != nil || terms.Terms[0].TotalAmount != "4500" {
		t.Fatalf("account quote lost %s", quote)
	}
	plan := resourcecap.NativePurchasePlan{Version: resourcecap.NativePurchasePlanVersion, Protocol: productfulfillmentcap.SaleImportProtocolNativeV10CommonV1, Purchase: channelcommercecap.Purchase{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, CurrencyCode: terms.CurrencyCode, BillingTermToken: terms.Terms[0].BillingTermToken, Quantity: 1, PurchaseCycles: 1, Values: evaluation.Values}}
	parameters, err := ExecuteDynamic(executor, "common-build", resourcecap.DynamicProviderMethodBuildProvisionParameters, commonJSON(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	operation := resourcecap.DynamicOperationRequest{RequestID: "common-selected-purchase", Capability: resourcecap.CapabilityProvision, Parameters: parameters, ProviderCorrelation: &resourcecap.ProviderCorrelation{DownstreamURL: "https://merchant.example/api/plugins/idc-resource/provider-callback", DownstreamID: 55}, ConfidentialInputNames: []string{"downstream_token"}}
	completed, err := ExecuteDynamic(executor, "common-execute", resourcecap.DynamicProviderMethodExecute, commonJSON(t, operation))
	if err != nil {
		t.Fatal(err)
	}
	var completion resourcecap.OperationResult
	if json.Unmarshal(completed, &completion) != nil || completion.Status != resourcecap.TaskStatusSucceeded || completion.ExternalResourceRef != "91" {
		t.Fatalf("selected source purchase did not converge %s", completed)
	}
	values[0].IntegerValue = func() *string { s := "101"; return &s }()
	input.Values = values
	if _, err = ExecuteDynamic(executor, "common-invalid", resourcecap.DynamicProviderMethodCatalogBillingTerms, commonJSON(t, input)); err == nil {
		t.Fatal("out-of-range quantity reached supplier calculation")
	}
	mu.Lock()
	defer mu.Unlock()
	if purchases != 1 {
		t.Fatalf("selected native cart count=%d", purchases)
	}
	if calculations != 2 {
		t.Fatalf("invalid input or evaluation made extra quote calls: %d", calculations)
	}
}

func TestCommonRevisionRetainsSchemaAndTermsWithoutFreezingAmounts(t *testing.T) {
	var envelope struct {
		Data commonDefinition `json:"data"`
	}
	if json.Unmarshal([]byte(commonSource), &envelope) != nil {
		t.Fatal("bad source fixture")
	}
	first, err := commonSourceRevision(envelope.Data, nil, "USD")
	if err != nil {
		t.Fatal(err)
	}
	envelope.Data.CustomCycles[0].Amount = json.RawMessage(`"999.00"`)
	envelope.Data.CustomCycles[0].Discount = json.RawMessage(`"1.00"`)
	changedPrice, err := commonSourceRevision(envelope.Data, nil, "USD")
	if err != nil || changedPrice != first {
		t.Fatal("live source price changed configuration identity")
	}
	envelope.Data.CustomCycles[0].Time = json.RawMessage(`2`)
	changedTerm, err := commonSourceRevision(envelope.Data, nil, "USD")
	if err != nil || changedTerm == first {
		t.Fatal("source billing change kept stale identity")
	}
}
