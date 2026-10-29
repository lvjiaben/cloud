package provisionresourcecap

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"strings"
	"testing"
	"time"
)

func TestProvisionTaskRequestRejectsLegacyRequestWithoutFrozenInterface(t *testing.T) {
	t.Parallel()
	legacy := TaskRequest{
		OperationNo: "op_1", CustomerNo: "customer_1",
		ConfigurationSnapshotNo: "config_1",
	}
	if _, err := TaskRequestDigest(legacy); err == nil {
		t.Fatal("TaskRequestDigest() accepted a legacy request without an exact interface revision")
	}
	legacy.RequestDigest = strings.Repeat("a", 64)
	if err := ValidateTaskRequest(legacy); err == nil {
		t.Fatal("ValidateTaskRequest() accepted a legacy request without an exact interface revision")
	}
}

func TestFrozenFulfillmentTaskRequestPreservesGenericSnapshots(t *testing.T) {
	t.Parallel()
	request := TaskRequest{
		OperationNo: "PFI_1", CustomerNo: "customer_1",
		ConfigurationSnapshotNo: "config_1", InterfaceInstanceNo: "pfi_11111111111111111111111111111111",
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		Fulfillment: FrozenFulfillmentIdentity{
			IntentNo: "PFI_1", OrderNo: "ORD_1", OrderItemNo: "ORI_1",
			ProductNo: "PRD_1", ProductVersionNo: "PV_1",
			ConfigurationRevision: "revision_1", UnitSequence: 1, Quantity: 1,
		},
		ProductConfiguration:       json.RawMessage(`{"merchant":"unchanged"}`),
		ProductConfigurationDigest: strings.Repeat("a", 64),
		CustomerConfiguration:      json.RawMessage(`{"customer":"unchanged"}`),
	}
	request.RequestDigest, _ = TaskRequestDigest(request)
	if err := ValidateTaskRequest(request); err != nil {
		t.Fatalf("ValidateTaskRequest() error = %v", err)
	}
	request = TaskRequest{
		OperationNo: "PFI_1", CustomerNo: "customer_1", ConfigurationSnapshotNo: "config_1",
		InterfaceInstanceNo:            "pfi_11111111111111111111111111111111",
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		Fulfillment:                    FrozenFulfillmentIdentity{IntentNo: "PFI_1", OrderNo: "ORD_1", OrderItemNo: "ORI_1", ProductNo: "PRD_1", ProductVersionNo: "PV_1", ConfigurationRevision: "revision_1", UnitSequence: 1, Quantity: 1},
		ProductConfiguration:           json.RawMessage(`{"merchant":"unchanged"}`), ProductConfigurationDigest: strings.Repeat("a", 64), CustomerConfiguration: json.RawMessage(`{"customer":"unchanged"}`),
	}
	request.RequestDigest, _ = TaskRequestDigest(request)
	request.InterfaceConfigurationRevision = "rci_22222222222222222222222222222222"
	if err := ValidateTaskRequest(request); err == nil {
		t.Fatal("ValidateTaskRequest() accepted a changed resource interface revision")
	}
}

func TestProvisionTaskProjectionRequiresTerminalTimeAndFailure(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	task := Task{
		ProviderTaskNo: "task_1", ChannelNo: "channel_1", Status: TaskStatusSucceeded,
		CapabilityEvidenceNo: "snapshot_1", CapabilityEvidenceDigest: strings.Repeat("a", 64),
		EvidenceDigest: strings.Repeat("b", 64), ObservedAt: now, CompletedAt: now,
	}
	if err := ValidateTask(task); err != nil {
		t.Fatalf("ValidateTask() error = %v", err)
	}
	task.CompletedAt = now.Add(-time.Second)
	if err := ValidateTask(task); err == nil {
		t.Fatal("ValidateTask() accepted completion before observation")
	}
	task.CompletedAt = time.Time{}
	if err := ValidateTask(task); err == nil {
		t.Fatal("ValidateTask() accepted a terminal task without completion time")
	}
	task.Status = TaskStatusResultUnknown
	task.FailureCode = "resource.result_unknown"
	if err := ValidateTask(task); err != nil {
		t.Fatalf("ValidateTask(result_unknown) error = %v", err)
	}
}

func TestOutcomeReportRequiresFrozenRequestAndIndependentEvidence(t *testing.T) {
	t.Parallel()
	report := OutcomeReport{
		ProviderTaskNo: "task_1", Request: validTaskRequest(), Status: OutcomeStatusSucceeded,
		TaskEvidenceDigest: strings.Repeat("a", 64), CoreEvidenceDigest: strings.Repeat("b", 64),
	}
	if err := ValidateOutcomeReport(report); err != nil {
		t.Fatalf("ValidateOutcomeReport() error = %v", err)
	}
	report.Request.ProductConfiguration = json.RawMessage(`{"merchant":"changed"}`)
	if err := ValidateOutcomeReport(report); err == nil {
		t.Fatal("ValidateOutcomeReport() accepted a mutated frozen request")
	}
	report.Request = validTaskRequest()
	report.Status = OutcomeStatus("result_unknown")
	if err := ValidateOutcomeReport(report); err == nil {
		t.Fatal("ValidateOutcomeReport() accepted a non-terminal resource observation")
	}
}

func validTaskRequest() TaskRequest {
	request := TaskRequest{
		OperationNo: "PFI_1", CustomerNo: "customer_1", ConfigurationSnapshotNo: "config_1",
		InterfaceInstanceNo:            "pfi_11111111111111111111111111111111",
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		Fulfillment: FrozenFulfillmentIdentity{
			IntentNo: "PFI_1", OrderNo: "ORD_1", OrderItemNo: "ORI_1",
			ProductNo: "PRD_1", ProductVersionNo: "PV_1",
			ConfigurationRevision: "revision_1", UnitSequence: 1, Quantity: 1,
		},
		ProductConfiguration:       json.RawMessage(`{"merchant":"unchanged"}`),
		ProductConfigurationDigest: strings.Repeat("a", 64),
		CustomerConfiguration:      json.RawMessage(`{"customer":"unchanged"}`),
	}
	request.RequestDigest, _ = TaskRequestDigest(request)
	return request
}

func TestNativePurchaseRequestIsBoundToItsExactFrozenInput(t *testing.T) {
	request := validTaskRequest()
	original := request.RequestDigest
	request.NativePurchaseProtocol = productfulfillmentcap.SaleImportProtocolNativeFinanceV1
	request.NativePurchase = &channelcommercecap.Purchase{PublicSlug: "41", ConfigurationRevision: "revision", BillingTermToken: "monthly", CurrencyCode: "CNY", PurchaseCycles: 1, Quantity: 1}
	digest, err := TaskRequestDigest(request)
	if err != nil || digest == original {
		t.Fatalf("native input was not fenced: %v", err)
	}
	request.RequestDigest = digest
	if err = ValidateTaskRequest(request); err != nil {
		t.Fatal(err)
	}
	request.NativePurchase.BillingTermToken = "annual"
	if ValidateTaskRequest(request) == nil {
		t.Fatal("changed native billing token reused request")
	}
	request.NativePurchase.PurchaseCycles = 2
	if _, err = TaskRequestDigest(request); err == nil {
		t.Fatal("unsupported native cycles accepted")
	}
}
