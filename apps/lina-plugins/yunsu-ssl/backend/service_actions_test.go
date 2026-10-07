package backend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

func TestServiceSupplierRequestsHaveBoundedDeadline(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"issued"}}`)}}}
	if _, err := ExecuteServiceSyncAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionPayload(t, "sync", "")); err != nil {
		t.Fatal(err)
	}
	if len(interfaces.contexts) != 1 {
		t.Fatalf("supplier contexts=%d", len(interfaces.contexts))
	}
	if deadline, ok := interfaces.contexts[0].Deadline(); !ok || deadline.Sub(time.Now()) > 30*time.Second || deadline.Sub(time.Now()) < 20*time.Second {
		t.Fatalf("supplier request has no bounded deadline: deadline=%v ok=%v", deadline, ok)
	}
	if err := interfaces.contexts[0].Err(); err != context.Canceled {
		t.Fatalf("supplier context was not cancelled after completion: %v", err)
	}
}

func TestUpdateValidationRequiresPendingOrderAndReturnsSupplierEvidence(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{
		{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"pending"}}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"success":true}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"verifying","dcv_info":{"method":"file","filename":"proof.txt","path":"/.well-known/pki-validation/proof.txt","content":"proof"}}}`)},
	}}
	payload := serviceActionPayload(t, "update-validation", "file")
	result, err := ExecuteUpdateValidationAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", payload)
	if err != nil || len(interfaces.requests) != 3 {
		t.Fatalf("result=%s calls=%d err=%v", result, len(interfaces.requests), err)
	}
	paths := []string{interfaces.requests[0].Path, interfaces.requests[1].Path, interfaces.requests[2].Path}
	if strings.Join(paths, ",") != "/order/sync,/order/update-dcv,/order/sync" || !strings.Contains(string(result), "proof.txt") {
		t.Fatalf("unexpected validation workflow: paths=%v result=%s", paths, result)
	}
}

func TestIssuedCertificateRejectsCancellationBeforeSupplierMutation(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{
		{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"issued","issued_at":"2026-08-28"}}`)},
	}}
	if _, err := ExecuteCancelApplicationAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionPayload(t, "cancel", "")); err == nil {
		t.Fatal("issued certificate cancellation was accepted")
	}
	if len(interfaces.requests) != 1 || interfaces.requests[0].Path != "/order/sync" {
		t.Fatalf("supplier mutation occurred after issuance: %#v", interfaces.requests)
	}
}

func TestCancellationDoesNotCallRefundOrDeleteHistory(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{
		{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"awaiting_validation"}}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"success":true}`)},
	}}
	result, err := ExecuteCancelApplicationAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionPayload(t, "cancel", ""))
	if err != nil || len(interfaces.requests) != 2 || interfaces.requests[1].Path != "/order/cancel" {
		t.Fatalf("unexpected cancellation: result=%s requests=%#v err=%v", result, interfaces.requests, err)
	}
	encoded, _ := json.Marshal(interfaces.requests)
	if strings.Contains(strings.ToLower(string(encoded)), "refund") || strings.Contains(strings.ToLower(string(encoded)), "delete") {
		t.Fatalf("cancellation attempted a refund or deletion: %s", encoded)
	}
}

func TestDetailRejectsUnsafeSupplierFilePath(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{
		StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"pending","validation_info":{"method":"file","path":"../../private.key","content":"bad"}}}`),
	}}}
	if _, err := ExecuteServiceDetailAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionPayload(t, "detail", "")); err == nil {
		t.Fatal("unsafe supplier file path was projected")
	}
}

func TestServiceSyncProjectsOnlyRealSupplierDates(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{
		StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"issued","issued_at":"2026-08-28","expires_at":"2027-08-28"}}`),
	}}}
	result, err := ExecuteServiceSyncAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionPayload(t, "sync", ""))
	if err != nil || !strings.Contains(string(result), `"issuedAt":"2026-08-28"`) || !strings.Contains(string(result), `"expiresAt":"2027-08-28"`) {
		t.Fatalf("supplier dates were not projected exactly: result=%s err=%v", result, err)
	}
}

