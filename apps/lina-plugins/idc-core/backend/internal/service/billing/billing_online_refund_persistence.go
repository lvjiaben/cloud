// This file persists immutable refund requests, selections, impacts, sources, and operation locks.

package billing

import (
	"context"
	"fmt"
	"strings"

	"lina-core/pkg/bizerr"
)

type refundInvoiceRow struct {
	InvoiceNo             string
	OrderNo               string
	CustomerNo            string
	CurrencyCode          string
	StatusCode            string
	TotalAmountMinor      int64
	CreditAmountMinor     int64
	RefundableAmountMinor int64
	RefundedAmountMinor   int64
	Version               int64
}

type refundInvoiceItemRow struct {
	InvoiceItemNo        string
	OrderItemNo          string
	DescriptionZhCN      string
	DescriptionEnUS      string
	RefundableCapMinor   int64
	RemainingAmountMinor int64
	FrozenQuantity       int
	RemainingQuantity    int
}

type refundEntitlementRow struct {
	EntitlementNo        string
	OrderItemNo          string
	ProductNameZhCN      string
	ProductNameEnUS      string
	Version              int64
	StatusCode           string
	UnavailableForRefund bool
}

type refundPaymentSourceRow struct {
	AllocationNo           string
	InvoiceItemNo          *string
	PaymentSourceTypeCode  string
	PaymentSourceReference string
	RemainingAmountMinor   int64
}

type refundPolicyRow struct {
	PolicyNo                 string
	CurrencyCode             string
	PolicyVersion            int64
	HighAmountThresholdMinor int64
	AutomaticApprovalEnabled bool
}

type refundPreviewImpactFlags struct {
	requiresTermination bool
	hasResourceImpact   bool
	hasDangerousAction  bool
}

