// This file prepares and persists immutable invoice transform evidence, then
// projects bounded source and destination summaries for trusted internal owners.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type transformItemDigestInput struct {
	SourceInvoiceNo     string `json:"sourceInvoiceNo"`
	SourceInvoiceItemNo string `json:"sourceInvoiceItemNo"`
	SourceContentDigest string `json:"sourceContentDigest"`
}

func (s *serviceImpl) prepareInvoiceTransformRows(
	prepared *preparedInvoiceTransform,
	groups [][]string,
) error {
	itemsByNo := make(map[string]*entity.BillingInvoiceItem, len(prepared.SourceItems))
	for _, item := range prepared.SourceItems {
		itemsByNo[item.InvoiceItemNo] = item
	}
	dueAt, err := earliestInvoiceTransformDue(prepared.Sources)
	if err != nil {
		return err
	}
	for index, source := range prepared.Sources {
		digest, digestErr := contentDigest(struct {
			TransformNo   string `json:"transformNo"`
			InvoiceNo     string `json:"invoiceNo"`
			Position      int    `json:"position"`
			ContentDigest string `json:"contentDigest"`
		}{prepared.TransformNo, source.InvoiceNo, index + 1, source.ContentDigest})
		if digestErr != nil {
			return digestErr
		}
		prepared.SourceRows = append(prepared.SourceRows, do.BillingInvoiceTransformSource{
			TenantId: 0, TransformNo: prepared.TransformNo, SourceInvoiceNo: source.InvoiceNo,
			CustomerNo: prepared.CustomerNo, CurrencyCode: prepared.CurrencyCode,
			Position: index + 1, SourceContentDigest: source.ContentDigest,
			ContentDigest: digest, CreatedAt: &prepared.Now,
		})
	}
	destinationDigests := make([]string, 0, len(groups))
	for index, group := range groups {
		destinationDigest, buildErr := s.prepareInvoiceTransformDestination(
			prepared, index+1, group, itemsByNo, dueAt,
		)
		if buildErr != nil {
			return buildErr
		}
		destinationDigests = append(destinationDigests, destinationDigest)
	}
	transformDigest, err := contentDigest(struct {
		TransformNo       string               `json:"transformNo"`
		Type              InvoiceTransformType `json:"type"`
		RequestDigest     string               `json:"requestDigest"`
		SourceInvoiceNos  []string             `json:"sourceInvoiceNos"`
		DestinationDigest []string             `json:"destinationDigests"`
	}{
		prepared.TransformNo, prepared.Command.Type, prepared.Command.RequestDigest,
		prepared.Command.SourceInvoiceNos, destinationDigests,
	})
	if err != nil {
		return err
	}
	prepared.ContentDigest = transformDigest
	return nil
}

