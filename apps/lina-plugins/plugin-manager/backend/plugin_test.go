// This file verifies built-in registration and the exact navigation replacement boundary.
package backend

import (
	"context"
	"io/fs"
	"lina-core/pkg/plugin/pluginhost"
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"
	"gopkg.in/yaml.v3"
	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/manifestcap"
	"lina-core/pkg/plugin/capability/plugincap"
	pluginmanager "lina-plugin-plugin-manager"
	managementapi "lina-plugin-plugin-manager/backend/api/management"
	v1 "lina-plugin-plugin-manager/backend/api/management/v1"
	"lina-plugin-plugin-manager/backend/internal/service/catalog"
)

func TestPluginRegistrationAndMenuReplacement(t *testing.T) {
	plugin, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok || len(plugin.GetRouteRegistrars()) != 1 || len(plugin.GetMenuFilters()) != 1 {
		t.Fatal("workspace registration is incomplete")
	}
	for _, tc := range []struct {
		key  string
		want bool
	}{{"extension:plugin:list", false}, {"extension:plugin:install", true}, {"plugin:plugin-manager:management", true}, {"extension", true}} {
		menu := pluginhost.NewMenuDescriptor(1, 0, "menu", "/test", "component", "plugin:list", tc.key, "M", 1, 1)
		got, err := filterMenu(context.Background(), menu)
		if err != nil || got != tc.want {
			t.Fatalf("menu %s: got %v %v", tc.key, got, err)
		}
	}
	if err := registerRoutes(context.Background(), nil); err == nil {
		t.Fatal("missing registrar accepted")
	}
}

type startupManifest struct{ manifestcap.Service }

func (startupManifest) Scan(_ context.Context, _, _ string, target any) error {
	body, err := fs.ReadFile(pluginmanager.EmbeddedFiles, "manifest/config/config.yaml")
	if err != nil {
		return err
	}
	var defaults struct {
		Catalog catalog.Config `yaml:"catalog"`
	}
	if err = yaml.Unmarshal(body, &defaults); err != nil {
		return err
	}
	*target.(*catalog.Config) = defaults.Catalog
	return nil
}

type startupConfig struct{ plugincap.ConfigService }

func (startupConfig) Scan(context.Context, string, any) error { return nil }

type startupPlugins struct{ plugincap.Service }

func (startupPlugins) Config() plugincap.ConfigService { return startupConfig{} }

type startupServices struct{ capability.Services }

func (startupServices) Manifest() manifestcap.Service { return startupManifest{} }
func (startupServices) Plugins() plugincap.Service    { return startupPlugins{} }

type startupRegistrar struct {
	pluginhost.HTTPRegistrar
	routes *startupRoutes
}

func (r startupRegistrar) Services() capability.Services     { return startupServices{} }
func (r startupRegistrar) Routes() pluginhost.RouteRegistrar { return r.routes }

type startupRoutes struct {
	pluginhost.RouteRegistrar
	group      *startupGroup
	middleware pluginhost.RouteMiddlewares
}

func (r *startupRoutes) APIPrefix() string                                    { return "/x/plugin-manager" }
func (r *startupRoutes) Group(_ string, register func(pluginhost.RouteGroup)) { register(r.group) }
func (r *startupRoutes) Middlewares() pluginhost.RouteMiddlewares             { return r.middleware }
func (r *startupRoutes) Err() error                                           { return nil }

type startupGroup struct {
	pluginhost.RouteGroup
	middlewareCount int
	controller      managementapi.IManagementV1
}

func (g *startupGroup) Group(_ string, register func(pluginhost.RouteGroup)) { register(g) }
func (g *startupGroup) Middleware(handlers ...pluginhost.RouteMiddleware) {
	g.middlewareCount += len(handlers)
}
func (g *startupGroup) Bind(handlers ...any) {
	g.controller = handlers[0].(managementapi.IManagementV1)
}

func TestStartupBindsConfiguredControllerAndProtectedMiddleware(t *testing.T) {
	group := &startupGroup{}
	noop := pluginhost.RouteMiddleware(func(*ghttp.Request) {})
	routes := &startupRoutes{group: group, middleware: pluginhost.NewRouteMiddlewares(noop, noop, noop, noop, noop, noop, noop, noop)}
	if err := registerRoutes(context.Background(), startupRegistrar{routes: routes}); err != nil {
		t.Fatal(err)
	}
	if group.middlewareCount != 8 || group.controller == nil {
		t.Fatal("protected controller binding is incomplete")
	}
	result, err := group.controller.Categories(context.Background(), &v1.CategoriesReq{})
	if err != nil || len(result.List) != 7 || result.List[2].Name != "服务器" {
		t.Fatalf("embedded startup configuration was not bound: %+v %v", result, err)
	}
}
