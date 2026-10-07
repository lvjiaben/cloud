// This file validates the closed management command vocabulary.
package taxinvoicemanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

var (
	businessNoPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
)

func (s *serviceImpl) authorize(ctx context.Context, permission, target string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	request := managementcap.AuthorizationRequest{Permission: permission, TargetNo: target}
	if managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, request)
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func normalizeReview(input ReviewInput) (ReviewInput, actionIdentity, error) {
	input.TaxInvoiceNo = strings.TrimSpace(input.TaxInvoiceNo)
	input.InvoiceTypeCode = strings.TrimSpace(input.InvoiceTypeCode)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !businessNoPattern.MatchString(input.TaxInvoiceNo) ||
		!validInvoiceType(input.InvoiceTypeCode) || input.ExpectedVersion <= 0 ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return ReviewInput{}, actionIdentity{}, bizerr.NewCode(CodeInvalid)
	}
	return input, deriveIdentity("approve", input.TaxInvoiceNo, input.IdempotencyKey,
		[]string{input.InvoiceTypeCode, strconv.FormatInt(input.ExpectedVersion, 10)}), nil
}

func normalizeSubmit(input SubmitInput) (SubmitInput, actionIdentity, error) {
	input.TaxInvoiceNo = strings.TrimSpace(input.TaxInvoiceNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !businessNoPattern.MatchString(input.TaxInvoiceNo) || input.ExpectedVersion <= 0 ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return SubmitInput{}, actionIdentity{}, bizerr.NewCode(CodeInvalid)
	}
	return input, deriveIdentity("submit", input.TaxInvoiceNo, input.IdempotencyKey,
		[]string{strconv.FormatInt(input.ExpectedVersion, 10)}), nil
}

func normalizeClose(action string, input CloseInput) (CloseInput, actionIdentity, error) {
	input.TaxInvoiceNo = strings.TrimSpace(input.TaxInvoiceNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.InternalNote = strings.TrimSpace(input.InternalNote)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if (action != "reject" && action != "cancel") ||
		!businessNoPattern.MatchString(input.TaxInvoiceNo) || !validReason(input.ReasonCode) ||
		!validInternalNote(input.InternalNote) || input.ExpectedVersion <= 0 ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return CloseInput{}, actionIdentity{}, bizerr.NewCode(CodeInvalid)
	}
	return input, deriveIdentity(action, input.TaxInvoiceNo, input.IdempotencyKey,
		[]string{input.ReasonCode, input.InternalNote, strconv.FormatInt(input.ExpectedVersion, 10)}), nil
}

func normalizeCapability(capability Capability) (Capability, bool) {
	capability.ProviderCode = strings.TrimSpace(capability.ProviderCode)
	capability.ReasonCode = strings.TrimSpace(capability.ReasonCode)
	if !businessNoPattern.MatchString(capability.ProviderCode) {
		return Capability{}, false
	}
	types := append([]string(nil), capability.SupportedInvoiceTypes...)
	sort.Strings(types)
	for index, value := range types {
		if !validInvoiceType(value) || index > 0 && value == types[index-1] {
			return Capability{}, false
		}
	}
	capability.SupportedInvoiceTypes = types
	if capability.Ready {
		if capability.ReasonCode != "" || len(types) == 0 {
			return Capability{}, false
		}
		return capability, true
	}
	if !businessNoPattern.MatchString(capability.ReasonCode) {
		return Capability{}, false
	}
	return capability, true
}

func capabilitySupports(capability Capability, invoiceType string) bool {
	index := sort.SearchStrings(capability.SupportedInvoiceTypes, invoiceType)
	return capability.Ready && index < len(capability.SupportedInvoiceTypes) &&
		capability.SupportedInvoiceTypes[index] == invoiceType
}

func validInvoiceType(value string) bool {
	switch value {
	case InvoiceTypeVATElectronicNormal, InvoiceTypeVATElectronicSpecial,
		InvoiceTypeDigitalNormal, InvoiceTypeDigitalSpecial:
		return true
	default:
		return false
	}
}

func validReason(value string) bool {
	switch value {
	case ReasonBuyerOrTaxInformationIncorrect, ReasonAmountOrBillingAdjustmentRequired,
		ReasonDuplicateRequest, ReasonBusinessNotInvoiceable,
		ReasonCustomerRequestedCancellation, ReasonOrderOrRefundChanged, ReasonOther:
		return true
	default:
		return false
	}
}

func validInternalNote(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 2000 {
		return false
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return false
		}
	}
	return true
}

type actionIdentity struct {
	ActionNo          string
	IdempotencyDigest string
	Fingerprint       string
}

func deriveIdentity(action, taxInvoiceNo, key string, fields []string) actionIdentity {
	digest := sha256Hex([]byte("tax-invoice-management-key\x00" + key))
	parts := append([]string{"tax-invoice-management-v1", action, taxInvoiceNo}, fields...)
	fingerprint := sha256Hex([]byte(strings.Join(parts, "\x00")))
	actionDigest := sha256Hex([]byte(strings.Join([]string{action, taxInvoiceNo, digest}, "\x00")))
	return actionIdentity{
		ActionNo: "tia_" + actionDigest[:32], IdempotencyDigest: digest, Fingerprint: fingerprint,
	}
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
