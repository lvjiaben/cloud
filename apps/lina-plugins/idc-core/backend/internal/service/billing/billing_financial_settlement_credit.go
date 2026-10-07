// This file owns credit-specific settlement preparation. It deliberately
// validates the immutable exposure and movement produced by the credit owner
// instead of treating a successful method return as accounting evidence.
package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type creditSettlementEvidenceRow struct {
	ExposureNo               string    `orm:"exposure_no"`
	CreditAccountNo          string    `orm:"credit_account_no"`
	CustomerNo               string    `orm:"customer_no"`
	CurrencyCode             string    `orm:"currency_code"`
	SourceType               string    `orm:"source_type_code"`
	SourceReference          string    `orm:"source_reference"`
	OrderNo                  string    `orm:"order_no"`
	InvoiceNo                string    `orm:"invoice_no"`
	OriginalPrincipalMinor   int64     `orm:"original_principal_minor"`
	UnbilledPrincipalMinor   int64     `orm:"unbilled_principal_minor"`
	BilledPrincipalMinor     int64     `orm:"billed_principal_minor"`
	RepaidPrincipalMinor     int64     `orm:"repaid_principal_minor"`
	ReleasedPrincipalMinor   int64     `orm:"released_principal_minor"`
	WrittenOffPrincipalMinor int64     `orm:"written_off_principal_minor"`
	ExposureStatus           string    `orm:"exposure_status_code"`
	ExposureSnapshotDigest   string    `orm:"exposure_snapshot_digest"`
	ExposureContentDigest    string    `orm:"exposure_content_digest"`
	ExposureVersion          int64     `orm:"exposure_version"`
	AccountStatus            string    `orm:"account_status_code"`
	AccountTermsNo           string    `orm:"account_terms_no"`
	AccountTermsVersion      int64     `orm:"account_terms_version"`
	AccountTotalMinor        int64     `orm:"account_total_minor"`
	AccountAvailableMinor    int64     `orm:"account_available_minor"`
	AccountUnbilledMinor     int64     `orm:"account_unbilled_minor"`
	AccountBilledMinor       int64     `orm:"account_billed_minor"`
	AccountDelinquentMinor   int64     `orm:"account_delinquent_minor"`
	AccountWrittenOffMinor   int64     `orm:"account_written_off_minor"`
	AccountValidFrom         time.Time `orm:"account_valid_from"`
	AccountValidUntil        time.Time `orm:"account_valid_until"`
	AccountVersion           int64     `orm:"account_version"`
	TermsContentDigest       string    `orm:"terms_content_digest"`
	MovementNo               string    `orm:"movement_no"`
	MovementType             string    `orm:"movement_type_code"`
	MovementAmountMinor      int64     `orm:"movement_amount_minor"`
	MovementVersionBefore    int64     `orm:"movement_version_before"`
	MovementVersionAfter     int64     `orm:"movement_version_after"`
	MovementStatusBefore     string    `orm:"movement_status_before"`
	MovementStatusAfter      string    `orm:"movement_status_after"`
	MovementTotalBefore      int64     `orm:"movement_total_before"`
	MovementTotalAfter       int64     `orm:"movement_total_after"`
	MovementAvailableBefore  int64     `orm:"movement_available_before"`
	MovementAvailableAfter   int64     `orm:"movement_available_after"`
	MovementUnbilledBefore   int64     `orm:"movement_unbilled_before"`
	MovementUnbilledAfter    int64     `orm:"movement_unbilled_after"`
	MovementBilledBefore     int64     `orm:"movement_billed_before"`
	MovementBilledAfter      int64     `orm:"movement_billed_after"`
	MovementDelinquentBefore int64     `orm:"movement_delinquent_before"`
	MovementDelinquentAfter  int64     `orm:"movement_delinquent_after"`
	MovementWrittenBefore    int64     `orm:"movement_written_before"`
	MovementWrittenAfter     int64     `orm:"movement_written_after"`
	MovementSourceType       string    `orm:"movement_source_type"`
	MovementSourceReference  string    `orm:"movement_source_reference"`
	MovementIdempotency      string    `orm:"movement_idempotency_digest"`
	MovementEvidence         string    `orm:"movement_evidence_digest"`
	MovementContent          string    `orm:"movement_content_digest"`
	MovementActorType        string    `orm:"movement_actor_type"`
	MovementActorID          int       `orm:"movement_actor_id"`
}