func (s *onlineRefundService) previewForCustomer(
	ctx context.Context,
	customerNo string,
	input RefundPreviewInput,
	lockInvoice bool,
) (*RefundPreview, error) {
	invoice, err := s.loadRefundInvoice(ctx, customerNo, input.OrderNo, lockInvoice)
	if err != nil {
		return nil, err
	}
	if validationErr := validateRefundInvoiceEligibility(invoice); validationErr != nil {
		return nil, validationErr
	}

	selections := append([]RefundSelectionInput(nil), input.Selections...)
	sortRefundSelections(selections)
	itemNos := make([]string, 0, len(selections))
	for _, selection := range selections {
		itemNos = append(itemNos, selection.InvoiceItemNo)
	}
	itemRows, err := s.loadRefundInvoiceItems(ctx, invoice.InvoiceNo, itemNos)
	if err != nil {
		return nil, err
	}
	itemsByNo := make(map[string]refundInvoiceItemRow, len(itemRows))
	orderItemNos := make([]string, 0, len(itemRows))
	for _, item := range itemRows {
		itemsByNo[item.InvoiceItemNo] = item
		orderItemNos = append(orderItemNos, item.OrderItemNo)
	}
	if len(itemsByNo) != len(uniqueRefundStrings(itemNos)) {
		return nil, bizerr.NewCode(CodeRefundNotAccessible)
	}
	entitlements, err := s.loadRefundEntitlements(
		ctx, customerNo, orderItemNos, lockInvoice,
	)
	if err != nil {
		return nil, err
	}
	entitlementsByNo := make(map[string]refundEntitlementRow, len(entitlements))
	for _, entitlement := range entitlements {
		entitlementsByNo[entitlement.EntitlementNo] = entitlement
	}
	creditInvoice := invoice.StatusCode == "credit_settled"
	var sources []refundPaymentSourceRow
	if !creditInvoice {
		sources, err = s.loadRefundPaymentSources(ctx, invoice.InvoiceNo)
		if err != nil {
			return nil, err
		}
	}
	policy, err := s.loadRefundPolicy(ctx, invoice.CurrencyCode)
	if err != nil {
		return nil, err
	}

	remainingBySource := make(map[string]int64, len(sources))
	for _, source := range sources {
		remainingBySource[source.AllocationNo] = source.RemainingAmountMinor
	}
	requestedTotal, err := validateRefundRequestedAmounts(selections, itemsByNo)
	if err != nil {
		return nil, err
	}
	var creditSources [][]RefundSourceAllocation
	if creditInvoice {
		plan, planErr := s.previewCreditInvoiceRefund(ctx, invoice, requestedTotal)
		if planErr != nil {
			return nil, planErr
		}
		creditSources, err = allocateCreditRefundSources(selections, plan)
		if err != nil {
			return nil, err
		}
	}
	preview := &RefundPreview{
		CustomerNo: audienceSafeCustomerNo(customerNo), OrderNo: invoice.OrderNo,
		InvoiceNo: invoice.InvoiceNo, CurrencyCode: invoice.CurrencyCode,
		Items: make([]RefundPreviewItem, 0, len(selections)),
	}
	for selectionIndex, selection := range selections {
		item, exists := itemsByNo[selection.InvoiceItemNo]
		if !exists || item.OrderItemNo == "" {
			return nil, bizerr.NewCode(CodeRefundNotAccessible)
		}
		previewItem, impactFlags, buildErr := buildRefundPreviewItem(
			selection, item, entitlementsByNo,
		)
		if buildErr != nil {
			return nil, buildErr
		}
		preview.RequiresEntitlementTermination = preview.RequiresEntitlementTermination ||
			impactFlags.requiresTermination
		preview.HasResourceImpact = preview.HasResourceImpact || impactFlags.hasResourceImpact
		preview.HasDangerousOperation = preview.HasDangerousOperation ||
			impactFlags.hasDangerousAction
		if creditInvoice {
			previewItem.Sources = creditSources[selectionIndex]
		} else {
			previewItem.Sources, err = allocateRefundSources(
				selection.AmountMinor, item.InvoiceItemNo, sources, remainingBySource,
			)
			if err != nil {
				return nil, err
			}
		}
		for _, source := range previewItem.Sources {
			if source.ExecutionOwner == "manual" {
				preview.HasFundAnomaly = true
			}
		}
		previewItem.ImpactDigest, err = contentDigest(struct {
			InvoiceItemNo string                    `json:"invoiceItemNo"`
			OrderItemNo   string                    `json:"orderItemNo"`
			Quantity      int                       `json:"quantity"`
			AmountMinor   int64                     `json:"amountMinor"`
			Remaining     int64                     `json:"refundableAmountMinor"`
			Impacts       []RefundEntitlementImpact `json:"impacts"`
			Sources       []RefundSourceAllocation  `json:"sources"`
		}{
			previewItem.InvoiceItemNo, previewItem.OrderItemNo,
			previewItem.Quantity, previewItem.AmountMinor,
			previewItem.RefundableAmountMinor,
			previewItem.Impacts, previewItem.Sources,
		})
		if err != nil {
			return nil, err
		}
		preview.AmountMinor += selection.AmountMinor
		preview.Items = append(preview.Items, previewItem)
	}
	if !creditInvoice && preview.AmountMinor > invoice.RefundableAmountMinor {
		return nil, bizerr.NewCode(CodeRefundNotEligible)
	}
	applyRefundApprovalPolicy(preview, policy)
	preview.ImpactDigest, err = contentDigest(struct {
		CustomerNo                     string              `json:"customerNo"`
		OrderNo                        string              `json:"orderNo"`
		InvoiceNo                      string              `json:"invoiceNo"`
		CurrencyCode                   string              `json:"currencyCode"`
		AmountMinor                    int64               `json:"amountMinor"`
		ApprovalMode                   string              `json:"approvalMode"`
		RequiresEntitlementTermination bool                `json:"requiresEntitlementTermination"`
		HasResourceImpact              bool                `json:"hasResourceImpact"`
		HasDangerousOperation          bool                `json:"hasDangerousOperation"`
		HasFundAnomaly                 bool                `json:"hasFundAnomaly"`
		HighAmount                     bool                `json:"highAmount"`
		PolicyNo                       string              `json:"policyNo,omitempty"`
		PolicyVersion                  int64               `json:"policyVersion,omitempty"`
		Items                          []RefundPreviewItem `json:"items"`
	}{
		preview.CustomerNo, preview.OrderNo, preview.InvoiceNo, preview.CurrencyCode,
		preview.AmountMinor, preview.ApprovalMode, preview.RequiresEntitlementTermination,
		preview.HasResourceImpact, preview.HasDangerousOperation, preview.HasFundAnomaly,
		preview.HighAmount, preview.PolicyNo, preview.PolicyVersion, preview.Items,
	})
	if err != nil {
		return nil, err
	}
	return preview, nil
}

