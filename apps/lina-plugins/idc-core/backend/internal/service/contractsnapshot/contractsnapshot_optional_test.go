package contractsnapshot

import "testing"

func TestValidOptionalContractOrderEligibility(t *testing.T) {
	base := orderRow{
		StatusCode: "fulfilled", InvoiceStatusCode: "paid",
		InvoicePaidAmountMinor: 100, InvoiceRefundedAmountMinor: 0,
		MerchantNo: "merchant-1", MerchantLegalName: "Merchant",
		MerchantCountryCode: "CN", MerchantVersion: 1,
	}
	tests := []struct {
		name   string
		mutate func(*orderRow)
		want   bool
	}{
		{name: "fulfilled order", want: true},
		{name: "partial refund", mutate: func(row *orderRow) {
			row.StatusCode = "partially_refunded"
			row.InvoiceStatusCode = "partially_refunded"
			row.InvoiceRefundedAmountMinor = 40
		}, want: true},
		{name: "cancelled order", mutate: func(row *orderRow) {
			row.StatusCode = "cancelled"
		}, want: false},
		{name: "closed invoice", mutate: func(row *orderRow) {
			row.InvoiceStatusCode = "cancelled"
		}, want: false},
		{name: "fully refunded", mutate: func(row *orderRow) {
			row.StatusCode = "refunded"
			row.InvoiceStatusCode = "refunded"
			row.InvoiceRefundedAmountMinor = 100
		}, want: false},
		{name: "future unknown order state", mutate: func(row *orderRow) {
			row.StatusCode = "closed"
		}, want: false},
		{name: "already required", mutate: func(row *orderRow) {
			row.RequiresContract = true
		}, want: false},
		{name: "stale frozen template", mutate: func(row *orderRow) {
			row.ContractTemplateNo = "ctpl_stale"
		}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := base
			if test.mutate != nil {
				test.mutate(&row)
			}
			if got := validOptionalContractOrder(&row); got != test.want {
				t.Fatalf("eligibility=%t want=%t row=%+v", got, test.want, row)
			}
		})
	}
}
