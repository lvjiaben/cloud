// HTTP roundtrips cover website-owned shared information, isolated template buckets and public navigation.

package public

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	adminapi "lina-plugin-idc-portal/backend/api/admin"
	adminv1 "lina-plugin-idc-portal/backend/api/admin/v1"
	publicv1 "lina-plugin-idc-portal/backend/api/public/v1"
	"lina-plugin-idc-portal/backend/cap/websitecap"
	admincontroller "lina-plugin-idc-portal/backend/internal/controller/admin"
	"lina-plugin-idc-portal/backend/internal/service/siteconfiguration"
)

// configurationHTTPRegistry supplies only installed-provider discovery and lifecycle admission in this fixture.
type configurationHTTPRegistry struct {
	pluginhost.BusinessExtensionRegistrar
	items []*capmodel.BusinessExtensionProviderEvidence
}

func (r *configurationHTTPRegistry) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: r.items, Total: len(r.items)}, nil
}
func (r *configurationHTTPRegistry) WithNewBusinessAdmission(ctx context.Context, input pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	for _, item := range r.items {
		if item.PluginID == input.PluginID {
			return action(ctx, item)
		}
	}
	return fmt.Errorf("fixture provider unavailable: %s", input.PluginID)
}

// configurationHTTPController binds production DTO metadata and controllers without creating host or customer state.
type configurationHTTPController struct {
	admin  adminapi.IAdminV1
	public *ControllerV1
}

func (c *configurationHTTPController) Get(ctx context.Context, req *adminv1.SiteConfigurationGetReq) (*adminv1.SiteConfigurationGetRes, error) {
	return c.admin.SiteConfigurationGet(ctx, req)
}
func (c *configurationHTTPController) Save(ctx context.Context, req *adminv1.SiteConfigurationSaveReq) (*adminv1.SiteConfigurationSaveRes, error) {
	return c.admin.SiteConfigurationSave(ctx, req)
}
func (c *configurationHTTPController) Brand(ctx context.Context, req *publicv1.BrandGetReq) (*publicv1.BrandGetRes, error) {
	return c.public.BrandGet(ctx, req)
}

