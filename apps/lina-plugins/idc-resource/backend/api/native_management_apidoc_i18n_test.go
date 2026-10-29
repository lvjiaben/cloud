// This file guards every native management API translation and its reviewed source metadata.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	idcresource "lina-plugin-idc-resource"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestNativeManagementAPIDocTranslations(t *testing.T) {
	content, err := fs.ReadFile(idcresource.EmbeddedFiles, "manifest/i18n/zh-CN/apidoc/customer-native-server.json")
	if err != nil {
		t.Fatal(err)
	}
	bundle := map[string]string{}
	if err = json.Unmarshal(content, &bundle); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, model := range []any{v1.NativeServerDetailReq{}, v1.NativeServerResolveReq{}, v1.NativeServerResolveRes{}, v1.NativeServerOperation{}, v1.NativeServerDetailRes{}, v1.NativeServerOperationCreateReq{}, v1.NativeServerOperationCreateRes{}, v1.NativeServerCredentialRevealReq{}, v1.NativeServerCredentialRevealRes{}, v1.NativeServerListReq{}, v1.NativeServerListItem{}, v1.NativeServerListRes{}, v1.NativeServerOutput{}, v1.NativeServerInformation{}, v1.NativeServerCapability{}, v1.NativeServerCommercialCapability{}, v1.NativeServerCommercialQuoteReq{}, v1.NativeServerCommercialQuoteRes{}, v1.NativeServerCommercialConfirmationReq{}, v1.NativeServerCommercialCheckoutReq{}, v1.NativeServerCommercialCheckoutRes{}, v1.NativeServerTrafficPackagesReq{}, v1.NativeServerTrafficPackagesRes{}, v1.NativeServerTrafficPackage{}, v1.NativeServerCommercialOperationsReq{}, v1.NativeServerCommercialOperationsRes{}, v1.NativeServerCommercialOperation{}, v1.NativeServerAccessoryBillingReq{}, v1.NativeServerAccessoryBillingRes{}, v1.NativeServerAccessoryBill{}, v1.NativeConfigurationLocalizedText{}, v1.NativeConfigurationOption{}, v1.NativeConfigurationUploadPolicy{}, v1.NativeConfigurationField{}, v1.NativeConfigurationValue{}, v1.NativeConfigurationIssue{}, v1.NativeServerConfigurationContextReq{}, v1.NativeServerConfigurationContextRes{}, v1.NativeServerTargetQuoteReq{}, v1.NativeServerTargetQuoteRes{}, v1.NativeServerConfirmationReq{}, v1.NativeServerConfirmationRes{}, v1.NativeServerFactsReq{}, v1.NativeServerFact{}, v1.NativeServerFactsRes{}, v1.NativeServerOperationDetailReq{}, v1.NativeServerOperationDetailRes{}, v1.NativeServerOutputRevealReq{}, v1.NativeServerOutputRevealRes{}, v1.NativeServerOperationsReq{}, v1.NativeServerOperationsRes{}, v1.NativeServerOptionsReq{}, v1.NativeServerImage{}, v1.NativeServerAccessory{}, v1.NativeServerOptionsRes{}} {
		item := reflect.TypeOf(model)
		for i := 0; i < item.NumField(); i++ {
			field := item.Field(i)
			for _, tag := range []string{"tags", "summary", "dc"} {
				source := field.Tag.Get(tag)
				if source == "" {
					continue
				}
				suffix := "fields." + strings.Split(field.Tag.Get("json"), ",")[0] + "." + tag
				if field.Anonymous {
					suffix = "meta." + tag
				}
				key := "plugins.idc_resource.api.accountnative.v1." + item.Name() + "." + suffix
				if strings.TrimSpace(bundle[key]) == "" || bundle[key] == source {
					t.Errorf("missing native translation %s", key)
				}
				sources[key] = source
			}
		}
	}
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for _, key := range keys {
		canonical.WriteString(key + "=" + sources[key] + "\n")
	}
	digest := sha256.Sum256([]byte(canonical.String()))
	if hex.EncodeToString(digest[:]) != "36748d9ed7c55f415eb302dc2f150e5777f6f1ef05917879f6812acbc7a6eb0e" {
		t.Fatal("native API source changed: review translations and update reviewed digest")
	}
}