func validateRefundRequestedAmounts(
	selections []RefundSelectionInput,
	itemsByNo map[string]refundInvoiceItemRow,
) (int64, error) {
	requestedByItem := make(map[string]int64, len(selections))
	requestedQuantityByItem := make(map[string]int, len(selections))
	requestedTotal := int64(0)
	for _, selection := range selections {
		item, exists := itemsByNo[selection.InvoiceItemNo]
		if !exists || item.OrderItemNo == "" {
			return 0, bizerr.NewCode(CodeRefundNotAccessible)
		}
		requestedByItem[item.InvoiceItemNo] += selection.AmountMinor
		requestedQuantityByItem[item.InvoiceItemNo] += selection.Quantity
		requestedTotal += selection.AmountMinor
		if selection.AmountMinor > item.RemainingAmountMinor ||
			requestedByItem[item.InvoiceItemNo] > item.RemainingAmountMinor ||
			selection.Quantity > item.RemainingQuantity ||
			requestedQuantityByItem[item.InvoiceItemNo] > item.RemainingQuantity {
			return 0, bizerr.NewCode(CodeRefundNotEligible)
		}
	}
	return requestedTotal, nil
}

func buildRefundPreviewItem(
	selection RefundSelectionInput,
	item refundInvoiceItemRow,
	entitlementsByNo map[string]refundEntitlementRow,
) (RefundPreviewItem, refundPreviewImpactFlags, error) {
	previewItem := RefundPreviewItem{
		InvoiceItemNo: item.InvoiceItemNo, OrderItemNo: item.OrderItemNo,
		Quantity:    selection.Quantity,
		AmountMinor: selection.AmountMinor, RefundableAmountMinor: item.RemainingAmountMinor,
		DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
	}
	flags := refundPreviewImpactFlags{}
	for _, entitlementNo := range selection.EntitlementNos {
		entitlement, found := entitlementsByNo[entitlementNo]
		if !found || entitlement.OrderItemNo != item.OrderItemNo {
			return RefundPreviewItem{}, refundPreviewImpactFlags{},
				bizerr.NewCode(CodeRefundNotAccessible)
		}
		if entitlement.UnavailableForRefund {
			return RefundPreviewItem{}, refundPreviewImpactFlags{}, bizerr.NewCode(CodeRefundNotEligible)
		}
		requiresTermination := entitlement.StatusCode != "terminated"
		impact := RefundEntitlementImpact{
			EntitlementNo: entitlement.EntitlementNo, OrderItemNo: entitlement.OrderItemNo,
			ProductNameZhCN: entitlement.ProductNameZhCN, ProductNameEnUS: entitlement.ProductNameEnUS,
			EntitlementVersion: entitlement.Version, StatusCode: entitlement.StatusCode,
			RequiresTermination: requiresTermination,
		}
		var err error
		impact.ImpactDigest, err = contentDigest(impact)
		if err != nil {
			return RefundPreviewItem{}, refundPreviewImpactFlags{}, err
		}
		previewItem.Impacts = append(previewItem.Impacts, impact)
		flags.requiresTermination = flags.requiresTermination || requiresTermination
		flags.hasResourceImpact = true
	}
	return previewItem, flags, nil
}

func validateRefundInvoiceEligibility(invoice *refundInvoiceRow) error {
	if invoice == nil {
		return bizerr.NewCode(CodeRefundNotAccessible)
	}
	ordinaryEligible := (invoice.StatusCode == "paid" ||
		invoice.StatusCode == "partially_refunded") && invoice.RefundableAmountMinor > 0
	creditEligible := invoice.StatusCode == "credit_settled" &&
		invoice.CreditAmountMinor == invoice.TotalAmountMinor && invoice.CreditAmountMinor > 0
	if !ordinaryEligible && !creditEligible {
		return bizerr.NewCode(CodeRefundNotEligible)
	}
	return nil
}

