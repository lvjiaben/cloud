package renewal

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type paymentPreflightRow struct {
	TrialConversion            bool
	OperationNo                string     `orm:"operation_no"`
	QuoteNo                    string     `orm:"quote_no"`
	CustomerNo                 string     `orm:"customer_no"`
	PayerCustomerNo            string     `orm:"payer_customer_no"`
	EntitlementNo              string     `orm:"entitlement_no"`
	ProductNo                  string     `orm:"product_no"`
	CurrencyCode               string     `orm:"currency_code"`
	InvoiceNo                  string     `orm:"invoice_no"`
	PriceSnapshotNo            string     `orm:"price_snapshot_no"`
	StatusCode                 string     `orm:"status_code"`
	ExecutionModeCode          string     `orm:"execution_mode_code"`
	CapabilityEvidenceNo       string     `orm:"capability_evidence_no"`
	CapabilityDigest           string     `orm:"capability_evidence_digest"`
	ContentDigest              string     `orm:"content_digest"`
	ExpectedEntitlementVersion int64      `orm:"expected_entitlement_version"`
	EntitlementVersion         int64      `orm:"entitlement_version"`
	EntitlementRenewalMode     string     `orm:"entitlement_renewal_mode"`
	EntitlementStatus          string     `orm:"entitlement_status"`
	EntitlementContractNo      *string    `orm:"entitlement_contract_no"`
	EntitlementContractDigest  *string    `orm:"entitlement_contract_digest"`
	EntitlementContractExpires *time.Time `orm:"entitlement_contract_expires_at"`
	InvoiceStatus              string     `orm:"invoice_status"`
	InvoiceTotalMinor          int64      `orm:"invoice_total_minor"`
	InvoicePaidMinor           int64      `orm:"invoice_paid_minor"`
	InvoiceCreditMinor         int64      `orm:"invoice_credit_minor"`
	PaymentDueAt               time.Time  `orm:"payment_due_at"`
}

type convergenceRow struct {
	TrialConversion            bool
	OperationNo                string     `orm:"operation_no"`
	QuoteNo                    string     `orm:"quote_no"`
	CustomerNo                 string     `orm:"customer_no"`
	PayerCustomerNo            string     `orm:"payer_customer_no"`
	EntitlementNo              string     `orm:"entitlement_no"`
	CurrencyCode               string     `orm:"currency_code"`
	InvoiceNo                  string     `orm:"invoice_no"`
	PriceSnapshotNo            string     `orm:"price_snapshot_no"`
	StatusCode                 string     `orm:"status_code"`
	ExecutionModeCode          string     `orm:"execution_mode_code"`
	CapabilityEvidenceNo       string     `orm:"capability_evidence_no"`
	CapabilityDigest           string     `orm:"capability_evidence_digest"`
	ContentDigest              string     `orm:"content_digest"`
	ExpectedEntitlementVersion int64      `orm:"expected_entitlement_version"`
	EntitlementVersion         int64      `orm:"entitlement_version"`
	EntitlementRenewalMode     string     `orm:"entitlement_renewal_mode"`
	EntitlementStatus          string     `orm:"entitlement_status"`
	EntitlementContractNo      *string    `orm:"entitlement_contract_no"`
	EntitlementContractDigest  *string    `orm:"entitlement_contract_digest"`
	EntitlementContractExpires *time.Time `orm:"entitlement_contract_expires_at"`
	InvoiceStatus              string     `orm:"invoice_status"`
	InvoiceTotalMinor          int64      `orm:"invoice_total_minor"`
	InvoicePaidMinor           int64      `orm:"invoice_paid_minor"`
	InvoiceCreditMinor         int64      `orm:"invoice_credit_minor"`
	PaymentDueAt               time.Time  `orm:"payment_due_at"`
	ProjectedExpiresAt         time.Time  `orm:"projected_expires_at"`
	CurrentExpiresAt           *time.Time `orm:"current_expires_at"`
	OperationVersion           int64      `orm:"operation_version"`
	MaximumSequence            int        `orm:"maximum_sequence"`
}

