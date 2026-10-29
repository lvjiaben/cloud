// This file checks source metadata and Chinese docs for the new readiness-only
// projections. No customer response gains local diagnostics or provider details.
package v1

import (
	"encoding/json"
	account "lina-plugin-idc-core/backend/api/account/v1"
	directory "lina-plugin-idc-core/backend/api/directoryaccount/v1"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestManagementPreparationDocumentation(t *testing.T) {
	translations := make(map[string]string)
	for _, name := range []string{"management-preparation.json", "product-directories.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "i18n", "zh-CN", "apidoc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &translations); err != nil {
			t.Fatal(err)
		}
	}

	for _, suffix := range []string{"meta.summary", "meta.dc", "meta.tags", "schema.dc"} {
		if translations["plugins.idc_core.api.admin.v1.ProductFulfillmentManagementRepairReq."+suffix] == "" {
			t.Errorf("missing repair documentation %s", suffix)
		}
	}
	for _, value := range []struct {
		model  any
		prefix string
		all    bool
	}{
		{ProductFulfillmentManagementRepairReq{}, "plugins.idc_core.api.admin.v1.", true},
		{ProductFulfillmentManagementRepairRes{}, "plugins.idc_core.api.admin.v1.", true},
		{ProductFulfillmentTaskDetail{}, "plugins.idc_core.api.admin.v1.", false},
		{ProductFulfillmentManagementPreparation{}, "plugins.idc_core.api.admin.v1.", true},
		{account.CommerceServiceEntitlement{}, "plugins.idc_core.api.account.v1.", false},
		{directory.Entitlement{}, "plugins.idc_core.api.directoryaccount.v1.", false},
	} {
		typ := reflect.TypeOf(value.model)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				continue
			}
			if !value.all && field.Name != "ManagementPreparation" && field.Name != "CanRepairManagement" {
				continue
			}
			key := value.prefix + typ.Name() + ".fields." + strings.Split(field.Tag.Get("json"), ",")[0] + ".dc"
			if field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" || translations[key] == "" {
				t.Errorf("missing metadata: %s", key)
			}
		}
	}
}