func applyRefundApprovalPolicy(preview *RefundPreview, policy *refundPolicyRow) {
	if policy == nil {
		preview.ApprovalMode = "manual"
		preview.HighAmount = true
		return
	}
	preview.PolicyNo = policy.PolicyNo
	preview.PolicyVersion = policy.PolicyVersion
	preview.HighAmount = preview.AmountMinor >= policy.HighAmountThresholdMinor
	preview.ApprovalMode = "manual"
	if policy.AutomaticApprovalEnabled && !preview.HasResourceImpact &&
		!preview.HasDangerousOperation && !preview.HasFundAnomaly {
		preview.ApprovalMode = "automatic"
	}
}

func (s *onlineRefundService) loadRefundInvoice(
	ctx context.Context, customerNo, orderNo string, lock bool,
) (*refundInvoiceRow, error) {
	if lock {
		// Settlement and native execution lock order before invoice. Acquiring
		// the parent here also prevents the later refund-order FK check from
		// creating an invoice -> order deadlock against those owners.
		if _, err := s.database.GetValue(ctx, `SELECT order_no FROM plugin_idc_core_commerce_order
WHERE tenant_id=0 AND order_no=? AND customer_no=? FOR UPDATE`, orderNo, customerNo); err != nil {
			return nil, err
		}
	}
	query := `
SELECT invoice_no,order_no,customer_no,currency_code,status_code,total_amount_minor,
       credit_amount_minor,
       refundable_amount_minor,refunded_amount_minor,version
FROM plugin_idc_core_billing_invoice
WHERE tenant_id=0 AND source_code='order' AND order_no=? AND customer_no=?`
	if lock {
		query += " FOR UPDATE"
	}
	var row *refundInvoiceRow
	err := s.database.GetScan(ctx, &row, query, orderNo, customerNo)
	return row, err
}

