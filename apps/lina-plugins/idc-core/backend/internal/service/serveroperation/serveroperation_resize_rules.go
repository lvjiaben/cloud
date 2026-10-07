package serveroperation

import (
	"math/big"
	"time"
)

type resizeDirection string

const (
	resizeDirectionUpgrade   resizeDirection = "upgrade"
	resizeDirectionDowngrade resizeDirection = "downgrade"
)

// classifyResize compares recurring totals only. Equal-price changes are
// treated as upgrades because they take effect immediately and still require
// the Provider task to succeed before the service snapshot changes.
func classifyResize(currentRecurringMinor, targetRecurringMinor int64) (resizeDirection, error) {
	if currentRecurringMinor < 0 || targetRecurringMinor < 0 {
		return "", ErrInvalidInput
	}
	if targetRecurringMinor < currentRecurringMinor {
		return resizeDirectionDowngrade, nil
	}
	return resizeDirectionUpgrade, nil
}

// proratedUpgradeAmount uses exact integer arithmetic and rounds any fraction
// up to the next minor currency unit. This prevents an upgrade from being
// undercharged while keeping the result stable across architectures.
func proratedUpgradeAmount(deltaMinor int64, periodStart, periodEnd, now time.Time) (int64, error) {
	periodStart, periodEnd, now = databaseTime(periodStart), databaseTime(periodEnd), databaseTime(now)
	if deltaMinor < 0 || !periodEnd.After(periodStart) || now.Before(periodStart) || !periodEnd.After(now) {
		return 0, ErrInvalidInput
	}
	if deltaMinor == 0 {
		return 0, nil
	}
	total := periodEnd.Sub(periodStart).Microseconds()
	remaining := periodEnd.Sub(now).Microseconds()
	if total <= 0 || remaining <= 0 {
		return 0, ErrInvalidInput
	}
	numerator := new(big.Int).Mul(big.NewInt(deltaMinor), big.NewInt(remaining))
	denominator := big.NewInt(total)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() || quotient.Sign() < 0 {
		return 0, ErrInvalidInput
	}
	return quotient.Int64(), nil
}

// diskShrinkRequested compares disk groups by their stable catalog group code.
// Removing a disk, reducing its selected quantity, or reducing its normalized
// size is a shrink and must be routed to manual migration instead of Provider
// resize automation.
func diskShrinkRequested(current, target map[string]diskSelection) bool {
	for groupCode, before := range current {
		after, exists := target[groupCode]
		if !exists || after.Quantity < before.Quantity || after.SizeGiB < before.SizeGiB {
			return true
		}
	}
	return false
}

type diskSelection struct {
	Quantity int
	SizeGiB  int64
}
