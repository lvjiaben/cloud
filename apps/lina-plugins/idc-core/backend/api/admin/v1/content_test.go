// This file verifies numbered content pagination metadata and every enabled API translation.

package v1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestContentPageDocumentation checks maintained DTO fields against the plugin's declared locales.
func TestContentPageDocumentation(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	manifest, err := os.ReadFile(filepath.Join(root, "plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		I18n struct {
			Enabled bool `yaml:"enabled"`
			Locales []struct {
				Locale string `yaml:"locale"`
			} `yaml:"locales"`
		} `yaml:"i18n"`
	}
	if err = yaml.Unmarshal(manifest, &config); err != nil {
		t.Fatal(err)
	}
	if !config.I18n.Enabled {
		t.Fatal("content API expects an enabled locale contract")
	}
	keys := []string{"plugins.idc_core.api.admin.v1.ContentListReq.meta.dc", "plugins.idc_core.api.admin.v1.ContentListRes.fields.total.dc"}
	request := reflect.TypeOf(ContentListReq{})
	for index := 1; index < request.NumField(); index++ {
		field := request.Field(index)
		for _, tag := range []string{"json", "dc", "eg"} {
			if _, ok := field.Tag.Lookup(tag); !ok {
				t.Fatalf("%s is missing %s", field.Name, tag)
			}
		}
		keys = append(keys, "plugins.idc_core.api.admin.v1.ContentListReq.fields."+field.Tag.Get("json")+".dc")
	}
	for _, locale := range config.I18n.Locales {
		if locale.Locale == "en-US" {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(root, "manifest", "i18n", locale.Locale, "apidoc", "index.json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		var messages map[string]string
		if err = json.Unmarshal(body, &messages); err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			if messages[key] == "" {
				t.Errorf("%s is missing %s", locale.Locale, key)
			}
		}
	}
}