func (s *onlineRefundService) loadRefundInvoiceItems(
	ctx context.Context, invoiceNo string, itemNos []string,
) ([]refundInvoiceItemRow, error) {
	unique := uniqueRefundStrings(itemNos)
	if len(unique) == 0 {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}
	args := make([]any, 0, len(unique)+1)
	args = append(args, invoiceNo)
	for _, value := range unique {
		args = append(args, value)
	}
	query := fmt.Sprintf(`
SELECT item.invoice_item_no,item.order_item_no,item.description_zh_cn,item.description_en_us,
       item.refundable_cap_minor,
       item.refundable_cap_minor
       - COALESCE((
           SELECT SUM(CASE WHEN allocation.direction_code='apply'
                           THEN allocation.amount_minor ELSE -allocation.amount_minor END)
           FROM plugin_idc_core_billing_refund_allocation allocation
           WHERE allocation.invoice_no=item.invoice_no
             AND allocation.invoice_item_no=item.invoice_item_no
       ),0)
       - COALESCE((
           SELECT SUM(completed.requested_amount_minor)
           FROM plugin_idc_core_refund_request_item completed
           JOIN plugin_idc_core_refund_request request
             ON request.refund_no=completed.refund_no
           WHERE completed.invoice_no=item.invoice_no
             AND completed.invoice_item_no=item.invoice_item_no
             AND request.status_code='succeeded'
             AND EXISTS (
                 SELECT 1
                 FROM plugin_idc_core_refund_source_allocation source
                 WHERE source.refund_item_no=completed.refund_item_no
                   AND source.credit_exposure_no IS NOT NULL
             )
       ),0)
       - COALESCE((
           SELECT SUM(reserved.requested_amount_minor)
           FROM plugin_idc_core_refund_request_item reserved
           JOIN plugin_idc_core_refund_request request
             ON request.refund_no=reserved.refund_no
           WHERE reserved.invoice_no=item.invoice_no
             AND reserved.invoice_item_no=item.invoice_item_no
             AND request.status_code IN (
                 'pending_review','approved','awaiting_balance_consent',
                 'executing','result_unknown','manual_confirmation'
             )
       ),0) AS remaining_amount_minor,
       quantity.quantity AS frozen_quantity,
       quantity.quantity
       - COALESCE((
           SELECT SUM(completed.quantity)
           FROM plugin_idc_core_refund_request_item completed
           JOIN plugin_idc_core_refund_request request ON request.refund_no=completed.refund_no
           WHERE completed.invoice_no=item.invoice_no
             AND completed.invoice_item_no=item.invoice_item_no
             AND request.status_code='succeeded'
       ),0)
       - COALESCE((
           SELECT SUM(reserved.quantity)
           FROM plugin_idc_core_refund_request_item reserved
           JOIN plugin_idc_core_refund_request request ON request.refund_no=reserved.refund_no
           WHERE reserved.invoice_no=item.invoice_no
             AND reserved.invoice_item_no=item.invoice_item_no
             AND request.status_code IN (
                 'pending_review','approved','awaiting_balance_consent',
                 'executing','result_unknown','manual_confirmation'
             )
       ),0) AS remaining_quantity
FROM plugin_idc_core_billing_invoice_item item
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity
  ON quantity.order_item_no=item.order_item_no
WHERE item.tenant_id=0 AND item.invoice_no=?
  AND item.invoice_item_no IN (%s)
ORDER BY item.line_number,item.id`, refundPlaceholders(len(unique)))
	var rows []refundInvoiceItemRow
	if err := s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *onlineRefundService) loadRefundEntitlements(
	ctx context.Context, customerNo string, orderItemNos []string, lock bool,
) ([]refundEntitlementRow, error) {
	unique := uniqueRefundStrings(orderItemNos)
	if len(unique) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(unique)+1)
	args = append(args, customerNo)
	for _, value := range unique {
		args = append(args, value)
	}
	query := fmt.Sprintf(`
SELECT entitlement.entitlement_no,entitlement.order_item_no,
       intent.product_name_zh_cn,intent.product_name_en_us,
       entitlement.version,entitlement.status_code,
       EXISTS (
           SELECT 1
		   FROM plugin_idc_core_refund_entitlement_impact impact
		   JOIN plugin_idc_core_refund_request request ON request.refund_no=impact.refund_no
		   WHERE impact.entitlement_no=entitlement.entitlement_no
             AND request.status_code IN (
                 'pending_review','approved','awaiting_balance_consent','executing',
                 'result_unknown','manual_confirmation','succeeded'
             )
       ) AS unavailable_for_refund
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id=entitlement.tenant_id
 AND intent.intent_no=entitlement.source_intent_no
WHERE entitlement.tenant_id=0 AND entitlement.customer_no=?
  AND entitlement.order_item_no IN (%s)
ORDER BY entitlement.order_item_no,entitlement.entitlement_no`, refundPlaceholders(len(unique)))
	if lock {
		query += " FOR UPDATE"
	}
	var rows []refundEntitlementRow
	if err := s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *onlineRefundService) loadRefundPaymentSources(
	ctx context.Context, invoiceNo string,
) ([]refundPaymentSourceRow, error) {
	var rows []refundPaymentSourceRow
	err := s.database.GetScan(ctx, &rows, `
WITH payment_reversals AS (
    SELECT original_allocation_no,COALESCE(SUM(amount_minor),0) AS amount_minor
    FROM plugin_idc_core_billing_payment_allocation
    WHERE invoice_no=? AND direction_code='reverse'
    GROUP BY original_allocation_no
), applied_refunds AS (
    SELECT applied.payment_allocation_no,COALESCE(SUM(applied.amount_minor),0) AS amount_minor
    FROM plugin_idc_core_billing_refund_allocation applied
    WHERE applied.invoice_no=? AND applied.direction_code='apply'
    GROUP BY applied.payment_allocation_no
), reversed_refunds AS (
    SELECT applied.payment_allocation_no,COALESCE(SUM(reversed.amount_minor),0) AS amount_minor
    FROM plugin_idc_core_billing_refund_allocation reversed
    JOIN plugin_idc_core_billing_refund_allocation applied
      ON applied.refund_allocation_no=reversed.original_refund_allocation_no
     AND applied.direction_code='apply'
    WHERE reversed.invoice_no=? AND reversed.direction_code='reverse'
    GROUP BY applied.payment_allocation_no
), completed_refunds AS (
    SELECT applied.payment_allocation_no,
           applied.amount_minor-COALESCE(reversed.amount_minor,0) AS amount_minor
    FROM applied_refunds applied
    LEFT JOIN reversed_refunds reversed
      ON reversed.payment_allocation_no=applied.payment_allocation_no
), reserved_refunds AS (
    SELECT source.payment_allocation_no,COALESCE(SUM(source.amount_minor),0) AS amount_minor
    FROM plugin_idc_core_refund_source_allocation source
    JOIN plugin_idc_core_refund_request request
      ON request.refund_no=source.refund_no
    WHERE request.invoice_no=? AND request.status_code IN (
        'pending_review','approved','awaiting_balance_consent',
        'executing','result_unknown','manual_confirmation'
    )
    GROUP BY source.payment_allocation_no
)
SELECT payment.allocation_no,payment.invoice_item_no,
       payment.payment_source_type_code,payment.payment_source_reference,
       payment.amount_minor-COALESCE(reversal.amount_minor,0)
       -COALESCE(refund.amount_minor,0)-COALESCE(reserved.amount_minor,0)
         AS remaining_amount_minor
FROM plugin_idc_core_billing_payment_allocation payment
LEFT JOIN payment_reversals reversal
  ON reversal.original_allocation_no=payment.allocation_no
LEFT JOIN completed_refunds refund
  ON refund.payment_allocation_no=payment.allocation_no
LEFT JOIN reserved_refunds reserved
  ON reserved.payment_allocation_no=payment.allocation_no
WHERE payment.invoice_no=? AND payment.direction_code='apply'
ORDER BY CASE WHEN payment.invoice_item_no IS NOT NULL THEN 0 ELSE 1 END,
         payment.created_at,payment.id`, invoiceNo, invoiceNo, invoiceNo, invoiceNo, invoiceNo)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *onlineRefundService) loadRefundPolicy(
	ctx context.Context, currency string,
) (*refundPolicyRow, error) {
	var row *refundPolicyRow
	err := s.database.GetScan(ctx, &row, `
SELECT policy_no,currency_code,policy_version,high_amount_threshold_minor,
       automatic_approval_enabled
FROM plugin_idc_core_refund_policy_version
WHERE tenant_id=0 AND currency_code=? AND effective_at<=CURRENT_TIMESTAMP
ORDER BY effective_at DESC,policy_version DESC
LIMIT 1`, currency)
	return row, err
}

