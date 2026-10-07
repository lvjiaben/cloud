package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type channelPaymentContextRow struct {
	CustomerNo string    `orm:"customer_no"`
	OrderNo    string    `orm:"order_no"`
	InvoiceNo  string    `orm:"invoice_no"`
	StatusCode string    `orm:"status_code"`
	ExpiresAt  time.Time `orm:"expires_at"`
}

type channelPaymentOrderRow struct {
	OrderNo          string    `orm:"order_no"`
	CustomerNo       string    `orm:"customer_no"`
	CurrencyCode     string    `orm:"currency_code"`
	TotalAmountMinor int64     `orm:"total_amount_minor"`
	StatusCode       string    `orm:"status_code"`
	PaymentDueAt     time.Time `orm:"payment_due_at"`
	Version          int64     `orm:"version"`
}

func (s *channelPaymentOwnerService) CurrentPaymentActor(ctx context.Context) (*billingcap.PaymentActor, error) {
	if s.principal == nil {
		return nil, gerror.New("payment actor requires an authenticated principal")
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive || audience.UserID <= 0 || audience.CustomerNo == "" {
		return nil, bizerr.NewCode(CodeInvoiceNotFound)
	}
	return &billingcap.PaymentActor{PrincipalRefDigest: billingcap.PrincipalReferenceDigest(audience.UserID)}, nil
}

func (s *channelPaymentOwnerService) PrepareChannelPayment(
	ctx context.Context,
	input billingcap.PrepareChannelPayment,
) (*billingcap.PreparedChannelPayment, error) {
	if err := billingcap.ValidatePrepareChannelPayment(input); err != nil {
		return nil, err
	}
	if s.principal == nil {
		return nil, gerror.New("channel payment preparation requires an authenticated principal")
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" {
		return nil, bizerr.NewCode(CodeInvoiceNotFound)
	}
	var result *billingcap.PreparedChannelPayment
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var paymentContext *channelPaymentContextRow
		if loadErr := tx.Ctx(ctx).GetScan(&paymentContext, `
SELECT customer_no,order_no,invoice_no,status_code,expires_at
FROM plugin_idc_core_commerce_payment_context
WHERE tenant_id=0 AND context_no=? AND customer_no=?
FOR UPDATE`, input.PaymentContextNo, audience.CustomerNo); loadErr != nil {
			return loadErr
		}
		if paymentContext == nil || paymentContext.StatusCode != "ready" {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		var order *channelPaymentOrderRow
		if loadErr := tx.Ctx(ctx).GetScan(&order, `
SELECT order_no,customer_no,currency_code,total_amount_minor,status_code,payment_due_at,version
FROM plugin_idc_core_commerce_order
WHERE tenant_id=0 AND order_no=? FOR UPDATE`, paymentContext.OrderNo); loadErr != nil {
			return loadErr
		}
		invoice, loadErr := lockInvoice(ctx, s.database, paymentContext.InvoiceNo)
		if loadErr != nil {
			return loadErr
		}
		now, loadErr := databaseTime(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		if validationErr := validateChannelPaymentPreparation(
			audience.CustomerNo, paymentContext, order, invoice, now,
		); validationErr != nil {
			return validationErr
		}
		aggregate, loadErr := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
		if loadErr != nil {
			return loadErr
		}
		if loadErr = validateInvoiceAggregate(invoice, aggregate); loadErr != nil {
			return loadErr
		}
		if transitionErr := transitionOrderToPaymentProcessing(ctx, tx, order); transitionErr != nil {
			return transitionErr
		}
		outstanding := invoice.TotalAmountMinor - aggregate.PaymentMinor - aggregate.WaiverMinor
		if outstanding <= 0 || input.BalanceAmountMinor < 0 || input.BalanceAmountMinor >= outstanding {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		digest, digestErr := contentDigest(struct {
			AttemptNo, ContextNo, InvoiceNo, OrderNo, CustomerNo, Currency string
			TotalMinor, BalanceMinor                                       int64
			ExpiresAt                                                      string
		}{input.PaymentAttemptNo, input.PaymentContextNo, invoice.InvoiceNo, order.OrderNo, audience.CustomerNo, invoice.CurrencyCode, outstanding, input.BalanceAmountMinor, paymentContext.ExpiresAt.UTC().Format(time.RFC3339Nano)})
		if digestErr != nil {
			return digestErr
		}
		holdNo := ""
		if input.BalanceAmountMinor > 0 {
			hold, holdErr := s.ledger.CreateHold(ctx, HoldCreateInput{
				CustomerNo: audience.CustomerNo, CurrencyCode: invoice.CurrencyCode,
				AmountMinor: input.BalanceAmountMinor, SourceType: channelPaymentLedgerSourceType,
				SourceReference: input.PaymentAttemptNo, SourceSnapshotDigest: digest,
				ExpiresAt: paymentContext.ExpiresAt.UTC(),
			})
			if holdErr != nil {
				return holdErr
			}
			holdNo = hold.HoldNo
		}
		result = &billingcap.PreparedChannelPayment{
			PaymentAttemptNo: input.PaymentAttemptNo, PaymentContextNo: input.PaymentContextNo,
			InvoiceNo: invoice.InvoiceNo, OrderNo: order.OrderNo,
			CustomerRefDigest: billingcap.PrincipalReferenceDigest(audience.UserID),
			CurrencyCode:      invoice.CurrencyCode, TotalAmountMinor: outstanding,
			ChannelAmountMinor: outstanding - input.BalanceAmountMinor,
			BalanceHoldNo:      holdNo, BalanceAmountMinor: input.BalanceAmountMinor,
			ExpiresAt: paymentContext.ExpiresAt.UTC(),
		}
		return billingcap.ValidatePreparedChannelPayment(*result)
	})
	return result, normalizeInvoiceError(err)
}

func validateChannelPaymentPreparation(
	customerNo string,
	paymentContext *channelPaymentContextRow,
	order *channelPaymentOrderRow,
	invoice *entity.BillingInvoice,
	now time.Time,
) error {
	if order == nil || invoice == nil || order.CustomerNo != customerNo ||
		invoice.CustomerNo != customerNo || invoice.OrderNo == nil || *invoice.OrderNo != order.OrderNo ||
		(order.StatusCode != "pending_payment" && order.StatusCode != "payment_processing") || invoice.StatusCode != "pending_payment" ||
		order.CurrencyCode != invoice.CurrencyCode || order.TotalAmountMinor != invoice.TotalAmountMinor ||
		!paymentContext.ExpiresAt.After(now) || !order.PaymentDueAt.After(now) {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func transitionOrderToPaymentProcessing(
	ctx context.Context,
	tx gdb.TX,
	order *channelPaymentOrderRow,
) error {
	if order.StatusCode != "pending_payment" {
		return nil
	}
	updated, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order
SET status_code='payment_processing',version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND order_no=? AND status_code='pending_payment' AND version=?`, order.OrderNo, order.Version)
	if err != nil {
		return err
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	order.StatusCode = "payment_processing"
	order.Version++
	return nil
}

func (s *channelPaymentOwnerService) ReleaseChannelPayment(
	ctx context.Context,
	input billingcap.ReleaseChannelPayment,
) (*billingcap.ReleasedChannelPayment, error) {
	if err := billingcap.ValidateReleaseChannelPayment(input); err != nil {
		return nil, err
	}
	var result *billingcap.ReleasedChannelPayment
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		hold, loadErr := loadHold(ctx, s.database, input.BalanceHoldNo, true)
		if loadErr != nil {
			return loadErr
		}
		if hold == nil || hold.SourceType != channelPaymentLedgerSourceType || hold.SourceReference != input.PaymentAttemptNo {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		released, releaseErr := s.ledger.ReleaseHold(ctx, input.BalanceHoldNo)
		if releaseErr != nil {
			return releaseErr
		}
		result = &billingcap.ReleasedChannelPayment{PaymentAttemptNo: input.PaymentAttemptNo, BalanceHoldNo: input.BalanceHoldNo, Status: released.Status, Reused: released.Reused}
		return nil
	})
	return result, normalizeInvoiceError(err)
}