func TestServiceSyncDecidesIdempotentAndForbiddenMutationsBeforeSubmit(t *testing.T) {
	for _, testCase := range []struct{ name, upstream, target, validation, expected string }{
		{"cancel already complete", "cancelled", "cancel", "", productfulfillmentcap.ManagedServiceQueryDecisionAlreadySucceeded},
		{"issued cancellation forbidden", "issued", "cancel", "", productfulfillmentcap.ManagedServiceQueryDecisionForbidden},
		{"same validation already applied", "pending", "update-validation", "file", productfulfillmentcap.ManagedServiceQueryDecisionAlreadySucceeded},
		{"pending revalidation can submit", "pending", "revalidate", "", productfulfillmentcap.ManagedServiceQueryDecisionSubmitAllowed},
	} {
		name, upstream, target, validation, expected := testCase.name, testCase.upstream, testCase.target, testCase.validation, testCase.expected
		t.Run(name, func(t *testing.T) {
			validationProjection := ""
			if validation != "" {
				validationProjection = `,"dcv_info":{"method":"` + validation + `","filename":"proof.txt","path":"/.well-known/pki-validation/proof.txt","content":"proof"}`
			}
			interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"` + upstream + `"` + validationProjection + `}}`)}}}
			result, err := ExecuteServiceSyncAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionQueryPayload(t, target, validation))
			if err != nil || !strings.Contains(string(result), `"decision":"`+expected+`"`) {
				t.Fatalf("unexpected decision result=%s err=%v", result, err)
			}
		})
	}
}

func TestRevalidationUsesPreIssuanceCheckThenRefresh(t *testing.T) {
	interfaces := &supplierInterfaceStub{results: []*supplierinterfacecap.OutboundHTTPResponse{
		{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"validation"}}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"success":true}`)},
		{StatusCode: 200, Data: json.RawMessage(`{"success":true,"data":{"status":"verifying"}}`)},
	}}
	result, err := ExecuteRevalidateAdapter(interfaces, "src_0123456789abcdef0123456789abcdef", serviceActionPayload(t, "revalidate", ""))
	if err != nil || len(interfaces.requests) != 3 {
		t.Fatalf("result=%s calls=%d err=%v", result, len(interfaces.requests), err)
	}
	paths := []string{interfaces.requests[0].Path, interfaces.requests[1].Path, interfaces.requests[2].Path}
	if strings.Join(paths, ",") != "/order/sync,/order/revalidate,/order/sync" {
		t.Fatalf("unexpected revalidation workflow: %v", paths)
	}
}

func serviceActionPayload(t *testing.T, actionKey, validationMethod string) json.RawMessage {
	t.Helper()
	product := testProductConfiguration(t, "ordinary", "dv", true, true)
	idempotencyKey := ""
	if actionKey != "detail" && actionKey != "sync" {
		idempotencyKey = "action-10001"
	}
	input, err := json.Marshal(map[string]string{"validationMethod": validationMethod})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(productfulfillmentcap.ManagedServiceActionRequest{
		Version: productfulfillmentcap.ManagedServiceActionVersion, ActionKey: actionKey,
		EntitlementNo: "ent_10001", IdempotencyKey: idempotencyKey, ExternalReference: "YS-10001",
		ProductConfiguration: product, Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func serviceActionQueryPayload(t *testing.T, targetAction, validationMethod string) json.RawMessage {
	t.Helper()
	product := testProductConfiguration(t, "ordinary", "dv", true, true)
	input, _ := json.Marshal(map[string]string{"validationMethod": validationMethod})
	payload, err := json.Marshal(productfulfillmentcap.ManagedServiceActionRequest{Version: productfulfillmentcap.ManagedServiceActionVersion, ActionKey: "sync", RequestedActionKey: targetAction, EntitlementNo: "ent_10001", ExternalReference: "YS-10001", ProductConfiguration: product, Input: input})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
