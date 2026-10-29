// This file verifies native finance catalog boundaries, source-price fidelity,
// sanitized text and one-read behavior without accessing a real supplier.

package finance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestCatalogListUsesOneReadAndVisibleFilteredTotal(t *testing.T) {
	executor := &recordingExecutor{results: []resourcecap.ConfidentialHTTPResponse{
		{StatusCode: 200, Authenticated: true},
		{StatusCode: 200, Data: json.RawMessage(`{"status":200,"data":{"count":999,"currency":"CNY","products":[{"name":"Group","products":[{"id":1,"name":"Alpha","description":"<p>SSD</p>","password":"private"},{"id":2,"name":"Beta","description":"SSD"},{"id":3,"name":"Gamma","description":"HDD"}]}]}}`)},
	}}
	result, err := ExecuteDynamic(executor, "catalog-test", resourcecap.DynamicProviderMethodListCatalog, json.RawMessage(`{"page":2,"pageSize":1,"keyword":"ssd"}`))
	if err != nil {
		t.Fatal(err)
	}
	var page resourceinterfaceprojectioncap.CatalogPage
	if json.Unmarshal(result, &page) != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].Reference != "2" {
		t.Fatalf("wrong visible page: %s", result)
	}
	if len(executor.requests) != 2 || executor.requests[0].Path != "/zjmf_api_login" || executor.requests[1].Method != http.MethodGet || executor.requests[1].Path != "/cart/all" || len(executor.requests[1].Query) != 0 {
		t.Fatalf("catalog introduced extra supplier calls or fake paging: %+v", executor.requests)
	}
	if slices.Contains(executor.requests[1].ResponseFields, "password") || slices.Contains(executor.requests[1].ResponseFields, "product_pricings") || strings.Contains(string(result), "private") {
		t.Fatalf("list leaked private fields or requested per-product prices: %s", result)
	}
}

func TestCatalogListRejectsPartialDuplicateAndOversizedSnapshots(t *testing.T) {
	query := resourceinterfaceprojectioncap.CatalogQuery{Page: 1, PageSize: 10}
	for name, raw := range map[string]string{
		"business failure":       `{"status":400,"data":{"products":[]}}`,
		"missing products":       `{"status":200,"data":{}}`,
		"missing group products": `{"status":200,"data":{"products":[{}]}}`,
		"duplicate":              `{"status":200,"data":{"products":[{"products":[{"id":1,"name":"A"},{"id":1,"name":"B"}]}]}}`,
		"unsafe id":              `{"status":200,"data":{"products":[{"products":[{"id":"../1","name":"A"}]}]}}`,
		"empty name":             `{"status":200,"data":{"products":[{"products":[{"id":1,"name":"<script>secret</script>"}]}]}}`,
		"response limit":         strings.Repeat(" ", catalogMaxResponseBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := projectCatalogList(json.RawMessage(raw), query); err == nil {
				t.Fatal("unsafe catalog snapshot was accepted")
			}
		})
	}
	products := make([]map[string]any, maxCandidateItems+1)
	for index := range products {
		products[index] = map[string]any{"id": index + 1, "name": fmt.Sprintf("Product %d", index+1)}
	}
	raw := mustCatalogJSON(t, map[string]any{"status": 200, "data": map[string]any{"products": []any{map[string]any{"products": products}}}})
	if _, err := projectCatalogList(raw, query); err == nil {
		t.Fatal("too many products were silently truncated")
	}
	products = products[:maxCandidateItems]
	raw = mustCatalogJSON(t, map[string]any{"status": 200, "data": map[string]any{"products": []any{map[string]any{"products": products}}}})
	page, err := projectCatalogList(raw, resourceinterfaceprojectioncap.CatalogQuery{Page: 52, PageSize: 10})
	if err != nil || page.Total != maxCandidateItems || len(page.Items) != 2 {
		t.Fatalf("exact snapshot limit failed: page=%+v err=%v", page, err)
	}
}

func TestCatalogListEmptyAndOutOfRangePagesHaveAccurateTotals(t *testing.T) {
	for _, raw := range []string{
		`{"status":200,"data":{"count":7,"products":[]}}`,
		`{"status":200,"data":{"count":7,"products":[{"products":[]}]}}`,
	} {
		page, err := projectCatalogList(json.RawMessage(raw), resourceinterfaceprojectioncap.CatalogQuery{Page: 1, PageSize: 10})
		if err != nil || page.Total != 0 || page.Items == nil || len(page.Items) != 0 {
			t.Fatalf("empty catalog was not a real empty page: %+v %v", page, err)
		}
	}
	raw := json.RawMessage(`{"status":200,"data":{"products":[{"products":[{"id":1,"name":"A"}]}]}}`)
	page, err := projectCatalogList(raw, resourceinterfaceprojectioncap.CatalogQuery{Page: 2, PageSize: 10})
	if err != nil || page.Total != 1 || len(page.Items) != 0 {
		t.Fatalf("page beyond end was not empty: %+v %v", page, err)
	}
}

