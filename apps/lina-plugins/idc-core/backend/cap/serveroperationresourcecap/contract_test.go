package serveroperationresourcecap

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestTaskRequestDigestRejectsTampering(t *testing.T) {
	request := TaskRequest{OperationNo: "SOP_1", ServiceNo: "SVC_1", ChannelNo: "CH_1", PoolNo: "POOL_1", Action: ActionStart, ExpectedServiceVersion: 1, Parameters: json.RawMessage(`{}`)}
	digest, err := TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestDigest = digest
	if err = ValidateTaskRequest(request); err != nil {
		t.Fatal(err)
	}
	request.Action = ActionStop
	if ValidateTaskRequest(request) == nil {
		t.Fatal("tampered request was accepted")
	}
}

func TestValidateOwnershipTransferRequest(t *testing.T) {
	request := OwnershipTransferRequest{TransferNo: "TRF_1", ServiceNo: "ent_0123456789abcdef0123456789abcdef",
		SourceCustomerNo: "10000", TargetCustomerNo: "10001"}
	if err := ValidateOwnershipTransferRequest(request); err != nil {
		t.Fatal(err)
	}
	request.TargetCustomerNo = request.SourceCustomerNo
	if ValidateOwnershipTransferRequest(request) == nil {
		t.Fatal("same-customer server transfer was accepted")
	}
}

func TestIPReservationRequestDigestRejectsChangedFrozenTerms(t *testing.T) {
	now := time.Now().UTC().Add(10 * time.Minute).Truncate(time.Millisecond)
	request := IPReservationRequest{
		Route:    CapabilityRoute{ServiceNo: "SVC_IP_1", ChannelNo: "CH_IP_1", PoolNo: "POOL_IP_1"},
		Quantity: 2, MaximumCapacityAgeSeconds: 300, ExpiresAt: now,
		IdempotencyKey: "ip-change-SOP_1",
	}
	digest, err := IPReservationRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestDigest = digest
	if err = ValidateIPReservationRequest(request); err != nil {
		t.Fatal(err)
	}
	request.Quantity = 3
	if ValidateIPReservationRequest(request) == nil {
		t.Fatal("changed IP quantity was accepted with the frozen digest")
	}
	request.Quantity = 2
	request.Route.PoolNo = "POOL_IP_2"
	if ValidateIPReservationRequest(request) == nil {
		t.Fatal("changed resource pool was accepted with the frozen digest")
	}
}

func TestIPReservationRequestRejectsUnsafeFreshnessAndProjection(t *testing.T) {
	request := IPReservationRequest{
		Route:    CapabilityRoute{ServiceNo: "SVC_IP_2", ChannelNo: "CH_IP_2", PoolNo: "POOL_IP_2"},
		Quantity: 1, MaximumCapacityAgeSeconds: 30,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute), IdempotencyKey: "ip-change-SOP_2",
	}
	if _, err := IPReservationRequestDigest(request); err == nil {
		t.Fatal("an unsafe inventory freshness window was accepted")
	}
	reservation := IPReservation{
		ReservationNo: "IPR_1", CapacityObservationNo: "CAP_1", Quantity: 1,
		Status: IPReservationReserved, ExpiresAt: request.ExpiresAt,
	}
	if err := ValidateIPReservation(reservation); err != nil {
		t.Fatal(err)
	}
	reservation.Status = "unknown"
	if ValidateIPReservation(reservation) == nil {
		t.Fatal("an unknown reservation status was accepted")
	}
	reservation.Status, reservation.Quantity = IPReservationReserved, 0
	if ValidateIPReservation(reservation) == nil {
		t.Fatal("a zero-quantity reservation was accepted")
	}
}

func TestValidateTaskConfidentialOutputsAreOpaqueAndPolicyBound(t *testing.T) {
	now := time.Now().UTC()
	handle, err := secretcap.NewHandle("secret-output-console")
	if err != nil {
		t.Fatal(err)
	}
	task := Task{ProviderTaskNo: "RST_2", Status: TaskStatusSucceeded,
		CapabilityEvidenceNo: "CAP_2", CapabilityEvidenceDigest: strings.Repeat("c", 64),
		EvidenceDigest: strings.Repeat("d", 64), ObservedAt: now, CompletedAt: now,
		SecretOutputs: []SecretOutput{{Name: "console_ticket",
			Policy: SecretOutputPolicyShortLivedTicket, Handle: handle,
			ExpiresAt: now.Add(time.Minute)}}}
	if err = ValidateTask(task); err != nil {
		t.Fatal(err)
	}
	task.Status = TaskStatusRunning
	task.CompletedAt = time.Time{}
	if ValidateTask(task) == nil {
		t.Fatal("unfinished task published a confidential output")
	}
	task.Status, task.CompletedAt = TaskStatusSucceeded, now
	task.SecretOutputs[0].ExpiresAt = time.Time{}
	if ValidateTask(task) == nil {
		t.Fatal("short-lived ticket without expiry was accepted")
	}
}

func TestValidateTaskForRequestRejectsOutputOnWrongAction(t *testing.T) {
	now := time.Now().UTC()
	handle, err := secretcap.NewHandle("secret-output-policy")
	if err != nil {
		t.Fatal(err)
	}
	request := TaskRequest{OperationNo: "SOP_output", ServiceNo: "SVC_output",
		ChannelNo: "CH_output", PoolNo: "POOL_output", Action: ActionConsole,
		ExpectedServiceVersion: 1, Parameters: json.RawMessage(`{}`)}
	request.RequestDigest, err = TaskRequestDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	task := Task{ProviderTaskNo: "RST_output", Status: TaskStatusSucceeded,
		CapabilityEvidenceNo: "CAP_output", CapabilityEvidenceDigest: strings.Repeat("e", 64),
		EvidenceDigest: strings.Repeat("f", 64), ObservedAt: now, CompletedAt: now,
		SecretOutputs: []SecretOutput{{Name: "consoleTicket",
			Policy: SecretOutputPolicyShortLivedTicket, Handle: handle,
			ExpiresAt: now.Add(time.Minute)}}}
	if err = ValidateTaskForRequest(task, request); err != nil {
		t.Fatal(err)
	}
	request.Action = ActionReboot
	request.RequestDigest, _ = TaskRequestDigest(request)
	if ValidateTaskForRequest(task, request) == nil {
		t.Fatal("console ticket was accepted for reboot")
	}
}

func TestValidateTaskRequiresTerminalEvidence(t *testing.T) {
	now := time.Now().UTC()
	task := Task{ProviderTaskNo: "RST_1", Status: TaskStatusSucceeded, CapabilityEvidenceNo: "CAP_1", CapabilityEvidenceDigest: strings.Repeat("a", 64), EvidenceDigest: strings.Repeat("b", 64), ObservedAt: now, CompletedAt: now}
	if err := ValidateTask(task); err != nil {
		t.Fatal(err)
	}
	task.CompletedAt = time.Time{}
	if ValidateTask(task) == nil {
		t.Fatal("terminal task without completion time was accepted")
	}
}
