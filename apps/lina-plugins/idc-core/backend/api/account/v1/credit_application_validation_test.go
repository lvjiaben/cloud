package v1

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestCreditApplicationUploadNumbersAreNotValidatedAsAString(t *testing.T) {
	req := &CreditApplicationSubmitReq{
		RequestedLimitMinor: "100000",
		CurrencyCode:        "CNY",
		RequestedValidFrom:  1_720_000_000_000,
		RequestedValidUntil: 1_751_536_000_000,
		UploadNos:           []string{"CEU_436C3C2CB82BD5ACB9B62C0771A828A0"},
		IdempotencyKey:      "credit-validation-0001",
	}
	if err := g.Validator().Data(req).Run(context.Background()); err != nil {
		t.Fatalf("one upload number must pass request binding validation: %v", err)
	}

	// The DTO deliberately validates only presence. Element-count limits are checked
	// before persistence by creditApplicationIntent, where slices are counted correctly.
	req.UploadNos = make([]string, 11)
	for index := range req.UploadNos {
		req.UploadNos[index] = "CEU_436C3C2CB82BD5ACB9B62C0771A828A0"
	}
	if err := g.Validator().Data(req).Run(context.Background()); err != nil {
		t.Fatalf("request binding must not measure the serialized slice as text: %v", err)
	}
}
