// This file freezes payment-time beneficiaries and rules, then posts bounded,
// replay-safe commission accruals using authoritative invoice and settlement data.
package commission

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"sort"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

const (
	operationPurchase       = "new_purchase"
	operationRenewal        = "renewal"
	beneficiarySales        = "sales"
	beneficiaryAffiliate    = "affiliate"
	statusPending           = "pending_maturity"
	statusMatured           = "matured"
	statusReversed          = "reversed"
	statusPartiallyReversed = "partially_reversed"
	accountActive           = "active"
	entryAccrual            = "accrual"
	entryReversal           = "reversal"
	sourceAccrual           = "commission_accrual"
	sourceReversal          = "commission_refund"
	maximumInvoiceItems     = 64
)

type commissionItem struct {
	InvoiceItemNo string `orm:"invoice_item_no"`
	ProductNo     string `orm:"product_no"`
	Amount        int64  `orm:"total_amount_minor"`
	Basis         int64
}
type beneficiary struct {
	Kind, Reference, Assignment, Attribution string
	ManualReferrerID                         int64
}
type accrualGroup struct {
	Beneficiary beneficiary
	Rule        entity.SalesCommissionRule
	Items       []commissionItem
	Basis       int64
	Tiers       []entity.SalesCommissionTier
}

// consumeAccrual accepts only the dedicated settlement event; fulfillment keeps its own event.
func (c *consumer) consumeAccrual(ctx context.Context, event outbox.ClaimedEvent) error {
	if event.EventVersion != 1 || event.Topic != AccrualEventType || event.AggregateType != "commerce_order" || !validReference(event.AggregateNo, 40) {
		return permanent("commission_accrual_event_invalid")
	}
	var payload struct {
		SettlementNo     string `json:"settlementNo"`
		OrderNo          string `json:"orderNo"`
		InvoiceNo        string `json:"invoiceNo"`
		CustomerNo       string `json:"customerNo"`
		CurrencyCode     string `json:"currencyCode"`
		BasisCode        string `json:"basisCode"`
		TotalAmountMinor int64  `json:"totalAmountMinor"`
		SettledAtUnixMS  int64  `json:"settledAtUnixMs"`
	}
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || payload.OrderNo != event.AggregateNo || !validReference(payload.InvoiceNo, 40) {
		return permanent("commission_accrual_event_invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return permanent("commission_accrual_event_invalid")
	}
	var settlement *entity.FinancialSettlement
	if err := c.database.Model(dao.FinancialSettlement.Table()).Ctx(ctx).Where(do.FinancialSettlement{TenantId: 0, SettlementNo: payload.SettlementNo}).Scan(&settlement); err != nil {
		return retryable(err)
	}
	if settlement == nil || settlement.OrderNo != payload.OrderNo || settlement.InvoiceNo != payload.InvoiceNo || settlement.CustomerNo != payload.CustomerNo || settlement.CurrencyCode != payload.CurrencyCode || settlement.BasisCode != payload.BasisCode || settlement.TotalAmountMinor != payload.TotalAmountMinor || settlement.SettledAt == nil || settlement.SettledAt.UnixMilli() != payload.SettledAtUnixMS {
		return permanent("commission_accrual_event_mismatch")
	}

	return c.accrueInvoice(ctx, payload.InvoiceNo, payload.SettlementNo)
}

// AccrueRenewals is a durable bounded poll: a receipt is written even if no rule matches.
func (c *consumer) AccrueRenewals(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > maximumWorkerBatch {
		return 0, permanent("commission_batch_invalid")
	}
	var rows []struct {
		InvoiceNo string `orm:"invoice_no"`
	}
	err := c.database.GetScan(ctx, &rows, `SELECT i.invoice_no FROM plugin_idc_core_billing_invoice i
 WHERE i.tenant_id=0 AND i.source_code='renewal' AND i.settled_at IS NOT NULL
 AND (i.paid_amount_minor>0 OR (i.total_amount_minor=0 AND i.status_code='paid'))
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commission_order_receipt r WHERE r.invoice_no=i.invoice_no)
 ORDER BY i.settled_at,i.id LIMIT ?`, limit)
	if err != nil {
		return 0, retryable(err)
	}
	for i, row := range rows {
		if err = c.accrueInvoice(ctx, row.InvoiceNo, ""); err != nil {
			return i, err
		}
	}
	return len(rows), nil
}

// accrueInvoice serializes all commission effects on the invoice, including early refunds.
func (c *consumer) accrueInvoice(ctx context.Context, invoiceNo, settlementNo string) error {
	return c.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var invoice *entity.BillingInvoice
		if err := c.database.Model(dao.BillingInvoice.Table()).Ctx(ctx).Where(do.BillingInvoice{TenantId: 0, InvoiceNo: invoiceNo}).LockUpdate().Scan(&invoice); err != nil {
			return retryable(err)
		}
		if invoice == nil {
			return permanent("commission_invoice_missing")
		}
		return c.accrueLockedInvoice(ctx, invoice, settlementNo)
	})
}

