// This file verifies category boundaries, exact totals, extension, and bounded reads.
package catalog

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/frame/g"

	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-plugin-manager/backend/api/management/v1"
)

type catalogReader struct {
	rows       []*pluginv1.PluginListItem
	calls      int
	incomplete bool
}

func (r *catalogReader) List(_ context.Context, page, size int, _, _ string) (*pluginv1.ListRes, error) {
	r.calls++
	// Validate against the public DTO so mocks reject oversized host pages too.
	if err := g.Validator().Data(pluginv1.ListReq{PageNum: page, PageSize: size}).Run(context.Background()); err != nil {
		return nil, err
	}
	start := min((page-1)*size, len(r.rows))
	end := min(start+size, len(r.rows))
	if r.incomplete {
		end = start
	}
	return &pluginv1.ListRes{List: r.rows[start:end], Total: len(r.rows)}, nil
}
func row(id string) *pluginv1.PluginListItem {
	return &pluginv1.PluginListItem{Id: id, Name: id, Type: pluginv1.PluginTypeDynamic, Distribution: pluginv1.PluginDistributionManaged}
}
func config() Config {
	return Config{Fallback: "system", Categories: []Category{{ID: "server", Name: "服务器", Plugins: []string{"idc-zjmfcloud", "idc-baota"}}, {ID: "cloud", Name: "云产品", Plugins: []string{"yunsu-ssl"}}, {ID: "system", Name: "系统功能"}}}
}

func TestCategoryPaginationFiltersBeforePaging(t *testing.T) {
	reader := &catalogReader{}
	for i := 0; i < 401; i++ {
		reader.rows = append(reader.rows, row(fmt.Sprintf("extension-%03d", i)))
	}
	reader.rows = append(reader.rows, row("idc-zjmfcloud"), row("idc-baota"), row("yunsu-ssl"))
	source := row("source-plugin")
	source.Type = pluginv1.PluginTypeSource
	reader.rows = append(reader.rows, source)
	builtin := row("builtin-plugin")
	builtin.Distribution = pluginv1.PluginDistributionBuiltin
	reader.rows = append(reader.rows, builtin)
	service, err := New(config(), reader)
	if err != nil {
		t.Fatal(err)
	}
	out, err := service.List(context.Background(), &v1.ListReq{Category: "server", PageNum: 2, PageSize: 1})
	if err != nil || out.Total != 2 || len(out.List) != 1 || out.List[0].Id != "idc-baota" {
		t.Fatalf("wrong filtered page: %+v %v", out, err)
	}
	if reader.calls != 5 {
		t.Fatalf("expected 5 collection calls, got %d", reader.calls)
	}
	out, err = service.List(context.Background(), &v1.ListReq{Category: "system", PageNum: 1, PageSize: 200})
	if err != nil || out.Total != 401 {
		t.Fatalf("source or builtin leaked into totals: %+v %v", out, err)
	}
}

func TestCategoryConfigurationCanGrowWithoutUIEnum(t *testing.T) {
	cfg := config()
	cfg.Categories = append(cfg.Categories, Category{ID: "dns", Name: "域名解析", Prefixes: []string{"dns-"}})
	service, err := New(cfg, &catalogReader{rows: []*pluginv1.PluginListItem{row("dns-example"), row("unknown"), row("yunsu-ssl")}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := service.List(context.Background(), &v1.ListReq{Category: "dns", PageNum: 1, PageSize: 20})
	if err != nil || out.Total != 1 || out.List[0].Id != "dns-example" {
		t.Fatalf("configured extension unavailable: %+v %v", out, err)
	}
	categories := service.Categories(context.Background())
	if categories[len(categories)-1].Name != "域名解析" {
		t.Fatal("configuration order lost")
	}
}

func TestCatalogRejectsIncompleteReadAndUnknownCategory(t *testing.T) {
	reader := &catalogReader{rows: []*pluginv1.PluginListItem{row("unknown")}, incomplete: true}
	service, err := New(config(), reader)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.List(context.Background(), &v1.ListReq{Category: "system", PageNum: 1, PageSize: 20})
	if !bizerr.Is(err, CodeCatalogIncomplete) {
		t.Fatalf("incomplete catalog was accepted: %v", err)
	}
	reader.calls = 0
	_, err = service.List(context.Background(), &v1.ListReq{Category: "unconfigured", PageNum: 1, PageSize: 20})
	if !bizerr.Is(err, CodeCategoryInvalid) || reader.calls != 0 {
		t.Fatalf("invalid category reached host: %v", err)
	}
	reader.incomplete = false
	reader.rows = nil
	for i := 0; i < 1001; i++ {
		reader.rows = append(reader.rows, row(fmt.Sprintf("p-%d", i)))
	}
	_, err = service.List(context.Background(), &v1.ListReq{Category: "system", PageNum: 1, PageSize: 20})
	if !bizerr.Is(err, CodeCatalogIncomplete) {
		t.Fatalf("overflow catalog accepted: %v", err)
	}
}

func TestConfigurationRejectsConflictsAndUsesSpecificMappings(t *testing.T) {
	cfg := config()
	cfg.Categories[0].Prefixes = []string{"idc-"}
	cfg.Categories[2].Prefixes = []string{"idc-zjmfmigrate"}
	service, err := New(cfg, &catalogReader{})
	if err != nil {
		t.Fatal(err)
	}
	concrete := service.(*serviceImpl)
	if concrete.category("idc-zjmfcloud") != "server" || concrete.category("idc-zjmfmigrate") != "system" {
		t.Fatal("explicit or longest prefix precedence lost")
	}
	cfg.Categories[1].Plugins = append(cfg.Categories[1].Plugins, "idc-baota")
	if _, err = New(cfg, &catalogReader{}); err == nil {
		t.Fatal("ambiguous exact assignment accepted")
	}
}
