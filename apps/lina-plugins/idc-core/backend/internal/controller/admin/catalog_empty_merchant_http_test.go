// Real HTTP binding distinguishes an explicit empty configuration from a
// missing/null array without bypassing the owner's schema validation.
package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type emptyMerchantHTTPService struct {
	productautomation.Service
	saves, evaluations int
}

func (s *emptyMerchantHTTPService) SaveVersionConfiguration(context.Context, productautomation.SaveVersionConfigurationInput) error {
	s.saves++
	return nil
}
func (*emptyMerchantHTTPService) GetVersionConfiguration(context.Context, string) (*productautomation.VersionConfiguration, error) {
	return &productautomation.VersionConfiguration{}, nil
}
func (s *emptyMerchantHTTPService) GetMerchantConfiguration(ctx context.Context, versionNo string) (*productautomation.VersionConfiguration, error) {
	return s.GetVersionConfiguration(ctx, versionNo)
}
func (s *emptyMerchantHTTPService) EvaluateVersionConfiguration(context.Context, productautomation.EvaluateVersionConfigurationInput) (*productautomation.ConfigurationEvaluation, error) {
	s.evaluations++
	return &productautomation.ConfigurationEvaluation{}, nil
}

type emptyMerchantHTTPController struct{ target *ControllerV1 }

type emptyMerchantHTTPWriter struct {
	catalog.ManagementService
	writes int
}

func (s *emptyMerchantHTTPWriter) SaveConfigurationPriceBatch(_ context.Context, _ string, rows []catalog.ConfigurationPriceBatchRow, _ catalog.ConfigurationPriceTargetSource) error {
	if len(rows) != 1 {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	s.writes++
	return nil
}

func (c *emptyMerchantHTTPController) Update(ctx context.Context, req *v1.CatalogProductVersionConfigurationUpdateReq) (*v1.CatalogProductVersionConfigurationUpdateRes, error) {
	return c.target.CatalogProductVersionConfigurationUpdate(ctx, req)
}
func (c *emptyMerchantHTTPController) Evaluate(ctx context.Context, req *v1.CatalogProductVersionConfigurationEvaluateReq) (*v1.CatalogProductVersionConfigurationEvaluateRes, error) {
	return c.target.CatalogProductVersionConfigurationEvaluate(ctx, req)
}

func TestEmptyMerchantConfigurationHTTP(t *testing.T) {
	service := &emptyMerchantHTTPService{}
	writer := &emptyMerchantHTTPWriter{}
	server := g.Server("empty-merchant-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/x/idc-core/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(func(r *ghttp.Request) {
			r.Middleware.Next()
			if r.GetError() != nil {
				r.Response.ClearBuffer()
				if code, ok := bizerr.As(r.GetError()); ok {
					r.Response.Write(code.RuntimeCode())
				} else {
					r.Response.Write("invalid binding")
				}
				return
			}
			r.Response.Write("accepted")
		})
		group.Bind(&emptyMerchantHTTPController{target: &ControllerV1{productAutomation: service, catalogPublication: &emptyMerchantPublication{}, catalogWriter: writer}})
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	for _, route := range []struct{ method, path string }{{"PUT", "/configuration"}, {"POST", "/configuration/evaluation"}} {
		for _, body := range []string{`{"merchantValues":[]}`, `{}`, `{"merchantValues":null}`} {
			url := "http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":") + "/x/idc-core/api/v1/admin/catalog/product-versions/ver_" + strings.Repeat("1", 32) + route.path
			req, err := http.NewRequest(route.method, url, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			payload, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil {
				t.Fatal(readErr, closeErr)
			}
			want := catalog.CodeManagementInvalid.RuntimeCode()
			if body == `{"merchantValues":[]}` {
				want = "accepted"
			}
			if string(payload) != want {
				t.Fatalf("%s %s: %s", route.path, body, payload)
			}
		}
	}
	if service.saves != 1 || service.evaluations != 1 {
		t.Fatalf("invalid arrays reached owner: %+v", service)
	}
	// Array bounds must count rows, not the length of their serialized JSON.
	// One ordinary price row is already longer than 256 characters.
	for _, count := range []int{1, 257} {
		rows := make([]json.RawMessage, count)
		for index := range rows {
			rows[index] = json.RawMessage(`{"fieldKey":"local_validation_method","candidateValue":"dns","quantity":false,"pricingMode":"fixed","active":true,"previousPriceNo":"","billingRuleNo":"bil_11111111111111111111111111111111","currencyCode":"CNY","tiers":[{"minimumQuantity":0,"maximumQuantity":null,"amountMinor":"100","setupFeeMinor":"0","renewalAmountMinor":"100"}]}`)
		}
		body, err := json.Marshal(struct {
			MerchantValues []string          `json:"merchantValues"`
			PriceRows      []json.RawMessage `json:"priceRows"`
		}{MerchantValues: []string{}, PriceRows: rows})
		if err != nil {
			t.Fatal(err)
		}
		url := "http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":") + "/x/idc-core/api/v1/admin/catalog/product-versions/ver_" + strings.Repeat("1", 32) + "/configuration"
		req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(string(body)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		payload, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal(readErr, closeErr)
		}
		want := "accepted"
		if count > 256 {
			want = catalog.CodeManagementInvalid.RuntimeCode()
		}
		if string(payload) != want {
			t.Fatalf("%d price rows: %s", count, payload)
		}
	}
	if writer.writes != 1 || service.saves != 2 {
		t.Fatalf("price row bounds reached owners incorrectly: writes=%d saves=%d", writer.writes, service.saves)
	}
}

// emptyMerchantPublication isolates request binding from persistence in this HTTP test.
type emptyMerchantPublication struct{ catalog.PublicationService }

func (*emptyMerchantPublication) WithSavedSettings(ctx context.Context, _ string, save func(context.Context) error) error {
	return save(ctx)
}