func (c *consumer) accrueLockedInvoice(ctx context.Context, invoice *entity.BillingInvoice, expectedSettlement string) error {
	var receipt *entity.CommissionOrderReceipt
	if err := c.database.Model(dao.CommissionOrderReceipt.Table()).Ctx(ctx).Where(do.CommissionOrderReceipt{InvoiceNo: invoice.InvoiceNo}).Scan(&receipt); err != nil {
		return err
	}
	if receipt != nil {
		return nil
	}
	var (
		settlementNo        string
		digest              string
		paid                int64
		settledAt           time.Time
		operation           = operationPurchase
		performanceEligible bool
	)
	if invoice.OrderNo != nil {
		var settlement *entity.FinancialSettlement
		if err := c.database.Model(dao.FinancialSettlement.Table()).Ctx(ctx).Where(do.FinancialSettlement{TenantId: 0, OrderNo: *invoice.OrderNo}).Scan(&settlement); err != nil {
			return err
		}
		if settlement == nil {
			return retryable(gerror.New("commission settlement is not yet committed"))
		}
		if settlement.InvoiceNo != invoice.InvoiceNo || settlement.CustomerNo != invoice.CustomerNo || settlement.CurrencyCode != invoice.CurrencyCode || (expectedSettlement != "" && settlement.SettlementNo != expectedSettlement) || settlement.SettledAt == nil {
			return permanent("commission_settlement_mismatch")
		}
		settlementNo, digest, settledAt = settlement.SettlementNo, settlement.ContentDigest, *settlement.SettledAt
		if settlement.BasisCode == string(billing.SettlementBasisFundedInvoice) {
			paid = settlement.PaidAmountMinor
		}
		performanceEligible = settlement.BasisCode == string(billing.SettlementBasisFundedInvoice) ||
			(settlement.TotalAmountMinor == 0 && (settlement.BasisCode == string(billing.SettlementBasisZeroAmount) || settlement.BasisCode == string(billing.SettlementBasisFullDiscount) || settlement.BasisCode == string(billing.SettlementBasisTrial)))
	} else {
		if invoice.SourceCode != operationRenewal {
			return nil
		}
		if invoice.SettledAt == nil {
			return permanent("commission_invoice_unsettled")
		}
		operation, settledAt = operationRenewal, *invoice.SettledAt
		value, err := c.database.GetValue(ctx, `SELECT COALESCE(SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END),0) FROM plugin_idc_core_billing_payment_allocation WHERE tenant_id=0 AND invoice_no=? AND payment_source_type_code IN ('balance','channel_payment','manual_receipt')`, invoice.InvoiceNo)
		if err != nil {
			return err
		}
		paid = value.Int64()
		performanceEligible = paid > 0 || (invoice.TotalAmountMinor == 0 && invoice.StatusCode == string(billing.InvoiceStatusPaid))
		settlementNo = "INV_" + commissionDigest(invoice.InvoiceNo)[:32]
		digest = commissionDigest(invoice.ContentDigest + ":" + strconv.FormatInt(paid, 10) + ":" + strconv.FormatInt(settledAt.UnixMilli(), 10))
	}
	if paid < 0 || paid > invoice.TotalAmountMinor {
		return permanent("commission_payment_invalid")
	}
	var beneficiaries []beneficiary
	if performanceEligible {
		var err error
		beneficiaries, err = c.freezePaymentFacts(ctx, invoice, settlementNo, digest, operation, paid, settledAt)
		if err != nil {
			return err
		}
	}
	if paid > 0 {
		items, err := c.loadItems(ctx, invoice, paid)
		if err != nil {
			return err
		}
		groups, err := c.prepareGroups(ctx, invoice.CurrencyCode, operation, settledAt, items, beneficiaries)
		if err != nil {
			return err
		}
		for _, group := range groups {
			if err = c.postGroup(ctx, invoice, settledAt, operation, group); err != nil {
				return err
			}
		}
	}
	_, err := c.database.Model(dao.CommissionOrderReceipt.Table()).Ctx(ctx).Data(do.CommissionOrderReceipt{TenantId: 0, OrderNo: invoice.OrderNo, InvoiceNo: invoice.InvoiceNo, SettlementNo: settlementNo, SourceDigest: digest}).Insert()
	return err
}

