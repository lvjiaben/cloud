// Renewal display-variable tests keep localization separate from billing facts.
package notification

import (
	idcnotify "lina-plugin-idc-notify"
	"strings"
	"testing"
	"testing/fstest"
)

// TestRenewalTemplateVariables verifies translated units, immutable inputs and
// unchanged unknown units without normalizing frozen money or period counts.
func TestRenewalTemplateVariables(t *testing.T) {
	labels := testRenewalPeriodLabels(t)
	for _, test := range []struct{ locale, unit, want string }{
		{"zh-CN", "day", "天"}, {"zh-CN", "month", "个月"}, {"zh-CN", "year", "年"},
		{"en-US", "day", "day"}, {"en-US", "month", "month"}, {"en-US", "year", "year"}, {"zh-CN", "week", "week"}, {"unknown", "month", "month"},
	} {
		t.Run(test.locale+"/"+test.unit, func(t *testing.T) {
			input := map[string]string{"period_unit": test.unit, "period_count": "12", "amount": "123.456", "tax_amount": "0.456", "currency": "KWD"}
			rendered, err := renewalTemplateVariables("billing.accessory_renewal_due", test.locale, input, labels)
			if err != nil {
				t.Fatal(err)
			}
			if rendered["period_unit"] != test.want {
				t.Fatalf("unit=%q want=%q", rendered["period_unit"], test.want)
			}
			for _, key := range []string{"period_count", "amount", "tax_amount", "currency"} {
				if rendered[key] != input[key] {
					t.Fatalf("frozen %s changed", key)
				}
			}
			if input["period_unit"] != test.unit {
				t.Fatal("translation mutated frozen variables")
			}
			rendered["amount"] = "changed"
			if input["amount"] != "123.456" {
				t.Fatal("rendering variables alias frozen input")
			}
			other, err := renewalTemplateVariables("billing.payment_result", test.locale, input, labels)
			if err != nil {
				t.Fatal(err)
			}
			if other["period_unit"] != test.unit {
				t.Fatal("unrelated event was translated")
			}
		})
	}
}

func testRenewalPeriodLabels(t *testing.T) renewalPeriodLabels {
	t.Helper()
	labels, err := loadRenewalPeriodLabels(idcnotify.EmbeddedFiles)
	if err != nil {
		t.Fatal(err)
	}
	return labels
}

func TestRenewalPeriodResources(t *testing.T) {
	labels := testRenewalPeriodLabels(t)
	for _, locale := range []string{"en-US", "zh-CN"} {
		if len(labels[locale]) != 3 {
			t.Fatalf("incomplete resource %s", locale)
		}
	}
	for _, test := range []struct{ name, data string }{
		{"missing", ""}, {"malformed", "{"}, {"missing_key", `{}`},
		{"blank", `{"notification.renewal.period.day":" "}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			resources := fstest.MapFS{}
			if test.data != "" {
				resources["manifest/i18n/en-US/renewal-period.json"] = &fstest.MapFile{Data: []byte(test.data)}
			}
			_, err := loadRenewalPeriodLabels(resources)
			if err == nil || !strings.Contains(err.Error(), "manifest/i18n/en-US/renewal-period.json") {
				t.Fatalf("missing resource evidence: %v", err)
			}
		})
	}
	english, err := idcnotify.EmbeddedFiles.ReadFile("manifest/i18n/en-US/renewal-period.json")
	if err != nil {
		t.Fatal(err)
	}
	_, err = loadRenewalPeriodLabels(fstest.MapFS{"manifest/i18n/en-US/renewal-period.json": &fstest.MapFile{Data: english}})
	if err == nil || !strings.Contains(err.Error(), "manifest/i18n/zh-CN/renewal-period.json") {
		t.Fatalf("missing Chinese resource accepted: %v", err)
	}
	_, err = renewalTemplateVariables(accessoryRenewalDueEventCode, "zh-CN", map[string]string{"period_unit": "month"}, nil)
	if err == nil {
		t.Fatal("missing loaded label silently accepted")
	}
}
