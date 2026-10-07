package supportcap

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreateRequestAndOperationDigest(t *testing.T) {
	textValue, boolValue := "rack preference", false
	request := CreateAnonymousPresalesRequest{
		DestinationKind: DestinationEmail, DestinationDigest: strings.Repeat("a", 64),
		CategoryNo: "CAT_public", Subject: "Need a server", Body: "Please contact me", OperationNo: "otp-flow:01234567",
		CustomFields: []CustomFieldValue{
			{FieldNo: "SCF_text", TextValue: &textValue},
			{FieldNo: "SCF_boolean", BooleanValue: &boolValue},
			{FieldNo: "SCF_select", OptionNos: []string{"SCO_one", "SCO_two"}},
		},
	}
	if err := ValidateCreateRequest(request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	first, err := OperationDigest(request.OperationNo)
	if err != nil || len(first) != 64 {
		t.Fatalf("operation digest=%q err=%v", first, err)
	}
	second, err := OperationDigest(request.OperationNo)
	if err != nil || second != first {
		t.Fatalf("operation digest is not stable: %q / %q err=%v", first, second, err)
	}
	renew, err := RenewOperationDigest(request.OperationNo)
	if err != nil || renew == first {
		t.Fatalf("renew digest=%q must be purpose-separated from create=%q err=%v", renew, first, err)
	}

	invalid := []CreateAnonymousPresalesRequest{
		{DestinationKind: "push", DestinationDigest: request.DestinationDigest, CategoryNo: request.CategoryNo, Subject: request.Subject, Body: request.Body, OperationNo: request.OperationNo},
		{DestinationKind: request.DestinationKind, DestinationDigest: strings.Repeat("A", 64), CategoryNo: request.CategoryNo, Subject: request.Subject, Body: request.Body, OperationNo: request.OperationNo},
		{DestinationKind: request.DestinationKind, DestinationDigest: request.DestinationDigest, CategoryNo: request.CategoryNo, Subject: " raw ", Body: request.Body, OperationNo: request.OperationNo},
		{DestinationKind: request.DestinationKind, DestinationDigest: request.DestinationDigest, CategoryNo: request.CategoryNo, Subject: request.Subject, Body: request.Body, OperationNo: "plain destination@example.com"},
	}
	ambiguous := request
	ambiguous.CustomFields = []CustomFieldValue{{FieldNo: "SCF_text", TextValue: &textValue, BooleanValue: &boolValue}}
	invalid = append(invalid, ambiguous)
	duplicateField := request
	duplicateField.CustomFields = []CustomFieldValue{{FieldNo: "SCF_text", TextValue: &textValue}, {FieldNo: "SCF_text", TextValue: &textValue}}
	invalid = append(invalid, duplicateField)
	duplicateOption := request
	duplicateOption.CustomFields = []CustomFieldValue{{FieldNo: "SCF_select", OptionNos: []string{"SCO_one", "SCO_one"}}}
	invalid = append(invalid, duplicateOption)
	for index := range invalid {
		if err = ValidateCreateRequest(invalid[index]); err == nil {
			t.Fatalf("invalid request %d accepted", index)
		}
	}
}

func TestValidateRenewRequestAndResult(t *testing.T) {
	request := RenewAnonymousPresalesAccessRequest{
		DestinationKind: DestinationSMS, DestinationDigest: strings.Repeat("b", 64),
		TicketNo: "TKT_existing_anonymous", OperationNo: "otp-renew:01234567",
	}
	if err := ValidateRenewRequest(request); err != nil {
		t.Fatalf("valid renewal rejected: %v", err)
	}
	invalid := request
	invalid.TicketNo = "customer@example.com"
	if err := ValidateRenewRequest(invalid); err == nil {
		t.Fatal("plaintext-like ticket number accepted")
	}
	result := RenewAnonymousPresalesAccessResult{
		TicketNo: request.TicketNo, AccessToken: "0123456789abcdef0123456789abcdef0123456789a",
		ExpiresAt: time.Now().UTC(),
	}
	if err := ValidateRenewResult(result); err != nil {
		t.Fatalf("valid renewal result rejected: %v", err)
	}
}

func TestValidateCreateResult(t *testing.T) {
	valid := CreateAnonymousPresalesResult{
		TicketNo:    "TKT_0123456789abcdef0123456789abcdef",
		AccessToken: "0123456789abcdef0123456789abcdef0123456789a",
		ExpiresAt:   time.Now().UTC(),
	}
	if err := ValidateCreateResult(valid); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	invalid := valid
	invalid.AccessToken = "plaintext@example.com"
	if err := ValidateCreateResult(invalid); err == nil {
		t.Fatal("malformed access token accepted")
	}
}
