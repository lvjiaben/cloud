package notification

import "testing"

func TestTemplateVariableWhitelistAndMissingValues(t *testing.T) {
	event := &eventRow{RequiredVariables: `["ticket_no","title"]`, OptionalVariables: `["message"]`}
	if err := validateTemplateVariables("Ticket {{ticket_no}}: {{title}} {{message}}", event); err != nil {
		t.Fatalf("valid template variables were rejected: %v", err)
	}
	if err := validateTemplateVariables("{{password}}", event); err == nil {
		t.Fatal("non-whitelisted template variable must be rejected")
	}
	if _, err := renderTemplate("{{ticket_no}} {{title}}", map[string]string{"ticket_no": "T-1"}); err == nil {
		t.Fatal("missing template variables must fail before delivery")
	}
	rendered, err := renderTemplate("{{ticket_no}} {{title}}", map[string]string{"ticket_no": "T-1", "title": "Example"})
	if err != nil || rendered != "T-1 Example" {
		t.Fatalf("template rendered incorrectly: %q %v", rendered, err)
	}
}

func TestChannelTestRequiresEveryRequiredEventVariable(t *testing.T) {
	event := &eventRow{RequiredVariables: `["ticket_no","title"]`, OptionalVariables: `["message"]`}
	if err := validateTestVariables(map[string]string{"ticket_no": "T-1", "title": "Example"}, event); err != nil {
		t.Fatalf("complete required variables were rejected: %v", err)
	}
	if err := validateTestVariables(map[string]string{"ticket_no": "T-1"}, event); err == nil {
		t.Fatal("missing required test variable must fail before any provider request")
	}
	if err := validateTestVariables(map[string]string{"ticket_no": "T-1", "title": ""}, event); err == nil {
		t.Fatal("empty required test variable must fail before any provider request")
	}
}

func TestSMSProviderMappingsUseOnlyTheConfiguredProviderShape(t *testing.T) {
	variables := map[string]string{"code": "482913", "minutes": "10"}
	tests := []struct {
		name       string
		template   templateRow
		wantID     string
		wantParams []string
		wantData   map[string]string
	}{
		{
			name: "tencent ordered parameters",
			template: templateRow{ProviderCode: providerTencentSMS, ProviderTemplateID: "2299001",
				ProviderSMSVariableOrder: `["code","minutes"]`, ProviderSMSDataMapping: `{}`},
			wantID: "2299001", wantParams: []string{"482913", "10"},
		},
		{
			name: "aliyun named parameters",
			template: templateRow{ProviderCode: providerAliyunSMS, ProviderTemplateID: "SMS_123456789",
				ProviderSMSVariableOrder: `[]`, ProviderSMSDataMapping: `{"code":"code","minutes":"minutes"}`},
			wantID: "SMS_123456789", wantData: map[string]string{"code": "482913", "minutes": "10"},
		},
		{
			name: "smsbao full rendered body",
			template: templateRow{ProviderCode: providerSMSBao, ProviderSMSVariableOrder: `[]`,
				ProviderSMSDataMapping: `{}`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id, parameters, data, err := materializeSMSProviderMapping(&test.template, variables)
			if err != nil {
				t.Fatalf("valid mapping was rejected: %v", err)
			}
			if id != test.wantID || !equalStrings(parameters, test.wantParams) || !equalStringMap(data, test.wantData) {
				t.Fatalf("unexpected provider mapping: id=%q parameters=%v data=%v", id, parameters, data)
			}
		})
	}
}

func TestSMSProviderMappingFailsClosedForWrongShapeOrProviderSwitch(t *testing.T) {
	variables := map[string]string{"code": "482913"}
	invalid := []templateRow{
		{ProviderCode: providerTencentSMS, ProviderTemplateID: "2299001", ProviderSMSVariableOrder: `["missing"]`, ProviderSMSDataMapping: `{}`},
		{ProviderCode: providerAliyunSMS, ProviderTemplateID: "SMS_123", ProviderSMSVariableOrder: `["code"]`, ProviderSMSDataMapping: `{}`},
		{ProviderCode: providerSMSBao, ProviderTemplateID: "fake-template-id", ProviderSMSVariableOrder: `[]`, ProviderSMSDataMapping: `{}`},
		{ProviderCode: "unknown", ProviderSMSVariableOrder: `[]`, ProviderSMSDataMapping: `{}`},
	}
	for index := range invalid {
		if _, _, _, err := materializeSMSProviderMapping(&invalid[index], variables); err == nil {
			t.Fatalf("invalid provider mapping %d must be rejected", index)
		}
	}

	template := templateRow{TemplateNo: "NTF_SMS_CODE_ZH", Version: 1, StatusCode: "active",
		ProviderCode: providerTencentSMS, ProviderTemplateID: "2299001",
		ProviderSMSVariableOrder: `["code"]`, ProviderSMSDataMapping: `{}`}
	if materializationTemplateReady("sms", providerAliyunSMS, template) {
		t.Fatal("template from the previous SMS provider must not be used after provider switch")
	}
	if !materializationTemplateReady("sms", providerTencentSMS, template) {
		t.Fatal("template matching the active SMS provider should remain usable")
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
