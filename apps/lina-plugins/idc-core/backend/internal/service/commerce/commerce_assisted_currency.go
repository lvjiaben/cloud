// This file binds non-sensitive enabled denominations into staff-assisted ordering.
package commerce

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
)

// AssistedCurrencyReader returns configured denominations without reading financial data.
type AssistedCurrencyReader interface {
	Enabled(context.Context) ([]string, error)
	IsEnabled(context.Context, string) (bool, error)
}

// BindAssistedCurrencies attaches the startup-owned reader before assisted routes are bound.
func BindAssistedCurrencies(service AssistedQuoteService, source AssistedCurrencyReader) error {
	impl, ok := service.(*assistedQuoteServiceImpl)
	if !ok || impl == nil || source == nil || impl.currencies != nil {
		return gerror.New("assisted currency source requires one binding")
	}
	impl.currencies = source
	return nil
}

func (s *assistedQuoteServiceImpl) admitAssistedCurrency(ctx context.Context, code string) error {
	if s.currencies == nil {
		return nil
	}
	enabled, err := s.currencies.IsEnabled(ctx, code)
	if err != nil {
		return err
	}
	if !enabled {
		return bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	return nil
}