func (s *onlineRefundService) previewCreditInvoiceRefund(
	ctx context.Context,
	invoice *refundInvoiceRow,
	amountMinor int64,
) (*CreditRefundPlan, error) {
	if invoice == nil || amountMinor <= 0 {
		return nil, bizerr.NewCode(CodeRefundNotEligible)
	}
	var rows []struct {
		ExposureNo string `orm:"exposure_no"`
	}
	if err := s.database.GetScan(ctx, &rows, `
SELECT exposure_no
FROM plugin_idc_core_credit_exposure
WHERE tenant_id=0 AND invoice_no=? AND order_no=? AND customer_no=?
  AND currency_code=? AND source_type_code='order'
ORDER BY id
LIMIT 2`, invoice.InvoiceNo, invoice.OrderNo, invoice.CustomerNo,
		invoice.CurrencyCode); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeRefundNotEligible)
	}
	if s.credit == nil {
		return nil, bizerr.NewCode(CodeRefundUnavailable)
	}
	plan, err := s.credit.PreviewCreditRefund(ctx, rows[0].ExposureNo, amountMinor)
	if err != nil {
		return nil, bizerr.NewCode(CodeRefundNotEligible)
	}
	if plan == nil || plan.CustomerNo != invoice.CustomerNo ||
		plan.CurrencyCode != invoice.CurrencyCode || plan.AmountMinor != amountMinor {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	return plan, nil
}

type creditRefundSourceCursor struct {
	allocation RefundSourceAllocation
	remaining  int64
}

