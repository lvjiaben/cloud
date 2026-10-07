package v1

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestBalanceRechargeCurrencyRequiresExactlyThreeCharacters(t *testing.T) {
	valid := &BalanceRechargeCreateReq{
		ChannelNo: "PAYCH_TEST_0001", ModeCode: "epay_cashier",
		CurrencyCode: "CNY", AmountMinor: "1234",
	}
	if err := g.Validator().Data(valid).Run(context.Background()); err != nil {
		t.Fatalf("valid three-character currency rejected: %v", err)
	}

	for _, currency := range []string{"CN", "CNYX"} {
		invalid := *valid
		invalid.CurrencyCode = currency
		if err := g.Validator().Data(&invalid).Run(context.Background()); err == nil {
			t.Fatalf("invalid currency length accepted: %q", currency)
		}
	}
}
