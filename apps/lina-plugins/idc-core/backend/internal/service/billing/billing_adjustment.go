// This file owns immutable adjustment policy revisions, reviewed receipt and
// waiver intents, idempotent replay, and authorized decisions.

package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) ConfigureApprovalPolicy(
	ctx context.Context,
	input AdjustmentPolicyInput,
) (*AdjustmentPolicy, error) {
	normalized, err := normalizeAdjustmentPolicy(input)
	if err != nil {
		return nil, err
	}
	var result *AdjustmentPolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, "adjustment_policy", normalized.CurrencyCode); lockErr != nil {
			return lockErr
		}
		current, loadErr := loadCurrentAdjustmentPolicy(ctx, s.database, normalized.CurrencyCode)
		if loadErr != nil {
			return loadErr
		}
		if current != nil && current.ReviewThresholdMinor == normalized.ReviewThresholdMinor {
			result = projectAdjustmentPolicy(current, true)
			return nil
		}
		version := int64(1)
		if current != nil {
			version = current.Version + 1
		}
		policyNo, numberErr := newBillingNumber(s.random, "bap_")
		if numberErr != nil {
			return numberErr
		}
		digest, digestErr := contentDigest(struct {
			PolicyNo             string `json:"policyNo"`
			CurrencyCode         string `json:"currencyCode"`
			ReviewThresholdMinor int64  `json:"reviewThresholdMinor"`
			Version              int64  `json:"version"`
			ConfiguredBy         int    `json:"configuredBy"`
		}{policyNo, normalized.CurrencyCode, normalized.ReviewThresholdMinor, version, normalized.ConfiguredBy})
		if digestErr != nil {
			return digestErr
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		row := do.BillingAdjustmentPolicy{
			TenantId: 0, PolicyNo: policyNo, CurrencyCode: normalized.CurrencyCode,
			ReviewThresholdMinor: normalized.ReviewThresholdMinor, Version: version,
			ConfiguredBy: normalized.ConfiguredBy, ContentDigest: digest, CreatedAt: &now,
		}
		if _, insertErr := s.database.Model(dao.BillingAdjustmentPolicy.Table()).Ctx(ctx).
			Data(row).Insert(); insertErr != nil {
			return gerror.Wrap(insertErr, "insert billing adjustment policy")
		}
		result = &AdjustmentPolicy{
			PolicyNo: policyNo, CurrencyCode: normalized.CurrencyCode,
			ReviewThresholdMinor: normalized.ReviewThresholdMinor, Version: version,
			ConfiguredBy: normalized.ConfiguredBy,
		}
		return nil
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) CreateManualReceipt(
	ctx context.Context,
	input ManualReceiptInput,
) (*FinancialAdjustment, error) {
	command, err := normalizeManualReceipt(input)
	if err != nil {
		return nil, err
	}
	return s.createFinancialAdjustment(ctx, command)
}

func (s *serviceImpl) CreateInvoiceWaiver(
	ctx context.Context,
	input InvoiceWaiverInput,
) (*FinancialAdjustment, error) {
	command, err := normalizeInvoiceWaiver(input)
	if err != nil {
		return nil, err
	}
	return s.createFinancialAdjustment(ctx, command)
}

// CreateMarketingGrantPosting records a reviewed request to credit one exact
// marketing value grant into the customer's non-withdrawable gift account.
func (s *serviceImpl) CreateMarketingGrantPosting(
	ctx context.Context,
	input MarketingGrantAdjustmentInput,
) (*FinancialAdjustment, error) {
	command, err := normalizeMarketingGrantAdjustment(input, adjustmentTypeMarketingGrantPost)
	if err != nil {
		return nil, err
	}
	return s.createFinancialAdjustment(ctx, command)
}