func (s *financialSettlementServiceImpl) prepareCreditFinancialSettlement(
	ctx context.Context,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	items settlementOrderItemSummary,
	identityDigest string,
	now time.Time,
	input CreditOrderSettlementInput,
) (preparedFinancialSettlement, error) {
	if !creditSettlementCandidateEligible(order, invoice, items, now) {
		return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementNotEligible)
	}
	aggregate, err := loadInvoiceAggregate(ctx, s.database, invoice.InvoiceNo)
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	if !creditSettlementAggregateEmpty(aggregate) {
		return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementNotEligible)
	}
	reserveDigest, err := contentDigest(struct {
		OrderNo, InvoiceNo, CustomerNo, CurrencyCode string
		AmountMinor                                  int64
		CheckoutRequestDigest, InvoiceSnapshotDigest string
		InvoiceContentDigest, IdentityDigest         string
	}{
		order.OrderNo, invoice.InvoiceNo, order.CustomerNo, order.CurrencyCode,
		order.TotalAmountMinor, order.CheckoutRequestDigest,
		invoice.SourceSnapshotDigest, invoice.ContentDigest, identityDigest,
	})
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	exposure, err := s.credits.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: order.CustomerNo, CurrencyCode: order.CurrencyCode,
		AmountMinor: order.TotalAmountMinor, SourceType: "order",
		SourceReference: order.OrderNo, OrderNo: order.OrderNo,
		InvoiceNo: invoice.InvoiceNo, SourceSnapshotDigest: reserveDigest,
		ActorType: input.ActorType, ActorID: input.ActorID,
	})
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	if !creditExposureMatchesReservation(exposure, order) {
		return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementIntegrity)
	}
	evidence, err := loadCreditSettlementEvidence(
		ctx, s, order, invoice, exposure.ExposureNo,
	)
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	evidenceDigest, err := validateCreditSettlementEvidence(
		evidence, order, invoice, reserveDigest, input, now,
	)
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	prepared := preparedFinancialSettlement{
		settlementNo: deterministicFinancialSettlementNo(order.OrderNo),
		orderNo:      order.OrderNo, invoiceNo: invoice.InvoiceNo,
		customerNo: order.CustomerNo, currencyCode: order.CurrencyCode,
		basis: SettlementBasisCredit, totalAmountMinor: order.TotalAmountMinor,
		creditAmountMinor:      order.TotalAmountMinor,
		identityEvidenceDigest: identityDigest, settledAt: now,
		sources: []preparedFinancialSettlementSource{{
			sourceType:      FinancialSettlementSourceCredit,
			sourceReference: order.OrderNo, creditExposureNo: exposure.ExposureNo,
			amountMinor: order.TotalAmountMinor, evidenceDigest: evidenceDigest,
		}},
	}
	if err = finalizePreparedFinancialSettlement(&prepared); err != nil {
		return preparedFinancialSettlement{}, err
	}
	return prepared, nil
}

func creditSettlementCandidateEligible(
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	items settlementOrderItemSummary,
	now time.Time,
) bool {
	return items.TrialCount == 0 && order.TotalAmountMinor > 0 &&
		order.StatusCode == "pending_payment" && order.PaymentDueAt != nil &&
		order.PaymentDueAt.After(now) &&
		invoice.StatusCode == string(InvoiceStatusPendingPayment) &&
		invoice.PaymentDueAt != nil && invoice.PaymentDueAt.After(now) &&
		invoice.PaidAmountMinor == 0 && invoice.WaivedAmountMinor == 0 &&
		invoice.CreditAmountMinor == 0 && invoice.RefundableAmountMinor == 0 &&
		invoice.RefundedAmountMinor == 0 && invoice.SettledAt == nil &&
		invoice.CancelledAt == nil
}

func creditSettlementAggregateEmpty(aggregate invoiceAggregate) bool {
	return aggregate.PaymentRows == 0 && aggregate.WaiverRows == 0 &&
		aggregate.RefundRows == 0 && aggregate.PaymentMinor == 0 &&
		aggregate.WaiverMinor == 0 && aggregate.RefundMinor == 0
}

