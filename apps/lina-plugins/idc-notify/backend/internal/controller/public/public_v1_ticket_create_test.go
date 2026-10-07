package public

import (
	"reflect"
	"testing"

	v1 "lina-plugin-idc-notify/backend/api/public/v1"
)

func TestSupportCustomFieldValuesPreservesTypedSubmission(t *testing.T) {
	textValue := "Dedicated server"
	numberValue := "12.50"
	dateValue := "2026-08-15"
	booleanValue := false
	input := []v1.TicketCustomFieldSubmission{
		{FieldNo: "SCF_text", TextValue: &textValue},
		{FieldNo: "SCF_number", NumberValue: &numberValue},
		{FieldNo: "SCF_date", DateValue: &dateValue},
		{FieldNo: "SCF_boolean", BooleanValue: &booleanValue},
		{FieldNo: "SCF_options", OptionNos: []string{"SCO_compute", "SCO_network"}},
	}

	result := supportCustomFieldValues(input)
	if len(result) != len(input) {
		t.Fatalf("unexpected result length: got=%d want=%d", len(result), len(input))
	}
	for index := range input {
		if result[index].FieldNo != input[index].FieldNo || result[index].TextValue != input[index].TextValue ||
			result[index].NumberValue != input[index].NumberValue || result[index].DateValue != input[index].DateValue ||
			result[index].BooleanValue != input[index].BooleanValue || !reflect.DeepEqual(result[index].OptionNos, input[index].OptionNos) {
			t.Fatalf("typed value changed at index %d: got=%+v want=%+v", index, result[index], input[index])
		}
	}
	input[4].OptionNos[0] = "SCO_changed"
	if result[4].OptionNos[0] != "SCO_compute" {
		t.Fatalf("option values alias the HTTP request buffer: %+v", result[4].OptionNos)
	}
}