// CreateMarketingGrantReversal records a reviewed request to reverse one exact
// unused marketing value-grant amount from the customer gift account.
func (s *serviceImpl) CreateMarketingGrantReversal(
	ctx context.Context,
	input MarketingGrantAdjustmentInput,
) (*FinancialAdjustment, error) {
	command, err := normalizeMarketingGrantAdjustment(input, adjustmentTypeMarketingGrantReverse)
	if err != nil {
		return nil, err
	}
	return s.createFinancialAdjustment(ctx, command)
}

func (s *serviceImpl) createFinancialAdjustment(
	ctx context.Context,
	command adjustmentCommand,
) (*FinancialAdjustment, error) {
	var result *FinancialAdjustment
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		invoice, statement, resolveErr := resolveAdjustmentTarget(ctx, s.database, &command)
		if resolveErr != nil {
			return resolveErr
		}
		if lockErr := s.lockSource(
			ctx, "adjustment_request", command.CustomerNo+":"+command.IdempotencyDigest,
		); lockErr != nil {
			return lockErr
		}
		requestDigest, digestErr := adjustmentRequestDigest(command)
		if digestErr != nil {
			return digestErr
		}
		existing, loadErr := loadAdjustmentReplay(
			ctx, s.database, command.CustomerNo, command.IdempotencyDigest,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if !adjustmentRequestMatches(existing, command, requestDigest) {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
			result, loadErr = s.projectFinancialAdjustment(ctx, existing, true)
			return loadErr
		}
		if validateErr := validateNewAdjustmentTarget(
			ctx, s.database, invoice, statement, command,
		); validateErr != nil {
			return validateErr
		}
		if command.InvoiceNo == "" && command.CreditStatementNo == "" {
			if customerErr := s.requireActiveCustomer(ctx, command.CustomerNo); customerErr != nil {
				return customerErr
			}
		}
		if _, merchantErr := s.loadMerchantNo(ctx, true); merchantErr != nil {
			return merchantErr
		}
		if command.InvoiceNo == "" && command.CreditStatementNo == "" {
			if _, accountErr := s.ensureAndLockAccounts(
				ctx, command.CurrencyCode, true, customerAdjustmentAccountSpec(command),
			); accountErr != nil {
				return accountErr
			}
		}
		if command.AdjustmentType == adjustmentTypeBalanceDebit {
			if balanceErr := s.requireAvailableAdjustmentBalance(ctx, command.AccountNo, command.AmountMinor); balanceErr != nil {
				return balanceErr
			}
		}
		// Exact action authorization is handled by the management owner. Historic
		// review policies remain immutable evidence but do not delay new postings.
		policy := adjustmentPolicySnapshot{}
		request, insertErr := s.insertAdjustmentRequest(ctx, command, policy, requestDigest)
		if insertErr != nil {
			return insertErr
		}
		result, insertErr = s.postFinancialAdjustment(
			ctx, request, invoice, statement, false,
		)
		return insertErr
	})
	return result, normalizeInvoiceError(err)
}