func creditExposureMatchesReservation(
	exposure *CreditExposure,
	order *entity.CommerceOrder,
) bool {
	return exposure != nil && exposure.ExposureNo != "" &&
		exposure.CustomerNo == order.CustomerNo &&
		exposure.CurrencyCode == order.CurrencyCode && exposure.SourceType == "order" &&
		exposure.SourceReference == order.OrderNo &&
		exposure.OriginalPrincipalMinor == order.TotalAmountMinor &&
		exposure.UnbilledPrincipalMinor == order.TotalAmountMinor &&
		exposure.StatusCode == "unbilled"
}

func loadCreditSettlementEvidence(
	ctx context.Context,
	service *financialSettlementServiceImpl,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	exposureNo string,
) (creditSettlementEvidenceRow, error) {
	var rows []creditSettlementEvidenceRow
	if err := service.database.GetScan(ctx, &rows, `
SELECT exposure.exposure_no, exposure.credit_account_no,
       exposure.customer_no, exposure.currency_code,
       exposure.source_type_code, exposure.source_reference,
       exposure.order_no, exposure.invoice_no,
       exposure.original_principal_minor, exposure.unbilled_principal_minor,
       exposure.billed_principal_minor, exposure.repaid_principal_minor,
       exposure.released_principal_minor, exposure.written_off_principal_minor,
       exposure.status_code AS exposure_status_code,
       exposure.source_snapshot_digest AS exposure_snapshot_digest,
       exposure.content_digest AS exposure_content_digest,
       exposure.version AS exposure_version,
       account.status_code AS account_status_code,
       account.current_terms_no AS account_terms_no,
       account.current_terms_version AS account_terms_version,
       account.total_limit_minor AS account_total_minor,
       account.available_limit_minor AS account_available_minor,
       account.unbilled_principal_minor AS account_unbilled_minor,
       account.billed_principal_minor AS account_billed_minor,
       account.delinquent_principal_minor AS account_delinquent_minor,
       account.written_off_principal_minor AS account_written_off_minor,
       account.valid_from AS account_valid_from,
       account.valid_until AS account_valid_until,
       account.version AS account_version,
       terms.content_digest AS terms_content_digest,
       movement.movement_no, movement.movement_type_code,
       movement.amount_minor AS movement_amount_minor,
       movement.account_version_before AS movement_version_before,
       movement.account_version_after AS movement_version_after,
       movement.status_before_code AS movement_status_before,
       movement.status_after_code AS movement_status_after,
       movement.total_limit_before_minor AS movement_total_before,
       movement.total_limit_after_minor AS movement_total_after,
       movement.available_before_minor AS movement_available_before,
       movement.available_after_minor AS movement_available_after,
       movement.unbilled_before_minor AS movement_unbilled_before,
       movement.unbilled_after_minor AS movement_unbilled_after,
       movement.billed_before_minor AS movement_billed_before,
       movement.billed_after_minor AS movement_billed_after,
       movement.delinquent_before_minor AS movement_delinquent_before,
       movement.delinquent_after_minor AS movement_delinquent_after,
       movement.written_off_before_minor AS movement_written_before,
       movement.written_off_after_minor AS movement_written_after,
       movement.source_type_code AS movement_source_type,
       movement.source_reference AS movement_source_reference,
       movement.idempotency_digest AS movement_idempotency_digest,
       movement.evidence_digest AS movement_evidence_digest,
       movement.content_digest AS movement_content_digest,
       movement.actor_type_code AS movement_actor_type,
       movement.actor_id AS movement_actor_id
FROM plugin_idc_core_credit_exposure exposure
JOIN plugin_idc_core_credit_account account
  ON account.tenant_id = exposure.tenant_id
 AND account.credit_account_no = exposure.credit_account_no
 AND account.customer_no = exposure.customer_no
 AND account.currency_code = exposure.currency_code
JOIN plugin_idc_core_credit_account_terms terms
  ON terms.tenant_id = account.tenant_id
 AND terms.terms_no = account.current_terms_no
 AND terms.terms_version = account.current_terms_version
 AND terms.credit_account_no = account.credit_account_no
 AND terms.customer_no = account.customer_no
 AND terms.currency_code = account.currency_code
JOIN plugin_idc_core_credit_account_movement movement
  ON movement.tenant_id = exposure.tenant_id
 AND movement.credit_account_no = exposure.credit_account_no
 AND movement.customer_no = exposure.customer_no
 AND movement.currency_code = exposure.currency_code
 AND movement.exposure_no = exposure.exposure_no
 AND movement.movement_type_code = 'reserve'
WHERE exposure.tenant_id = 0 AND exposure.exposure_no = ?
  AND exposure.order_no = ? AND exposure.invoice_no = ?
  AND exposure.customer_no = ? AND exposure.currency_code = ?
ORDER BY exposure.id, movement.id
LIMIT 2`, exposureNo, order.OrderNo, invoice.InvoiceNo,
		order.CustomerNo, order.CurrencyCode); err != nil {
		return creditSettlementEvidenceRow{}, gerror.Wrap(err, "load credit settlement evidence")
	}
	if len(rows) != 1 {
		return creditSettlementEvidenceRow{}, bizerr.NewCode(CodeSettlementIntegrity)
	}
	return rows[0], nil
}

