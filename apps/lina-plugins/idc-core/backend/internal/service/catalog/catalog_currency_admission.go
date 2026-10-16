// This file injects merchant denomination admission into every new public quote path.
package catalog

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
)

// CurrencyReader is the non-sensitive owner admission contract for new quotes.
type CurrencyReader interface {
	IsEnabled(context.Context, string) (bool, error)
	IsRemoved(context.Context, string) (bool, error)
}

// BindCurrencies binds a stateless authoritative admission source during startup.
func BindCurrencies(reader CatalogReader, source CurrencyReader) error {
	impl, ok := reader.(*publicCatalogReaderImpl)
	if !ok || impl == nil || source == nil || impl.currencies != nil {
		return gerror.New("catalog currencies require one admission source")
	}
	impl.currencies = source
	return nil
}
func (s *publicCatalogReaderImpl) admitCurrency(ctx context.Context, code string) error {
	if s.currencies == nil {
		return nil
	} // Legacy internal readers do not serve configured production purchase routes.
	enabled, err := s.currencies.IsEnabled(ctx, code)
	if err != nil {
		return err
	}
	if !enabled {
		return bizerr.NewCode(CodePublicQuoteInvalid)
	}
	return nil
}

// admitRenewalCurrency rejects removed configuration while retaining the existing temporary-disable renewal rule.
func (s *publicCatalogReaderImpl) admitRenewalCurrency(ctx context.Context, code string) error {
	if s.currencies == nil {
		return nil
	}
	removed, err := s.currencies.IsRemoved(ctx, code)
	if err != nil {
		return err
	}
	if removed {
		return bizerr.NewCode(CodePublicQuoteInvalid)
	}
	return nil
}
