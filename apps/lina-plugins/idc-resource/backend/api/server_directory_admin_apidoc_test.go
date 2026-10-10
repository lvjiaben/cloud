// This file prevents missing localized metadata on the staff directory bridge DTOs.
package api

import (
	"encoding/json"
	"io/fs"
	idcresource "lina-plugin-idc-resource"
	v1 "lina-plugin-idc-resource/backend/api/adminserver/v1"
	"reflect"
	"strings"
	"testing"
)

func TestStaffDirectoryBridgeAPIDocTranslations(t *testing.T) {
	t.Parallel()
	content, err := fs.ReadFile(idcresource.EmbeddedFiles, "manifest/i18n/zh-CN/apidoc/server-directory-admin.json")
	if err != nil {
		t.Fatal(err)
	}
	bundle := map[string]string{}
	if err := json.Unmarshal(content, &bundle); err != nil {
		t.Fatal(err)
	}
	for _, dto := range []any{v1.CloudServerListReq{}, v1.CloudServerListRes{}, v1.CloudServerDetailReq{}, v1.CloudServerDetailRes{}} {
		model := reflect.TypeOf(dto)
		prefix := "plugins.idc_resource.api.adminserver.v1." + model.Name() + "."
		for i := 0; i < model.NumField(); i++ {
			field := model.Field(i)
			if field.Name == "Meta" {
				for _, tag := range []string{"tags", "summary", "dc"} {
					if field.Tag.Get(tag) != "" && strings.TrimSpace(bundle[prefix+"meta."+tag]) == "" {
						t.Errorf("missing %smeta.%s", prefix, tag)
					}
				}
				if field.Tag.Get("dc") != "" && strings.TrimSpace(bundle[prefix+"schema.dc"]) == "" {
					t.Errorf("missing %sschema.dc", prefix)
				}
			} else if field.Tag.Get("dc") != "" {
				key := prefix + "fields." + field.Tag.Get("json") + ".dc"
				if strings.TrimSpace(bundle[key]) == "" {
					t.Errorf("missing %s", key)
				}
			}
		}
	}
}
