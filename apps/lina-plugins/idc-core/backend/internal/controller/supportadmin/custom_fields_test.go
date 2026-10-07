// This file verifies admin custom-field request mapping and immutable selected-option response projection.
package supportadmin

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func TestAdminCustomFieldMappersKeepConcurrencyAndSnapshotBoundaries(t *testing.T) {
	input := fromAdminCustomFieldSetting(v1.SupportCustomFieldSettingInput{
		FieldType: "single_select", NameZhCN: "用途", NameEnUS: "Purpose",
		AppliesCustomer: true, Visibility: "customer_visible", Required: true,
		Status: "active", Options: []v1.SupportCustomFieldOptionInput{{
			OptionNo: "SCO_support", NameZhCN: "支持", NameEnUS: "Support",
			Status: "active", ExpectedVersion: 4,
		}},
	}, 7)
	if input.ExpectedVersion != 7 || len(input.Options) != 1 || input.Options[0].ExpectedVersion != 4 {
		t.Fatalf("admin setting mapper lost optimistic versions: %+v", input)
	}

	values := toAdminCustomFieldValues([]supportdesk.CustomFieldValue{{
		FieldNo: "SCF_purpose", FieldType: "single_select", Visibility: "customer_visible",
		NameZhCN: "用途", NameEnUS: "Purpose", DefinitionVersion: 3,
		NormalizedValue: "SCO_support", SelectedOptions: []supportdesk.CustomFieldOption{{
			OptionNo: "SCO_support", NameZhCN: "支持", NameEnUS: "Support",
			Status: supportdesk.SettingStatusDisabled, Version: 99,
		}},
	}})
	payload, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, `"definitionVersion":3`) || strings.Contains(text, `"status"`) || strings.Contains(text, `"version":99`) {
		t.Fatalf("admin snapshot mixed current option configuration into immutable labels: %s", text)
	}
}