// PayCurrentCustomer derives every financial dimension from one owned
// operation, rejects an expired intent before debit, and safely converges a
// successful billing replay after process interruption.
func (s *serviceImpl) PayCurrentCustomer(
	ctx context.Context,
	input PayInput,
) (*Settlement, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || !validReference(audience.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.payOwned(
		ctx, audience.CustomerNo, audience.CustomerNo, input, "customer", audience.CustomerNo,
	)
}

func (s *serviceImpl) PayForChannel(
	ctx context.Context,
	serviceOwnerCustomerNo string,
	payerCustomerNo string,
	input PayInput,
	settlementMethod string,
	channelID string,
) (*Settlement, error) {
	if !validReference(serviceOwnerCustomerNo, 40) || !validReference(payerCustomerNo, 40) ||
		!validReference(channelID, 64) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	switch settlementMethod {
	case "balance":
		return s.payOwned(ctx, serviceOwnerCustomerNo, payerCustomerNo, input, "system", channelID)
	case "credit":
		return s.payOwnedFromCredit(ctx, serviceOwnerCustomerNo, payerCustomerNo, input, channelID)
	default:
		return nil, bizerr.NewCode(CodeInvalid)
	}
}

func (s *serviceImpl) payOwned(
	ctx context.Context,
	customerNo string,
	payerCustomerNo string,
	input PayInput,
	actorType string,
	actorReference string,
) (*Settlement, error) {
	if !validReference(customerNo, 40) || !validReference(payerCustomerNo, 40) ||
		(actorType != "customer" && actorType != "system") ||
		!validReference(actorReference, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if input.OperationNo != strings.TrimSpace(input.OperationNo) ||
		!validReference(input.OperationNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var settled *Settlement
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Keep the operation, service, and invoice locked from the last eligibility
		// check through balance posting and service convergence. This makes a paid
		// renewal and an expiry suspend/terminate task serialize on the same service
		// row, so neither flow can commit half of its state and then lose a version
		// race to the other.
		preflight, loadErr := loadPaymentPreflight(
			ctx, s.database, customerNo, payerCustomerNo, input.OperationNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if preflight == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if validateErr := validatePaymentPreflight(ctx, s.database, preflight); validateErr != nil {
			return validateErr
		}
		if preflight.InvoiceStatus == string(billing.InvoiceStatusPendingPayment) {
			if err := s.requireConversionQuote(ctx, preflight.QuoteNo); err != nil {
				return err
			}
			if err := s.requireConversionQuote(ctx, preflight.QuoteNo); err != nil {
				return err
			}
			if err := s.catalog.RequireRenewalSale(ctx, preflight.ProductNo); err != nil {
				return bizerr.NewCode(CodePriceUnavailable)
			}
		}
		payment, paymentErr := s.invoices.PayFromBalance(ctx, billing.InvoiceBalancePaymentInput{
			InvoiceNo: preflight.InvoiceNo, SourceReference: renewalPaymentSource(input.OperationNo),
			SourceSnapshotDigest: preflight.ContentDigest,
		})
		if paymentErr != nil {
			if bizerr.Is(paymentErr, billing.CodeInsufficientBalance) {
				return bizerr.NewCode(CodeBalanceInsufficient)
			}
			return paymentErr
		}
		if payment == nil || payment.Posting == nil || payment.Invoice == nil ||
			payment.Invoice.InvoiceNo != preflight.InvoiceNo ||
			payment.Invoice.CustomerNo != payerCustomerNo ||
			payment.Invoice.CurrencyCode != preflight.CurrencyCode ||
			payment.Invoice.Status != billing.InvoiceStatusPaid ||
			payment.Posting.TransactionType != "balance_payment" ||
			payment.Posting.CurrencyCode != preflight.CurrencyCode ||
			!validReference(payment.Posting.TransactionNo, 40) {
			return bizerr.NewCode(CodeConflict)
		}
		operation, operationReused, convergeErr := s.convergePaidOperation(
			ctx, customerNo, payerCustomerNo, input.OperationNo, payment.Posting.TransactionNo,
			actorType, actorReference,
		)
		if convergeErr != nil {
			return convergeErr
		}
		settled = &Settlement{
			balancePayment: payment,
			Operation:      operation, LedgerTransactionNo: payment.Posting.TransactionNo,
			Reused: payment.Reused || operationReused,
		}
		return nil
	})
	if err != nil {
		return nil, normalizeError(err)
	}
	return settled, nil
}

func (s *serviceImpl) payOwnedFromCredit(
	ctx context.Context,
	customerNo string,
	payerCustomerNo string,
	input PayInput,
	channelID string,
) (*Settlement, error) {
	if input.OperationNo != strings.TrimSpace(input.OperationNo) ||
		!validReference(input.OperationNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var settled *Settlement
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		preflight, loadErr := loadPaymentPreflight(
			ctx, s.database, customerNo, payerCustomerNo, input.OperationNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if preflight == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if preflight.StatusCode == "ready" || preflight.StatusCode == "succeeded" {
			if preflight.InvoiceStatus != string(billing.InvoiceStatusCreditSettled) ||
				preflight.InvoiceCreditMinor != preflight.InvoiceTotalMinor {
				return bizerr.NewCode(CodeConflict)
			}
			operation, reused, convergeErr := s.projectSettledOperation(
				ctx, customerNo, payerCustomerNo, input.OperationNo,
			)
			if convergeErr != nil {
				return convergeErr
			}
			settled = &Settlement{Operation: operation, Reused: reused}
			return nil
		}
		if validateErr := validatePaymentPreflight(ctx, s.database, preflight); validateErr != nil {
			return validateErr
		}
		if err := s.requireConversionQuote(ctx, preflight.QuoteNo); err != nil {
			return err
		}
		if err := s.catalog.RequireRenewalSale(ctx, preflight.ProductNo); err != nil {
			return bizerr.NewCode(CodePriceUnavailable)
		}
		snapshotDigest, digestErr := contentDigest(struct {
			Kind, ChannelID, CustomerNo, PayerCustomerNo, OperationNo, InvoiceNo, CurrencyCode string
			AmountMinor                                                                        int64
		}{"channel_renewal_credit", channelID, customerNo, payerCustomerNo,
			preflight.OperationNo, preflight.InvoiceNo, preflight.CurrencyCode,
			preflight.InvoiceTotalMinor})
		if digestErr != nil {
			return digestErr
		}
		usage, usageErr := billing.NewCreditUsageService(s.database)
		if usageErr != nil {
			return usageErr
		}
		exposure, usageErr := usage.ReserveCredit(ctx, billing.CreditReserveInput{
			CustomerNo: payerCustomerNo, CurrencyCode: preflight.CurrencyCode,
			AmountMinor: preflight.InvoiceTotalMinor, SourceType: "renewal",
			SourceReference: preflight.OperationNo, InvoiceNo: preflight.InvoiceNo,
			SourceSnapshotDigest: snapshotDigest, ActorType: "system", ActorID: 0,
		})
		if usageErr != nil {
			return usageErr
		}
		if exposure == nil || exposure.CustomerNo != payerCustomerNo ||
			exposure.CurrencyCode != preflight.CurrencyCode ||
			exposure.OriginalPrincipalMinor != preflight.InvoiceTotalMinor {
			return bizerr.NewCode(CodeConflict)
		}
		now, nowErr := renewalDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_billing_invoice
SET status_code='credit_settled',credit_amount_minor=total_amount_minor,
    settled_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND invoice_no=? AND customer_no=? AND currency_code=?
  AND status_code='pending_payment' AND paid_amount_minor=0
  AND credit_amount_minor=0 AND waived_amount_minor=0`, now, preflight.InvoiceNo,
			payerCustomerNo, preflight.CurrencyCode)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "settle channel renewal invoice from credit")
		}
		affected, affectedErr := updated.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		operation, reused, convergeErr := s.convergeCreditOperation(
			ctx, customerNo, payerCustomerNo, input.OperationNo,
			exposure.ExposureNo, channelID,
		)
		if convergeErr != nil {
			return convergeErr
		}
		settled = &Settlement{Operation: operation, CreditExposureNo: exposure.ExposureNo,
			Reused: exposure.Reused || reused}
		return nil
	})
	if err != nil {
		return nil, normalizeError(err)
	}
	return settled, nil
}

func renewalPaymentSource(operationNo string) string {
	return "renewal.operation." + operationNo
}

func loadPaymentPreflight(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	payerCustomerNo string,
	operationNo string,
) (*paymentPreflightRow, error) {
	var row *paymentPreflightRow
	query := `
SELECT operation.operation_no, operation.quote_no, operation.customer_no, operation.payer_customer_no,
       operation.subject_no AS entitlement_no, service.product_no,
       operation.currency_code, operation.invoice_no, operation.price_snapshot_no,
       operation.status_code, operation.execution_mode_code,
       operation.capability_evidence_no, operation.capability_evidence_digest,
       operation.content_digest, operation.expected_subject_version AS expected_entitlement_version,
       service.charge_type_code='trial' AS trial_conversion,
 service.version AS entitlement_version,
       service.renewal_mode_code AS entitlement_renewal_mode,
	   service.status_code AS entitlement_status,
       CASE WHEN operation.resource_owner_code = service.entitlement_owner_id
              AND operation.channel_no = service.plugin_release_version
              AND operation.pool_no = service.contract_version
            THEN COALESCE(NULLIF(service.renewal_method, ''), service.plugin_id)
            ELSE NULL END AS entitlement_contract_no,
       service.declaration_digest AS entitlement_contract_digest,
       invoice.payment_due_at AS entitlement_contract_expires_at,
       invoice.status_code AS invoice_status,
       invoice.total_amount_minor AS invoice_total_minor,
       invoice.paid_amount_minor AS invoice_paid_minor,
       invoice.credit_amount_minor AS invoice_credit_minor,
       invoice.payment_due_at
FROM plugin_idc_core_business_operation operation
INNER JOIN plugin_idc_core_service_entitlement service
    ON service.tenant_id = operation.tenant_id
   AND service.entitlement_no = operation.subject_no
   AND service.customer_no = operation.customer_no
INNER JOIN plugin_idc_core_billing_invoice invoice
    ON invoice.tenant_id = operation.tenant_id
   AND invoice.invoice_no = operation.invoice_no
   AND invoice.customer_no = operation.payer_customer_no
   AND invoice.currency_code = operation.currency_code
WHERE operation.tenant_id = 0 AND operation.operation_type_code = 'renewal'
	  AND operation.operation_no = ? AND operation.customer_no = ?
	  AND operation.payer_customer_no = ?
FOR UPDATE OF operation, service, invoice`
	if err := database.GetScan(ctx, &row, query, operationNo, customerNo, payerCustomerNo); err != nil {
		return nil, gerror.Wrap(err, "load renewal payment preflight")
	}
	return row, nil
}

func validatePaymentPreflight(
	ctx context.Context,
	database gdb.DB,
	row *paymentPreflightRow,
) error {
	if row == nil || row.InvoiceTotalMinor <= 0 ||
		(row.ExecutionModeCode != "local_entitlement" && row.ExecutionModeCode != "provider_confirmed") ||
		row.EntitlementRenewalMode != row.ExecutionModeCode ||
		(row.EntitlementStatus != "active" && row.EntitlementStatus != "expired" && !(row.EntitlementStatus == "suspended" && (row.TrialConversion || row.StatusCode == "succeeded"))) ||
		row.EntitlementContractNo == nil || *row.EntitlementContractNo != row.CapabilityEvidenceNo ||
		row.EntitlementContractDigest == nil || *row.EntitlementContractDigest != row.CapabilityDigest {
		return bizerr.NewCode(CodeConflict)
	}
	if row.StatusCode == "ready" || row.StatusCode == "succeeded" {
		if !settledInvoiceProjection(row.InvoiceStatus, row.InvoiceTotalMinor,
			row.InvoicePaidMinor, row.InvoiceCreditMinor) {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	}
	if row.StatusCode != "awaiting_financial_settlement" ||
		row.EntitlementVersion != row.ExpectedEntitlementVersion ||
		row.EntitlementContractExpires == nil {
		return bizerr.NewCode(CodeConflict)
	}
	if row.InvoiceStatus == string(billing.InvoiceStatusPaid) &&
		row.InvoicePaidMinor == row.InvoiceTotalMinor {
		return nil
	}
	if row.InvoiceStatus != string(billing.InvoiceStatusPendingPayment) || row.InvoicePaidMinor != 0 {
		return bizerr.NewCode(CodeConflict)
	}
	now, err := renewalDatabaseNow(ctx, database)
	if err != nil {
		return err
	}
	if !row.PaymentDueAt.After(now) || !row.EntitlementContractExpires.After(now) {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func settledInvoiceProjection(status string, total, paid, credit int64) bool {
	return (status == string(billing.InvoiceStatusPaid) && paid == total && credit == 0) ||
		(status == string(billing.InvoiceStatusCreditSettled) && credit == total && paid == 0)
}

func (s *serviceImpl) convergePaidOperation(
	ctx context.Context,
	customerNo string,
	payerCustomerNo string,
	operationNo string,
	ledgerTransactionNo string,
	actorType string,
	actorReference string,
) (*Operation, bool, error) {
	var result *Operation
	reused := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := lockConvergenceRow(ctx, s.database, customerNo, payerCustomerNo, operationNo, ledgerTransactionNo)
		if err != nil {
			return err
		}
		if row == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if row.InvoiceStatus != string(billing.InvoiceStatusPaid) ||
			row.InvoicePaidMinor != row.InvoiceTotalMinor || row.InvoiceCreditMinor != 0 || row.InvoiceTotalMinor <= 0 {
			return bizerr.NewCode(CodeConflict)
		}
		if row.StatusCode == "ready" || row.StatusCode == "succeeded" {
			result = projectConvergedOperation(row, true)
			reused = true
			return nil
		}
		if row.StatusCode != "awaiting_financial_settlement" ||
			row.EntitlementVersion != row.ExpectedEntitlementVersion ||
			row.EntitlementRenewalMode != row.ExecutionModeCode ||
			(row.EntitlementStatus != "active" && row.EntitlementStatus != "expired" && !(row.EntitlementStatus == "suspended" && (row.TrialConversion || row.StatusCode == "succeeded"))) ||
			row.EntitlementContractNo == nil || *row.EntitlementContractNo != row.CapabilityEvidenceNo ||
			row.EntitlementContractDigest == nil || *row.EntitlementContractDigest != row.CapabilityDigest ||
			row.CurrentExpiresAt == nil || !row.ProjectedExpiresAt.After(*row.CurrentExpiresAt) {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := renewalDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		finalStatus := "ready"
		if row.ExecutionModeCode == "local_entitlement" {
			finalStatus = "succeeded"
		}
		if err = s.updateConvergedOperation(
			ctx, row, finalStatus, now, ledgerTransactionNo, actorType, actorReference,
			"renewal.balance_settled",
		); err != nil {
			return err
		}
		if row.ExecutionModeCode == "local_entitlement" {
			updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_service_entitlement
SET status_code = 'active', expires_at = ?, current_price_snapshot_no = ?,
    last_renewal_operation_no = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND entitlement_no = ? AND customer_no = ? AND version = ?
  AND expires_at < ?`, row.ProjectedExpiresAt, row.PriceSnapshotNo, row.OperationNo,
				row.EntitlementNo, row.CustomerNo, row.ExpectedEntitlementVersion,
				row.ProjectedExpiresAt)
			if updateErr != nil {
				return gerror.Wrap(updateErr, "advance local renewal entitlement")
			}
			affected, affectedErr := updated.RowsAffected()
			if affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		if row.TrialConversion && row.ExecutionModeCode == "local_entitlement" {
			if err := s.finishTrialConversion(ctx, row.EntitlementNo, row.CustomerNo, row.OperationNo, row.EntitlementStatus); err != nil {
				return err
			}
		}
		row.StatusCode = finalStatus
		result = projectConvergedOperation(row, false)
		return nil
	})
	return result, reused, err
}

func (s *serviceImpl) projectSettledOperation(
	ctx context.Context,
	customerNo string,
	payerCustomerNo string,
	operationNo string,
) (*Operation, bool, error) {
	row, err := loadPaymentPreflight(ctx, s.database, customerNo, payerCustomerNo, operationNo)
	if err != nil || row == nil {
		return nil, false, err
	}
	return &Operation{
		OperationNo: row.OperationNo, QuoteNo: row.QuoteNo, InvoiceNo: row.InvoiceNo,
		EntitlementNo: row.EntitlementNo, CurrencyCode: row.CurrencyCode,
		Status: row.StatusCode, RenewalMode: row.ExecutionModeCode,
		TotalAmountMinor: row.InvoiceTotalMinor, PaymentDueAt: row.PaymentDueAt.UTC(),
		Reused: true,
	}, true, nil
}

func (s *serviceImpl) convergeCreditOperation(
	ctx context.Context,
	customerNo string,
	payerCustomerNo string,
	operationNo string,
	exposureNo string,
	channelID string,
) (*Operation, bool, error) {
	var result *Operation
	reused := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := lockCreditConvergenceRow(
			ctx, s.database, customerNo, payerCustomerNo, operationNo, exposureNo,
		)
		if err != nil {
			return err
		}
		if row == nil || row.InvoiceTotalMinor <= 0 ||
			row.InvoiceStatus != string(billing.InvoiceStatusCreditSettled) ||
			row.InvoiceCreditMinor != row.InvoiceTotalMinor || row.InvoicePaidMinor != 0 {
			return bizerr.NewCode(CodeConflict)
		}
		if row.StatusCode == "ready" || row.StatusCode == "succeeded" {
			result = projectConvergedOperation(row, true)
			reused = true
			return nil
		}
		if row.StatusCode != "awaiting_financial_settlement" ||
			row.EntitlementVersion != row.ExpectedEntitlementVersion ||
			row.EntitlementRenewalMode != row.ExecutionModeCode ||
			(row.EntitlementStatus != "active" && row.EntitlementStatus != "expired" && !(row.EntitlementStatus == "suspended" && (row.TrialConversion || row.StatusCode == "succeeded"))) ||
			row.EntitlementContractNo == nil || *row.EntitlementContractNo != row.CapabilityEvidenceNo ||
			row.EntitlementContractDigest == nil || *row.EntitlementContractDigest != row.CapabilityDigest ||
			row.CurrentExpiresAt == nil || !row.ProjectedExpiresAt.After(*row.CurrentExpiresAt) {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := renewalDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		finalStatus := "ready"
		if row.ExecutionModeCode == "local_entitlement" {
			finalStatus = "succeeded"
		}
		if err = s.updateConvergedOperation(
			ctx, row, finalStatus, now, "credit:"+exposureNo,
			"system", channelID, "renewal.credit_settled",
		); err != nil {
			return err
		}
		if row.ExecutionModeCode == "local_entitlement" {
			updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_service_entitlement
SET status_code='active',expires_at=?,current_price_snapshot_no=?,
    last_renewal_operation_no=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND version=?
  AND expires_at<?`, row.ProjectedExpiresAt, row.PriceSnapshotNo, row.OperationNo,
				row.EntitlementNo, row.CustomerNo, row.ExpectedEntitlementVersion,
				row.ProjectedExpiresAt)
			if updateErr != nil {
				return gerror.Wrap(updateErr, "advance credit renewal entitlement")
			}
			affected, affectedErr := updated.RowsAffected()
			if affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		if row.TrialConversion && row.ExecutionModeCode == "local_entitlement" {
			if err := s.finishTrialConversion(ctx, row.EntitlementNo, row.CustomerNo, row.OperationNo, row.EntitlementStatus); err != nil {
				return err
			}
		}
		row.StatusCode = finalStatus
		result = projectConvergedOperation(row, false)
		return nil
	})
	return result, reused, err
}

func lockCreditConvergenceRow(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	payerCustomerNo string,
	operationNo string,
	exposureNo string,
) (*convergenceRow, error) {
	var row *convergenceRow
	if err := database.GetScan(ctx, &row, `
SELECT operation.operation_no,operation.quote_no,operation.customer_no,
       operation.payer_customer_no,operation.subject_no AS entitlement_no,
       operation.currency_code,operation.invoice_no,operation.price_snapshot_no,
       operation.status_code,operation.execution_mode_code,
       operation.capability_evidence_no,operation.capability_evidence_digest,
       operation.content_digest,operation.expected_subject_version AS expected_entitlement_version,
       operation.version AS operation_version,service.charge_type_code='trial' AS trial_conversion,
 service.version AS entitlement_version,
       service.renewal_mode_code AS entitlement_renewal_mode,
       service.status_code AS entitlement_status,
       CASE WHEN operation.resource_owner_code = service.entitlement_owner_id
              AND operation.channel_no = service.plugin_release_version
              AND operation.pool_no = service.contract_version
            THEN COALESCE(NULLIF(service.renewal_method, ''), service.plugin_id)
            ELSE NULL END AS entitlement_contract_no,
       service.declaration_digest AS entitlement_contract_digest,
       invoice.payment_due_at AS entitlement_contract_expires_at,
       service.expires_at AS current_expires_at,invoice.status_code AS invoice_status,
       invoice.total_amount_minor AS invoice_total_minor,
       invoice.paid_amount_minor AS invoice_paid_minor,
       invoice.credit_amount_minor AS invoice_credit_minor,invoice.payment_due_at,
       price.projected_expires_at,
       COALESCE((SELECT MAX(event_sequence)
                 FROM plugin_idc_core_business_operation_event event
                 WHERE event.operation_no=operation.operation_no),0)::INT AS maximum_sequence
FROM plugin_idc_core_business_operation operation
JOIN plugin_idc_core_service_entitlement service
  ON service.tenant_id=operation.tenant_id AND service.entitlement_no=operation.subject_no
 AND service.customer_no=operation.customer_no
JOIN plugin_idc_core_service_price_snapshot price
  ON price.tenant_id=operation.tenant_id AND price.price_snapshot_no=operation.price_snapshot_no
 AND price.subject_no=operation.subject_no
JOIN plugin_idc_core_billing_invoice invoice
  ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
 AND invoice.customer_no=operation.payer_customer_no
JOIN plugin_idc_core_credit_exposure exposure
  ON exposure.tenant_id=operation.tenant_id AND exposure.exposure_no=?
 AND exposure.customer_no=operation.payer_customer_no
 AND exposure.currency_code=operation.currency_code
 AND exposure.source_type_code='renewal' AND exposure.source_reference=operation.operation_no
 AND exposure.invoice_no=operation.invoice_no
WHERE operation.tenant_id=0 AND operation.operation_type_code='renewal'
  AND operation.operation_no=? AND operation.customer_no=?
  AND operation.payer_customer_no=?
FOR UPDATE OF operation,service,invoice`, exposureNo, operationNo, customerNo,
		payerCustomerNo); err != nil {
		return nil, gerror.Wrap(err, "lock credit renewal operation")
	}
	return row, nil
}

func lockConvergenceRow(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	payerCustomerNo string,
	operationNo string,
	ledgerTransactionNo string,
) (*convergenceRow, error) {
	var row *convergenceRow
	if err := database.GetScan(ctx, &row, `
SELECT operation.operation_no, operation.quote_no, operation.customer_no, operation.payer_customer_no,
       operation.subject_no AS entitlement_no,
       operation.currency_code, operation.invoice_no, operation.price_snapshot_no,
       operation.status_code, operation.execution_mode_code,
       operation.capability_evidence_no, operation.capability_evidence_digest,
       operation.content_digest, operation.expected_subject_version AS expected_entitlement_version,
       operation.version AS operation_version,
       service.charge_type_code='trial' AS trial_conversion,
 service.version AS entitlement_version,
       service.renewal_mode_code AS entitlement_renewal_mode,
	   service.status_code AS entitlement_status,
       CASE WHEN operation.resource_owner_code = service.entitlement_owner_id
              AND operation.channel_no = service.plugin_release_version
              AND operation.pool_no = service.contract_version
            THEN COALESCE(NULLIF(service.renewal_method, ''), service.plugin_id)
            ELSE NULL END AS entitlement_contract_no,
       service.declaration_digest AS entitlement_contract_digest,
       invoice.payment_due_at AS entitlement_contract_expires_at,
       service.expires_at AS current_expires_at,
       invoice.status_code AS invoice_status,
       invoice.total_amount_minor AS invoice_total_minor,
       invoice.paid_amount_minor AS invoice_paid_minor,
       invoice.credit_amount_minor AS invoice_credit_minor,
       invoice.payment_due_at,
       price.projected_expires_at,
       COALESCE((SELECT MAX(event_sequence)
                 FROM plugin_idc_core_business_operation_event event
                 WHERE event.operation_no = operation.operation_no), 0)::INT AS maximum_sequence
FROM plugin_idc_core_business_operation operation
INNER JOIN plugin_idc_core_service_entitlement service
    ON service.tenant_id = operation.tenant_id
   AND service.entitlement_no = operation.subject_no
   AND service.customer_no = operation.customer_no
INNER JOIN plugin_idc_core_service_price_snapshot price
    ON price.tenant_id = operation.tenant_id
   AND price.price_snapshot_no = operation.price_snapshot_no
   AND price.subject_no = operation.subject_no
INNER JOIN plugin_idc_core_billing_invoice invoice
    ON invoice.tenant_id = operation.tenant_id
   AND invoice.invoice_no = operation.invoice_no
INNER JOIN plugin_idc_core_billing_payment_allocation allocation
    ON allocation.tenant_id = operation.tenant_id
   AND allocation.invoice_no = operation.invoice_no
   AND allocation.ledger_transaction_no = ?
   AND allocation.payment_source_type_code = 'balance'
   AND allocation.direction_code = 'apply'
WHERE operation.tenant_id = 0 AND operation.operation_type_code = 'renewal'
  AND operation.operation_no = ? AND operation.customer_no = ?
  AND operation.payer_customer_no = ? AND invoice.customer_no = operation.payer_customer_no
FOR UPDATE OF operation, service, invoice`, ledgerTransactionNo, operationNo, customerNo, payerCustomerNo); err != nil {
		return nil, gerror.Wrap(err, "lock paid renewal operation")
	}
	return row, nil
}

func (s *serviceImpl) updateConvergedOperation(
	ctx context.Context,
	row *convergenceRow,
	finalStatus string,
	now time.Time,
	ledgerTransactionNo string,
	actorType string,
	actorReference string,
	settlementReason string,
) error {
	var executionStarted any
	var completed any
	if finalStatus == "succeeded" {
		executionStarted = now
		completed = now
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code = ?, financially_settled_at = ?, execution_started_at = ?,
    completed_at = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND operation_no = ? AND customer_no = ?
  AND status_code = 'awaiting_financial_settlement' AND version = ?`, finalStatus,
		now, executionStarted, completed, row.OperationNo, row.CustomerNo, row.OperationVersion)
	if err != nil {
		return gerror.Wrap(err, "converge paid renewal operation")
	}
	affected, err := updated.RowsAffected()
	if err != nil || affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	financialEvidence, err := contentDigest(struct {
		OperationNo, InvoiceNo, LedgerTransactionNo, Status string
		At                                                  time.Time
	}{row.OperationNo, row.InvoiceNo, ledgerTransactionNo, "financially_settled", now})
	if err != nil {
		return err
	}
	finalEvidence, err := contentDigest(struct {
		OperationNo, EntitlementNo, PriceSnapshotNo, Status string
		ProjectedExpiresAt, At                              time.Time
	}{row.OperationNo, row.EntitlementNo, row.PriceSnapshotNo, finalStatus,
		row.ProjectedExpiresAt, now})
	if err != nil {
		return err
	}
	if err = s.insertOperationEvent(ctx, row, row.MaximumSequence+1,
		"awaiting_financial_settlement", "financially_settled",
		settlementReason, financialEvidence, actorType, actorReference, now); err != nil {
		return err
	}
	reason := "renewal.provider_ready"
	if finalStatus == "succeeded" {
		reason = "renewal.local_entitlement_succeeded"
	}
	return s.insertOperationEvent(ctx, row, row.MaximumSequence+2,
		"financially_settled", finalStatus, reason, finalEvidence,
		actorType, actorReference, now)
}

func (s *serviceImpl) insertOperationEvent(
	ctx context.Context,
	row *convergenceRow,
	sequence int,
	fromStatus string,
	toStatus string,
	reason string,
	evidence string,
	actorType string,
	actorReference string,
	now time.Time,
) error {
	eventNo, err := newNumber(s.random, "OEV_")
	if err != nil {
		return err
	}
	content, err := contentDigest(struct {
		EventNo, OperationNo, EntitlementNo, FromStatus, ToStatus, Reason, Evidence string
		ActorType, ActorReference                                                   string
		Sequence                                                                    int
		At                                                                          time.Time
	}{eventNo, row.OperationNo, row.EntitlementNo, fromStatus, toStatus, reason,
		evidence, actorType, actorReference, sequence, now})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event
    (tenant_id, event_no, operation_no, subject_type_code, subject_no, event_sequence,
     from_status_code, to_status_code, reason_code, actor_type_code,
     actor_reference, evidence_digest, content_digest, created_at)
VALUES (0, ?, ?, 'entitlement', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, eventNo,
		row.OperationNo, row.EntitlementNo, sequence, fromStatus, toStatus, reason,
		actorType, actorReference, evidence, content, now); err != nil {
		return gerror.Wrap(err, "append renewal operation event")
	}
	return nil
}

func projectConvergedOperation(row *convergenceRow, reused bool) *Operation {
	return &Operation{
		OperationNo: row.OperationNo, QuoteNo: row.QuoteNo, InvoiceNo: row.InvoiceNo,
		EntitlementNo: row.EntitlementNo, CurrencyCode: row.CurrencyCode,
		Status: row.StatusCode, RenewalMode: row.ExecutionModeCode,
		TotalAmountMinor: row.InvoiceTotalMinor, PaymentDueAt: row.PaymentDueAt.UTC(),
		Reused: reused,
	}
}