func validateCreditSettlementEvidence(
	row creditSettlementEvidenceRow,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	reserveDigest string,
	actor CreditOrderSettlementInput,
	now time.Time,
) (string, error) {
	amount := order.TotalAmountMinor
	if !validCreditExposureEvidence(row, order, invoice, reserveDigest) ||
		!validCreditAccountEvidence(row, amount, now) ||
		!validCreditMovementEvidence(row, order, amount, reserveDigest, actor) {
		return "", bizerr.NewCode(CodeSettlementIntegrity)
	}
	expectedExposure, err := contentDigest(struct {
		ExposureNo, CreditAccountNo, CustomerNo, CurrencyCode string
		SourceType, SourceReference, OrderNo, InvoiceNo       string
		AmountMinor                                           int64
		SourceSnapshotDigest                                  string
	}{
		row.ExposureNo, row.CreditAccountNo, row.CustomerNo, row.CurrencyCode,
		row.SourceType, row.SourceReference, row.OrderNo, row.InvoiceNo,
		amount, row.ExposureSnapshotDigest,
	})
	if err != nil || expectedExposure != row.ExposureContentDigest {
		return "", bizerr.NewCode(CodeSettlementIntegrity)
	}
	expectedIdempotency, err := contentDigest(struct {
		CreditAccountNo, MovementType, SourceType, SourceReference string
	}{row.CreditAccountNo, "reserve", "order", order.OrderNo})
	if err != nil || expectedIdempotency != row.MovementIdempotency {
		return "", bizerr.NewCode(CodeSettlementIntegrity)
	}
	expectedMovement, err := contentDigest(struct {
		MovementNo, CreditAccountNo, ExposureNo, StatementNo, RepaymentNo    string
		MovementType, StatusBefore, StatusAfter, SourceType, SourceReference string
		AmountMinor, VersionBefore, VersionAfter                             int64
		AvailableBefore, AvailableAfter, UnbilledBefore, UnbilledAfter       int64
		BilledBefore, BilledAfter, DelinquentBefore, DelinquentAfter         int64
		WrittenOffBefore, WrittenOffAfter                                    int64
		EvidenceDigest, ActorType                                            string
		ActorID                                                              int
	}{
		row.MovementNo, row.CreditAccountNo, row.ExposureNo, "", "",
		"reserve", row.MovementStatusBefore, row.MovementStatusAfter,
		"order", order.OrderNo, amount, row.MovementVersionBefore,
		row.MovementVersionAfter, row.MovementAvailableBefore,
		row.MovementAvailableAfter, row.MovementUnbilledBefore,
		row.MovementUnbilledAfter, row.MovementBilledBefore,
		row.MovementBilledAfter, row.MovementDelinquentBefore,
		row.MovementDelinquentAfter, row.MovementWrittenBefore,
		row.MovementWrittenAfter, reserveDigest, actor.ActorType, actor.ActorID,
	})
	if err != nil || expectedMovement != row.MovementContent {
		return "", bizerr.NewCode(CodeSettlementIntegrity)
	}
	evidenceDigest, err := contentDigest(struct {
		ExposureNo, ExposureContentDigest string
		MovementNo, MovementContentDigest string
		TermsNo, TermsContentDigest       string
		TermsVersion                      int64
	}{
		row.ExposureNo, row.ExposureContentDigest, row.MovementNo,
		row.MovementContent, row.AccountTermsNo, row.TermsContentDigest,
		row.AccountTermsVersion,
	})
	if err != nil {
		return "", err
	}
	return evidenceDigest, nil
}