func (s *serviceImpl) prepareInvoiceTransformDestination(
	prepared *preparedInvoiceTransform,
	position int,
	group []string,
	itemsByNo map[string]*entity.BillingInvoiceItem,
	dueAt time.Time,
) (string, error) {
	invoiceNo, err := newBillingNumber(s.random, "inv_")
	if err != nil {
		return "", err
	}
	amounts := destinationAmounts{}
	itemDigests := make([]transformItemDigestInput, 0, len(group))
	preparedItems := make([]do.BillingInvoiceItem, 0, len(group))
	preparedDerivations := make([]do.BillingInvoiceItemDerivation, 0, len(group))
	for index, sourceItemNo := range group {
		source := itemsByNo[sourceItemNo]
		if source == nil {
			return "", bizerr.NewCode(CodeInvoiceIntegrity)
		}
		if err = addDestinationItemAmounts(&amounts, source); err != nil {
			return "", err
		}
		itemNo, numberErr := newBillingNumber(s.random, "bii_")
		if numberErr != nil {
			return "", numberErr
		}
		itemDigest, digestErr := contentDigest(struct {
			TransformNo        string `json:"transformNo"`
			DestinationInvoice string `json:"destinationInvoiceNo"`
			DestinationItem    string `json:"destinationInvoiceItemNo"`
			LineNumber         int    `json:"lineNumber"`
			SourceItem         string `json:"sourceInvoiceItemNo"`
			SourceDigest       string `json:"sourceContentDigest"`
		}{prepared.TransformNo, invoiceNo, itemNo, index + 1, source.InvoiceItemNo, source.ContentDigest})
		if digestErr != nil {
			return "", digestErr
		}
		preparedItems = append(preparedItems, do.BillingInvoiceItem{
			TenantId: 0, InvoiceItemNo: itemNo, InvoiceNo: invoiceNo,
			CustomerNo: prepared.CustomerNo, CurrencyCode: prepared.CurrencyCode,
			LineNumber: index + 1, ItemTypeCode: "derived", SourceReference: source.InvoiceItemNo,
			DescriptionZhCn: source.DescriptionZhCn, DescriptionEnUs: source.DescriptionEnUs,
			SubtotalMinor: source.SubtotalMinor, DiscountMinor: source.DiscountMinor,
			NetAmountMinor: source.NetAmountMinor, TaxAmountMinor: source.TaxAmountMinor,
			TotalAmountMinor:   source.TotalAmountMinor,
			RefundableCapMinor: source.RefundableCapMinor, ContentDigest: itemDigest,
			CreatedAt: &prepared.Now,
		})
		derivationDigest, digestErr := contentDigest(struct {
			TransformNo        string `json:"transformNo"`
			SourceInvoice      string `json:"sourceInvoiceNo"`
			SourceItem         string `json:"sourceInvoiceItemNo"`
			DestinationInvoice string `json:"destinationInvoiceNo"`
			DestinationItem    string `json:"destinationInvoiceItemNo"`
			SourceDigest       string `json:"sourceContentDigest"`
			DestinationDigest  string `json:"destinationContentDigest"`
		}{
			prepared.TransformNo, source.InvoiceNo, source.InvoiceItemNo,
			invoiceNo, itemNo, source.ContentDigest, itemDigest,
		})
		if digestErr != nil {
			return "", digestErr
		}
		preparedDerivations = append(preparedDerivations, do.BillingInvoiceItemDerivation{
			TenantId: 0, TransformNo: prepared.TransformNo,
			SourceInvoiceNo: source.InvoiceNo, SourceInvoiceItemNo: source.InvoiceItemNo,
			DestinationInvoiceNo: invoiceNo, DestinationInvoiceItemNo: itemNo,
			CustomerNo: prepared.CustomerNo, CurrencyCode: prepared.CurrencyCode,
			SourceContentDigest: source.ContentDigest, DestinationContentDigest: itemDigest,
			ContentDigest: derivationDigest, CreatedAt: &prepared.Now,
		})
		itemDigests = append(itemDigests, transformItemDigestInput{
			source.InvoiceNo, source.InvoiceItemNo, source.ContentDigest,
		})
	}
	if amounts.TotalAmountMinor <= 0 {
		return "", bizerr.NewCode(CodeInvoiceConflict)
	}
	invoiceDigest, err := contentDigest(struct {
		TransformNo string                     `json:"transformNo"`
		Position    int                        `json:"position"`
		Items       []transformItemDigestInput `json:"items"`
		Amounts     destinationAmounts         `json:"amounts"`
		PaymentDue  int64                      `json:"paymentDueUnixMilli"`
	}{prepared.TransformNo, position, itemDigests, amounts, dueAt.UnixMilli()})
	if err != nil {
		return "", err
	}
	status := InvoiceStatusPendingPayment
	if !dueAt.After(prepared.Now) {
		status = InvoiceStatusOverdue
	}
	prepared.Destinations = append(prepared.Destinations, do.BillingInvoice{
		TenantId: 0, InvoiceNo: invoiceNo, CustomerNo: prepared.CustomerNo,
		SourceCode: "consolidation", CurrencyCode: prepared.CurrencyCode,
		StatusCode: status, ItemCount: len(group), SubtotalMinor: amounts.SubtotalMinor,
		DiscountMinor: amounts.DiscountMinor, NetAmountMinor: amounts.NetAmountMinor,
		TaxAmountMinor: amounts.TaxAmountMinor, TotalAmountMinor: amounts.TotalAmountMinor,
		PaidAmountMinor: 0, RefundableAmountMinor: 0, RefundedAmountMinor: 0,
		SourceSnapshotDigest: prepared.Command.RequestDigest, ContentDigest: invoiceDigest,
		PaymentDueAt: &dueAt, IssuedAt: &prepared.Now, Version: 1,
		CreatedAt: &prepared.Now, UpdatedAt: &prepared.Now,
	})
	destinationRowDigest, err := contentDigest(struct {
		TransformNo   string `json:"transformNo"`
		InvoiceNo     string `json:"invoiceNo"`
		Position      int    `json:"position"`
		ContentDigest string `json:"contentDigest"`
	}{prepared.TransformNo, invoiceNo, position, invoiceDigest})
	if err != nil {
		return "", err
	}
	prepared.DestinationRows = append(
		prepared.DestinationRows,
		do.BillingInvoiceTransformDestination{
			TenantId: 0, TransformNo: prepared.TransformNo,
			DestinationInvoiceNo: invoiceNo, CustomerNo: prepared.CustomerNo,
			CurrencyCode: prepared.CurrencyCode, Position: position,
			DestinationContentDigest: invoiceDigest, ContentDigest: destinationRowDigest,
			CreatedAt: &prepared.Now,
		},
	)
	prepared.DestinationItems = append(prepared.DestinationItems, preparedItems...)
	prepared.DerivationRows = append(prepared.DerivationRows, preparedDerivations...)
	return invoiceDigest, nil
}

