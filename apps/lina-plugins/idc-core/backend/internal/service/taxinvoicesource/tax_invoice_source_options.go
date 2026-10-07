// This file builds the bounded current-customer request-form projection.
package taxinvoicesource

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// ListCurrentCustomerOptions returns the customer's active title profiles and
// paid billing invoices with their current remaining invoiceable amounts.
func (s *serviceImpl) ListCurrentCustomerOptions(
	ctx context.Context,
	usageReader taxinvoicecap.AllocationUsageReader,
) (*taxinvoicecap.RequestOptions, error) {
	subject, err := currentCustomer(ctx, s.customers)
	if err != nil || nilDependency(usageReader) {
		return nil, gerror.New("tax invoice request options are not accessible")
	}
	var result *taxinvoicecap.RequestOptions
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, txErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); txErr != nil {
			return txErr
		}
		observedAt, txErr := databaseNow(ctx, tx)
		if txErr != nil {
			return txErr
		}
		profiles, txErr := loadProfiles(ctx, tx, subject.CustomerNo)
		if txErr != nil {
			return txErr
		}
		invoices, txErr := loadInvoiceOptions(ctx, tx, subject.CustomerNo)
		if txErr != nil {
			return txErr
		}
		invoiceNos := make([]string, len(invoices))
		for index := range invoices {
			invoiceNos[index] = invoices[index].InvoiceNo
		}
		usage, txErr := readUsage(ctx, usageReader, subject.CustomerNo, invoiceNos, false)
		if txErr != nil {
			return txErr
		}
		options, txErr := buildInvoiceOptions(invoices, usage)
		if txErr != nil {
			return txErr
		}
		result = &taxinvoicecap.RequestOptions{
			CustomerNo: subject.CustomerNo,
			Profiles:   projectProfileSummaries(profiles),
			Invoices:   options,
			ObservedAt: observedAt,
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "load current customer tax invoice request options")
	}
	return result, nil
}

// buildInvoiceOptions combines core payment/refund truth with compliance-owned
// prior allocations without exposing either owner's internal data model.
func buildInvoiceOptions(
	rows []invoiceRow,
	usage map[string]taxinvoicecap.InvoiceUsage,
) ([]taxinvoicecap.InvoiceOption, error) {
	result := make([]taxinvoicecap.InvoiceOption, 0, len(rows))
	for _, row := range rows {
		if err := validateInvoiceRow(&row); err != nil {
			return nil, err
		}
		source := row.PaidAmountMinor - row.RefundedAmountMinor
		allocated := usage[row.InvoiceNo].AllocatedGrossMinor
		if allocated < 0 || allocated > source {
			return nil, gerror.New("paid billing invoice allocation usage exceeds source amount")
		}
		result = append(result, taxinvoicecap.InvoiceOption{
			BillingInvoiceNo: row.InvoiceNo, OrderNo: row.OrderNo,
			CurrencyCode: row.CurrencyCode, SourceGrossMinor: source,
			AlreadyAllocatedMinor: allocated, RemainingGrossMinor: source - allocated,
			BillingInvoiceVersion: row.Version, BillingContentDigest: row.ContentDigest,
			SettledAt: row.SettledAt.UTC(),
		})
	}
	return result, nil
}

// projectProfileSummaries removes secret references from picker results.
func projectProfileSummaries(rows []profileRow) []taxinvoicecap.ProfileSummary {
	result := make([]taxinvoicecap.ProfileSummary, 0, len(rows))
	for index := range rows {
		result = append(result, projectProfileSummary(&rows[index]))
	}
	return result
}

// projectProfileSummary maps one profile to its customer-safe projection.
func projectProfileSummary(row *profileRow) taxinvoicecap.ProfileSummary {
	return taxinvoicecap.ProfileSummary{
		ProfileNo: row.ProfileNo, TitleTypeCode: row.TitleTypeCode,
		InvoiceTitle: row.InvoiceTitle, TaxRegistrationNo: row.TaxRegistrationNo,
		RegisteredAddress: row.RegisteredAddress, RegisteredPhone: row.RegisteredPhone,
		BankName: row.BankName, BankAccountMasked: row.BankAccountMasked,
		IsDefault: row.IsDefault, Version: row.Version,
	}
}

// projectProfileSnapshot adds only the opaque bank-secret reference needed by
// the compliance owner when it seals the immutable request.
func projectProfileSnapshot(row *profileRow) taxinvoicecap.ProfileSnapshot {
	return taxinvoicecap.ProfileSnapshot{
		ProfileSummary:       projectProfileSummary(row),
		BankAccountSecretRef: row.BankAccountSecretRef,
	}
}