func (c *consumer) loadItems(ctx context.Context, invoice *entity.BillingInvoice, paid int64) ([]commissionItem, error) {
	var items []commissionItem
	err := c.database.GetScan(ctx, &items, `SELECT i.invoice_item_no,i.total_amount_minor,COALESCE(o.product_no,e.product_no,'') AS product_no
 FROM plugin_idc_core_billing_invoice_item i
 LEFT JOIN plugin_idc_core_commerce_order_item o ON o.order_item_no=i.order_item_no
 LEFT JOIN plugin_idc_core_service_entitlement e ON i.item_type_code='renewal' AND e.entitlement_no=i.source_reference
 WHERE i.tenant_id=0 AND i.invoice_no=? ORDER BY i.line_number,i.id LIMIT ?`, invoice.InvoiceNo, maximumInvoiceItems+1)
	if err != nil {
		return nil, err
	}
	if len(items) > maximumInvoiceItems || len(items) == 0 {
		return nil, permanent("commission_invoice_items_invalid")
	}
	weights := make([]int64, len(items))
	var total int64
	for i, item := range items {
		if item.Amount < 0 || item.ProductNo == "" {
			return nil, permanent("commission_product_missing")
		}
		weights[i] = item.Amount
		total += item.Amount
	}
	if total != invoice.TotalAmountMinor {
		return nil, permanent("commission_invoice_total_mismatch")
	}
	portions, err := allocateProportionally(paid, weights)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Basis = portions[i]
	}
	return items, nil
}