func TestCatalogDetailPreservesReadonlySchemaWithoutRawPricesWithOneRead(t *testing.T) {
	data := catalogDetailFixture(t)
	executor := &recordingExecutor{results: []resourcecap.ConfidentialHTTPResponse{
		{StatusCode: 200, Authenticated: true},
		{StatusCode: 200, Data: mustCatalogJSON(t, map[string]any{"status": 200, "data": data})},
	}}
	result, err := ExecuteDynamic(executor, "catalog-test", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"41"}`))
	if err != nil {
		t.Fatal(err)
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if json.Unmarshal(result, &detail) != nil || detail.Product.Name != "Native server" {
		t.Fatalf("invalid source detail: %s", result)
	}
	assertNativeRemoteSchedule(t, &detail)

	if len(detail.ProductConfiguration) != 2 || detail.ProductConfiguration[0].FieldKey != "upstream_product_id" || detail.ProductConfiguration[0].OptionValue == nil || *detail.ProductConfiguration[0].OptionValue != "41" {
		t.Fatalf("declared upstream product selection was not preserved: %s", result)
	}
	if strings.Contains(string(result), "secret") || strings.Contains(string(result), "cpu") || strings.Contains(string(result), "<script") {
		t.Fatalf("private HTML or guessed hardware reached DTO: %s", result)
	}
	if len(executor.requests) != 2 || executor.requests[1].Path != "/cart/get_product_config" || executor.requests[1].Query["pid"][0] != "41" || executor.requests[1].Method != http.MethodGet {
		t.Fatalf("detail was not one product-scoped read: %+v", executor.requests)
	}
	for _, field := range []string{"product_pricings", "pricings", "code", "monthly", "msetupfee"} {
		if !slices.Contains(executor.requests[1].ResponseFields, field) {
			t.Fatalf("price field %s was not requested", field)
		}
	}
	for _, field := range []string{"password", "username", "jwt", "server_group", "msg"} {
		if slices.Contains(executor.requests[1].ResponseFields, field) {
			t.Fatalf("unneeded supplier field %s was requested", field)
		}
	}
}

func TestCatalogRecurringColumnsKeepTheirExactIntervalsAndFees(t *testing.T) {
	for _, test := range []struct {
		amount string
		fee    string
		period resourceinterfaceprojectioncap.CatalogPeriod
		count  int
	}{
		{"hour", "hsetupfee", resourceinterfaceprojectioncap.CatalogPeriodHour, 1},
		{"day", "dsetupfee", resourceinterfaceprojectioncap.CatalogPeriodDay, 1},
		{"quarterly", "qsetupfee", resourceinterfaceprojectioncap.CatalogPeriodMonth, 3},
		{"semiannually", "ssetupfee", resourceinterfaceprojectioncap.CatalogPeriodMonth, 6},
		{"biennially", "bsetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 2},
		{"tenly", "tensetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 10},
	} {
		t.Run(test.amount, func(t *testing.T) {
			row := map[string]json.RawMessage{"code": json.RawMessage(`"CNY"`), test.amount: json.RawMessage(`"18.50"`), test.fee: json.RawMessage(`"3.20"`)}
			prices, complete, err := projectCatalogPrices([]map[string]json.RawMessage{row}, catalogPaymentRecurring)
			if err != nil || complete || len(prices) != 1 || prices[0].Period != test.period || prices[0].PeriodCount != test.count || prices[0].Amount != "18.50" || prices[0].SetupFee != "3.20" {
				t.Fatalf("wrong source interval mapping: %+v complete=%v err=%v", prices, complete, err)
			}
		})
	}
}

func TestCatalogDetailDoesNotInventMissingPricesOrTrialPeriods(t *testing.T) {
	for _, name := range []string{"missing fee", "missing cycle", "trial", "unknown pay type", "missing pay type", "missing option prices"} {
		t.Run(name, func(t *testing.T) {
			data := catalogDetailFixture(t)
			row := data["product_pricings"].([]any)[0].(map[string]any)
			product := data["products"].(map[string]any)
			switch name {
			case "missing fee":
				delete(row, "msetupfee")
			case "missing cycle":
				delete(row, "hour")
			case "trial":
				product["pay_type"] = `{"pay_type":"recurring","pay_ontrial_status":1,"pay_ontrial_cycle":7}`
				row["ontrial"], row["ontrialfee"] = "1.00", "0.00"
			case "unknown pay type":
				product["pay_type"] = `{"pay_type":"free"}`
			case "missing pay type":
				delete(product, "pay_type")
			case "missing option prices":
				groups := data["config_groups"].([]any)
				option := groups[0].(map[string]any)["options"].([]any)[0].(map[string]any)
				delete(option["sub"].([]any)[0].(map[string]any), "pricings")
			}
			detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
			if err != nil || detail.PriceScheduleComplete {
				t.Fatalf("incomplete source became a complete schedule: %+v %v", detail, err)
			}
			if name == "missing fee" && len(detail.BasePrices) != 0 {
				t.Fatal("missing installation fee became zero")
			}
			if (name == "unknown pay type" || name == "missing pay type") && len(detail.BasePrices) != 0 {
				t.Fatal("unknown billing metadata became a usable schedule")
			}
		})
	}
}

func TestCatalogDetailRejectsMismatchedIdentityAndMalformedPriceEvidence(t *testing.T) {
	for _, name := range []string{"wrong product", "negative price", "exponent", "missing currency", "duplicate price", "duplicate option", "negative setup", "malformed billing"} {
		t.Run(name, func(t *testing.T) {
			data := catalogDetailFixture(t)
			row := data["product_pricings"].([]any)[0].(map[string]any)
			switch name {
			case "wrong product":
				data["products"].(map[string]any)["id"] = 42
			case "negative price":
				row["monthly"] = "-2.00"
			case "exponent":
				row["monthly"] = json.Number("1e4")
			case "missing currency":
				delete(row, "code")
			case "duplicate price":
				data["product_pricings"] = []any{row, row}
			case "duplicate option":
				group := data["config_groups"].([]any)[0].(map[string]any)
				option := group["options"].([]any)[0]
				group["options"] = []any{option, option}
			case "negative setup":
				row["msetupfee"] = "-1"
			case "malformed billing":
				data["products"].(map[string]any)["pay_type"] = `{"pay_type":`
			}
			if _, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"}); err == nil {
				t.Fatal("invalid source evidence was accepted")
			}
		})
	}
}

func TestCatalogOnceDoesNotExposeInactiveRecurringPrices(t *testing.T) {
	data := catalogDetailFixture(t)
	data["products"].(map[string]any)["pay_type"] = `{"pay_type":"onetime"}`
	data["config_groups"] = []any{}
	row := data["product_pricings"].([]any)[0].(map[string]any)
	row["onetime"], row["osetupfee"] = "0.00", "5.00"
	detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil || len(detail.ImportSnapshot.Prices) != 1 || detail.ImportSnapshot.Prices[0].ChargeType != "one_time" || len(detail.BasePrices) > 0 {
		t.Fatalf("inactive recurring fields leaked into the schedule: %+v %v", detail, err)
	}
}

func TestCatalogTextRemovesExecutableMarkupAttributesAndControls(t *testing.T) {
	text, err := catalogPlainText(`<div title="secret-attribute">Visible<script>secret-script</script><style>secret-style</style><!--secret-comment--><iframe>secret-frame</iframe><b>text</b>&lt;img src=x&gt;`+"\n\x01"+`</div>`, 256, false)
	if err != nil || strings.Contains(text, "secret") || strings.ContainsAny(text, "<>\n\x01") || !strings.Contains(text, "Visible text") {
		t.Fatalf("unsafe text projection: %q %v", text, err)
	}
}

// catalogDetailFixture returns fresh source-shaped maps for each test. All
// finance recurring columns are present; only monthly is configured for sale.
func catalogDetailFixture(t *testing.T) map[string]any {
	t.Helper()
	base := map[string]any{"code": "CNY"}
	choice := map[string]any{"code": "CNY"}
	for _, cycle := range catalogCycles {
		base[cycle.amount], choice[cycle.amount] = "-1.00", "-1.00"
		base[cycle.setup], choice[cycle.setup] = "0.00", "0.00"
	}
	base["monthly"], base["msetupfee"] = "12.3456", "0.15"
	choice["monthly"] = "2.50"
	return map[string]any{
		"flag":             false,
		"customfields":     []any{},
		"advanced":         []any{},
		"products":         map[string]any{"id": 41, "name": "<b>Native server</b>", "description": "<p>Clean <script>secret</script>description</p>", "pay_type": `{"pay_type":"recurring"}`, "password": "secret"},
		"product_pricings": []any{base},
		"config_groups":    []any{map[string]any{"id": 1, "options": []any{map[string]any{"id": 10, "option_name": "Custom choice", "option_type": 1, "sub": []any{map[string]any{"id": 20, "option_name": "Default", "pricings": []any{choice}}}}}}},
	}
}

// mustCatalogJSON encodes only test fixtures and fails within the owning test.
func mustCatalogJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
