// This file prepares one immutable, server-calculated tax-invoice source snapshot.
package taxinvoicesource

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// PrepareCurrentCustomerRequest validates the customer selection, locks the
// selected paid invoices, and calculates all item and tax amounts server-side.
func (s *serviceImpl) PrepareCurrentCustomerRequest(
	ctx context.Context,
	usageReader taxinvoicecap.AllocationUsageReader,
	profileNo string,
	selections []taxinvoicecap.Selection,
) (*taxinvoicecap.RequestSnapshot, error) {
	if nilDependency(usageReader) || !businessNoPattern.MatchString(profileNo) ||
		len(selections) == 0 || len(selections) > taxinvoicecap.MaximumSelections {
		return nil, gerror.New("tax invoice request source is not accessible")
	}
	requested, invoiceNos, err := normalizeSelections(selections)
	if err != nil {
		return nil, err
	}
	subject, err := currentCustomer(ctx, s.customers)
	if err != nil {
		return nil, gerror.New("tax invoice request source is not accessible")
	}
	var snapshot *taxinvoicecap.RequestSnapshot
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		observedAt, txErr := databaseNow(ctx, tx)
		if txErr != nil {
			return txErr
		}
		profile, txErr := loadSelectedProfile(ctx, tx, subject.CustomerNo, profileNo)
		if txErr != nil {
			return txErr
		}
		merchant, txErr := loadMerchant(ctx, tx)
		if txErr != nil {
			return txErr
		}
		invoices, txErr := lockSelectedInvoices(ctx, tx, subject.CustomerNo, invoiceNos)
		if txErr != nil {
			return txErr
		}
		items, txErr := loadSelectedItems(ctx, tx, invoiceNos)
		if txErr != nil {
			return txErr
		}
		usage, txErr := readUsage(ctx, usageReader, subject.CustomerNo, invoiceNos, true)
		if txErr != nil {
			return txErr
		}
		snapshot, txErr = buildRequestSnapshot(
			subject, profile, merchant, invoices, items, usage, requested, observedAt,
		)
		return txErr
	})
	if err != nil {
		if snapshot != nil {
			snapshot.Clear()
		}
		return nil, gerror.Wrap(err, "prepare current customer tax invoice request source")
	}
	return snapshot, nil
}

// buildRequestSnapshot composes bounded invoice allocations into one immutable
// request snapshot and rejects mixed currencies or mismatched totals.
func buildRequestSnapshot(
	subject *identitycap.CustomerSubject,
	profile *profileRow,
	merchant *merchantRow,
	invoices []invoiceRow,
	items []itemRow,
	usage map[string]taxinvoicecap.InvoiceUsage,
	requested map[string]int64,
	observedAt time.Time,
) (*taxinvoicecap.RequestSnapshot, error) {
	options, err := buildInvoiceOptions(invoices, usage)
	if err != nil {
		return nil, err
	}
	itemsByInvoice := make(map[string][]itemRow, len(invoices))
	for _, item := range items {
		itemsByInvoice[item.InvoiceNo] = append(itemsByInvoice[item.InvoiceNo], item)
	}
	result := &taxinvoicecap.RequestSnapshot{
		CustomerNo: subject.CustomerNo, PrincipalReference: subject.PrincipalReference,
		ActorUserID: subject.ActorUserID, Buyer: projectProfileSnapshot(profile),
		Seller: taxinvoicecap.MerchantSnapshot{
			MerchantNo: merchant.MerchantNo, LegalName: merchant.LegalName,
			CountryCode: merchant.CountryCode, Version: merchant.Version,
		},
		ObservedAt: observedAt,
	}
	for _, option := range options {
		amount := requested[option.BillingInvoiceNo]
		if amount <= 0 || amount > option.RemainingGrossMinor {
			result.Clear()
			return nil, gerror.New("requested tax invoice amount exceeds the remaining source amount")
		}
		if result.CurrencyCode == "" {
			result.CurrencyCode = option.CurrencyCode
		} else if result.CurrencyCode != option.CurrencyCode {
			result.Clear()
			return nil, gerror.New("selected billing invoices use different currencies")
		}
		allocatedItems, net, tax, allocationErr := allocateInvoiceItems(
			itemsByInvoice[option.BillingInvoiceNo], usage[option.BillingInvoiceNo],
			option.RemainingGrossMinor, amount,
		)
		if allocationErr != nil {
			result.Clear()
			return nil, allocationErr
		}
		if addOverflow(result.GrossAmountMinor, amount) || addOverflow(result.NetAmountMinor, net) ||
			addOverflow(result.TaxAmountMinor, tax) {
			result.Clear()
			return nil, gerror.New("tax invoice request amount overflows")
		}
		result.GrossAmountMinor += amount
		result.NetAmountMinor += net
		result.TaxAmountMinor += tax
		result.Invoices = append(result.Invoices, taxinvoicecap.InvoiceSnapshot{
			InvoiceOption: option, RequestedGrossMinor: amount,
			NetAmountMinor: net, TaxAmountMinor: tax, Items: allocatedItems,
		})
	}
	if len(result.Invoices) != len(requested) || result.GrossAmountMinor <= 0 ||
		result.GrossAmountMinor != result.NetAmountMinor+result.TaxAmountMinor {
		result.Clear()
		return nil, gerror.New("tax invoice request source is internally inconsistent")
	}
	return result, nil
}
