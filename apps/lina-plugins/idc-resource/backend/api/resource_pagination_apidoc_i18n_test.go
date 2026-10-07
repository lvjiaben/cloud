// This file validates pagination translations against the actual API field metadata, not English source keys.

package api

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	idcresource "lina-plugin-idc-resource"
	adminv1 "lina-plugin-idc-resource/backend/api/admin/v1"
)

func TestResourcePaginationAPIDocTranslations(t *testing.T) {
	t.Parallel()
	content, err := fs.ReadFile(idcresource.EmbeddedFiles, "manifest/i18n/zh-CN/apidoc/resource-pagination.json")
	if err != nil {
		t.Fatal(err)
	}
	bundle := make(map[string]string)
	if err = json.Unmarshal(content, &bundle); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		model  any
		field  string
		source string
	}{
		{adminv1.ProviderCapacityObservationListReq{}, "Page", "One-based page number; cannot be combined with a cursor"},
		{adminv1.ProviderCapacityObservationListRes{}, "Total", "Exact total within the current staff scope and filters"},
		{adminv1.ProviderHealthListReq{}, "Page", "One-based page number; cannot be combined with a cursor"},
		{adminv1.ProviderHealthListRes{}, "Total", "Exact total within the current staff scope and filters"},
		{adminv1.ProviderTaskListReq{}, "Page", "One-based page number; cannot be combined with a cursor"},
		{adminv1.ProviderTaskListRes{}, "Total", "Exact total within the current staff scope and filters"},
		{adminv1.ServerRenewalListReq{}, "Page", "One-based page number; cannot be combined with a cursor"},
		{adminv1.ServerRenewalListRes{}, "Total", "Exact total within the current staff scope and filters"},
		{adminv1.ResourceOverviewReq{}, "PoolPage", "One-based resource pool page number"},
		{adminv1.ResourceOverviewReq{}, "PoolPageSize", "Resource pool page size"},
		{adminv1.ResourceOverviewReq{}, "FinancePage", "One-based financial snapshot page number"},
		{adminv1.ResourceOverviewReq{}, "FinancePageSize", "Financial snapshot page size"},
		{adminv1.ResourceOverviewSummary{}, "PoolTotal", "Exact resource pool count within the current staff scope"},
		{adminv1.ResourceOverviewSummary{}, "FinanceTotal", "Exact latest financial snapshot count within the current staff scope"},
	} {
		model := reflect.TypeOf(item.model)
		field, exists := model.FieldByName(item.field)
		if !exists || field.Tag.Get("dc") != item.source {
			t.Fatalf("review changed source metadata for %s.%s", model.Name(), item.field)
		}
		key := "plugins.idc_resource.api.admin.v1." + model.Name() + ".fields." + field.Tag.Get("json") + ".dc"
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing API translation %s", key)
		}
		delete(bundle, key)
	}
	if len(bundle) != 0 {
		t.Fatalf("unexpected or unnamespaced pagination translations: %v", bundle)
	}
}
