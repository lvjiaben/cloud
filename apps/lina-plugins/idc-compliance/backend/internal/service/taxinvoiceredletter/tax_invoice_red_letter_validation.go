// This file validates red-letter identifiers, evidence, amounts, and replay fingerprints.

package taxinvoiceredletter

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
)

var (
	businessNoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,191}$`)
	currencyPattern   = regexp.MustCompile(`^[A-Z]{3}$`)
	digestPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	mediaTypePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]{0,63}/[a-z0-9][a-z0-9.+-]{0,63}$`)
)

type identity struct {
	digest      string
	fingerprint string
	actionNo    string
}

func actionIdentity(scope, key, fingerprint string) (identity, error) {
	key = strings.TrimSpace(key)
	if len(key) < 16 || len(key) > 128 || strings.TrimSpace(fingerprint) == "" {
		return identity{}, bizerr.NewCode(CodeInvalid)
	}
	digest := sha256.Sum256([]byte(scope + "\x00" + key))
	fp := sha256.Sum256([]byte(scope + "\x00" + fingerprint))
	return identity{
		digest: hex.EncodeToString(digest[:]), fingerprint: hex.EncodeToString(fp[:]),
		actionNo: "rla_" + hex.EncodeToString(digest[:20]),
	}, nil
}

func taskIdentity(operation string, allocations []string) (string, string) {
	sort.Strings(allocations)
	canonical := operation + "\x00" + strings.Join(allocations, "\x00")
	digest := sha256.Sum256([]byte(canonical))
	encoded := hex.EncodeToString(digest[:])
	return "rlt_" + encoded[:40], encoded
}

func candidateNo(taskNo, refundNo, invoiceAllocationNo string) string {
	digest := sha256.Sum256([]byte(taskNo + "\x00" + refundNo + "\x00" + invoiceAllocationNo))
	return "rlc_" + hex.EncodeToString(digest[:20])
}

func allocationNo(taskNo, refundNo, invoiceAllocationNo string) string {
	digest := sha256.Sum256([]byte("allocation\x00" + taskNo + "\x00" + refundNo + "\x00" + invoiceAllocationNo))
	return "rla_" + hex.EncodeToString(digest[:20])
}

func validateTaskInput(taskNo string, version int64) error {
	if !businessNoPattern.MatchString(strings.TrimSpace(taskNo)) || version <= 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func normalizeSelections(input ResolveInput) (ResolveInput, identity, error) {
	input.TaskNo = strings.TrimSpace(input.TaskNo)
	if err := validateTaskInput(input.TaskNo, input.ExpectedVersion); err != nil ||
		len(input.Selections) == 0 || len(input.Selections) > 200 {
		return ResolveInput{}, identity{}, bizerr.NewCode(CodeInvalid)
	}
	seen := make(map[string]struct{}, len(input.Selections))
	parts := make([]string, 0, len(input.Selections))
	for index := range input.Selections {
		selection := &input.Selections[index]
		selection.RefundAllocationNo = strings.TrimSpace(selection.RefundAllocationNo)
		selection.TaxInvoiceAllocationNo = strings.TrimSpace(selection.TaxInvoiceAllocationNo)
		key := selection.RefundAllocationNo + "\x00" + selection.TaxInvoiceAllocationNo
		if !businessNoPattern.MatchString(selection.RefundAllocationNo) ||
			!businessNoPattern.MatchString(selection.TaxInvoiceAllocationNo) ||
			selection.GrossAmountMinor <= 0 {
			return ResolveInput{}, identity{}, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := seen[key]; duplicate {
			return ResolveInput{}, identity{}, bizerr.NewCode(CodeInvalid)
		}
		seen[key] = struct{}{}
		parts = append(parts, key+"\x00"+strconv.FormatInt(selection.GrossAmountMinor, 10))
	}
	sort.Strings(parts)
	ident, err := actionIdentity("resolve_allocation", input.IdempotencyKey, input.TaskNo+"\x00"+strings.Join(parts, "\x01"))
	return input, ident, err
}

func normalizeCompletion(input ManualCompletionInput) (ManualCompletionInput, identity, error) {
	input.TaskNo = strings.TrimSpace(input.TaskNo)
	input.EvidenceReference = strings.TrimSpace(input.EvidenceReference)
	input.EvidenceSHA256 = strings.TrimSpace(input.EvidenceSHA256)
	input.UploadedObjectRef = strings.TrimSpace(input.UploadedObjectRef)
	input.MediaType = strings.ToLower(strings.TrimSpace(input.MediaType))
	if err := validateTaskInput(input.TaskNo, input.ExpectedVersion); err != nil ||
		!businessNoPattern.MatchString(input.EvidenceReference) || !digestPattern.MatchString(input.EvidenceSHA256) ||
		input.OccurredAt.IsZero() || input.OccurredAt.After(timeNowUTC().Add(5*time.Minute)) {
		return ManualCompletionInput{}, identity{}, bizerr.NewCode(CodeInvalid)
	}
	hasObject := input.UploadedObjectRef != ""
	if (!hasObject && (input.MediaType != "" || input.ByteSize != 0)) ||
		(hasObject && (input.MediaType == "" || input.ByteSize <= 0)) {
		return ManualCompletionInput{}, identity{}, bizerr.NewCode(CodeInvalid)
	}
	if hasObject && (!validOpaqueReference(input.UploadedObjectRef, 512) ||
		!mediaTypePattern.MatchString(input.MediaType) || input.ByteSize > 20*1024*1024) {
		return ManualCompletionInput{}, identity{}, bizerr.NewCode(CodeInvalid)
	}
	fingerprint := strings.Join([]string{input.TaskNo, input.EvidenceReference, input.EvidenceSHA256,
		input.UploadedObjectRef, input.MediaType, strconv.FormatInt(input.ByteSize, 10),
		input.OccurredAt.UTC().Format(time.RFC3339Nano)}, "\x00")
	ident, err := actionIdentity("manual_complete", input.IdempotencyKey, fingerprint)
	return input, ident, err
}

func validOpaqueReference(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

var timeNowUTC = func() time.Time { return time.Now().UTC() }
