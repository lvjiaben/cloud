// This file creates one immutable pending-review request in a bounded transaction.

package taxinvoicerequest

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type requestRow struct {
	TaxInvoiceNo     string    `orm:"tax_invoice_no"`
	RequestSHA256    string    `orm:"request_sha256"`
	CustomerNo       string    `orm:"customer_no"`
	InvoiceTypeCode  string    `orm:"invoice_type_code"`
	StatusCode       string    `orm:"status_code"`
	TaxCurrencyCode  string    `orm:"tax_currency_code"`
	GrossAmountMinor int64     `orm:"gross_amount_minor"`
	NetAmountMinor   int64     `orm:"net_amount_minor"`
	TaxAmountMinor   int64     `orm:"tax_amount_minor"`
	SubmittedAt      time.Time `orm:"submitted_at"`
	Version          int64     `orm:"version"`
}

type creationState struct {
	row      *requestRow
	snapshot *taxinvoicecap.RequestSnapshot
	bundle   *snapshotBundle
	payload  []byte
	reused   bool
}

// CreateCurrentCustomer creates exactly one reservation-bearing request. The
// outer transaction and the core source's nested transaction share one context,
// so source row locks remain held until compliance allocations commit.
func (s *serviceImpl) CreateCurrentCustomer(
	ctx context.Context,
	input CreateInput,
) (*Request, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := normalizeInput(input)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInvalid)
	}
	defer clear(normalized.Selections)
	options, err := s.ListCurrentCustomerOptions(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInvalid)
	}
	taxInvoiceNo, requestDigest, err := deriveRequestIdentity(options.CustomerNo, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInvalid)
	}
	existing, err := s.loadRequest(ctx, taxInvoiceNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if existing != nil {
		return projectExisting(existing, options.CustomerNo, requestDigest, true)
	}
	if _, err = s.privacyGate.RequireSensitiveCollection(ctx, "tax_invoice", "tax_invoice_evidence"); err != nil {
		return nil, bizerr.WrapCode(err, CodeInvalid)
	}

	state := new(creationState)
	defer state.clear()
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, lockErr := tx.GetValue(
			"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
			"tax-invoice-request:"+taxInvoiceNo,
		); lockErr != nil {
			return gerror.Wrap(lockErr, "lock tax invoice request identity")
		}
		current, loadErr := s.loadRequest(ctx, taxInvoiceNo)
		if loadErr != nil {
			return loadErr
		}
		if current != nil {
			state.row = current
			state.reused = true
			return nil
		}
		state.snapshot, loadErr = s.source.PrepareCurrentCustomerRequest(
			ctx, s, normalized.ProfileNo, normalized.Selections,
		)
		if loadErr != nil {
			return bizerr.WrapCode(loadErr, CodeInvalid)
		}
		if state.snapshot == nil || state.snapshot.CustomerNo != options.CustomerNo {
			return gerror.New("tax invoice source customer changed during submission")
		}
		var digests snapshotDigests
		state.bundle, digests, state.payload, loadErr = newSnapshotBundle(taxInvoiceNo, state.snapshot)
		if loadErr != nil {
			return loadErr
		}
		created, sealErr := s.sealSnapshot(
			ctx, requestDigest, state.bundle, digests, state.payload,
		)
		if sealErr != nil {
			return sealErr
		}
		state.row, loadErr = s.persistRequest(
			ctx, state.snapshot, taxInvoiceNo, requestDigest,
			created.Handle.OpaqueRef(), digests,
		)
		return loadErr
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if state.row == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return projectExisting(state.row, options.CustomerNo, requestDigest, state.reused)
}

func (state *creationState) clear() {
	if state == nil {
		return
	}
	if state.snapshot != nil {
		state.snapshot.Clear()
	}
	if state.bundle != nil {
		state.bundle.Request.Clear()
	}
	clear(state.payload)
}

func (s *serviceImpl) loadRequest(ctx context.Context, taxInvoiceNo string) (*requestRow, error) {
	var rows []*requestRow
	err := s.database.Model(dao.TaxInvoice.Table()).Ctx(ctx).
		Fields(`tax_invoice_no, request_sha256, customer_no, invoice_type_code,
status_code, tax_currency_code, gross_amount_minor, net_amount_minor,
tax_amount_minor, submitted_at, version`).
		Where(do.TaxInvoice{
			TenantId: idccap.DefaultTenantID, TaxInvoiceNo: taxInvoiceNo,
		}).Limit(2).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load tax invoice request")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 || rows[0] == nil {
		return nil, gerror.New("tax invoice request identity is duplicated")
	}
	return rows[0], nil
}

