package renewal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
)

func normalizeQuoteInput(input QuoteInput) (QuoteInput, error) {
	if input.EntitlementNo != strings.TrimSpace(input.EntitlementNo) ||
		input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) ||
		!validReference(input.EntitlementNo, 40) || input.RenewalCycles < 1 ||
		input.RenewalCycles > 120 || !validReference(input.IdempotencyKey, 128) ||
		len(input.AssetNumbers) > maximumQuoteAssets ||
		(input.FormalBillingRuleNo != "" && !validReference(input.FormalBillingRuleNo, 40)) {
		return QuoteInput{}, bizerr.NewCode(CodeInvalid)
	}
	assets := append([]string(nil), input.AssetNumbers...)
	sort.Strings(assets)
	for index, asset := range assets {
		if asset != strings.TrimSpace(asset) || !validReference(asset, 128) ||
			(index > 0 && assets[index-1] == asset) {
			return QuoteInput{}, bizerr.NewCode(CodeInvalid)
		}
	}
	input.AssetNumbers = assets
	return input, nil
}

func normalizeConfirmInput(input ConfirmInput) (ConfirmInput, error) {
	if input.QuoteNo != strings.TrimSpace(input.QuoteNo) ||
		input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) ||
		!validReference(input.QuoteNo, 40) || !validReference(input.IdempotencyKey, 128) {
		return ConfirmInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func validReference(value string, maximum int) bool {
	return len(value) >= 1 && len(value) <= maximum && businessReferencePattern.MatchString(value)
}

func contentDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func idempotencyDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func newNumber(random io.Reader, prefix string) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value[:]), nil
}

func projectedExpiry(
	original time.Time,
	now time.Time,
	periodUnit string,
	periodCount int,
	cycles int,
) (time.Time, error) {
	original = original.UTC().Truncate(time.Microsecond)
	now = now.UTC().Truncate(time.Microsecond)
	base := original
	if now.After(base) {
		base = now
	}
	if periodCount < 1 || cycles < 1 || periodCount > math.MaxInt/cycles {
		return time.Time{}, bizerr.NewCode(CodeInvalid)
	}
	count := periodCount * cycles
	var result time.Time
	switch periodUnit {
	case "hour":
		seconds := int64(count) * int64(time.Hour/time.Second)
		if seconds <= 0 || base.Unix() > math.MaxInt64-seconds {
			return time.Time{}, bizerr.NewCode(CodeInvalid)
		}
		result = time.Unix(base.Unix()+seconds, int64(base.Nanosecond())).UTC()
	case "day":
		result = base.AddDate(0, 0, count)
	case "month":
		result = addClampedMonths(base, count)
	case "year":
		if count > math.MaxInt/12 {
			return time.Time{}, bizerr.NewCode(CodeInvalid)
		}
		result = addClampedMonths(base, count*12)
	default:
		return time.Time{}, bizerr.NewCode(CodePriceUnavailable)
	}
	result = result.UTC().Truncate(time.Microsecond)
	if !result.After(original) || !result.After(base) {
		return time.Time{}, bizerr.NewCode(CodeInvalid)
	}
	return result, nil
}

func addClampedMonths(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	firstTarget := time.Date(year, month, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location()).
		AddDate(0, months, 0)
	lastDay := firstTarget.AddDate(0, 1, -1).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(
		firstTarget.Year(), firstTarget.Month(), day,
		value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location(),
	)
}

func earlierTime(left time.Time, right time.Time) time.Time {
	if right.Before(left) {
		return right
	}
	return left
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}