func (s *serviceImpl) ReviewAdjustment(
	ctx context.Context,
	input AdjustmentReviewInput,
) (*FinancialAdjustment, error) {
	normalized, err := normalizeAdjustmentReview(input)
	if err != nil {
		return nil, err
	}
	var result *FinancialAdjustment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		request, loadErr := lockAdjustmentRequest(ctx, s.database, normalized.AdjustmentNo)
		if loadErr != nil {
			return loadErr
		}
		if request == nil {
			return bizerr.NewCode(CodeInvoiceNotFound)
		}
		existing, loadErr := loadAdjustmentApproval(ctx, s.database, request.AdjustmentNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.ReviewerId != normalized.ReviewerID ||
				existing.DecisionCode != string(normalized.Decision) ||
				existing.CommentDigest != normalized.CommentDigest ||
				existing.EvidenceDigest != normalized.EvidenceDigest {
				return bizerr.NewCode(CodeInvoiceConflict)
			}
			result, loadErr = s.projectFinancialAdjustment(ctx, request, true)
			return loadErr
		}
		if !request.ApprovalRequired || request.StatusCode != string(AdjustmentStatusPendingReview) {
			return bizerr.NewCode(CodeInvoiceConflict)
		}
		if insertErr := s.insertAdjustmentApproval(ctx, request, normalized); insertErr != nil {
			return insertErr
		}
		if normalized.Decision == AdjustmentDecisionReject {
			if updateErr := updateAdjustmentStatus(
				ctx, s.database, request, string(AdjustmentStatusRejected), "", "", nil,
			); updateErr != nil {
				return updateErr
			}
			request.StatusCode = string(AdjustmentStatusRejected)
			request.Version++
			result = projectAdjustmentRequest(request, false)
			return nil
		}
		var invoice *entity.BillingInvoice
		var statement *creditStatementRow
		if request.InvoiceNo != nil {
			invoice, loadErr = lockInvoice(ctx, s.database, *request.InvoiceNo)
			if loadErr != nil {
				return loadErr
			}
		}
		if request.CreditStatementNo != "" {
			statement, loadErr = loadCreditStatement(
				ctx, s.database, request.CreditStatementNo,
			)
			if loadErr != nil {
				return loadErr
			}
			if statement == nil {
				return bizerr.NewCode(CodeCreditNotFound)
			}
			if _, loadErr = loadCreditAccountByNo(
				ctx, s.database, statement.CreditAccountNo, true,
			); loadErr != nil {
				return loadErr
			}
			statement, loadErr = loadCreditStatementForUpdate(
				ctx, s.database, request.CreditStatementNo,
			)
			if loadErr != nil {
				return loadErr
			}
		}
		result, loadErr = s.postFinancialAdjustment(
			ctx, request, invoice, statement, false,
		)
		return loadErr
	})
	return result, normalizeInvoiceError(err)
}

func resolveAdjustmentTarget(
	ctx context.Context,
	database gdb.DB,
	command *adjustmentCommand,
) (*entity.BillingInvoice, *creditStatementRow, error) {
	if command.InvoiceNo == "" && command.CreditStatementNo == "" {
		accountType := command.AccountType
		if accountType == "" {
			accountType = AccountTypeCustomerBalance
		}
		command.AccountNo = deterministicAccountNo(
			"customer", command.CustomerNo, accountType, command.CurrencyCode,
		)
		return nil, nil, nil
	}
	if command.CreditStatementNo != "" {
		statement, err := loadCreditStatement(
			ctx, database, command.CreditStatementNo,
		)
		if err != nil {
			return nil, nil, err
		}
		if statement == nil {
			return nil, nil, bizerr.NewCode(CodeCreditNotFound)
		}
		if _, err = loadCreditAccountByNo(
			ctx, database, statement.CreditAccountNo, true,
		); err != nil {
			return nil, nil, err
		}
		statement, err = loadCreditStatementForUpdate(
			ctx, database, command.CreditStatementNo,
		)
		if err != nil {
			return nil, nil, err
		}
		command.CustomerNo = statement.CustomerNo
		command.CurrencyCode = statement.CurrencyCode
		return nil, statement, nil
	}
	invoice, err := lockInvoice(ctx, database, command.InvoiceNo)
	if err != nil {
		return nil, nil, err
	}
	if invoice == nil {
		return nil, nil, bizerr.NewCode(CodeInvoiceNotFound)
	}
	command.CustomerNo = invoice.CustomerNo
	command.CurrencyCode = invoice.CurrencyCode
	return invoice, nil, nil
}

