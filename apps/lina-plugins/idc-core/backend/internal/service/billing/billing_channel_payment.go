// This file owns the only core boundary through which the payment plugin may
// post verified channel money to an order invoice.

package billing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const channelPaymentLedgerSourceType = "payment_attempt"

type channelPaymentOwnerService struct {
	database    gdb.DB
	ledger      *serviceImpl
	settlements FinancialSettlementService
	principal   principal.Reader
	events      outbox.Enqueuer
	tasks       asynctask.Service
	closures    PaymentClosureOwner
}

type PaymentClosureOwner interface {
	ClaimPaymentClosure(context.Context) (*billingcap.PaymentClosureWork, error)
	ReportPaymentClosure(context.Context, billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error)
}

var _ billingcap.Service = (*channelPaymentOwnerService)(nil)

// NewChannelPaymentOwnerService composes verified channel funding with the
// core settlement authority. It accepts no customer, order, merchant, or
// mutable invoice status from the payment owner.
func NewChannelPaymentOwnerService(
	database gdb.DB,
	settlements FinancialSettlementService,
	events outbox.Enqueuer,
	tasks asynctask.Service,
	closures PaymentClosureOwner,
	readers ...principal.Reader,
) (billingcap.Service, error) {
	if isNilFinanceDirectoryDependency(database) ||
		isNilFinanceDirectoryDependency(settlements) ||
		isNilFinanceDirectoryDependency(events) ||
		isNilFinanceDirectoryDependency(tasks) || isNilFinanceDirectoryDependency(closures) {
		return nil, gerror.New("channel payment owner requires database, settlement, event, and task services")
	}
	ledger, err := newBalanceService(database, rand.Reader)
	if err != nil {
		return nil, err
	}
	var principalReader principal.Reader
	if len(readers) > 0 {
		principalReader = readers[0]
	}
	return &channelPaymentOwnerService{
		database: database, ledger: ledger, settlements: settlements,
		principal: principalReader, events: events, tasks: tasks, closures: closures,
	}, nil
}

func (s *channelPaymentOwnerService) ClaimPaymentClosure(ctx context.Context) (*billingcap.PaymentClosureWork, error) {
	return s.closures.ClaimPaymentClosure(ctx)
}

func (s *channelPaymentOwnerService) ReportPaymentClosure(ctx context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	if err := billingcap.ValidateReportPaymentClosure(input); err != nil {
		return nil, err
	}
	return s.closures.ReportPaymentClosure(ctx, input)
}

func (s *channelPaymentOwnerService) ConfirmChannelPayment(
	ctx context.Context,
	input billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	if err := billingcap.ValidateConfirmedChannelPayment(input); err != nil {
		return nil, err
	}
	result, err := s.postConfirmedChannelPayment(ctx, input)
	if err != nil {
		return nil, normalizeInvoiceError(err)
	}
	if result == nil {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	if err = s.attachChannelPaymentSettlement(ctx, result); err != nil {
		return nil, err
	}
	if err = billingcap.ValidateChannelPaymentResult(*result); err != nil {
		return nil, gerror.Wrap(err, "project channel payment owner result")
	}
	return result, nil
}

func (s *channelPaymentOwnerService) postConfirmedChannelPayment(
	ctx context.Context,
	input billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	var result *billingcap.ChannelPaymentResult
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var postErr error
		result, postErr = s.postConfirmedChannelPaymentLocked(ctx, input)
		return postErr
	})
	return result, err
}

