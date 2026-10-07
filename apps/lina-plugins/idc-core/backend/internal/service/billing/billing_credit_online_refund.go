// This file closes the normal order-refund workflow over credit-settled
// invoices. External money is returned first; only after every effective
// source succeeds does the credit owner atomically reduce the exposure.

package billing

import (
	"context"
	"sort"
	"time"

	"lina-core/pkg/bizerr"
)

type creditOnlineRefundCommandRow struct {
	CommandNo                   string `orm:"command_no"`
	RefundSourceNo              string `orm:"refund_source_no"`
	CreditExposureNo            string `orm:"credit_exposure_no"`
	CreditComponent             string `orm:"credit_component_code"`
	CreditRepaymentAllocationNo string `orm:"credit_repayment_allocation_no"`
	CreditPlanDigest            string `orm:"credit_plan_digest"`
	PaymentSourceType           string `orm:"payment_source_type_code"`
	Destination                 string `orm:"destination_code"`
	AmountMinor                 int64  `orm:"amount_minor"`
	Status                      string `orm:"status_code"`
	ResultEvidence              string `orm:"result_evidence_digest"`
}

type creditOnlineRefundFundingAggregate struct {
	AmountMinor int64
	Destination CreditRefundDestination
	Evidence    []struct {
		CommandNo      string `json:"commandNo"`
		AmountMinor    int64  `json:"amountMinor"`
		Destination    string `json:"destination"`
		EvidenceDigest string `json:"evidenceDigest"`
	}
}

