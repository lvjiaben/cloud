// This file prevents checkout of an older quote after its currency configuration is removed.
package commerce

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
)

// CheckoutCurrencyReader distinguishes removed configuration from existing money history.
type CheckoutCurrencyReader interface {
	IsRemoved(context.Context, string) (bool, error)
}

// BindCheckoutCurrencies binds shared admission before serving checkout routes; existing order replays remain valid.
func BindCheckoutCurrencies(owner Service, source CheckoutCurrencyReader) error {
	impl, ok := owner.(*serviceImpl)
	if !ok || impl == nil || source == nil || impl.currencies != nil {
		return gerror.New("checkout currencies require one reader")
	}
	impl.currencies = source
	return nil
}
func (s *serviceImpl) admitCheckoutCurrency(ctx context.Context, code string) error {
	if s.currencies == nil {
		return nil
	}
	removed, err := s.currencies.IsRemoved(ctx, code)
	if err != nil {
		return bizerr.WrapCode(err, CodeQuoteNotAvailable)
	}
	if removed {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return nil
}
