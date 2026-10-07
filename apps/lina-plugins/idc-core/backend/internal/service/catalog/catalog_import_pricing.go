// This file calculates imported draft prices from exact decimal text. Currency
// precision must come from the local currency authority; rounding occurs only
// after markup, and neither floating point nor a currency fallback is used.

package catalog

import (
	"math/big"
	"regexp"

	"lina-core/pkg/bizerr"
)

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
