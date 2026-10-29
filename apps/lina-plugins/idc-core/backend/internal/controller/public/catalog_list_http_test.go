// HTTP binding tests preserve bracketed slug arrays and return complete anonymous card projections.

package public

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	v1 "lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// catalogListHTTPReader captures one batched request rather than per-product detail calls.
type catalogListHTTPReader struct {
	catalog.PublicCatalogReader
	input catalog.PublicCatalogSearchInput
	calls int
}

func (s *catalogListHTTPReader) Search(_ context.Context, input catalog.PublicCatalogSearchInput) (*catalog.PublicCatalogPage, error) {
	s.input = input
	s.calls++
	return &catalog.PublicCatalogPage{Products: []catalog.PublicCatalogProduct{{PublicSlug: "alpha-one", NameZhCN: "商品名称", NameEnUS: "Real product", SummaryZhCN: "商品摘要", SummaryEnUS: "Real summary", Currencies: []string{"CNY"}, Availability: "available"}}, Categories: []catalog.PublicCatalogCategory{}, AvailableCurrencies: []string{"CNY"}}, nil
}

// catalogListHTTPController binds only the production list method to its actual API metadata.
type catalogListHTTPController struct{ target *ControllerV1 }

func (c *catalogListHTTPController) List(ctx context.Context, req *v1.CatalogProductListReq) (*v1.CatalogProductListRes, error) {
	return c.target.CatalogProductList(ctx, req)
}

func TestPublicCatalogListHTTPBindsBracketedSlugs(t *testing.T) {
	reader := &catalogListHTTPReader{}
	server := g.Server("public-catalog-slugs-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(&catalogListHTTPController{target: &ControllerV1{catalog: reader}})
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	query := url.Values{"publicSlugs[]": {"alpha-two", "alpha-one"}, "limit": {"24"}, "namePrefix": {"Alpha"}, "currency": {"CNY"}}
	endpoint := "http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":") + "/api/v1/public/catalog/products"
	for _, q := range []string{query.Encode(), "limit=24"} {
		response, err := http.Get(endpoint + "?" + q)
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			Code int                      `json:"code"`
			Data v1.CatalogProductListRes `json:"data"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&output)
		closeErr := response.Body.Close()
		if decodeErr != nil || closeErr != nil || output.Code != 0 {
			t.Fatalf("HTTP failed: %+v decode=%v close=%v", output, decodeErr, closeErr)
		}
		if len(output.Data.Products) != 1 || output.Data.Products[0].NameEnUS != "Real product" || output.Data.Products[0].SummaryEnUS != "Real summary" {
			t.Fatalf("card sales text lost: %+v", output.Data.Products)
		}
		if q == query.Encode() {
			if !reflect.DeepEqual(reader.input.PublicSlugs, []string{"alpha-two", "alpha-one"}) || reader.input.Limit != 24 || reader.input.NamePrefix != "Alpha" || reader.input.Currency != "CNY" {
				t.Fatalf("array/filter binding failed: %+v", reader.input)
			}
		} else if len(reader.input.PublicSlugs) != 0 {
			t.Fatalf("omitted array gained a filter: %+v", reader.input)
		}
	}
	if reader.calls != 2 {
		t.Fatalf("expected one service list call per request, got %d", reader.calls)
	}
}