// prepareGroups loads rule revisions and tiers in sets; fixed commission is calculated once per rule per invoice.
func (c *consumer) prepareGroups(ctx context.Context, currency, operation string, at time.Time, items []commissionItem, owners []beneficiary) ([]accrualGroup, error) {
	if len(owners) == 0 {
		return nil, nil
	}
	products := make([]string, 0, len(items))
	for _, item := range items {
		products = append(products, item.ProductNo)
	}
	ownerTypes := make([]string, 0, len(owners))
	for _, owner := range owners {
		ownerTypes = append(ownerTypes, owner.Kind)
	}
	var rules []entity.SalesCommissionRule
	err := c.database.Model(dao.SalesCommissionRule.Table()).Ctx(ctx).Where(do.SalesCommissionRule{TenantId: 0, CurrencyCode: currency, OperationTypeCode: operation}).WhereIn("beneficiary_type_code", ownerTypes).Where("(product_no IS NULL OR product_no IN (?))", products).WhereLTE("effective_at", at).Where("NOT EXISTS (SELECT 1 FROM plugin_idc_core_sales_commission_rule n WHERE n.tenant_id=plugin_idc_core_sales_commission_rule.tenant_id AND n.beneficiary_type_code=plugin_idc_core_sales_commission_rule.beneficiary_type_code AND n.operation_type_code=plugin_idc_core_sales_commission_rule.operation_type_code AND n.currency_code=plugin_idc_core_sales_commission_rule.currency_code AND COALESCE(n.product_no,'')=COALESCE(plugin_idc_core_sales_commission_rule.product_no,'') AND n.effective_at<=? AND n.version>plugin_idc_core_sales_commission_rule.version)", at).OrderDesc("version").OrderDesc("priority").Scan(&rules)
	if err != nil {
		return nil, err
	}
	latest := map[string]entity.SalesCommissionRule{}
	for _, rule := range rules {
		scope := rule.BeneficiaryTypeCode + ":"
		if rule.ProductNo != nil {
			scope += *rule.ProductNo
		}
		if _, exists := latest[scope]; !exists {
			latest[scope] = rule
		}
	}
	groups := map[string]*accrualGroup{}
	for _, owner := range owners {
		for _, item := range items {
			if item.Basis <= 0 {
				continue
			}
			rule, ok := latest[owner.Kind+":"+item.ProductNo]
			if !ok {
				rule, ok = latest[owner.Kind+":"]
			}
			if !ok || rule.StatusCode != "published" || (rule.ExpiresAt != nil && !rule.ExpiresAt.After(at)) {
				continue
			}
			key := owner.Kind + ":" + owner.Reference + ":" + rule.RuleNo
			group := groups[key]
			if group == nil {
				group = &accrualGroup{Beneficiary: owner, Rule: rule}
				groups[key] = group
			}
			group.Items = append(group.Items, item)
			group.Basis += item.Basis
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ruleNos := make([]string, 0, len(keys))
	for _, key := range keys {
		if groups[key].Rule.CalculationTypeCode == saleschannel.CommissionTiered {
			ruleNos = append(ruleNos, groups[key].Rule.RuleNo)
		}
	}
	var tiers []entity.SalesCommissionTier
	if len(ruleNos) > 0 {
		if err = c.database.Model(dao.SalesCommissionTier.Table()).Ctx(ctx).WhereIn("rule_no", ruleNos).Scan(&tiers); err != nil {
			return nil, err
		}
	}
	tiersByRule := map[string][]entity.SalesCommissionTier{}
	for _, tier := range tiers {
		tiersByRule[tier.RuleNo] = append(tiersByRule[tier.RuleNo], tier)
	}
	result := make([]accrualGroup, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		group.Tiers = tiersByRule[group.Rule.RuleNo]
		result = append(result, *group)
	}

	return result, nil
}

func (c *consumer) postGroup(ctx context.Context, invoice *entity.BillingInvoice, at time.Time, operation string, group accrualGroup) error {
	snapshot, err := c.rules.CalculateCommission(ctx, saleschannel.CommissionRequest{Now: at, OperationTypeCode: operation, BeneficiaryTypeCode: group.Beneficiary.Kind, ProductNo: group.Items[0].ProductNo, CurrencyCode: invoice.CurrencyCode, BaseAmountMinor: group.Basis, Rule: &group.Rule, Tiers: group.Tiers})
	if err != nil {
		return err
	}
	if snapshot.AmountMinor <= 0 {
		return nil
	}
	weights := make([]int64, len(group.Items))
	for i, item := range group.Items {
		weights[i] = item.Basis
	}
	amounts, err := allocateProportionally(snapshot.AmountMinor, weights)
	if err != nil {
		return err
	}
	for i, item := range group.Items {
		if amounts[i] == 0 {
			continue
		}
		number := "COM_" + commissionDigest(invoice.InvoiceNo + ":" + item.InvoiceItemNo + ":" + group.Beneficiary.Kind + ":" + group.Beneficiary.Reference + ":" + group.Rule.RuleNo)[:32]
		data := do.CommissionAccrual{TenantId: 0, CommissionNo: number, OrderNo: invoice.OrderNo, InvoiceNo: invoice.InvoiceNo, InvoiceItemNo: item.InvoiceItemNo, BeneficiaryTypeCode: group.Beneficiary.Kind, BeneficiaryReference: group.Beneficiary.Reference, CurrencyCode: invoice.CurrencyCode, RuleVersion: snapshot.RuleVersion, BasisAmountMinor: item.Basis, InvoiceItemAmountMinor: item.Amount, OriginalAmountMinor: amounts[i], PendingAmountMinor: amounts[i], MaturedAmountMinor: 0, ReversedAmountMinor: 0, StatusCode: statusPending, MaturesAt: &snapshot.MaturesAt, CommissionRuleNo: snapshot.RuleNo, OperationTypeCode: operation}
		if group.Beneficiary.Assignment != "" {
			data.SalesAssignmentNo = group.Beneficiary.Assignment
		}
		if group.Beneficiary.Attribution != "" {
			data.AffiliateAttributionNo = group.Beneficiary.Attribution
		}
		if snapshot.TierNo != "" {
			data.CommissionTierNo = snapshot.TierNo
		}
		if _, err = c.database.Model(dao.CommissionAccrual.Table()).Ctx(ctx).Data(data).Insert(); err != nil {
			return err
		}
		account, err := c.lockAccount(ctx, group.Beneficiary, invoice.CurrencyCode, true)
		if err != nil {
			return err
		}
		if err = c.appendAccountEntry(ctx, account, entryAccrual, sourceAccrual, number, amounts[i], amounts[i], 0, 0, 0); err != nil {
			return err
		}
	}
	return nil
}

func commissionDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// allocateProportionally distributes integer remainder in stable item order without overflow.
func allocateProportionally(amount int64, weights []int64) ([]int64, error) {
	total := new(big.Int)
	for _, weight := range weights {
		if weight < 0 {
			return nil, permanent("commission_amount_invalid")
		}
		total.Add(total, big.NewInt(weight))
	}
	if amount < 0 || total.Sign() <= 0 {
		return nil, permanent("commission_amount_invalid")
	}
	result := make([]int64, len(weights))
	remaining := amount
	for i, weight := range weights {
		part := new(big.Int).Mul(big.NewInt(amount), big.NewInt(weight))
		part.Quo(part, total)
		if !part.IsInt64() {
			return nil, permanent("commission_amount_overflow")
		}
		result[i] = part.Int64()
		remaining -= result[i]
	}
	for i, weight := range weights {
		if remaining == 0 {
			break
		}
		if weight > 0 {
			result[i]++
			remaining--
		}
	}
	return result, nil
}

// Mature delegates the due transition to the shared sales owner.
func (c *consumer) Mature(ctx context.Context, limit int) (int, error) {
	matured, err := c.rules.MatureDueCommissions(ctx, time.Now(), limit)
	if err != nil {
		return matured, err
	}
	recovered, err := c.recoverAvailable(ctx, limit)
	return matured + recovered, err
}