func (s *channelPaymentOwnerService) postConfirmedChannelPaymentLocked(
	ctx context.Context,
	input billingcap.ConfirmedChannelPayment,
) (*billingcap.ChannelPaymentResult, error) {
	if err := s.ledger.lockSource(
		ctx, channelPaymentLedgerSourceType, input.PaymentAttemptNo,
	); err != nil {
		return nil, err
	}
	existing, err := loadTransactionBySource(
		ctx, s.database, channelPaymentLedgerSourceType, input.PaymentAttemptNo,
	)
	if err != nil {
		return nil, err
	}
	invoice, err := lockInvoice(ctx, s.database, input.InvoiceNo)
	if err != nil {
		return nil, err
	}
	if invoice == nil || invoice.OrderNo == nil ||
		!validBusinessReference(*invoice.OrderNo, 40) ||
		invoice.SourceCode != "order" || invoice.CurrencyCode != input.CurrencyCode {
		return nil, bizerr.NewCode(CodeInvoiceNotFound)
	}
	aggregate, err := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
	if err != nil {
		return nil, err
	}
	if err = validateInvoiceAggregate(invoice, aggregate); err != nil {
		return nil, err
	}
	if existing == nil {
		if !invoiceAcceptsPayment(invoice) ||
			input.TotalAmountMinor > invoice.TotalAmountMinor-aggregate.PaymentMinor-aggregate.WaiverMinor {
			return nil, bizerr.NewCode(CodeInvoiceConflict)
		}
		if err = s.ledger.requireActiveCustomer(ctx, invoice.CustomerNo); err != nil {
			return nil, err
		}
	}
	merchantNo, err := s.ledger.loadMerchantNo(ctx, existing == nil)
	if err != nil {
		return nil, err
	}
	cashSpec := merchantCashSpec(merchantNo)
	receivableSpec := merchantReceivableSpec(merchantNo)
	accounts, err := s.ledger.ensureAndLockAccounts(
		ctx, input.CurrencyCode, existing == nil, cashSpec, receivableSpec,
	)
	if err != nil {
		return nil, err
	}
	cashAccountNo := deterministicAccountNo(
		cashSpec.OwnerType, cashSpec.OwnerReference, cashSpec.AccountType, input.CurrencyCode,
	)
	receivableAccountNo := deterministicAccountNo(
		receivableSpec.OwnerType, receivableSpec.OwnerReference,
		receivableSpec.AccountType, input.CurrencyCode,
	)
	transaction, _, err := s.ledger.postLocked(
		ctx, "payment", input.CurrencyCode,
		channelPaymentLedgerSourceType, input.PaymentAttemptNo,
		input.EvidenceDigest, "",
		[]entrySemantic{
			{AccountNo: cashAccountNo, Direction: "debit", Role: "cash",
				AmountMinor: input.AmountMinor, InvoiceNo: invoice.InvoiceNo,
				Reason: "channel_payment"},
			{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable",
				AmountMinor: input.AmountMinor, InvoiceNo: invoice.InvoiceNo,
				Reason: "channel_payment"},
		},
		accounts,
	)
	if err != nil {
		return nil, err
	}
	paid, err := s.ledger.ApplyPayment(ctx, InvoicePaymentInput{
		InvoiceNo: invoice.InvoiceNo, SourceType: PaymentSourceChannel,
		SourceReference:     input.PaymentAttemptNo,
		LedgerTransactionNo: transaction.TransactionNo, AmountMinor: input.AmountMinor,
	})
	if err != nil {
		return nil, err
	}
	channelAllocation := paid
	balancePosting, balanceAllocation, err := s.consumePreparedBalance(ctx, invoice, input)
	if err != nil {
		return nil, err
	}
	if balanceAllocation != nil {
		paid = balanceAllocation
	}
	if paid == nil || paid.InvoiceNo != invoice.InvoiceNo || paid.OrderNo != *invoice.OrderNo ||
		paid.CurrencyCode != input.CurrencyCode || paid.AllocationNo == "" ||
		paid.PaidAmountMinor < input.AmountMinor {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	result := &billingcap.ChannelPaymentResult{
		PaymentAttemptNo: input.PaymentAttemptNo,
		TransactionNo:    transaction.TransactionNo, AllocationNo: channelAllocation.AllocationNo,
		InvoiceNo: paid.InvoiceNo, OrderNo: paid.OrderNo, CurrencyCode: paid.CurrencyCode,
		AmountMinor: input.AmountMinor, TotalAmountMinor: input.TotalAmountMinor,
		BalanceAmountMinor: input.BalanceAmountMinor, PaidAmountMinor: paid.PaidAmountMinor,
		InvoiceStatus: string(paid.Status), FundingReused: existing != nil || paid.Reused,
	}
	if balancePosting != nil && balanceAllocation != nil {
		result.BalanceTransactionNo = balancePosting.TransactionNo
		result.BalanceAllocationNo = balanceAllocation.AllocationNo
		result.FundingReused = result.FundingReused || balancePosting.Reused || balanceAllocation.Reused
	}
	return result, nil
}

func (s *channelPaymentOwnerService) consumePreparedBalance(
	ctx context.Context,
	invoice *entity.BillingInvoice,
	input billingcap.ConfirmedChannelPayment,
) (*Posting, *Invoice, error) {
	if input.BalanceAmountMinor <= 0 {
		return nil, nil, nil
	}
	hold, err := loadHold(ctx, s.database, input.BalanceHoldNo, true)
	if err != nil {
		return nil, nil, err
	}
	if hold == nil || hold.CustomerNo != invoice.CustomerNo || hold.CurrencyCode != input.CurrencyCode ||
		hold.SourceType != channelPaymentLedgerSourceType || hold.SourceReference != input.PaymentAttemptNo ||
		hold.AmountMinor != input.BalanceAmountMinor {
		return nil, nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	balanceReference := channelBalanceReference(input.PaymentAttemptNo)
	posting, err := s.ledger.ConsumeHold(ctx, HoldConsumeInput{
		HoldNo: input.BalanceHoldNo, InvoiceNo: invoice.InvoiceNo, AmountMinor: input.BalanceAmountMinor,
		SourceReference: balanceReference, SourceSnapshotDigest: input.EvidenceDigest,
	})
	if err != nil {
		return nil, nil, err
	}
	allocation, err := s.ledger.ApplyPayment(ctx, InvoicePaymentInput{
		InvoiceNo: invoice.InvoiceNo, SourceType: PaymentSourceBalance,
		SourceReference: balanceReference, LedgerTransactionNo: posting.TransactionNo,
		AmountMinor: input.BalanceAmountMinor,
	})
	if err != nil {
		return nil, nil, err
	}
	return posting, allocation, nil
}

func channelBalanceReference(attemptNo string) string {
	digest := sha256.Sum256([]byte("channel-balance\x00" + attemptNo))
	return "pbc_" + hex.EncodeToString(digest[:16])
}

func (s *channelPaymentOwnerService) attachChannelPaymentSettlement(
	ctx context.Context,
	result *billingcap.ChannelPaymentResult,
) error {
	if result.InvoiceStatus != string(InvoiceStatusPaid) {
		return nil
	}
	settled, err := s.settlements.SettleOrder(ctx, result.OrderNo)
	if err != nil {
		// The funding transaction and allocation have already committed. The
		// payment owner must retry this same immutable confirmation.
		return err
	}
	if settled == nil || settled.OrderNo != result.OrderNo ||
		settled.InvoiceNo != result.InvoiceNo ||
		settled.CurrencyCode != result.CurrencyCode ||
		settled.Basis != SettlementBasisFundedInvoice {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	result.SettlementNo = settled.SettlementNo
	result.SettlementReused = settled.Reused
	return nil
}