func validCreditExposureEvidence(
	row creditSettlementEvidenceRow,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	reserveDigest string,
) bool {
	amount := order.TotalAmountMinor
	return validBusinessReference(row.ExposureNo, 40) &&
		validBusinessReference(row.CreditAccountNo, 40) &&
		row.CustomerNo == order.CustomerNo && row.CurrencyCode == order.CurrencyCode &&
		row.SourceType == "order" && row.SourceReference == order.OrderNo &&
		row.OrderNo == order.OrderNo && row.InvoiceNo == invoice.InvoiceNo &&
		row.OriginalPrincipalMinor == amount && row.UnbilledPrincipalMinor == amount &&
		row.BilledPrincipalMinor == 0 && row.RepaidPrincipalMinor == 0 &&
		row.ReleasedPrincipalMinor == 0 && row.WrittenOffPrincipalMinor == 0 &&
		row.ExposureStatus == "unbilled" && row.ExposureVersion == 1 &&
		row.ExposureSnapshotDigest == reserveDigest &&
		canonicalDigestPattern.MatchString(row.ExposureContentDigest)
}

func validCreditAccountEvidence(
	row creditSettlementEvidenceRow,
	amount int64,
	now time.Time,
) bool {
	return row.AccountStatus == "active" && row.AccountVersion > 1 &&
		row.AccountTermsVersion > 0 && validBusinessReference(row.AccountTermsNo, 40) &&
		canonicalDigestPattern.MatchString(row.TermsContentDigest) &&
		!row.AccountValidFrom.After(now) && row.AccountValidUntil.After(now) &&
		row.AccountTotalMinor > 0 && row.AccountAvailableMinor >= 0 &&
		row.AccountUnbilledMinor >= amount && row.AccountBilledMinor >= 0 &&
		row.AccountDelinquentMinor >= 0 && row.AccountWrittenOffMinor >= 0 &&
		row.AccountDelinquentMinor <= row.AccountBilledMinor &&
		row.AccountTotalMinor == row.AccountAvailableMinor+row.AccountUnbilledMinor+
			row.AccountBilledMinor+row.AccountWrittenOffMinor
}

func validCreditMovementEvidence(
	row creditSettlementEvidenceRow,
	order *entity.CommerceOrder,
	amount int64,
	reserveDigest string,
	actor CreditOrderSettlementInput,
) bool {
	return validBusinessReference(row.MovementNo, 40) &&
		row.MovementType == "reserve" && row.MovementAmountMinor == amount &&
		row.MovementVersionAfter == row.MovementVersionBefore+1 &&
		row.MovementVersionAfter == row.AccountVersion &&
		row.MovementStatusBefore == "active" && row.MovementStatusAfter == "active" &&
		row.MovementTotalBefore == row.MovementTotalAfter &&
		row.MovementTotalAfter == row.AccountTotalMinor &&
		row.MovementAvailableBefore-row.MovementAvailableAfter == amount &&
		row.MovementAvailableAfter == row.AccountAvailableMinor &&
		row.MovementUnbilledAfter-row.MovementUnbilledBefore == amount &&
		row.MovementUnbilledAfter == row.AccountUnbilledMinor &&
		row.MovementBilledBefore == row.MovementBilledAfter &&
		row.MovementBilledAfter == row.AccountBilledMinor &&
		row.MovementDelinquentBefore == row.MovementDelinquentAfter &&
		row.MovementDelinquentAfter == row.AccountDelinquentMinor &&
		row.MovementWrittenBefore == row.MovementWrittenAfter &&
		row.MovementWrittenAfter == row.AccountWrittenOffMinor &&
		row.MovementSourceType == "order" &&
		row.MovementSourceReference == order.OrderNo && row.MovementEvidence == reserveDigest &&
		row.MovementActorType == actor.ActorType && row.MovementActorID == actor.ActorID &&
		canonicalDigestPattern.MatchString(row.MovementIdempotency) &&
		canonicalDigestPattern.MatchString(row.MovementContent)
}
