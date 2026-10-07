// This file verifies real HTTP binding for traffic-package collections and
// route identity. Business eligibility and stock rules have PostgreSQL tests.

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

type trafficHTTPPricingStub struct {
	serveroperation.AccessoryPricing
	writes         []serveroperation.TrafficPackageInput
	productNumbers []string
}

func (s *trafficHTTPPricingStub) SaveTrafficPackage(_ context.Context, input serveroperation.TrafficPackageInput) (*serveroperation.TrafficPackage, error) {
	s.writes = append(s.writes, input)
	return &serveroperation.TrafficPackage{PackageNo: input.PackageNo, ProductNos: input.ProductNos}, nil
}

func (s *trafficHTTPPricingStub) TrafficPackageProducts(_ context.Context, _ string, numbers []string) ([]catalog.TrafficPackageProduct, error) {
	s.productNumbers = numbers
	return []catalog.TrafficPackageProduct{}, nil
}

type trafficHTTPController struct{ target *ControllerV1 }

func (c *trafficHTTPController) Create(ctx context.Context, request *v1.TrafficPackageCreateReq) (*v1.TrafficPackageCreateRes, error) {
	return c.target.TrafficPackageCreate(ctx, request)
}

func (c *trafficHTTPController) Update(ctx context.Context, request *v1.TrafficPackageUpdateReq) (*v1.TrafficPackageUpdateRes, error) {
	return c.target.TrafficPackageUpdate(ctx, request)
}

func (c *trafficHTTPController) Products(ctx context.Context, request *v1.TrafficPackageProductListReq) (*v1.TrafficPackageProductListRes, error) {
	return c.target.TrafficPackageProductList(ctx, request)
}

func TestTrafficPackageHTTPBindingPreservesCollectionsAndRouteIdentity(t *testing.T) {
	pricing := &trafficHTTPPricingStub{}
	server := g.Server("traffic-package-http-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/x/idc-core/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(func(request *ghttp.Request) {
			request.Middleware.Next()
			request.Response.WriteJson(g.Map{"data": request.GetHandlerResponse()})
		})
		group.Bind(&trafficHTTPController{target: &ControllerV1{accessoryPricing: pricing}})
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	base := "http://127.0.0.1:" + strings.TrimPrefix(server.GetListenedAddress(), ":") + "/x/idc-core/api/v1/admin/resource/"
	client := &http.Client{}
	for _, count := range []int{0, 1, 100, 101} {
		t.Run(fmt.Sprintf("product_count_%d", count), func(t *testing.T) {
			numbers := make([]string, count)
			for i := range numbers {
				numbers[i] = fmt.Sprintf("prd_%032d", i+1)
			}
			payload := g.Map{"nameZhCN": "100 GB", "nameEnUS": "100 GB", "currencyCode": "CNY", "trafficGb": 100, "amountMinor": 1000, "productNos": numbers, "stockQuantity": nil}
			trafficHTTPDo(t, client, http.MethodPost, base+"traffic-packages", payload)
			if len(pricing.writes) == 0 {
				t.Fatal("request did not reach its owner")
			}
			got := pricing.writes[len(pricing.writes)-1]
			if !reflect.DeepEqual(got.ProductNos, numbers) || got.StockQuantity != nil {
				t.Fatalf("collection or unlimited stock changed during binding: %+v", got)
			}
		})
	}
	if len(pricing.writes) != 4 {
		t.Fatalf("each collection must reach service cardinality validation, got %d", len(pricing.writes))
	}
	trafficHTTPDo(t, client, http.MethodPut, base+"traffic-packages/tpk_route?packageNo=tpk_query", g.Map{
		"packageNo": "tpk_body", "expectedVersion": 7, "nameZhCN": "100 GB", "nameEnUS": "100 GB", "currencyCode": "CNY", "trafficGb": 100, "amountMinor": 1000, "productNos": []string{"prd_one"}, "stockQuantity": 0,
	})
	last := pricing.writes[len(pricing.writes)-1]
	if last.PackageNo != "tpk_route" || last.ExpectedVersion != 7 || last.StockQuantity == nil || *last.StockQuantity != 0 {
		t.Fatalf("route identity, optimistic version or zero stock lost: %+v", last)
	}
	query := url.Values{"productNos[]": {"prd_first", "prd_second"}}
	trafficHTTPDo(t, client, http.MethodGet, base+"traffic-package-products?"+query.Encode(), nil)
	if !reflect.DeepEqual(pricing.productNumbers, []string{"prd_first", "prd_second"}) {
		t.Fatalf("multi-selection label lookup binding lost IDs: %v", pricing.productNumbers)
	}
}

// trafficHTTPDo consumes and closes every response so no test connection leaks.
func trafficHTTPDo(t *testing.T, client *http.Client, method, address string, payload any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(method, address, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	result, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d: %s", response.StatusCode, result)
	}
}