func projectExisting(row *requestRow, customerNo, requestDigest string, reused bool) (*Request, error) {
	if row == nil || row.CustomerNo != customerNo || row.RequestSHA256 != requestDigest {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if !validBusinessNo(row.TaxInvoiceNo, 64) || row.SubmittedAt.IsZero() ||
		row.GrossAmountMinor <= 0 || row.GrossAmountMinor != row.NetAmountMinor+row.TaxAmountMinor ||
		row.Version < 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &Request{
		TaxInvoiceNo: row.TaxInvoiceNo, CustomerNo: row.CustomerNo,
		StatusCode: row.StatusCode, InvoiceTypeCode: row.InvoiceTypeCode,
		CurrencyCode: row.TaxCurrencyCode, GrossAmountMinor: row.GrossAmountMinor,
		NetAmountMinor: row.NetAmountMinor, TaxAmountMinor: row.TaxAmountMinor,
		SubmittedAt: row.SubmittedAt.UTC(), Version: row.Version, Reused: reused,
	}, nil
}

func (s *serviceImpl) persistRequest(
	ctx context.Context,
	snapshot *taxinvoicecap.RequestSnapshot,
	taxInvoiceNo, requestDigest, snapshotRef string,
	digests snapshotDigests,
) (*requestRow, error) {
	if snapshot == nil || snapshotRef == "" || len(digests.Items) == 0 {
		return nil, gerror.New("tax invoice persistence snapshot is invalid")
	}
	submittedAt := snapshot.ObservedAt.UTC()
	_, err := s.database.Model(dao.TaxInvoice.Table()).Ctx(ctx).Data(do.TaxInvoice{
		TenantId: idccap.DefaultTenantID, TaxInvoiceNo: taxInvoiceNo,
		RequestSha256: requestDigest, CustomerNo: snapshot.CustomerNo,
		PrincipalNo:       snapshot.PrincipalReference,
		SellerSnapshotRef: snapshotRef, SellerSnapshotSha256: digests.Seller,
		BuyerSnapshotRef: snapshotRef, BuyerSnapshotSha256: digests.Buyer,
		InvoiceTypeCode: string(InvoiceTypePendingConfirmation),
		StatusCode:      string(StatusPendingReview), TaxCurrencyCode: snapshot.CurrencyCode,
		GrossAmountMinor: snapshot.GrossAmountMinor, NetAmountMinor: snapshot.NetAmountMinor,
		TaxAmountMinor: snapshot.TaxAmountMinor, BillingCurrencyCode: snapshot.CurrencyCode,
		SubmittedAt: &submittedAt, Version: 1,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "insert tax invoice request")
	}
	allocationRows := make([]do.TaxInvoiceAllocation, 0, len(snapshot.Invoices))
	itemRows := make([]do.TaxInvoiceItem, 0)
	lineNo := 0
	for _, invoice := range snapshot.Invoices {
		currentAllocationNo := allocationNo(taxInvoiceNo, invoice.BillingInvoiceNo)
		allocationRows = append(allocationRows, do.TaxInvoiceAllocation{
			TenantId: idccap.DefaultTenantID, AllocationNo: currentAllocationNo,
			TaxInvoiceNo: taxInvoiceNo, BillingInvoiceNo: invoice.BillingInvoiceNo,
			OrderNo: invoice.OrderNo, AllocatedGrossMinor: invoice.RequestedGrossMinor,
			AllocatedNetMinor: invoice.NetAmountMinor, AllocatedTaxMinor: invoice.TaxAmountMinor,
			RefundedGrossMinor: 0, RedLetteredGrossMinor: 0,
		})
		for _, item := range invoice.Items {
			lineNo++
			itemRows = append(itemRows, do.TaxInvoiceItem{
				TenantId:     idccap.DefaultTenantID,
				ItemNo:       itemNo(taxInvoiceNo, item.BillingInvoiceItemNo),
				TaxInvoiceNo: taxInvoiceNo, LineNo: lineNo,
				ItemSnapshotRef:       snapshotRef,
				ItemSnapshotSha256:    digests.Items[item.BillingInvoiceItemNo],
				TaxClassificationCode: item.TaxClassificationCode,
				TaxRatePpm:            item.TaxRatePPM, GrossAmountMinor: item.GrossAmountMinor,
				NetAmountMinor: item.NetAmountMinor, TaxAmountMinor: item.TaxAmountMinor,
				AllocationNo:         currentAllocationNo,
				BillingInvoiceItemNo: item.BillingInvoiceItemNo,
			})
		}
	}
	if len(allocationRows) == 0 || len(itemRows) == 0 ||
		len(itemRows) > taxinvoicecap.MaximumItemsPerRequest {
		return nil, gerror.New("tax invoice persistence allocations are invalid")
	}
	if _, err = s.database.Model(dao.TaxInvoiceAllocation.Table()).Ctx(ctx).
		Data(allocationRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert tax invoice allocations")
	}
	if _, err = s.database.Model(dao.TaxInvoiceItem.Table()).Ctx(ctx).
		Data(itemRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert tax invoice items")
	}
	return &requestRow{
		TaxInvoiceNo: taxInvoiceNo, RequestSHA256: requestDigest,
		CustomerNo:      snapshot.CustomerNo,
		InvoiceTypeCode: string(InvoiceTypePendingConfirmation),
		StatusCode:      string(StatusPendingReview), TaxCurrencyCode: snapshot.CurrencyCode,
		GrossAmountMinor: snapshot.GrossAmountMinor, NetAmountMinor: snapshot.NetAmountMinor,
		TaxAmountMinor: snapshot.TaxAmountMinor, SubmittedAt: submittedAt, Version: 1,
	}, nil
}
