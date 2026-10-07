package billing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

func normalizeConfirmedRecharge(input ConfirmedRechargeInput) (ConfirmedRechargeInput, error) {
	if !exactlyTrimmed(input.CustomerNo, input.CurrencyCode, input.SourceReference, input.SourceSnapshotDigest) {
		return ConfirmedRechargeInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	if !validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.AmountMinor) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) {
		return ConfirmedRechargeInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeBalanceConsumption(input BalanceConsumptionInput) (BalanceConsumptionInput, error) {
	if !exactlyTrimmed(input.CustomerNo, input.CurrencyCode, input.SourceReference, input.SourceSnapshotDigest) {
		return BalanceConsumptionInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	if !validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.AmountMinor) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) {
		return BalanceConsumptionInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeReversal(input ReversalInput) (ReversalInput, error) {
	if !exactlyTrimmed(input.OriginalTransactionNo, input.SourceReference, input.SourceSnapshotDigest) {
		return ReversalInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.OriginalTransactionNo = strings.TrimSpace(input.OriginalTransactionNo)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	if !validBusinessReference(input.OriginalTransactionNo, 40) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) {
		return ReversalInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeHoldCreate(input HoldCreateInput) (HoldCreateInput, error) {
	if !exactlyTrimmed(
		input.CustomerNo, input.CurrencyCode, input.SourceType,
		input.SourceReference, input.SourceSnapshotDigest,
	) {
		return HoldCreateInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	input.ExpiresAt = input.ExpiresAt.UTC().Truncate(time.Microsecond)
	if !validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.AmountMinor) || !sourceTypePattern.MatchString(input.SourceType) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) || input.ExpiresAt.IsZero() {
		return HoldCreateInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeHoldConsume(input HoldConsumeInput) (HoldConsumeInput, error) {
	if !exactlyTrimmed(input.HoldNo, input.InvoiceNo, input.SourceReference, input.SourceSnapshotDigest) {
		return HoldConsumeInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.HoldNo = strings.TrimSpace(input.HoldNo)
	input.InvoiceNo = strings.TrimSpace(input.InvoiceNo)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.SourceSnapshotDigest = strings.TrimSpace(input.SourceSnapshotDigest)
	if !validBusinessReference(input.HoldNo, 40) || !validAmount(input.AmountMinor) ||
		(input.InvoiceNo != "" && !validBusinessReference(input.InvoiceNo, 40)) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) {
		return HoldConsumeInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func validAmount(value int64) bool {
	return value > 0 && value <= maximumAmountMinor
}

func exactlyTrimmed(values ...string) bool {
	for _, value := range values {
		if value != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}

func validBusinessReference(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func newBillingNumber(random io.Reader, prefix string) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate billing business number")
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

func contentDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode billing content digest")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func deterministicAccountNo(ownerType string, ownerReference string, accountType string, currency string) string {
	digest := sha256.Sum256([]byte(ownerType + "\x00" + ownerReference + "\x00" + accountType + "\x00" + currency))
	return "bac_" + hex.EncodeToString(digest[:16])
}
