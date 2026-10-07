// This file validates current-customer identity, source rows, and selections.
package taxinvoicesource

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

var (
	businessNoPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	currencyPattern        = regexp.MustCompile(`^[A-Z]{3}$`)
	digestPattern          = regexp.MustCompile(`^[0-9a-f]{64}$`)
	refundOperationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	taxCategoryPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

// currentCustomer resolves and validates the authenticated customer subject.
func currentCustomer(
	ctx context.Context,
	reader identitycap.CurrentCustomerReader,
) (*identitycap.CustomerSubject, error) {
	if ctx == nil || ctx.Err() != nil || nilDependency(reader) {
		return nil, gerror.New("current customer is not accessible")
	}
	subject, err := reader.CurrentCustomer(ctx)
	if err != nil || subject == nil || !businessNoPattern.MatchString(subject.CustomerNo) ||
		subject.ActorUserID <= 0 || strings.TrimSpace(subject.PrincipalReference) == "" {
		return nil, gerror.New("current customer is not accessible")
	}
	return subject, nil
}

// normalizeSelections validates customer-controlled amounts and returns a
// stable invoice lock order.
func normalizeSelections(
	selections []taxinvoicecap.Selection,
) (map[string]int64, []string, error) {
	requested := make(map[string]int64, len(selections))
	invoiceNos := make([]string, 0, len(selections))
	for _, selection := range selections {
		if !businessNoPattern.MatchString(selection.BillingInvoiceNo) ||
			selection.RequestedGrossMinor <= 0 {
			return nil, nil, gerror.New("tax invoice request selection is invalid")
		}
		if _, duplicate := requested[selection.BillingInvoiceNo]; duplicate {
			return nil, nil, gerror.New("tax invoice request selection is duplicated")
		}
		requested[selection.BillingInvoiceNo] = selection.RequestedGrossMinor
		invoiceNos = append(invoiceNos, selection.BillingInvoiceNo)
	}
	sort.Strings(invoiceNos)
	return requested, invoiceNos, nil
}

// validateProfileSummaryRow rejects incomplete customer-safe profile projections.
func validateProfileSummaryRow(row *profileRow) error {
	if row == nil || !businessNoPattern.MatchString(row.ProfileNo) || row.Version <= 0 ||
		(row.TitleTypeCode != "individual" && row.TitleTypeCode != "enterprise") ||
		strings.TrimSpace(row.InvoiceTitle) == "" {
		return gerror.New("customer invoice profile source is inconsistent")
	}
	return nil
}

// validateProfile additionally rejects mismatched selected-profile secret data.
func validateProfile(row *profileRow) error {
	if err := validateProfileSummaryRow(row); err != nil ||
		((row.BankAccountMasked == "") != (row.BankAccountSecretRef == "")) {
		return gerror.New("customer invoice profile source is inconsistent")
	}
	return nil
}

// validateInvoiceRow checks immutable paid-invoice source invariants.
func validateInvoiceRow(row *invoiceRow) error {
	if row == nil || !businessNoPattern.MatchString(row.InvoiceNo) ||
		!currencyPattern.MatchString(row.CurrencyCode) || row.SettledAt == nil ||
		row.Version <= 0 || row.PaidAmountMinor <= row.RefundedAmountMinor ||
		row.RefundedAmountMinor < 0 || strings.TrimSpace(row.ContentDigest) == "" {
		return gerror.New("paid billing invoice source is inconsistent")
	}
	return nil
}

// validateItemRow checks amount, description, lineage, and tax-fact invariants.
func validateItemRow(row *itemRow) error {
	if row == nil || !businessNoPattern.MatchString(row.InvoiceNo) ||
		!businessNoPattern.MatchString(row.InvoiceItemNo) ||
		strings.TrimSpace(row.SourceReference) == "" ||
		(strings.TrimSpace(row.DescriptionZhCN) == "" && strings.TrimSpace(row.DescriptionEnUS) == "") ||
		row.TotalAmountMinor <= 0 || row.NetAmountMinor < 0 || row.TaxAmountMinor < 0 ||
		row.RefundedAmountMinor < 0 ||
		row.TotalAmountMinor != row.NetAmountMinor+row.TaxAmountMinor ||
		row.RefundedAmountMinor > row.TotalAmountMinor ||
		!taxCategoryPattern.MatchString(row.TaxCategoryCode) ||
		strings.TrimSpace(row.SourceContentDigest) == "" {
		return gerror.New("billing invoice item tax source is inconsistent")
	}
	switch row.TaxTreatmentCode {
	case "taxable":
		if row.TaxRateBasisPoints == nil || *row.TaxRateBasisPoints < 0 || *row.TaxRateBasisPoints > 10000 {
			return gerror.New("billing invoice item taxable rate is inconsistent")
		}
	case "zero_rated":
		if row.TaxRateBasisPoints == nil || *row.TaxRateBasisPoints != 0 {
			return gerror.New("billing invoice item zero rate is inconsistent")
		}
	case "exempt":
		if row.TaxRateBasisPoints != nil {
			return gerror.New("billing invoice item exempt rate is inconsistent")
		}
	default:
		return gerror.New("billing invoice item tax treatment is inconsistent")
	}
	return nil
}
