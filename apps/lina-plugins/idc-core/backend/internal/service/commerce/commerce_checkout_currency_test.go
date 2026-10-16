// These tests distinguish removed configuration from temporary disable and preserve failure-closed checkout admission.
package commerce

import (
	"context"
	"errors"
	"lina-core/pkg/bizerr"
	"testing"
)

type checkoutCurrencyStub struct {
	removed bool
	err     error
}

func (s checkoutCurrencyStub) IsRemoved(context.Context, string) (bool, error) {
	return s.removed, s.err
}
func TestCheckoutRemovedCurrencyAdmission(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		removed bool
		err     error
		reject  bool
	}{{"removed", true, nil, true}, {"existing", false, nil, false}, {"storage failure", false, errors.New("offline"), true}} {
		t.Run(scenario.name, func(t *testing.T) {
			owner := &serviceImpl{currencies: checkoutCurrencyStub{scenario.removed, scenario.err}}
			err := owner.admitCheckoutCurrency(context.Background(), "USD")
			if scenario.reject && !bizerr.Is(err, CodeQuoteNotAvailable) || !scenario.reject && err != nil {
				t.Fatalf("checkout admission: %v", err)
			}
		})
	}
}
