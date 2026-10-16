// This file calculates imported draft prices from exact decimal text. Currency
// precision must come from the local currency authority; rounding occurs only
// after markup, and neither floating point nor a currency fallback is used.

package catalog

import (
	"encoding/json"
	"math/big"
	"regexp"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// calculatePortableImportPrices freezes each explicit upstream component after
// markup. Whole-item fixed prices remain replacements, including prices above
// the ordinary configuration total; discounts remain whole-item percentages.
func calculatePortableImportPrices(snapshot *channelcommercecap.ImportSnapshot, percent string) ([]importPrice, error) {
	if snapshot == nil || snapshot.Validate() != nil {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	encoded, err := json.Marshal(snapshot.Prices)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementInvalid)
	}
	var schedules []channelcommercecap.ImportPriceSchedule
	if err = json.Unmarshal(encoded, &schedules); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementInvalid)
	}
	result := make([]importPrice, 0, len(schedules))
	for index := range schedules {
		schedule := &schedules[index]
		if _, err = importCurrencyMinorUnit(schedule.CurrencyCode); err != nil {
			return nil, err
		}
		if err = markupImportBands(schedule.Base, percent); err != nil {
			return nil, err
		}
		for option := range schedule.Options {
			if err = markupImportBands(schedule.Options[option].Bands, percent); err != nil {
				return nil, err
			}
		}
		for rule := schedule.WholeItemRule; rule != nil; rule = rule.Prior {
			if rule.FixedAmountMinor == nil {
				continue
			}
			amount, amountErr := markupImportMinor(*rule.FixedAmountMinor, percent)
			if amountErr != nil {
				return nil, amountErr
			}
			rule.FixedAmountMinor = &amount
		}
		result = append(result, importPrice{portable: schedule})
	}
	return result, nil
}

func markupImportBands(bands []channelcommercecap.ImportPriceBand, percent string) error {
	for index := range bands {
		band := &bands[index]
		var err error
		band.AmountMinor, err = markupImportMinor(band.AmountMinor, percent)
		if err != nil {
			return err
		}
		band.SetupAmountMinor, err = markupImportMinor(band.SetupAmountMinor, percent)
		if err != nil {
			return err
		}
		if band.RenewalAmountMinor != nil {
			renewal, renewalErr := markupImportMinor(*band.RenewalAmountMinor, percent)
			if renewalErr != nil {
				return renewalErr
			}
			band.RenewalAmountMinor = &renewal
		}
	}
	return nil
}

func markupImportMinor(amount int64, percent string) (int64, error) {
	if amount < 0 {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	markup, err := parseImportPriceDecimal(percent)
	if err != nil {
		return 0, err
	}
	markup.Quo(markup, big.NewRat(100, 1)).Add(markup, big.NewRat(1, 1))
	markup.Mul(markup, new(big.Rat).SetInt64(amount))
	return roundImportPriceMinor(markup, 0)
}

// importPriceDecimalPattern restricts math/big's parser to unsigned decimal
// amounts, excluding signs, exponents, fractions, separators and special values.
var importPriceDecimalPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

// importMarkupPriceMinor adds markupPercent percent to an upstream major-unit
// amount and rounds the final price half up to the local currency's minor unit.
// For example, "100" with "10" produces 11000 at precision 2. Zero amounts and
// zero markup are valid. Invalid text, negative precision and int64 overflow
// return CodeManagementInvalid; no business maximum percentage is imposed.
func importMarkupPriceMinor(upstreamAmount, markupPercent string, minorUnit int) (int64, error) {
	amount, err := parseImportPriceDecimal(upstreamAmount)
	if err != nil {
		return 0, err
	}
	markup, err := parseImportPriceDecimal(markupPercent)
	if err != nil {
		return 0, err
	}
	markup.Quo(markup, big.NewRat(100, 1))
	markup.Add(markup, big.NewRat(1, 1))
	amount.Mul(amount, markup)
	return roundImportPriceMinor(amount, minorUnit)
}

// importFinalPriceMinor rounds an explicitly entered final major-unit price to
// minor units without applying markup or consulting an upstream price. It has
// the same decimal, precision and overflow rules as importMarkupPriceMinor.
func importFinalPriceMinor(amount string, minorUnit int) (int64, error) {
	parsed, err := parseImportPriceDecimal(amount)
	if err != nil {
		return 0, err
	}
	return roundImportPriceMinor(parsed, minorUnit)
}

// parseImportPriceDecimal reuses math/big's exact decimal parser after limiting
// its broader accepted syntax. Input-size limits belong to the import contract.
func parseImportPriceDecimal(raw string) (*big.Rat, error) {
	if !importPriceDecimalPattern.MatchString(raw) {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	return value, nil
}

// roundImportPriceMinor scales an exact nonnegative price, rounds a half unit
// upward and checks the rounded result before narrowing it to int64.
func roundImportPriceMinor(amount *big.Rat, minorUnit int) (int64, error) {
	if minorUnit < 0 {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	if amount.Sign() == 0 {
		return 0, nil
	}
	// A positive numerator is at least one. Beyond this denominator-dependent
	// precision even that minimum exceeds int64; reject before a huge exponent.
	if minorUnit > len(amount.Denom().String())+18 {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	var (
		scale     = new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(minorUnit)), nil)
		numerator = new(big.Int).Mul(amount.Num(), scale)
		quotient  = new(big.Int)
		remainder = new(big.Int)
	)
	quotient.QuoRem(numerator, amount.Denom(), remainder)
	if remainder.Lsh(remainder, 1).Cmp(amount.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	return quotient.Int64(), nil
}