func validateNewAdjustmentTarget(
	ctx context.Context,
	database gdb.DB,
	invoice *entity.BillingInvoice,
	statement *creditStatementRow,
	command adjustmentCommand,
) error {
	if statement != nil {
		if statement.StatementNo != command.CreditStatementNo ||
			(statement.StatusCode != "open" && statement.StatusCode != "partially_paid" &&
				statement.StatusCode != "overdue") || statement.WrittenOffMinor != 0 ||
			command.AmountMinor > statement.OutstandingMinor {
			return bizerr.NewCode(CodeCreditConflict)
		}
		return nil
	}
	if invoice == nil {
		return nil
	}
	aggregate, err := loadInvoiceAggregate(ctx, database, invoice.InvoiceNo)
	if err != nil {
		return err
	}
	if err = validateInvoiceAggregate(invoice, aggregate); err != nil {
		return err
	}
	aggregateOutstanding := invoice.TotalAmountMinor - aggregate.PaymentMinor - aggregate.WaiverMinor
	if !invoiceAcceptsPayment(invoice) || aggregateOutstanding <= 0 ||
		command.AmountMinor > aggregateOutstanding {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func loadCurrentAdjustmentPolicy(
	ctx context.Context,
	database gdb.DB,
	currency string,
) (*entity.BillingAdjustmentPolicy, error) {
	var policy *entity.BillingAdjustmentPolicy
	columns := dao.BillingAdjustmentPolicy.Columns()
	if err := database.Model(dao.BillingAdjustmentPolicy.Table()).Ctx(ctx).
		Fields(
			columns.PolicyNo, columns.CurrencyCode, columns.ReviewThresholdMinor,
			columns.Version, columns.ConfiguredBy, columns.ContentDigest,
		).
		Where(columns.TenantId, 0).Where(columns.CurrencyCode, currency).
		OrderDesc(columns.Version).Limit(1).Scan(&policy); err != nil {
		return nil, gerror.Wrap(err, "load current billing adjustment policy")
	}
	return policy, nil
}

func projectAdjustmentPolicy(row *entity.BillingAdjustmentPolicy, reused bool) *AdjustmentPolicy {
	return &AdjustmentPolicy{
		PolicyNo: row.PolicyNo, CurrencyCode: row.CurrencyCode,
		ReviewThresholdMinor: row.ReviewThresholdMinor, Version: row.Version,
		ConfiguredBy: row.ConfiguredBy, Reused: reused,
	}
}

func (s *serviceImpl) insertAdjustmentRequest(
	ctx context.Context,
	command adjustmentCommand,
	policy adjustmentPolicySnapshot,
	requestDigest string,
) (*entity.BillingAdjustmentRequest, error) {
	adjustmentNo, err := newBillingNumber(s.random, "bar_")
	if err != nil {
		return nil, err
	}
	now, err := databaseTime(ctx, s.database)
	if err != nil {
		return nil, err
	}
	status := "approved"
	if policy.ApprovalRequired {
		status = string(AdjustmentStatusPendingReview)
	}
	var invoiceNo *string
	var creditStatementNo any
	var accountNo *string
	var accountType *string
	if command.InvoiceNo != "" {
		invoiceNo = &command.InvoiceNo
	} else if command.CreditStatementNo != "" {
		creditStatementNo = command.CreditStatementNo
	} else {
		accountNo = &command.AccountNo
		accountTypeValue := command.AccountType
		if accountTypeValue == "" {
			accountTypeValue = AccountTypeCustomerBalance
		}
		accountType = &accountTypeValue
	}
	var policyNo *string
	var policyVersion any
	if policy.PolicyNo != "" {
		policyNo = &policy.PolicyNo
		policyVersion = policy.Version
	}
	row := do.BillingAdjustmentRequest{
		TenantId: 0, AdjustmentNo: adjustmentNo, CustomerNo: command.CustomerNo,
		InvoiceNo: invoiceNo, CreditStatementNo: creditStatementNo,
		AccountNo: accountNo, AccountTypeCode: accountType,
		CurrencyCode: command.CurrencyCode, AdjustmentTypeCode: command.AdjustmentType,
		AmountMinor: command.AmountMinor, ReasonCode: command.ReasonCode,
		EvidenceDigest: command.EvidenceDigest, IdempotencyDigest: command.IdempotencyDigest,
		RequestDigest: requestDigest, ApprovalRequired: policy.ApprovalRequired,
		StatusCode: status, RequestedBy: command.RequestedBy, Version: 1,
		CreatedAt: &now, UpdatedAt: &now, ProofReference: nullableAdjustmentValue(command.ProofReference),
		ReceivingAccountReference: nullableAdjustmentValue(command.ReceivingAccountReference),
		BusinessPurposeCode:       nullableAdjustmentValue(command.BusinessPurposeCode),
		PolicyNo:                  policyNo, PolicyVersion: policyVersion,
	}
	if _, err = s.database.Model(dao.BillingAdjustmentRequest.Table()).Ctx(ctx).Data(row).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert billing adjustment request")
	}
	return &entity.BillingAdjustmentRequest{
		AdjustmentNo: adjustmentNo, CustomerNo: command.CustomerNo, InvoiceNo: invoiceNo,
		CreditStatementNo: command.CreditStatementNo,
		AccountNo:         accountNo, AccountTypeCode: accountType, CurrencyCode: command.CurrencyCode,
		AdjustmentTypeCode: command.AdjustmentType, AmountMinor: command.AmountMinor,
		ReasonCode: command.ReasonCode, EvidenceDigest: command.EvidenceDigest,
		IdempotencyDigest: command.IdempotencyDigest, RequestDigest: requestDigest,
		ApprovalRequired: policy.ApprovalRequired, StatusCode: status,
		RequestedBy: command.RequestedBy, Version: 1, CreatedAt: &now, UpdatedAt: &now,
		ProofReference:            command.ProofReference,
		ReceivingAccountReference: command.ReceivingAccountReference,
		BusinessPurposeCode:       command.BusinessPurposeCode, PolicyNo: policy.PolicyNo,
		PolicyVersion: policy.Version,
	}, nil
}

func customerAdjustmentAccountSpec(command adjustmentCommand) accountSpec {
	if command.AccountType == AccountTypeCustomerGift {
		return accountSpec{
			OwnerType: "customer", OwnerReference: command.CustomerNo,
			CustomerNo: command.CustomerNo, AccountType: AccountTypeCustomerGift,
			NormalBalance: "credit",
		}
	}
	return customerBalanceSpec(command.CustomerNo)
}

func nullableAdjustmentValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func loadAdjustmentReplay(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	idempotencyDigest string,
) (*entity.BillingAdjustmentRequest, error) {
	var request *entity.BillingAdjustmentRequest
	columns := dao.BillingAdjustmentRequest.Columns()
	if err := database.Model(dao.BillingAdjustmentRequest.Table()).Ctx(ctx).
		Where(columns.TenantId, 0).Where(columns.CustomerNo, customerNo).
		Where(columns.IdempotencyDigest, idempotencyDigest).LockUpdate().Scan(&request); err != nil {
		return nil, gerror.Wrap(err, "load billing adjustment replay")
	}
	return request, nil
}

func lockAdjustmentRequest(
	ctx context.Context,
	database gdb.DB,
	adjustmentNo string,
) (*entity.BillingAdjustmentRequest, error) {
	var request *entity.BillingAdjustmentRequest
	columns := dao.BillingAdjustmentRequest.Columns()
	if err := database.Model(dao.BillingAdjustmentRequest.Table()).Ctx(ctx).
		Where(columns.TenantId, 0).Where(columns.AdjustmentNo, adjustmentNo).
		LockUpdate().Scan(&request); err != nil {
		return nil, gerror.Wrap(err, "lock billing adjustment request")
	}
	return request, nil
}

func adjustmentRequestMatches(
	request *entity.BillingAdjustmentRequest,
	command adjustmentCommand,
	requestDigest string,
) bool {
	return request != nil && request.AdjustmentTypeCode == command.AdjustmentType &&
		nullableString(request.InvoiceNo) == command.InvoiceNo &&
		request.CreditStatementNo == command.CreditStatementNo &&
		request.CustomerNo == command.CustomerNo && request.CurrencyCode == command.CurrencyCode &&
		nullableString(request.AccountNo) == command.AccountNo &&
		request.AmountMinor == command.AmountMinor && request.ReasonCode == command.ReasonCode &&
		request.ProofReference == command.ProofReference &&
		request.ReceivingAccountReference == command.ReceivingAccountReference &&
		request.BusinessPurposeCode == command.BusinessPurposeCode &&
		request.EvidenceDigest == command.EvidenceDigest && request.RequestDigest == requestDigest &&
		request.RequestedBy == command.RequestedBy
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *serviceImpl) insertAdjustmentApproval(
	ctx context.Context,
	request *entity.BillingAdjustmentRequest,
	input AdjustmentReviewInput,
) error {
	approvalNo, err := newBillingNumber(s.random, "baa_")
	if err != nil {
		return err
	}
	now, err := databaseTime(ctx, s.database)
	if err != nil {
		return err
	}
	row := do.BillingAdjustmentApproval{
		TenantId: 0, ApprovalNo: approvalNo, AdjustmentNo: request.AdjustmentNo,
		RequesterIdSnapshot: request.RequestedBy, ReviewerId: input.ReviewerID,
		DecisionCode: input.Decision, CommentDigest: input.CommentDigest,
		EvidenceDigest: input.EvidenceDigest, DecidedAt: &now, CreatedAt: &now,
	}
	if _, err = s.database.Model(dao.BillingAdjustmentApproval.Table()).Ctx(ctx).Data(row).Insert(); err != nil {
		return gerror.Wrap(err, "insert billing adjustment approval")
	}
	return nil
}

func loadAdjustmentApproval(
	ctx context.Context,
	database gdb.DB,
	adjustmentNo string,
) (*entity.BillingAdjustmentApproval, error) {
	var approval *entity.BillingAdjustmentApproval
	columns := dao.BillingAdjustmentApproval.Columns()
	if err := database.Model(dao.BillingAdjustmentApproval.Table()).Ctx(ctx).
		Where(columns.TenantId, 0).Where(columns.AdjustmentNo, adjustmentNo).
		Scan(&approval); err != nil {
		return nil, gerror.Wrap(err, "load billing adjustment approval")
	}
	return approval, nil
}

func updateAdjustmentStatus(
	ctx context.Context,
	database gdb.DB,
	request *entity.BillingAdjustmentRequest,
	status string,
	transactionNo string,
	transactionType string,
	postedAt *time.Time,
) error {
	now, err := databaseTime(ctx, database)
	if err != nil {
		return err
	}
	data := map[string]any{
		"status_code": status, "version": request.Version + 1,
		"updated_at": now,
	}
	if transactionNo != "" {
		data["ledger_transaction_no"] = transactionNo
		data["ledger_transaction_type_code"] = transactionType
		data["posted_at"] = postedAt
	}
	columns := dao.BillingAdjustmentRequest.Columns()
	updated, err := database.Model(dao.BillingAdjustmentRequest.Table()).Ctx(ctx).Data(data).
		Where(columns.TenantId, 0).Where(columns.AdjustmentNo, request.AdjustmentNo).
		Where(columns.Version, request.Version).Update()
	if err != nil {
		return gerror.Wrap(err, "update billing adjustment status")
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read billing adjustment status update")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func projectAdjustmentRequest(
	request *entity.BillingAdjustmentRequest,
	reused bool,
) *FinancialAdjustment {
	return &FinancialAdjustment{
		AdjustmentNo: request.AdjustmentNo, AdjustmentType: request.AdjustmentTypeCode,
		Status: FinancialAdjustmentStatus(request.StatusCode), CustomerNo: request.CustomerNo,
		InvoiceNo: nullableString(request.InvoiceNo), CurrencyCode: request.CurrencyCode,
		CreditStatementNo: request.CreditStatementNo,
		AmountMinor:       request.AmountMinor, ApprovalRequired: request.ApprovalRequired,
		RequestedBy:         request.RequestedBy,
		LedgerTransactionNo: nullableString(request.LedgerTransactionNo), Reused: reused,
	}
}