func allocateCreditRefundSources(
	selections []RefundSelectionInput,
	plan *CreditRefundPlan,
) ([][]RefundSourceAllocation, error) {
	if plan == nil || len(selections) == 0 || plan.AmountMinor <= 0 {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	cursors := make([]creditRefundSourceCursor, 0, len(plan.FundingSources)+2)
	appendComponent := func(component CreditRefundComponent, amount int64) {
		if amount <= 0 {
			return
		}
		cursors = append(cursors, creditRefundSourceCursor{
			allocation: RefundSourceAllocation{
				PaymentSourceType:      "credit",
				PaymentSourceReference: plan.ExposureNo,
				ExecutionOwner:         "core",
				CreditExposureNo:       plan.ExposureNo,
				CreditComponent:        component,
				CreditPlanDigest:       plan.PlanDigest,
			},
			remaining: amount,
		})
	}
	appendComponent(CreditRefundUnbilled, plan.UnbilledReleaseMinor)
	appendComponent(CreditRefundBilled, plan.BilledReductionMinor)
	for _, source := range plan.FundingSources {
		paymentType, owner, ok := creditRefundExecutionSource(source.FundingSourceType)
		if !ok || source.AmountMinor <= 0 {
			return nil, bizerr.NewCode(CodeRefundIntegrity)
		}
		cursors = append(cursors, creditRefundSourceCursor{
			allocation: RefundSourceAllocation{
				PaymentSourceType:           paymentType,
				PaymentSourceReference:      source.FundingSourceReference,
				ExecutionOwner:              owner,
				CreditExposureNo:            plan.ExposureNo,
				CreditComponent:             CreditRefundRepaid,
				CreditRepaymentAllocationNo: source.RepaymentAllocationNo,
				CreditPlanDigest:            plan.PlanDigest,
			},
			remaining: source.AmountMinor,
		})
	}
	result := make([][]RefundSourceAllocation, len(selections))
	cursorIndex := 0
	for selectionIndex, selection := range selections {
		left := selection.AmountMinor
		for left > 0 && cursorIndex < len(cursors) {
			cursor := &cursors[cursorIndex]
			used := minCreditAmount(left, cursor.remaining)
			allocation := cursor.allocation
			allocation.AmountMinor = used
			var err error
			allocation.ContentDigest, err = contentDigest(allocation)
			if err != nil {
				return nil, err
			}
			result[selectionIndex] = append(result[selectionIndex], allocation)
			left -= used
			cursor.remaining -= used
			if cursor.remaining == 0 {
				cursorIndex++
			}
		}
		if left != 0 {
			return nil, bizerr.NewCode(CodeRefundIntegrity)
		}
	}
	if cursorIndex != len(cursors) {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	return result, nil
}

func creditRefundExecutionSource(sourceType string) (string, string, bool) {
	switch sourceType {
	case "online_payment":
		return "channel_payment", "payment", true
	case "customer_balance":
		return "balance", "core", true
	case "manual_receipt":
		return "manual_receipt", "manual", true
	case "migration":
		return "migration", "manual", true
	default:
		return "", "", false
	}
}

func allocateRefundSources(
	amount int64,
	invoiceItemNo string,
	sources []refundPaymentSourceRow,
	remaining map[string]int64,
) ([]RefundSourceAllocation, error) {
	left := amount
	result := make([]RefundSourceAllocation, 0, 2)
	for _, source := range sources {
		available := remaining[source.AllocationNo]
		if available <= 0 || (source.InvoiceItemNo != nil && *source.InvoiceItemNo != invoiceItemNo) {
			continue
		}
		used := available
		if used > left {
			used = left
		}
		allocation := RefundSourceAllocation{
			PaymentAllocationNo:    source.AllocationNo,
			PaymentSourceType:      source.PaymentSourceTypeCode,
			PaymentSourceReference: source.PaymentSourceReference,
			ExecutionOwner:         refundExecutionOwner(source.PaymentSourceTypeCode),
			AmountMinor:            used,
		}
		var err error
		allocation.ContentDigest, err = contentDigest(allocation)
		if err != nil {
			return nil, err
		}
		result = append(result, allocation)
		remaining[source.AllocationNo] -= used
		left -= used
		if left == 0 {
			return result, nil
		}
	}
	return nil, bizerr.NewCode(CodeRefundNotEligible)
}

func refundPlaceholders(count int) string {
	values := make([]string, count)
	for index := range values {
		values[index] = "?"
	}
	return strings.Join(values, ",")
}

func uniqueRefundStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func audienceSafeCustomerNo(value string) string {
	return value
}
