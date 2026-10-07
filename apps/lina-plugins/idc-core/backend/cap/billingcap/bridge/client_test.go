package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/billingcap"
)

func TestBridgeInvokesExactOwnerMethodAndValidatesResult(t *testing.T) {
	t.Parallel()
	input := billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_bridge_0001", InvoiceNo: "INV_bridge_0001",
		CurrencyCode: "CNY", AmountMinor: 29900, TotalAmountMinor: 29900, EvidenceDigest: strings.Repeat("a", 64),
	}
	calls := 0
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		calls++
		if owner != billingcap.OwnerPluginID || service != billingcap.ServiceName ||
			version != string(billingcap.ContractVersion) ||
			method != string(billingcap.MethodChannelPaymentConfirm) ||
			resourceRef != "" || table != "" {
			t.Fatalf("unexpected bridge identity: %q %q %q %q %q %q", owner, service, version, method, resourceRef, table)
		}
		var decoded billingcap.ConfirmedChannelPayment
		if decodeErr := json.Unmarshal(payload, &decoded); decodeErr != nil || decoded != input {
			t.Fatalf("unexpected bridge payload=%+v err=%v", decoded, decodeErr)
		}
		return json.Marshal(billingcap.ChannelPaymentResult{
			PaymentAttemptNo: input.PaymentAttemptNo,
			TransactionNo:    "btx_0123456789abcdef0123456789abcdef",
			AllocationNo:     "bpa_0123456789abcdef0123456789abcdef",
			InvoiceNo:        input.InvoiceNo, OrderNo: "ORD_bridge_0001", CurrencyCode: "CNY",
			AmountMinor: 29900, TotalAmountMinor: 29900, PaidAmountMinor: 29900, InvoiceStatus: "paid",
			SettlementNo: "fst_0123456789abcdef0123456789abcdef",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ConfirmChannelPayment(context.Background(), input)
	if err != nil || result == nil || result.SettlementNo == "" || calls != 1 {
		t.Fatalf("bridge result=%+v calls=%d err=%v", result, calls, err)
	}
}
