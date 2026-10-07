// This file executes atomic same-customer, same-currency invoice merge and
// whole-item split. Source commercial evidence is never moved or rewritten.

package billing

import (
	"context"
	"sort"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type preparedInvoiceTransform struct {
	Command          invoiceTransformCommand
	TransformNo      string
	CustomerNo       string
	CurrencyCode     string
	Now              time.Time
	Sources          []*entity.BillingInvoice
	SourceItems      []*entity.BillingInvoiceItem
	Destinations     []do.BillingInvoice
	DestinationItems []do.BillingInvoiceItem
	SourceRows       []do.BillingInvoiceTransformSource
	DestinationRows  []do.BillingInvoiceTransformDestination
	DerivationRows   []do.BillingInvoiceItemDerivation
	ContentDigest    string
}

type destinationAmounts struct {
	SubtotalMinor      int64
	DiscountMinor      int64
	NetAmountMinor     int64
	TaxAmountMinor     int64
	TotalAmountMinor   int64
	RefundableCapMinor int64
}

func (s *serviceImpl) MergeInvoices(
	ctx context.Context,
	input InvoiceMergeInput,
) (*InvoiceTransform, error) {
	command, err := normalizeInvoiceMerge(input)
	if err != nil {
		return nil, err
	}
	return s.executeInvoiceTransform(ctx, command)
}

func (s *serviceImpl) SplitInvoice(
	ctx context.Context,
	input InvoiceSplitInput,
) (*InvoiceTransform, error) {
	command, err := normalizeInvoiceSplit(input)
	if err != nil {
		return nil, err
	}
	return s.executeInvoiceTransform(ctx, command)
}

func (s *serviceImpl) executeInvoiceTransform(
	ctx context.Context,
	command invoiceTransformCommand,
) (*InvoiceTransform, error) {
	var result *InvoiceTransform
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(
			ctx, "invoice_transform", command.IdempotencyDigest,
		); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadInvoiceTransformHeader(
			ctx, s.database, command.IdempotencyDigest,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.TransformTypeCode != string(command.Type) ||
				existing.RequestDigest != command.RequestDigest ||
				existing.RequestedBy != command.RequestedBy {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
			result, loadErr = loadInvoiceTransformProjection(ctx, s.database, existing, true)
			return loadErr
		}
		prepared, prepareErr := s.prepareInvoiceTransform(ctx, command)
		if prepareErr != nil {
			return prepareErr
		}
		if persistErr := persistInvoiceTransform(ctx, s.database, prepared); persistErr != nil {
			return persistErr
		}
		created := &entity.BillingInvoiceTransform{
			TransformNo: prepared.TransformNo, TransformTypeCode: string(command.Type),
			CustomerNo: prepared.CustomerNo, CurrencyCode: prepared.CurrencyCode,
			SourceCount: len(prepared.Sources), DestinationCount: len(prepared.Destinations),
			ItemCount: len(prepared.SourceItems), IdempotencyDigest: command.IdempotencyDigest,
			RequestDigest: command.RequestDigest, ContentDigest: prepared.ContentDigest,
			RequestedBy: command.RequestedBy, CompletedAt: &prepared.Now, CreatedAt: &prepared.Now,
		}
		result, loadErr = loadInvoiceTransformProjection(ctx, s.database, created, false)
		return loadErr
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) prepareInvoiceTransform(
	ctx context.Context,
	command invoiceTransformCommand,
) (*preparedInvoiceTransform, error) {
	sources, err := lockInvoiceTransformSources(ctx, s.database, command.SourceInvoiceNos)
	if err != nil {
		return nil, err
	}
	if err = validateInvoiceTransformSources(command, sources); err != nil {
		return nil, err
	}
	if err = rejectInvoiceTransformFinancialActivity(
		ctx, s.database, command.SourceInvoiceNos,
	); err != nil {
		return nil, err
	}
	items, err := loadInvoiceTransformItems(ctx, s.database, command.SourceInvoiceNos)
	if err != nil {
		return nil, err
	}
	if err = validateInvoiceTransformItems(sources, items); err != nil {
		return nil, err
	}
	groups, err := destinationGroupsForTransform(command, items)
	if err != nil {
		return nil, err
	}
	now, err := databaseTime(ctx, s.database)
	if err != nil {
		return nil, err
	}
	transformNo, err := newBillingNumber(s.random, "bit_")
	if err != nil {
		return nil, err
	}
	prepared := &preparedInvoiceTransform{
		Command: command, TransformNo: transformNo, CustomerNo: sources[0].CustomerNo,
		CurrencyCode: sources[0].CurrencyCode, Now: now, Sources: sources, SourceItems: items,
	}
	if err = s.prepareInvoiceTransformRows(prepared, groups); err != nil {
		return nil, err
	}
	return prepared, nil
}

func lockInvoiceTransformSources(
	ctx context.Context,
	database gdb.DB,
	invoiceNos []string,
) ([]*entity.BillingInvoice, error) {
	rows := make([]*entity.BillingInvoice, 0, len(invoiceNos))
	columns := dao.BillingInvoice.Columns()
	if err := database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
		Fields(
			columns.InvoiceNo, columns.CustomerNo, columns.OrderNo, columns.SourceCode,
			columns.CurrencyCode, columns.StatusCode, columns.ItemCount,
			columns.SubtotalMinor, columns.DiscountMinor, columns.NetAmountMinor,
			columns.TaxAmountMinor, columns.TotalAmountMinor, columns.PaidAmountMinor,
			columns.WaivedAmountMinor, columns.CreditAmountMinor,
			columns.RefundableAmountMinor, columns.RefundedAmountMinor,
			columns.SourceSnapshotDigest, columns.ContentDigest, columns.PaymentDueAt,
			columns.IssuedAt, columns.SettledAt, columns.CancelledAt, columns.Version,
			columns.CreatedAt, columns.UpdatedAt,
		).
		Where(columns.TenantId, 0).WhereIn(columns.InvoiceNo, invoiceNos).
		OrderAsc(columns.InvoiceNo).Limit(maximumInvoiceTransformInvoiceCount).
		LockUpdate().Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "lock invoice transform sources")
	}
	return rows, nil
}

