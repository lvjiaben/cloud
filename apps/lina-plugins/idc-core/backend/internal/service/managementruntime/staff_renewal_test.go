package managementruntime

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

type staffRenewalServiceFake struct {
	quotedCustomer, confirmedCustomer string
	confirmedActor                    int
}

func (f *staffRenewalServiceFake) QuoteForStaff(
	_ context.Context, customerNo string, input renewal.QuoteInput,
) (*renewal.Quote, error) {
	f.quotedCustomer = customerNo
	return &renewal.Quote{
		QuoteNo: "QUO_1", EntitlementNo: input.EntitlementNo, CurrencyCode: "CNY",
		RenewalCycles: input.RenewalCycles, TotalAmountMinor: 9900,
		OriginalExpiresAt: time.Unix(100, 0), ProjectedExpiresAt: time.Unix(200, 0),
	}, nil
}

func (f *staffRenewalServiceFake) ConfirmForStaff(
	_ context.Context, customerNo string, _ renewal.ConfirmInput, actor int,
) (*renewal.Operation, error) {
	f.confirmedCustomer, f.confirmedActor = customerNo, actor
	return &renewal.Operation{
		OperationNo: "OP_1", QuoteNo: "QUO_1", InvoiceNo: "INV_1",
		EntitlementNo: "ENT_1", CurrencyCode: "CNY", Status: "payment_pending",
		TotalAmountMinor: 9900,
	}, nil
}

func TestStaffRenewalAdapterKeepsCoreBusinessFactsAuthoritative(t *testing.T) {
	fake := &staffRenewalServiceFake{}
	manager := &staffRenewalManager{service: fake}
	quote, err := manager.QuoteForStaff(context.Background(), managementcap.StaffRenewalQuoteInput{
		CustomerNo: "CUS_1", EntitlementNo: "ENT_1", RenewalCycles: 2,
		IdempotencyKey: "preview-key-1",
	})
	if err != nil || quote == nil || quote.TotalAmountMinor != 9900 ||
		quote.ProjectedExpiresAt.Unix() != 200 || fake.quotedCustomer != "CUS_1" {
		t.Fatalf("unexpected governed staff quote: quote=%+v err=%v fake=%+v", quote, err, fake)
	}
	operation, err := manager.ConfirmForStaff(context.Background(), managementcap.StaffRenewalConfirmInput{
		CustomerNo: "CUS_1", QuoteNo: "QUO_1", ActorUserID: 7,
		IdempotencyKey: "confirm-key-1",
	})
	if err != nil || operation == nil || operation.InvoiceNo != "INV_1" ||
		fake.confirmedCustomer != "CUS_1" || fake.confirmedActor != 7 {
		t.Fatalf("unexpected governed staff confirmation: operation=%+v err=%v fake=%+v", operation, err, fake)
	}
}