func (s *channelPaymentOwnerService) postSuccessfulCreditRefundSource(
	ctx context.Context,
	posting *refundPostingContext,
	now time.Time,
) (string, error) {
	if posting == nil || posting.CreditExposureNo == "" {
		return "", bizerr.NewCode(CodeRefundIntegrity)
	}
	rows, err := s.loadEffectiveCreditRefundCommands(ctx, posting.RefundNo)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", bizerr.NewCode(CodeRefundIntegrity)
	}
	for _, row := range rows {
		if row.CreditExposureNo == "" || row.CreditExposureNo != posting.CreditExposureNo ||
			row.CreditPlanDigest == "" || row.CreditPlanDigest != posting.CreditPlanDigest {
			return "", bizerr.NewCode(CodeRefundIntegrity)
		}
		if row.Status != "succeeded" {
			return "executing", nil
		}
	}

	if s.ledger == nil {
		return "", bizerr.NewCode(CodeRefundUnavailable)
	}
	var request struct {
		AmountMinor int64 `orm:"requested_amount_minor"`
	}
	if err = s.database.GetScan(ctx, &request, `
SELECT requested_amount_minor
FROM plugin_idc_core_refund_request
WHERE tenant_id=0 AND refund_no=? AND status_code='executing'
FOR UPDATE`, posting.RefundNo); err != nil {
		return "", err
	}
	if request.AmountMinor <= 0 {
		return "", bizerr.NewCode(CodeRefundIntegrity)
	}
	plan, err := s.ledger.PreviewCreditRefund(
		ctx, posting.CreditExposureNo, request.AmountMinor,
	)
	if err != nil || plan == nil || plan.PlanDigest != posting.CreditPlanDigest {
		return "", bizerr.NewCode(CodeRefundConflict)
	}
	fundingResults, evidenceDigest, err := aggregateCreditOnlineRefundFunding(
		posting.RefundNo, rows,
	)
	if err != nil {
		return "", err
	}
	result, err := s.ledger.ApplyConfirmedCreditRefund(ctx, CreditRefundInput{
		ExposureNo: posting.CreditExposureNo, AmountMinor: request.AmountMinor,
		PlanDigest: plan.PlanDigest, SourceReference: "online_refund:" + posting.RefundNo,
		EvidenceDigest: evidenceDigest, ActorType: "system", ActorID: 0,
		FundingResults: fundingResults,
	})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", bizerr.NewCode(CodeRefundIntegrity)
	}
	for _, row := range rows {
		sourcePosting := *posting
		sourcePosting.CommandNo = row.CommandNo
		sourcePosting.RefundSourceNo = row.RefundSourceNo
		sourcePosting.AmountMinor = row.AmountMinor
		sourcePosting.PaymentSourceType = row.PaymentSourceType
		sourcePosting.Destination = row.Destination
		postedSourceType, sourceTypeErr := creditRefundPostedSourceType(row)
		if sourceTypeErr != nil {
			return "", sourceTypeErr
		}
		if err = s.enqueueRefundSourcePosted(
			ctx, &sourcePosting, postedSourceType, now,
		); err != nil {
			return "", err
		}
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='succeeded',completed_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='executing'`,
		now, now, posting.RefundNo)
	if err != nil {
		return "", err
	}
	if err = requireSingleRefundExecutionUpdate(updated); err != nil {
		return "", err
	}
	if err = s.enqueueCompletedRefundEvents(ctx, posting, now); err != nil {
		return "", err
	}
	return "succeeded", nil
}

func (s *channelPaymentOwnerService) loadEffectiveCreditRefundCommands(
	ctx context.Context,
	refundNo string,
) ([]creditOnlineRefundCommandRow, error) {
	rows := make([]creditOnlineRefundCommandRow, 0, 16)
	err := s.database.GetScan(ctx, &rows, `
SELECT DISTINCT ON (source.refund_source_no)
       command.command_no,source.refund_source_no,
       COALESCE(source.credit_exposure_no,'') AS credit_exposure_no,
       COALESCE(source.credit_component_code,'') AS credit_component_code,
       COALESCE(source.credit_repayment_allocation_no,'') AS credit_repayment_allocation_no,
       COALESCE(source.credit_plan_digest,'') AS credit_plan_digest,
       source.payment_source_type_code,command.destination_code,
       command.amount_minor,command.status_code,
       COALESCE(command.result_evidence_digest,'') AS result_evidence_digest
FROM plugin_idc_core_refund_source_allocation source
JOIN plugin_idc_core_refund_execution_command command
  ON command.refund_source_no=source.refund_source_no
WHERE source.tenant_id=0 AND source.refund_no=?
ORDER BY source.refund_source_no,
         CASE command.destination_code WHEN 'same_currency_balance' THEN 0 ELSE 1 END,
         command.id DESC`, refundNo)
	return rows, err
}

func aggregateCreditOnlineRefundFunding(
	refundNo string,
	rows []creditOnlineRefundCommandRow,
) ([]CreditRefundFundingResult, string, error) {
	aggregates := make(map[string]*creditOnlineRefundFundingAggregate)
	for _, row := range rows {
		if row.AmountMinor <= 0 || row.ResultEvidence == "" {
			return nil, "", bizerr.NewCode(CodeRefundIntegrity)
		}
		if row.CreditComponent != string(CreditRefundRepaid) {
			if row.CreditComponent != string(CreditRefundUnbilled) &&
				row.CreditComponent != string(CreditRefundBilled) {
				return nil, "", bizerr.NewCode(CodeRefundIntegrity)
			}
			continue
		}
		if row.CreditRepaymentAllocationNo == "" {
			return nil, "", bizerr.NewCode(CodeRefundIntegrity)
		}
		destination := CreditRefundDestinationOriginalSource
		if row.Destination == "same_currency_balance" {
			destination = CreditRefundDestinationBalance
		} else if row.Destination != "original_channel" {
			return nil, "", bizerr.NewCode(CodeRefundIntegrity)
		}
		aggregate := aggregates[row.CreditRepaymentAllocationNo]
		if aggregate == nil {
			aggregate = &creditOnlineRefundFundingAggregate{Destination: destination}
			aggregates[row.CreditRepaymentAllocationNo] = aggregate
		} else if aggregate.Destination != destination {
			return nil, "", bizerr.NewCode(CodeRefundIntegrity)
		}
		aggregate.AmountMinor += row.AmountMinor
		aggregate.Evidence = append(aggregate.Evidence, struct {
			CommandNo      string `json:"commandNo"`
			AmountMinor    int64  `json:"amountMinor"`
			Destination    string `json:"destination"`
			EvidenceDigest string `json:"evidenceDigest"`
		}{row.CommandNo, row.AmountMinor, row.Destination, row.ResultEvidence})
	}
	allocationNos := make([]string, 0, len(aggregates))
	for allocationNo := range aggregates {
		allocationNos = append(allocationNos, allocationNo)
	}
	sort.Strings(allocationNos)
	results := make([]CreditRefundFundingResult, 0, len(allocationNos))
	for _, allocationNo := range allocationNos {
		aggregate := aggregates[allocationNo]
		evidenceDigest, err := contentDigest(aggregate.Evidence)
		if err != nil {
			return nil, "", err
		}
		results = append(results, CreditRefundFundingResult{
			RepaymentAllocationNo: allocationNo,
			AmountMinor:           aggregate.AmountMinor,
			RefundSourceReference: "online_refund:" + refundNo + ":" + allocationNo,
			EvidenceDigest:        evidenceDigest, Destination: aggregate.Destination,
		})
	}
	evidenceDigest, err := contentDigest(struct {
		RefundNo string                         `json:"refundNo"`
		Rows     []creditOnlineRefundCommandRow `json:"rows"`
	}{refundNo, rows})
	if err != nil {
		return nil, "", err
	}
	return results, evidenceDigest, nil
}

func creditRefundPostedSourceType(
	row creditOnlineRefundCommandRow,
) (RefundSourceType, error) {
	if row.CreditComponent != string(CreditRefundRepaid) {
		if row.CreditComponent != string(CreditRefundUnbilled) &&
			row.CreditComponent != string(CreditRefundBilled) {
			return "", bizerr.NewCode(CodeRefundIntegrity)
		}
		return RefundSourceCreditRelease, nil
	}
	if row.Destination == "same_currency_balance" || row.PaymentSourceType == "balance" {
		return RefundSourceBalanceCredit, nil
	}
	switch row.PaymentSourceType {
	case "channel_payment":
		return RefundSourceChannel, nil
	case "manual_receipt":
		return RefundSourceManual, nil
	case "migration":
		return RefundSourceMigration, nil
	default:
		return "", bizerr.NewCode(CodeRefundIntegrity)
	}
}
