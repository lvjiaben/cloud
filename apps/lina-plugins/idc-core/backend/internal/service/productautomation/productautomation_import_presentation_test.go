// These tests enforce immutable safe visibility/default policy after import.
package productautomation

import (
	"encoding/json"
	"fmt"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"strings"
	"testing"
)

func TestPortableCustomerPresentationIsImmutable(t *testing.T) {
	snapshot := portableAutomationSnapshot()
	hidden := false
	snapshot.ProtocolVersion = channelcommercecap.ImportPurchaseCycleLimitProtocolVersion
	snapshot.Mappings[0].CustomerVisible = &hidden
	snapshot.Mappings[0].DefaultValue = json.RawMessage(`"paid-region"`)
	raw, err := portableCustomerPresentationJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyPortableCustomerPresentation(snapshot, string(raw)); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{`{}`, `{"region":{"customerVisible":true,"defaultValue":"paid-region"}}`, `{"region":{"customerVisible":false,"defaultValue":"other"}}`, `{"region":{"customerVisible":false,"defaultValue":"paid-region","basePriceIncluded":true}}`} {
		if verifyPortableCustomerPresentation(snapshot, changed) == nil {
			t.Fatalf("policy override accepted: %s", changed)
		}
	}
	snapshot.Schema.Fields[0].Type = capmodel.ConfigurationFieldSecret
	if _, err = portableCustomerPresentationJSON(snapshot); err == nil {
		t.Fatal("hidden secret accepted")
	}
}

func TestPortableCustomerPresentationRejectsUnreadableAggregate(t *testing.T) {
	snapshot := portableAutomationSnapshot()
	snapshot.ProtocolVersion = channelcommercecap.ImportPurchaseCycleLimitProtocolVersion
	snapshot.Schema.Fields = nil
	snapshot.Mappings = nil
	hidden := false
	raw, _ := json.Marshal(strings.Repeat("x", 1024))
	for i := 0; i < 128; i++ {
		key := fmt.Sprintf("field_%d", i)
		snapshot.Schema.Fields = append(snapshot.Schema.Fields, &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{EnUS: "Field", ZhCN: "字段"}})
		snapshot.Mappings = append(snapshot.Mappings, channelcommercecap.ImportFieldMapping{FieldKey: key, FieldToken: "opaque", CustomerVisible: &hidden, DefaultValue: raw})
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("transport fixture invalid: %v", err)
	}
	if _, err := portableCustomerPresentationJSON(snapshot); err == nil {
		t.Fatal("presentation larger than persisted reader bound accepted")
	}
}
