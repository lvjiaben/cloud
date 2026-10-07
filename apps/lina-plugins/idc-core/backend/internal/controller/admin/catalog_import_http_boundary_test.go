// This file verifies started HTTP binding preserves exact monetary strings and
// whole price collections, without GoFrame string-length rules on slice fields.

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"io"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"net/http"
	"strings"
	"testing"
)

type catalogImportHTTPStub struct {
	counts []int
	amount string
}

func (s *catalogImportHTTPStub) Import(_ context.Context, input catalog.ImportInput) (*catalog.ImportResult, error) {
	s.counts = append(s.counts, len(input.Prices))
	if len(input.Prices) > 0 {
		s.amount = input.Prices[0].Amount
	}
	return &catalog.ImportResult{Item: catalog.ProductDraft{ProductNo: "created", VersionNo: "draft"}}, nil
}

type catalogImportHTTPController struct{ target *ControllerV1 }

func (c *catalogImportHTTPController) Create(ctx context.Context, req *v1.CatalogProductImportReq) (*v1.CatalogProductImportRes, error) {
	return c.target.CatalogProductImport(ctx, req)
}

func TestCatalogImportHTTPCollectionBoundsAndDecimalText(t *testing.T) {
	service := &catalogImportHTTPStub{}
	server := g.Server("catalog-import-http-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/x/idc-core/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(func(request *ghttp.Request) {
			request.Middleware.Next()
			if code, ok := bizerr.As(request.GetError()); ok {
				request.Response.ClearBuffer()
				request.Response.WriteJson(g.Map{"errorCode": code.RuntimeCode()})
				return
			}
			request.Response.WriteJson(g.Map{"data": request.GetHandlerResponse()})
		})
		group.Bind(&catalogImportHTTPController{target: &ControllerV1{catalogImport: service}})
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	for _, count := range []int{1, 120, 121} {
		prices := make([]v1.CatalogImportPrice, count)
		for i := range prices {
			prices[i] = v1.CatalogImportPrice{CurrencyCode: "CNY", Period: "month", PeriodCount: 1, Amount: "92233720368547758.07", SetupFee: "0"}
		}
		body, err := json.Marshal(v1.CatalogProductImportReq{RequestKey: "http-import-000001", InterfaceInstanceNo: "pfi_" + strings.Repeat("1", 32), CategoryNo: "ctg_" + strings.Repeat("2", 32), Reference: "101", PricingMode: "manual", Prices: prices})
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.Post("http://127.0.0.1:"+strings.TrimPrefix(server.GetListenedAddress(), ":")+"/x/idc-core/api/v1/admin/catalog/product-imports", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		if count > 120 {
			if !strings.Contains(string(payload), catalog.CodeManagementInvalid.RuntimeCode()) {
				t.Fatalf("over-limit response: %s", payload)
			}
			continue
		}
		var output struct {
			Data v1.CatalogProductImportRes `json:"data"`
		}
		if err = json.Unmarshal(payload, &output); err != nil {
			t.Fatal(string(payload), err)
		}
		if output.Data.Item == nil || output.Data.Item.ProductNo != "created" {
			t.Fatalf("binding failed or duplicate item wrapper: %s", payload)
		}
	}
	if len(service.counts) != 2 || service.counts[0] != 1 || service.counts[1] != 120 || service.amount != "92233720368547758.07" {
		t.Fatalf("HTTP lost collection/decimal: %+v", service)
	}
}
