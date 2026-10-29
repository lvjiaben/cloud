// These tests exercise V10's actual catalog projection and confidential request
// boundary with source-shaped, fictional data. No external supplier is called.

package v10

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// catalogExecutor provides isolated responses while recording every host call.
type catalogExecutor struct {
	steps        []recordedStep
	data         json.RawMessage
	status       int
	authenticate bool
}

func (e *catalogExecutor) Execute(_ string, sequence uint8, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ConfidentialHTTPRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	e.steps = append(e.steps, recordedStep{sequence: sequence, request: request})
	response := resourcecap.ConfidentialHTTPResponse{StatusCode: e.status, Data: e.data}
	if request.Kind == resourcecap.ConfidentialHTTPKindAuthenticate {
		response = resourcecap.ConfidentialHTTPResponse{StatusCode: http.StatusOK, Authenticated: e.authenticate}
	}
	return json.Marshal(response)
}

func TestCatalogListUsesOneV10ReadAndExactFilteredPagination(t *testing.T) {
	// Official ProductController::product returns a full data.list with no total
	// or paging parameter. Extra count/price/private fields must not be trusted.
	executor := &catalogExecutor{status: http.StatusOK, authenticate: true, data: json.RawMessage(`{"status":200,"msg":"secret-message","data":{"count":999,"currency_code":"CNY","list":[{"id":103,"name":"<b>Cloud C</b>","description":"<p>Shared pool</p>","price":"9.99","cycle":"month"},{"id":102,"name":"Cloud B","description":"Shared pool"},{"id":101,"name":"Other","description":"private pool"}]}}`)}
	raw, err := ExecuteDynamic(executor, "v10-catalog-list", resourcecap.DynamicProviderMethodListCatalog, json.RawMessage(`{"page":2,"pageSize":1,"keyword":"CLOUD"}`))
	if err != nil {
		t.Fatal(err)
	}
	var page resourceinterfaceprojectioncap.CatalogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	query := resourceinterfaceprojectioncap.CatalogQuery{Page: 2, PageSize: 1, Keyword: "CLOUD"}
	if page.Validate(query) != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].Reference != "102" {
		t.Fatalf("incorrect visible pagination: %s", raw)
	}
	if len(executor.steps) != 2 || executor.steps[0].request.Path != "/api/v1/auth" {
		t.Fatalf("unexpected request count: %+v", executor.steps)
	}
	step := executor.steps[1]
	if step.sequence != 2 || step.request.Method != http.MethodGet || step.request.Path != "/api/v1/product" ||
		!step.request.UseAuthentication || len(step.request.Query) != 0 || len(step.request.Form) != 0 || len(step.request.SecretFormFields) != 0 {
		t.Fatalf("catalog read changed the proven endpoint: %+v", step)
	}
	if !slices.Equal(step.request.ResponseFields, []string{"status", "data", "list", "id", "name", "description"}) {
		t.Fatalf("catalog list response allowlist expanded: %v", step.request.ResponseFields)
	}
	for _, forbidden := range []string{"secret-message", "currency_code", "price", "count"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("catalog exposed %s: %s", forbidden, raw)
		}
	}
}