func validateInvoiceTransformSources(
	command invoiceTransformCommand,
	sources []*entity.BillingInvoice,
) error {
	if len(sources) != len(command.SourceInvoiceNos) || len(sources) == 0 {
		return bizerr.NewCode(CodeInvoiceNotFound)
	}
	customerNo := sources[0].CustomerNo
	currency := sources[0].CurrencyCode
	totalItems := 0
	for index, invoice := range sources {
		if invoice == nil || invoice.InvoiceNo != command.SourceInvoiceNos[index] ||
			invoice.CustomerNo != customerNo || invoice.CurrencyCode != currency ||
			invoice.IssuedAt == nil || invoice.PaymentDueAt == nil ||
			invoice.SettledAt != nil || invoice.CancelledAt != nil || invoice.Version < 1 ||
			invoice.TotalAmountMinor <= 0 || invoice.PaidAmountMinor != 0 ||
			invoice.WaivedAmountMinor != 0 || invoice.CreditAmountMinor != 0 ||
			invoice.RefundableAmountMinor != 0 || invoice.RefundedAmountMinor != 0 ||
			!canonicalDigestPattern.MatchString(invoice.SourceSnapshotDigest) ||
			!canonicalDigestPattern.MatchString(invoice.ContentDigest) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		status := InvoiceStatus(invoice.StatusCode)
		if status != InvoiceStatusPendingPayment && status != InvoiceStatusOverdue {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if invoice.ItemCount < 1 ||
			totalItems > maximumInvoiceTransformItemCount-invoice.ItemCount {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		totalItems += invoice.ItemCount
	}
	if totalItems < 2 || totalItems > maximumInvoiceTransformItemCount {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func rejectInvoiceTransformFinancialActivity(
	ctx context.Context,
	database gdb.DB,
	invoiceNos []string,
) error {
	paymentCount, err := database.Model(dao.BillingPaymentAllocation.Table()).Ctx(ctx).
		Where(dao.BillingPaymentAllocation.Columns().TenantId, 0).
		WhereIn(dao.BillingPaymentAllocation.Columns().InvoiceNo, invoiceNos).Count()
	if err != nil {
		return gerror.Wrap(err, "count invoice transform payment allocations")
	}
	refundCount, err := database.Model(dao.BillingRefundAllocation.Table()).Ctx(ctx).
		Where(dao.BillingRefundAllocation.Columns().TenantId, 0).
		WhereIn(dao.BillingRefundAllocation.Columns().InvoiceNo, invoiceNos).Count()
	if err != nil {
		return gerror.Wrap(err, "count invoice transform refund allocations")
	}
	if paymentCount != 0 || refundCount != 0 {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func loadInvoiceTransformItems(
	ctx context.Context,
	database gdb.DB,
	invoiceNos []string,
) ([]*entity.BillingInvoiceItem, error) {
	items := make([]*entity.BillingInvoiceItem, 0)
	columns := dao.BillingInvoiceItem.Columns()
	if err := database.Model(dao.BillingInvoiceItem.Table()).Ctx(ctx).
		Fields(
			columns.InvoiceItemNo, columns.InvoiceNo, columns.CustomerNo,
			columns.CurrencyCode, columns.LineNumber, columns.ItemTypeCode,
			columns.OrderNo, columns.OrderItemNo, columns.SourceReference,
			columns.DescriptionZhCn, columns.DescriptionEnUs, columns.SubtotalMinor,
			columns.DiscountMinor, columns.NetAmountMinor, columns.TaxAmountMinor,
			columns.TotalAmountMinor, columns.RefundableCapMinor, columns.ContentDigest,
			columns.CreatedAt,
		).
		Where(columns.TenantId, 0).WhereIn(columns.InvoiceNo, invoiceNos).
		OrderAsc(columns.InvoiceNo).OrderAsc(columns.LineNumber).
		Limit(maximumInvoiceTransformItemCount).Scan(&items); err != nil {
		return nil, gerror.Wrap(err, "load invoice transform source items")
	}
	return items, nil
}

func validateInvoiceTransformItems(
	sources []*entity.BillingInvoice,
	items []*entity.BillingInvoiceItem,
) error {
	expected := make(map[string]*entity.BillingInvoice, len(sources))
	expectedCount := 0
	for _, invoice := range sources {
		expected[invoice.InvoiceNo] = invoice
		expectedCount += invoice.ItemCount
	}
	if len(items) != expectedCount || len(items) < 2 ||
		len(items) > maximumInvoiceTransformItemCount {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	seen := make(map[string]struct{}, len(items))
	amountsByInvoice := make(map[string]*destinationAmounts, len(sources))
	for _, invoice := range sources {
		amountsByInvoice[invoice.InvoiceNo] = &destinationAmounts{}
	}
	for _, item := range items {
		invoice := expected[item.InvoiceNo]
		if item == nil || invoice == nil || item.CustomerNo != invoice.CustomerNo ||
			item.CurrencyCode != invoice.CurrencyCode || item.LineNumber < 1 ||
			item.TotalAmountMinor < 0 || item.RefundableCapMinor < 0 ||
			item.RefundableCapMinor > item.TotalAmountMinor ||
			!canonicalDigestPattern.MatchString(item.ContentDigest) {
			return bizerr.NewCode(CodeInvoiceIntegrity)
		}
		if _, duplicate := seen[item.InvoiceItemNo]; duplicate {
			return bizerr.NewCode(CodeInvoiceIntegrity)
		}
		if err := addDestinationItemAmounts(amountsByInvoice[item.InvoiceNo], item); err != nil {
			return err
		}
		seen[item.InvoiceItemNo] = struct{}{}
	}
	for _, invoice := range sources {
		amounts := amountsByInvoice[invoice.InvoiceNo]
		if amounts == nil || amounts.SubtotalMinor != invoice.SubtotalMinor ||
			amounts.DiscountMinor != invoice.DiscountMinor ||
			amounts.NetAmountMinor != invoice.NetAmountMinor ||
			amounts.TaxAmountMinor != invoice.TaxAmountMinor ||
			amounts.TotalAmountMinor != invoice.TotalAmountMinor {
			return bizerr.NewCode(CodeInvoiceIntegrity)
		}
	}
	return nil
}

func destinationGroupsForTransform(
	command invoiceTransformCommand,
	items []*entity.BillingInvoiceItem,
) ([][]string, error) {
	available := make(map[string]struct{}, len(items))
	all := make([]string, 0, len(items))
	for _, item := range items {
		available[item.InvoiceItemNo] = struct{}{}
		all = append(all, item.InvoiceItemNo)
	}
	sort.Strings(all)
	if command.Type == InvoiceTransformMerge {
		return [][]string{all}, nil
	}
	seen := make(map[string]struct{}, len(items))
	for _, group := range command.DestinationGroups {
		for _, itemNo := range group {
			if _, ok := available[itemNo]; !ok {
				return nil, bizerr.NewCode(CodeInvoiceConflict)
			}
			seen[itemNo] = struct{}{}
		}
	}
	if len(seen) != len(items) {
		return nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	return command.DestinationGroups, nil
}