func earliestInvoiceTransformDue(sources []*entity.BillingInvoice) (time.Time, error) {
	var due time.Time
	for _, source := range sources {
		if source == nil || source.PaymentDueAt == nil {
			return time.Time{}, bizerr.NewCode(CodeInvoiceIntegrity)
		}
		candidate := source.PaymentDueAt.UTC()
		if due.IsZero() || candidate.Before(due) {
			due = candidate
		}
	}
	if due.IsZero() {
		return time.Time{}, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	return due, nil
}

func addDestinationItemAmounts(
	amounts *destinationAmounts,
	item *entity.BillingInvoiceItem,
) error {
	if amounts == nil || item == nil || item.SubtotalMinor < 0 || item.DiscountMinor < 0 ||
		item.NetAmountMinor != item.SubtotalMinor-item.DiscountMinor ||
		item.TaxAmountMinor < 0 || item.TotalAmountMinor != item.NetAmountMinor+item.TaxAmountMinor ||
		item.RefundableCapMinor < 0 || item.RefundableCapMinor > item.TotalAmountMinor {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	values := []*int64{
		&amounts.SubtotalMinor, &amounts.DiscountMinor, &amounts.NetAmountMinor,
		&amounts.TaxAmountMinor, &amounts.TotalAmountMinor, &amounts.RefundableCapMinor,
	}
	additions := []int64{
		item.SubtotalMinor, item.DiscountMinor, item.NetAmountMinor,
		item.TaxAmountMinor, item.TotalAmountMinor, item.RefundableCapMinor,
	}
	for index, addition := range additions {
		if *values[index] > maximumAmountMinor-addition {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		*values[index] += addition
	}
	return nil
}

func persistInvoiceTransform(
	ctx context.Context,
	database gdb.DB,
	prepared *preparedInvoiceTransform,
) error {
	if prepared == nil || prepared.ContentDigest == "" || len(prepared.Destinations) == 0 ||
		len(prepared.DestinationItems) != len(prepared.SourceItems) ||
		len(prepared.DerivationRows) != len(prepared.SourceItems) {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	header := do.BillingInvoiceTransform{
		TenantId: 0, TransformNo: prepared.TransformNo,
		TransformTypeCode: prepared.Command.Type, CustomerNo: prepared.CustomerNo,
		CurrencyCode: prepared.CurrencyCode, SourceCount: len(prepared.Sources),
		DestinationCount: len(prepared.Destinations), ItemCount: len(prepared.SourceItems),
		IdempotencyDigest: prepared.Command.IdempotencyDigest,
		RequestDigest:     prepared.Command.RequestDigest, ContentDigest: prepared.ContentDigest,
		RequestedBy: prepared.Command.RequestedBy, CompletedAt: &prepared.Now,
		CreatedAt: &prepared.Now,
	}
	if _, err := database.Model(dao.BillingInvoiceTransform.Table()).Ctx(ctx).Data(header).Insert(); err != nil {
		return gerror.Wrap(err, "insert billing invoice transform")
	}
	if _, err := database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
		Data(prepared.Destinations).Insert(); err != nil {
		return gerror.Wrap(err, "insert transformed billing invoices")
	}
	if _, err := database.Model(dao.BillingInvoiceItem.Table()).Ctx(ctx).
		Data(prepared.DestinationItems).Insert(); err != nil {
		return gerror.Wrap(err, "insert transformed billing invoice items")
	}
	if _, err := database.Model(dao.BillingInvoiceTransformSource.Table()).Ctx(ctx).
		Data(prepared.SourceRows).Insert(); err != nil {
		return gerror.Wrap(err, "insert billing invoice transform sources")
	}
	if _, err := database.Model(dao.BillingInvoiceTransformDestination.Table()).Ctx(ctx).
		Data(prepared.DestinationRows).Insert(); err != nil {
		return gerror.Wrap(err, "insert billing invoice transform destinations")
	}
	if _, err := database.Model(dao.BillingInvoiceItemDerivation.Table()).Ctx(ctx).
		Data(prepared.DerivationRows).Insert(); err != nil {
		return gerror.Wrap(err, "insert billing invoice item derivations")
	}
	return cancelInvoiceTransformSources(ctx, database, prepared.Sources, prepared.Now)
}

func cancelInvoiceTransformSources(
	ctx context.Context,
	database gdb.DB,
	sources []*entity.BillingInvoice,
	now time.Time,
) error {
	if len(sources) == 0 || len(sources) > maximumInvoiceTransformInvoiceCount {
		return bizerr.NewCode(CodeInvoiceIntegrity)
	}
	var query strings.Builder
	query.WriteString(`
UPDATE plugin_idc_core_billing_invoice AS invoice
SET status_code = 'cancelled', cancelled_at = ?, settled_at = NULL,
    version = invoice.version + 1, updated_at = ?
FROM (VALUES `)
	arguments := make([]any, 0, 2+len(sources)*2)
	arguments = append(arguments, now, now)
	for index, source := range sources {
		if index > 0 {
			query.WriteString(", ")
		}
		query.WriteString("(CAST(? AS varchar), CAST(? AS bigint))")
		arguments = append(arguments, source.InvoiceNo, source.Version)
	}
	query.WriteString(`) AS expected(invoice_no, version)
WHERE invoice.tenant_id = 0
  AND invoice.invoice_no = expected.invoice_no
  AND invoice.version = expected.version
  AND invoice.status_code IN ('pending_payment', 'overdue')
  AND invoice.paid_amount_minor = 0
  AND invoice.waived_amount_minor = 0
  AND invoice.refundable_amount_minor = 0
  AND invoice.refunded_amount_minor = 0
  AND invoice.settled_at IS NULL
  AND invoice.cancelled_at IS NULL`)
	updated, err := database.Exec(ctx, query.String(), arguments...)
	if err != nil {
		return gerror.Wrap(err, "cancel billing invoice transform sources")
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read billing invoice transform source cancellation")
	}
	if affected != int64(len(sources)) {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func loadInvoiceTransformHeader(
	ctx context.Context,
	database gdb.DB,
	idempotencyDigest string,
) (*entity.BillingInvoiceTransform, error) {
	var transform *entity.BillingInvoiceTransform
	columns := dao.BillingInvoiceTransform.Columns()
	if err := database.Model(dao.BillingInvoiceTransform.Table()).Ctx(ctx).
		Fields(
			columns.TransformNo, columns.TransformTypeCode, columns.CustomerNo,
			columns.CurrencyCode, columns.SourceCount, columns.DestinationCount,
			columns.ItemCount, columns.IdempotencyDigest, columns.RequestDigest,
			columns.ContentDigest, columns.RequestedBy, columns.CompletedAt, columns.CreatedAt,
		).
		Where(columns.TenantId, 0).Where(columns.IdempotencyDigest, idempotencyDigest).
		Scan(&transform); err != nil {
		return nil, gerror.Wrap(err, "load billing invoice transform replay")
	}
	return transform, nil
}

func loadInvoiceTransformProjection(
	ctx context.Context,
	database gdb.DB,
	header *entity.BillingInvoiceTransform,
	reused bool,
) (*InvoiceTransform, error) {
	if header == nil || header.SourceCount < 1 || header.DestinationCount < 1 ||
		header.SourceCount > maximumInvoiceTransformInvoiceCount ||
		header.DestinationCount > maximumInvoiceTransformInvoiceCount ||
		!canonicalDigestPattern.MatchString(header.ContentDigest) {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	sources := make([]*entity.BillingInvoiceTransformSource, 0, header.SourceCount)
	sourceColumns := dao.BillingInvoiceTransformSource.Columns()
	if err := database.Model(dao.BillingInvoiceTransformSource.Table()).Ctx(ctx).
		Fields(sourceColumns.SourceInvoiceNo, sourceColumns.Position).
		Where(sourceColumns.TenantId, 0).Where(sourceColumns.TransformNo, header.TransformNo).
		OrderAsc(sourceColumns.Position).Limit(maximumInvoiceTransformInvoiceCount).
		Scan(&sources); err != nil {
		return nil, gerror.Wrap(err, "load billing invoice transform source projection")
	}
	destinations := make([]*entity.BillingInvoiceTransformDestination, 0, header.DestinationCount)
	destinationColumns := dao.BillingInvoiceTransformDestination.Columns()
	if err := database.Model(dao.BillingInvoiceTransformDestination.Table()).Ctx(ctx).
		Fields(destinationColumns.DestinationInvoiceNo, destinationColumns.Position).
		Where(destinationColumns.TenantId, 0).
		Where(destinationColumns.TransformNo, header.TransformNo).
		OrderAsc(destinationColumns.Position).Limit(maximumInvoiceTransformInvoiceCount).
		Scan(&destinations); err != nil {
		return nil, gerror.Wrap(err, "load billing invoice transform destination projection")
	}
	if len(sources) != header.SourceCount || len(destinations) != header.DestinationCount {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	destinationNos := make([]string, 0, len(destinations))
	for _, destination := range destinations {
		destinationNos = append(destinationNos, destination.DestinationInvoiceNo)
	}
	invoices, err := loadInvoiceTransformDestinations(ctx, database, destinationNos)
	if err != nil {
		return nil, err
	}
	byNo := make(map[string]*entity.BillingInvoice, len(invoices))
	for _, invoice := range invoices {
		byNo[invoice.InvoiceNo] = invoice
	}
	projection := &InvoiceTransform{
		TransformNo: header.TransformNo, Type: InvoiceTransformType(header.TransformTypeCode),
		CustomerNo: header.CustomerNo, CurrencyCode: header.CurrencyCode,
		SourceInvoiceNos:    make([]string, 0, len(sources)),
		DestinationInvoices: make([]*Invoice, 0, len(destinations)), Reused: reused,
	}
	for _, source := range sources {
		projection.SourceInvoiceNos = append(projection.SourceInvoiceNos, source.SourceInvoiceNo)
	}
	for _, destination := range destinations {
		invoice := byNo[destination.DestinationInvoiceNo]
		if invoice == nil {
			return nil, bizerr.NewCode(CodeInvoiceIntegrity)
		}
		projection.DestinationInvoices = append(
			projection.DestinationInvoices, projectInvoice(invoice, "", false, false),
		)
	}
	return projection, nil
}

func loadInvoiceTransformDestinations(
	ctx context.Context,
	database gdb.DB,
	invoiceNos []string,
) ([]*entity.BillingInvoice, error) {
	invoices := make([]*entity.BillingInvoice, 0, len(invoiceNos))
	columns := dao.BillingInvoice.Columns()
	if err := database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
		Fields(
			columns.InvoiceNo, columns.CustomerNo, columns.OrderNo, columns.CurrencyCode,
			columns.StatusCode, columns.TotalAmountMinor, columns.PaidAmountMinor,
			columns.WaivedAmountMinor, columns.CreditAmountMinor,
			columns.RefundableAmountMinor, columns.RefundedAmountMinor, columns.PaymentDueAt,
			columns.IssuedAt, columns.SettledAt, columns.CancelledAt, columns.Version,
		).
		Where(columns.TenantId, 0).WhereIn(columns.InvoiceNo, invoiceNos).
		Limit(maximumInvoiceTransformInvoiceCount).Scan(&invoices); err != nil {
		return nil, gerror.Wrap(err, "load billing invoice transform destination invoices")
	}
	if len(invoices) != len(invoiceNos) {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	return invoices, nil
}