func TestCatalogDetailDoesNotPromoteV10StartingPriceToCompleteSchedule(t *testing.T) {
	executor := &catalogExecutor{status: http.StatusOK, authenticate: true, data: json.RawMessage(`{"status":200,"data":{"product":{"id":101,"name":"Cloud","description":"<p>Service</p>","price":12.345,"pay_type":"recurring_prepayment","cycle":"月","auto_setup":1,"password":"private-password"},"self_defined_field":[{"field_type":"password","field_name":"hidden-secret"}],"config_groups":[{"cpu":16}]}}`)}
	raw, err := ExecuteDynamic(executor, "v10-catalog-detail", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"101"}`))
	if err != nil {
		t.Fatal(err)
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Validate(resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "101"}) != nil ||
		detail.Product.Description != "Service" || detail.PriceScheduleComplete || len(detail.BasePrices) != 0 || len(detail.Options) != 0 {
		t.Fatalf("V10 detail invented full pricing or configuration: %s", raw)
	}
	if len(detail.ProductConfiguration) != 1 || detail.ProductConfiguration[0].FieldKey != "upstream_product_id" ||
		detail.ProductConfiguration[0].OptionValue == nil || *detail.ProductConfiguration[0].OptionValue != "101" {
		t.Fatalf("declared upstream product selection was not preserved: %s", raw)
	}
	if len(executor.steps) != 3 || executor.steps[2].request.Path != "/api/v1/product/101/resource" || executor.steps[1].request.Path != "/api/v1/product/101" || executor.steps[1].request.Method != http.MethodGet {
		t.Fatalf("detail fetched additional/module/write endpoints: %+v", executor.steps)
	}
	if !slices.Equal(executor.steps[1].request.ResponseFields, commonProductResponseFields) {
		t.Fatalf("detail response allowlist expanded: %v", executor.steps[1].request.ResponseFields)
	}
	for _, forbidden := range []string{"12.345", "private-password", "hidden-secret", "config_groups", "cpu"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("unexpected supplier detail leaked: %s", raw)
		}
	}
}

func TestCatalogSnapshotsRejectMissingDuplicateOversizedOrFinanceShapes(t *testing.T) {
	query := resourceinterfaceprojectioncap.CatalogQuery{Page: 1, PageSize: 100}
	for _, raw := range []string{
		`{}`, `{"status":200,"data":{}}`, `{"status":200,"data":{"list":null}}`,
		`{"status":401,"data":{"list":[],"status":200}}`,
		`{"status":200,"data":{"products":[{"products":[{"id":1,"name":"Finance"}]}]}}`,
		`{"status":200,"data":{"list":[{"id":1,"name":"A"},{"id":"1","name":"B"}]}}`,
		`{"status":200,"data":{"list":[{"id":"01","name":"A"}]}}`,
		`{"status":200,"data":{"list":[{"id":1,"name":"<script>secret</script>"}]}}`,
		strings.Repeat(" ", catalogMaxResponseBytes+1),
	} {
		if _, err := projectCatalogList(json.RawMessage(raw), query); err == nil {
			t.Fatalf("unsafe list accepted: %.150s", raw)
		}
	}
	products := make([]catalogProductWire, catalogMaxProducts+1)
	for index := range products {
		products[index] = catalogProductWire{ID: json.RawMessage(fmt.Sprint(index + 1)), Name: "Cloud"}
	}
	encoded, err := json.Marshal(map[string]any{"status": 200, "data": map[string]any{"list": products}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectCatalogList(encoded, query); err == nil {
		t.Fatal("oversized snapshot silently truncated")
	}
	encoded, err = json.Marshal(map[string]any{"status": 200, "data": map[string]any{"list": products[:catalogMaxProducts]}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := projectCatalogList(encoded, query)
	if err != nil || page.Total != catalogMaxProducts || len(page.Items) != 100 {
		t.Fatalf("maximum bounded snapshot rejected: page=%+v err=%v", page, err)
	}
	for _, pageNumber := range []int{1, 999} {
		page, err := projectCatalogList(json.RawMessage(`{"status":200,"data":{"list":[]}}`), resourceinterfaceprojectioncap.CatalogQuery{Page: pageNumber, PageSize: 20})
		if err != nil || page.Total != 0 || page.Items == nil || len(page.Items) != 0 {
			t.Fatalf("empty visible snapshot was not preserved: %+v err=%v", page, err)
		}
	}
}

func TestCatalogDetailRejectsWrongOrUnavailableSelection(t *testing.T) {
	query := resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "101"}
	for _, raw := range []string{
		`{"status":200,"data":{"product":{}}}`, `{"status":200,"data":{"product":null}}`,
		`{"status":200,"data":{"product":{"id":102,"name":"Different"}}}`,
		`{"status":403,"data":{"product":{"id":101,"name":"Hidden"}}}`,
		`{"status":200,"data":{"products":{"id":101,"name":"Finance structure"}}}`,
	} {
		if _, err := projectCatalogDetail(json.RawMessage(raw), query); err == nil {
			t.Fatalf("unavailable/different product accepted: %s", raw)
		}
	}
}

func TestCatalogInvalidInputsAndUpstreamFailureNeverProduceData(t *testing.T) {
	for _, input := range []struct{ method, payload string }{
		{resourcecap.DynamicProviderMethodListCatalog, `{"page":0,"pageSize":10}`},
		{resourcecap.DynamicProviderMethodListCatalog, `{"page":1,"pageSize":101}`},
		{resourcecap.DynamicProviderMethodListCatalog, `{"page":1,"pageSize":10,"query":"unexpected"}`},
		{resourcecap.DynamicProviderMethodReadCatalogProduct, `{"reference":"101/../102"}`},
		{resourcecap.DynamicProviderMethodReadCatalogProduct, `{"reference":"101"} {}`},
	} {
		executor := &catalogExecutor{authenticate: true}
		if data, err := ExecuteDynamic(executor, "invalid", input.method, json.RawMessage(input.payload)); err == nil || data != nil || len(executor.steps) != 1 {
			t.Fatalf("invalid selection reached product API: data=%s err=%v calls=%+v", data, err, executor.steps)
		}
	}
	for _, executor := range []*catalogExecutor{
		{authenticate: false},
		{authenticate: true, status: 502},
		{authenticate: true, status: 200, data: json.RawMessage(strings.Repeat(" ", catalogMaxResponseBytes+1))},
	} {
		if data, err := ExecuteDynamic(executor, "failure", resourcecap.DynamicProviderMethodListCatalog, json.RawMessage(`{"page":1,"pageSize":10}`)); err == nil || data != nil {
			t.Fatalf("unproven supplier response returned success: %s %v", data, err)
		}
	}
}

func TestCatalogTextStripsMarkupScriptsAttributesAndEmbeddedSecrets(t *testing.T) {
	input := `<b data-token="attribute-secret">Visible</b><script>script-secret</script><style>style-secret</style><iframe>frame-secret</iframe><object>object-secret</object><template>template-secret</template><!--comment-secret--><p>Text &amp; more</p>`
	result, err := catalogPlainText(input, 256, false)
	if err != nil || result != "Visible Text & more" {
		t.Fatalf("unsafe presentation output: %q %v", result, err)
	}
	result, err = catalogPlainText("&lt;b&gt;literal&lt;/b&gt;\n\tend", 256, false)
	if err != nil || strings.ContainsAny(result, "<>\n\t") {
		t.Fatalf("tag or control characters escaped normalization: %q %v", result, err)
	}
	for _, input := range []string{strings.Repeat("文", 257), string([]byte{0xff}), strings.Repeat(" ", 64<<10+1)} {
		if _, err := catalogPlainText(input, 256, false); err == nil {
			t.Fatal("invalid or excessive text accepted")
		}
	}
}

func TestCatalogErrorTranslationsMatchStableCodes(t *testing.T) {
	for _, locale := range []string{"en-US", "zh-CN"} {
		data, err := os.ReadFile("../../../manifest/i18n/" + locale + "/plugin.json")
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{codeCatalogInvalid.MessageKey(), codeCatalogUnavailable.MessageKey()} {
			if messages[key] == "" || messages[key] == key {
				t.Fatalf("catalog error translation missing in %s: %s", locale, key)
			}
		}
	}
}
