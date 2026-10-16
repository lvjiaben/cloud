// This file computes the agreed remaining-period difference with exact integer rounding.

package entitlementupgrade

import (
	"lina-core/pkg/bizerr"
	"math/big"
)

// ProrateDifference charges only the remaining fraction of a nonnegative
// current-period price difference. It rounds half up to the minor currency
// unit with integer arithmetic, avoiding overflow and floating-point drift.
// Tax is deliberately applied afterwards by the shared commerce tax owner.
func ProrateDifference(input PeriodPrice) (int64, error) {
	if input.ActualMinor < 0 || input.TargetMinor < input.ActualMinor ||
		input.StartedAt.IsZero() || input.ExpiresAt.IsZero() || input.Now.IsZero() ||
		!input.ExpiresAt.After(input.StartedAt) || input.Now.Before(input.StartedAt) ||
		!input.Now.Before(input.ExpiresAt) {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	var (
		whole       = input.ExpiresAt.Sub(input.StartedAt)
		remaining   = input.ExpiresAt.Sub(input.Now)
		numerator   = new(big.Int).Mul(big.NewInt(input.TargetMinor-input.ActualMinor), big.NewInt(int64(remaining)))
		denominator = big.NewInt(int64(whole))
		quotient    = new(big.Int)
		remainder   = new(big.Int)
	)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	return quotient.Int64(), nil
}
