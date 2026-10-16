// These tests prove configured currency admission runs before either new quote reads product data.
package catalog

import (
	"bytes"
	"context"
	"errors"
	"lina-core/pkg/bizerr"
	"testing"
)

type currencyAdmissionStub struct {
	enabled bool
	removed bool
	err     error
	codes   []string
}

func (s *currencyAdmissionStub) IsEnabled(_ context.Context, code string) (bool, error) {
	s.codes = append(s.codes, code)
	return s.enabled, s.err
}
func (s *currencyAdmissionStub) IsRemoved(context.Context, string) (bool, error) {
	return s.removed, s.err
}
func TestRemovedCurrencyRejectsRenewal(t *testing.T) {
	source := &currencyAdmissionStub{removed: true}
	reader := &publicCatalogReaderImpl{currencies: source}
	if _, err := reader.AssembleRenewalQuote(context.Background(), RenewalQuoteInput{CurrencyCode: "USD"}); !bizerr.Is(err, CodePublicQuoteInvalid) {
		t.Fatalf("removed renewal admitted: %v", err)
	}
	source.removed = false
	if err := reader.admitRenewalCurrency(context.Background(), "USD"); err != nil {
		t.Fatalf("temporary disable changed renewal behavior: %v", err)
	}
}

// TestTrialConversionRequiresSaleCurrency proves conversion rejects disabled
// denominations before reading catalog data, while preserving owner failures.
func TestTrialConversionRequiresSaleCurrency(t *testing.T) {
	for _, storageFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "storage failure"}[storageFailure], func(t *testing.T) {
			source := &currencyAdmissionStub{}
			if storageFailure {
				source.err = errors.New("currency admission unavailable")
			}
			reader := &publicCatalogReaderImpl{currencies: source}
			lines, err := reader.AssembleRenewalQuote(context.Background(), RenewalQuoteInput{
				CurrencyCode: "USD", TrialConversion: true,
			})
			if lines != nil || storageFailure && !errors.Is(err, source.err) ||
				!storageFailure && !bizerr.Is(err, CodePublicQuoteInvalid) {
				t.Fatalf("conversion currency admitted: lines=%+v err=%v", lines, err)
			}
			if len(source.codes) != 1 || source.codes[0] != "USD" {
				t.Fatalf("conversion did not use sale currency admission: %v", source.codes)
			}
		})
	}
}
func TestNewQuotesRejectDisabledCurrency(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "reader failure"}[fail], func(t *testing.T) {
			codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{1}, 32))
			if err != nil {
				t.Fatal(err)
			}
			source := &currencyAdmissionStub{}
			if fail {
				source.err = errors.New("currency storage unavailable")
			}
			reader := &publicCatalogReaderImpl{revision: codec}
			if err = BindCurrencies(reader, source); err != nil {
				t.Fatal(err)
			}
			_, err = reader.QuoteConfiguration(context.Background(), PublicConfigurationQuoteInput{PublicSlug: "example", CurrencyCode: "USD"})
			if fail && !errors.Is(err, source.err) || !fail && !bizerr.Is(err, CodePublicQuoteInvalid) {
				t.Fatalf("legacy quote admitted: %v", err)
			}
			_, err = reader.QuotePurchaseConfiguration(context.Background(), PublicPurchaseConfigurationQuoteInput{CurrencyCode: "USD"})
			if fail && !errors.Is(err, source.err) || !fail && !bizerr.Is(err, CodePublicQuoteInvalid) {
				t.Fatalf("structured quote admitted: %v", err)
			}
			if len(source.codes) != 2 || source.codes[0] != "USD" || source.codes[1] != "USD" {
				t.Fatalf("wrong admission inputs: %v", source.codes)
			}
			source.enabled = true
			source.err = nil
			if err = reader.admitCurrency(context.Background(), "USD"); err != nil {
				t.Fatalf("enabled currency rejected: %v", err)
			}
		})
	}
}