func TestWebsiteCommonHTTPPersistenceAndProjection(t *testing.T) {
	var (
		root     = t.TempDir()
		schema   = &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{}}
		pages    = []websitecap.Page{{ID: "home", NameZh: "首页", NameEn: "Home", Path: "/"}, {ID: "products", NameZh: "商品", NameEn: "Products", Path: "/products.html"}}
		metadata = &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"renderer": websitecap.Renderer, "pages": pages}}
	)
	for _, key := range []string{"herotitle", "herodescription", "heroimage", "hero1href", "relatedproductselectricity", "relatedproductsweb", "relatedproductsgame", "relatedproductsgoingsea", "relatedproductsfinancial", "relatedproductsgovernment", "relatedproductssafety"} {
		schema.Fields = append(schema.Fields, &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldMultiLine})
	}
	registry := &configurationHTTPRegistry{items: []*capmodel.BusinessExtensionProviderEvidence{{PluginID: "website-main", InterfaceConfiguration: schema, ProviderMetadata: metadata}, {PluginID: "website-old", InterfaceConfiguration: schema, ProviderMetadata: metadata}}}
	service, err := siteconfiguration.New(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	old := siteconfiguration.Settings{SelectedPlugin: "website-old", Common: siteconfiguration.Common{SiteName: "Old site", LogoURL: "/legacy.png"}, Config: map[string]any{"herotitle": "Old template retained"}}
	if err = service.Save(t.Context(), old); err != nil {
		t.Fatal(err)
	}
	server := g.Server("website-common-http-" + guid.S())
	server.SetPort(0)
	server.SetDumpRouterMap(false)
	server.Group("/x/idc-portal/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(&configurationHTTPController{admin: admincontroller.NewV1(nil, service, nil), public: &ControllerV1{sites: service}})
	})
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if shutdownErr := server.Shutdown(); shutdownErr != nil {
			t.Error(shutdownErr)
		}
	})
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/x/idc-portal/api/v1", server.GetListenedPort())
	call := func(method, path string, input any, output any) int {
		t.Helper()
		var body bytes.Buffer
		if input != nil {
			if encodeErr := json.NewEncoder(&body).Encode(input); encodeErr != nil {
				t.Fatal(encodeErr)
			}
		}
		request, requestErr := http.NewRequestWithContext(t.Context(), method, endpoint+path, &body)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		response, responseErr := http.DefaultClient.Do(request)
		if responseErr != nil {
			t.Fatal(responseErr)
		}
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&envelope)
		closeErr := response.Body.Close()
		if decodeErr != nil || closeErr != nil {
			t.Fatalf("HTTP decode=%v close=%v", decodeErr, closeErr)
		}
		if envelope.Code == 0 && output != nil {
			if decodeErr = json.Unmarshal(envelope.Data, output); decodeErr != nil {
				t.Fatal(decodeErr)
			}
		}
		return envelope.Code
	}
	common := adminv1.WebsiteCommon{SiteName: "Fixture site", SiteTitle: "Fixture title", CompanyName: "Fixture company", Description: "Fixture description", ContactPhone: "4000000000", ContactEmail: "fixture@example.test", LogoURL: "/legacy.png", HeaderLogoURL: "/header.png", FooterLogoURL: "/footer.png", Copyright: "© Fixture", BusinessQQ: "123456", CompanyAddress: "Fixture park", EnterpriseWechatQRCodeURL: "/enterprise.png", WechatOfficialQRCodeURL: "/official.png", ICPNumber: "ICP fixture", ICPURL: "https://records.example/icp", PoliceNumber: "Police fixture", PoliceURL: "https://records.example/police", TelecomLicenseNumber: "Telecom fixture", TelecomLicenseURL: "https://records.example/telecom"}
	config := map[string]any{}
	for _, field := range schema.Fields {
		config[field.Key] = "fixture " + field.Key
	}
	config["heroimage"] = "/hero.png"
	config["hero1href"] = "/products.html"
	nav := &websitecap.Navigation{Header: []websitecap.NavigationItem{{ID: "products", PageID: "products", NameZh: "商品", NameEn: "Products", SortOrder: 2}, {ID: "home", PageID: "home", NameZh: "首页", NameEn: "Home", SortOrder: 1}, {ID: "login", PageID: "customer-login", ParentID: "home", NameZh: "登录", NameEn: "Sign in"}}, Footer: []websitecap.NavigationItem{{ID: "external", URL: "https://example.test/docs", NameZh: "文档", NameEn: "Docs"}}}
	input := adminv1.SiteConfigurationSaveReq{SelectedPlugin: "website-main", Common: common, Config: config, Navigation: nav}
	if code := call(http.MethodPut, "/admin/site/configuration", input, nil); code != 0 {
		t.Fatalf("save code=%d", code)
	}
	var view adminv1.SiteConfigurationGetRes
	if code := call(http.MethodGet, "/admin/site/configuration", nil, &view); code != 0 || !reflect.DeepEqual(view.Common, common) || !reflect.DeepEqual(view.Config, config) || !reflect.DeepEqual(view.Navigation, nav) {
		t.Fatalf("saved fields lost: code=%d common=%+v", code, view.Common)
	}
	if !reflect.DeepEqual(view.Configurations["website-old"], old.Config) || len(view.Plugins) != 2 || len(view.Plugins[0].Schema.Fields) != 11 {
		t.Fatalf("template buckets or declared schema lost: %+v", view.Configurations)
	}
	var projection publicv1.BrandGetRes
	if code := call(http.MethodGet, "/public/brand", nil, &projection); code != 0 || projection.Brand == nil {
		t.Fatalf("public code=%d", code)
	}
	if projection.Brand.LogoURL != "/legacy.png" || projection.Brand.HeaderLogoURL != "/header.png" || projection.Brand.FooterLogoURL != "/footer.png" || projection.Brand.Copyright != common.Copyright || projection.Brand.BusinessQQ != common.BusinessQQ || projection.Brand.CompanyAddress != common.CompanyAddress || projection.Brand.EnterpriseWechatQRCodeURL != common.EnterpriseWechatQRCodeURL || projection.Brand.WechatOfficialQRCodeURL != common.WechatOfficialQRCodeURL || projection.Brand.TelecomLicenseNumber != common.TelecomLicenseNumber || projection.Brand.TelecomLicenseURL != common.TelecomLicenseURL || projection.Brand.PoliceNumber != common.PoliceNumber || projection.Brand.ICPNumber != common.ICPNumber {
		t.Fatalf("public fields lost: %+v", projection.Brand)
	}
	if len(projection.Brand.Navigation) != 2 || projection.Brand.Navigation[0].Href != "/" || !projection.Brand.Navigation[0].Children[0].Customer || !projection.Brand.FooterNavigation[0].External {
		t.Fatalf("navigation projection failed: %+v", projection.Brand)
	}
	before, err := os.ReadFile(filepath.Join(root, "website-configuration.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []adminv1.WebsiteCommon{func() adminv1.WebsiteCommon { v := common; v.HeaderLogoURL = "javascript:alert(1)"; return v }(), func() adminv1.WebsiteCommon { v := common; v.TelecomLicenseURL = "//unsafe.example"; return v }()} {
		input.Common = invalid
		if code := call(http.MethodPut, "/admin/site/configuration", input, nil); code == 0 {
			t.Fatal("unsafe input accepted")
		}
	}
	after, err := os.ReadFile(filepath.Join(root, "website-configuration.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed save changed stored settings")
	}
	restarted, err := siteconfiguration.New(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.Get(t.Context())
	if err != nil || !reflect.DeepEqual(restored.Common, siteconfiguration.Common(common)) || !reflect.DeepEqual(restored.Configurations["website-old"], old.Config) {
		t.Fatalf("restart lost shared or independent settings: %v", err)
	}
}
