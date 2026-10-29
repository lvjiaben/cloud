// These tests guard the new staff route boundary, language coverage and startup
// dependency failure without pretending to exercise a running authenticated API.

package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lina-plugin-idc-core/backend/api/renewalstaff/v1"
	"lina-plugin-idc-core/backend/internal/service/renewalnotice"
)

func TestRenewalStaffControllerRequiresStartupDependencies(t *testing.T) {
	if controller, err := newRenewalStaffController(nil, nil, nil, nil); err == nil || controller != nil {
		t.Fatal("missing startup dependencies were accepted")
	}
}

func TestRenewalStaffRouteAndTranslationContracts(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "manifest", "i18n", "zh-CN", "apidoc", "renewal-staff.json"))
	if err != nil {
		t.Fatal(err)
	}
	translations := make(map[string]string)
	if err = json.Unmarshal(content, &translations); err != nil {
		t.Fatal(err)
	}
	types := []reflect.Type{
		reflect.TypeOf(v1.WorkItemListReq{}), reflect.TypeOf(v1.WorkItem{}), reflect.TypeOf(v1.WorkItemListRes{}),
		reflect.TypeOf(v1.NotificationSyncCreateReq{}), reflect.TypeOf(v1.NotificationSyncCreateRes{}),
	}
	for _, typ := range types {
		prefix := "plugins.idc_core.api.renewalstaff.v1." + typ.Name() + "."
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if field.Name == "Meta" {
				if field.Tag.Get("permission") != renewalnotice.Permission {
					t.Fatalf("unguarded route %s", typ.Name())
				}
				for _, key := range []string{"tags", "summary", "dc"} {
					if field.Tag.Get(key) == "" || translations[prefix+"meta."+key] == "" {
						t.Errorf("missing %s %s", typ.Name(), key)
					}
				}
				continue
			}
			name := field.Tag.Get("json")
			if name == "" || field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" || translations[prefix+"fields."+name+".dc"] == "" {
				t.Errorf("incomplete field contract %s.%s", typ.Name(), field.Name)
			}
		}
	}
	list, _ := reflect.TypeOf(v1.WorkItemListReq{}).FieldByName("Meta")
	sync, _ := reflect.TypeOf(v1.NotificationSyncCreateReq{}).FieldByName("Meta")
	if list.Tag.Get("method") != "get" || sync.Tag.Get("method") != "post" {
		t.Fatal("query and side-effect methods diverged")
	}
	if _, ok := reflect.TypeOf(v1.NotificationSyncCreateReq{}).FieldByName("UserID"); ok {
		t.Fatal("sync must derive its recipient from current identity")
	}
}
