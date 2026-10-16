// Real HTTP binding distinguishes an explicit empty configuration from a
// missing/null array without bypassing the owner's schema validation.
package admin

import (
	"context"
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
func (s *emptyMerchantHTTPService) EvaluateVersionConfiguration(context.Context, productautomation.EvaluateVersionConfigurationInput) (*productautomation.ConfigurationEvaluation, error) {
	s.evaluations++
	return &productautomation.ConfigurationEvaluation{}, nil
}

type emptyMerchantHTTPController struct{ target *ControllerV1 }

func (c *emptyMerchantHTTPController) Update(ctx context.Context, req *v1.CatalogProductVersionConfigurationUpdateReq) (*v1.CatalogProductVersionConfigurationUpdateRes, error) {
	return c.target.CatalogProductVersionConfigurationUpdate(ctx, req)
}
func (c *emptyMerchantHTTPController) Evaluate(ctx context.Context, req *v1.CatalogProductVersionConfigurationEvaluateReq) (*v1.CatalogProductVersionConfigurationEvaluateRes, error) {
	return c.target.CatalogProductVersionConfigurationEvaluate(ctx, req)
}

func TestEmptyMerchantConfigurationHTTP(t *testing.T) {
	service := &emptyMerchantHTTPService{}
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
		group.Bind(&emptyMerchantHTTPController{target: &ControllerV1{productAutomation: service}})
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
}
