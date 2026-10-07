// This file guards the new server directory pagination metadata and its plugin-owned translations.

package api

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	idccore "lina-plugin-idc-core"
	adminv1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func TestServerDirectoryPaginationAPIDocTranslations(t *testing.T) {
	t.Parallel()
	content, err := fs.ReadFile(idccore.EmbeddedFiles, "manifest/i18n/zh-CN/apidoc/server-directory-pagination.json")
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
		{adminv1.ServerServiceListReq{}, "Page", "One-based page number; cannot be combined with a cursor"},
		{adminv1.ServerServiceListRes{}, "Total", "Exact total within the current staff scope and filters"},
	} {
		model := reflect.TypeOf(item.model)
		field, exists := model.FieldByName(item.field)
		if !exists || field.Tag.Get("dc") != item.source {
			t.Fatalf("review changed source metadata for %s.%s", model.Name(), item.field)
		}
		key := "plugins.idc_core.api.admin.v1." + model.Name() + ".fields." + field.Tag.Get("json") + ".dc"
		if strings.TrimSpace(bundle[key]) == "" {
			t.Errorf("missing API translation %s", key)
		}
		delete(bundle, key)
	}
	if len(bundle) != 0 {
		t.Fatalf("unexpected or unnamespaced pagination translations: %v", bundle)
	}
}
