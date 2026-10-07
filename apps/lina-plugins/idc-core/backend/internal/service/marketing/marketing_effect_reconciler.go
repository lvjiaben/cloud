// This file projects authoritative local marketing facts without calling external services.

package marketing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

type EffectReconciler interface {
	Run(context.Context, int) (int64, error)
}

type effectReconcilerImpl struct{ database gdb.DB }

func NewEffectReconciler(database gdb.DB) (EffectReconciler, error) {
	if database == nil {
		return nil, gerror.New("marketing effect reconciler requires a database")
	}
	return &effectReconcilerImpl{database: database}, nil
}

func (r *effectReconcilerImpl) Run(ctx context.Context, limit int) (int64, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > 1000 {
		return 0, gerror.New("marketing effect reconciliation input is invalid")
	}
	queries := []string{reconcileClaimFactsSQL, reconcileUseFactsSQL, reconcileOrderFactsSQL, reconcilePaymentFactsSQL, reconcileRefundFactsSQL, reconcileCostFactsSQL, reconcileCostReversalFactsSQL, reconcileRefundArbitrageSQL}
	var total int64
	for _, query := range queries {
		result, err := r.database.Exec(ctx, query, limit)
		if err != nil {
			return total, gerror.Wrap(err, "reconcile marketing effect facts")
		}
		count, err := result.RowsAffected()
		if err != nil {
			return total, gerror.Wrap(err, "read marketing effect reconciliation result")
		}
		total += count
	}
	return total, nil
}

const reconcileClaimFactsSQL = `
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('claim:' || asset.asset_no), asset.campaign_no, asset.customer_no,
       'claim', asset.currency_code, 0, 'promotion_asset', asset.asset_no,
       asset.benefit_snapshot_digest, asset.claimed_at
FROM plugin_idc_core_marketing_promotion_asset asset
WHERE asset.tenant_id = 0 AND asset.customer_no IS NOT NULL AND asset.claimed_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                  WHERE fact.source_type_code = 'promotion_asset' AND fact.source_reference = asset.asset_no
                    AND fact.event_type_code = 'claim' AND fact.campaign_no = asset.campaign_no)
ORDER BY asset.id LIMIT ?
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcileUseFactsSQL = `
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('use:' || reservation.reservation_no), reservation.campaign_no,
       reservation.customer_no, 'use', reservation.currency_code, 0, 'campaign_reservation',
       reservation.reservation_no, reservation.benefit_snapshot_digest, reservation.consumed_at
FROM plugin_idc_core_marketing_campaign_reservation reservation
WHERE reservation.tenant_id = 0 AND reservation.status_code = 'consumed'
  AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                  WHERE fact.source_type_code = 'campaign_reservation' AND fact.source_reference = reservation.reservation_no
                    AND fact.event_type_code = 'use' AND fact.campaign_no = reservation.campaign_no)
ORDER BY reservation.id LIMIT ?
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcileOrderFactsSQL = `
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('order:' || order_row.order_no || ':' || reservation.campaign_no),
       reservation.campaign_no, order_row.customer_no, 'order', order_row.currency_code, 0,
       'commerce_order', order_row.order_no, order_row.checkout_request_digest, order_row.created_at
FROM plugin_idc_core_commerce_order order_row
JOIN plugin_idc_core_marketing_campaign_reservation reservation
  ON reservation.quote_reference = order_row.quote_no AND reservation.status_code = 'consumed'
WHERE order_row.tenant_id = 0
  AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                  WHERE fact.source_type_code = 'commerce_order' AND fact.source_reference = order_row.order_no
                    AND fact.event_type_code = 'order' AND fact.campaign_no = reservation.campaign_no)
ORDER BY order_row.id, reservation.id LIMIT ?
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcilePaymentFactsSQL = `
WITH source AS (
  SELECT allocation.allocation_no, allocation.amount_minor, allocation.direction_code,
         allocation.content_digest, allocation.created_at, invoice.order_no,
         invoice.customer_no AS owner_no, invoice.currency_code AS owner_currency
  FROM plugin_idc_core_billing_payment_allocation allocation
  JOIN plugin_idc_core_billing_invoice invoice ON invoice.invoice_no = allocation.invoice_no
  WHERE invoice.order_no IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                    WHERE fact.source_type_code = 'payment_allocation' AND fact.source_reference = allocation.allocation_no)
  ORDER BY allocation.id LIMIT ?
), weights AS (
  SELECT source.allocation_no, source.amount_minor, source.direction_code, source.content_digest,
         source.created_at, source.owner_no, source.owner_currency, reservation.campaign_no,
         reservation.quoted_value_minor,
         sum(reservation.quoted_value_minor) OVER (PARTITION BY source.allocation_no) AS total_weight
  FROM source JOIN plugin_idc_core_commerce_order order_row ON order_row.order_no = source.order_no
  JOIN plugin_idc_core_marketing_campaign_reservation reservation
    ON reservation.quote_reference = order_row.quote_no AND reservation.status_code = 'consumed'
), bases AS (
  SELECT allocation_no, amount_minor, direction_code, content_digest, created_at, owner_no,
         owner_currency, campaign_no, quoted_value_minor, total_weight,
         floor(amount_minor::NUMERIC * quoted_value_minor / NULLIF(total_weight, 0))::BIGINT AS base_amount,
         row_number() OVER (PARTITION BY allocation_no ORDER BY quoted_value_minor DESC, campaign_no) AS allocation_rank
  FROM weights WHERE total_weight > 0
), allocated AS (
  SELECT allocation_no, direction_code, content_digest, created_at, owner_no, owner_currency,
         campaign_no, base_amount + CASE WHEN allocation_rank <= amount_minor - sum(base_amount) OVER (PARTITION BY allocation_no) THEN 1 ELSE 0 END AS allocated_amount
  FROM bases
)
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('payment:' || allocation_no || ':' || campaign_no), campaign_no, owner_no,
       CASE WHEN direction_code = 'apply' THEN 'receipt' ELSE 'receipt_reversal' END,
       owner_currency, allocated_amount, 'payment_allocation', allocation_no, content_digest, created_at
FROM allocated WHERE allocated_amount > 0
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcileRefundFactsSQL = `
WITH source AS (
  SELECT allocation.refund_allocation_no, allocation.amount_minor, allocation.direction_code,
         allocation.content_digest, allocation.created_at, invoice.order_no,
         invoice.customer_no AS owner_no, invoice.currency_code AS owner_currency
  FROM plugin_idc_core_billing_refund_allocation allocation
  JOIN plugin_idc_core_billing_invoice invoice ON invoice.invoice_no = allocation.invoice_no
  WHERE invoice.order_no IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                    WHERE fact.source_type_code = 'refund_allocation' AND fact.source_reference = allocation.refund_allocation_no)
  ORDER BY allocation.id LIMIT ?
), weights AS (
  SELECT source.refund_allocation_no, source.amount_minor, source.direction_code,
         source.content_digest, source.created_at, source.owner_no, source.owner_currency,
         reservation.campaign_no, reservation.quoted_value_minor,
         sum(reservation.quoted_value_minor) OVER (PARTITION BY source.refund_allocation_no) AS total_weight
  FROM source JOIN plugin_idc_core_commerce_order order_row ON order_row.order_no = source.order_no
  JOIN plugin_idc_core_marketing_campaign_reservation reservation
    ON reservation.quote_reference = order_row.quote_no AND reservation.status_code = 'consumed'
), bases AS (
  SELECT refund_allocation_no, amount_minor, direction_code, content_digest, created_at,
         owner_no, owner_currency, campaign_no, quoted_value_minor, total_weight,
         floor(amount_minor::NUMERIC * quoted_value_minor / NULLIF(total_weight, 0))::BIGINT AS base_amount,
         row_number() OVER (PARTITION BY refund_allocation_no ORDER BY quoted_value_minor DESC, campaign_no) AS allocation_rank
  FROM weights WHERE total_weight > 0
), allocated AS (
  SELECT refund_allocation_no, direction_code, content_digest, created_at, owner_no,
         owner_currency, campaign_no, base_amount + CASE WHEN allocation_rank <= amount_minor - sum(base_amount) OVER (PARTITION BY refund_allocation_no) THEN 1 ELSE 0 END AS allocated_amount
  FROM bases
)
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('refund:' || refund_allocation_no || ':' || campaign_no), campaign_no, owner_no,
       CASE WHEN direction_code = 'apply' THEN 'refund' ELSE 'refund_reversal' END,
       owner_currency, allocated_amount, 'refund_allocation', refund_allocation_no, content_digest, created_at
FROM allocated WHERE allocated_amount > 0
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcileCostFactsSQL = `
WITH source AS (
  SELECT order_row.order_no, order_row.quote_no, order_row.customer_no, order_row.currency_code,
         order_row.created_at, sum(cost.total_cost_minor)::BIGINT AS amount_minor
  FROM plugin_idc_core_commerce_order order_row
  JOIN plugin_idc_core_commerce_order_cost_snapshot cost ON cost.order_no = order_row.order_no
  WHERE NOT EXISTS (SELECT 1 FROM plugin_idc_core_commerce_order_cost_snapshot missing
                    WHERE missing.order_no = order_row.order_no AND missing.status_code = 'missing')
    AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                    WHERE fact.source_type_code = 'order_cost' AND fact.source_reference = order_row.order_no)
  GROUP BY order_row.order_no, order_row.quote_no, order_row.customer_no, order_row.currency_code, order_row.created_at, order_row.id
  ORDER BY order_row.id LIMIT ?
), weights AS (
  SELECT source.order_no, source.quote_no, source.customer_no, source.currency_code,
         source.created_at, source.amount_minor, reservation.campaign_no,
         reservation.quoted_value_minor,
         sum(reservation.quoted_value_minor) OVER (PARTITION BY source.order_no) AS total_weight
  FROM source JOIN plugin_idc_core_marketing_campaign_reservation reservation
    ON reservation.quote_reference = source.quote_no AND reservation.status_code = 'consumed'
), bases AS (
  SELECT order_no, customer_no, currency_code, created_at, amount_minor, campaign_no,
         quoted_value_minor, total_weight,
         floor(amount_minor::NUMERIC * quoted_value_minor / NULLIF(total_weight, 0))::BIGINT AS base_amount,
         row_number() OVER (PARTITION BY order_no ORDER BY quoted_value_minor DESC, campaign_no) AS allocation_rank
  FROM weights WHERE total_weight > 0
), allocated AS (
  SELECT order_no, customer_no, currency_code, created_at, campaign_no,
         base_amount + CASE WHEN allocation_rank <= amount_minor - sum(base_amount) OVER (PARTITION BY order_no) THEN 1 ELSE 0 END AS allocated_amount
  FROM bases
)
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('cost:' || order_no || ':' || campaign_no), campaign_no, customer_no,
       'cost', currency_code, allocated_amount, 'order_cost', order_no,
       md5('cost:' || order_no) || md5(order_no || ':cost'), created_at
FROM allocated WHERE allocated_amount > 0
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcileCostReversalFactsSQL = `
WITH source AS (
  SELECT reversal.reversal_no, reversal.currency_code, reversal.amount_minor,
         reversal.content_digest, reversal.occurred_at, cost.order_no,
         order_row.quote_no, order_row.customer_no
  FROM plugin_idc_core_commerce_order_cost_reversal reversal
  JOIN plugin_idc_core_commerce_order_cost_snapshot cost ON cost.order_item_no = reversal.order_item_no
  JOIN plugin_idc_core_commerce_order order_row ON order_row.order_no = cost.order_no
  WHERE NOT EXISTS (SELECT 1 FROM plugin_idc_core_marketing_effect_fact fact
                    WHERE fact.source_type_code = 'cost_reversal' AND fact.source_reference = reversal.reversal_no)
  ORDER BY reversal.id LIMIT ?
), weights AS (
  SELECT source.reversal_no, source.currency_code, source.amount_minor,
         source.content_digest, source.occurred_at, source.customer_no,
         reservation.campaign_no, reservation.quoted_value_minor,
         sum(reservation.quoted_value_minor) OVER (PARTITION BY source.reversal_no) AS total_weight
  FROM source JOIN plugin_idc_core_marketing_campaign_reservation reservation
    ON reservation.quote_reference = source.quote_no AND reservation.status_code = 'consumed'
), bases AS (
  SELECT reversal_no, currency_code, amount_minor, content_digest, occurred_at,
         customer_no, campaign_no, quoted_value_minor, total_weight,
         floor(amount_minor::NUMERIC * quoted_value_minor / NULLIF(total_weight, 0))::BIGINT AS base_amount,
         row_number() OVER (PARTITION BY reversal_no ORDER BY quoted_value_minor DESC, campaign_no) AS allocation_rank
  FROM weights WHERE total_weight > 0
), allocated AS (
  SELECT reversal_no, currency_code, content_digest, occurred_at, customer_no,
         campaign_no, base_amount + CASE WHEN allocation_rank <= amount_minor - sum(base_amount) OVER (PARTITION BY reversal_no) THEN 1 ELSE 0 END AS allocated_amount
  FROM bases
)
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
SELECT 0, 'mfx_' || md5('cost-reversal:' || reversal_no || ':' || campaign_no), campaign_no, customer_no,
       'cost_reversal', currency_code, allocated_amount, 'cost_reversal', reversal_no, content_digest, occurred_at
FROM allocated WHERE allocated_amount > 0
ON CONFLICT (source_type_code, source_reference, event_type_code, campaign_no) DO NOTHING`

const reconcileRefundArbitrageSQL = `
WITH marketing_orders AS (
  SELECT DISTINCT order_row.order_no, order_row.quote_no, order_row.customer_no, order_row.created_at,
         order_row.status_code
  FROM plugin_idc_core_commerce_order order_row
  JOIN plugin_idc_core_marketing_campaign_reservation reservation
    ON reservation.quote_reference=order_row.quote_no AND reservation.status_code='consumed'
  WHERE order_row.tenant_id=0 AND order_row.created_at>=CURRENT_TIMESTAMP-INTERVAL '30 days'
), refunded_orders AS (
  SELECT orders.order_no, orders.quote_no, orders.customer_no
  FROM marketing_orders orders
  WHERE orders.status_code IN ('partially_refunded','refunded')
), customer_counts AS (
  SELECT orders.customer_no, count(DISTINCT orders.order_no)::INT AS order_count,
         count(DISTINCT refunded.order_no)::INT AS refunded_count
  FROM marketing_orders orders
  LEFT JOIN refunded_orders refunded ON refunded.order_no=orders.order_no
  GROUP BY orders.customer_no
  HAVING count(DISTINCT refunded.order_no)>=3
     AND count(DISTINCT refunded.order_no)*100>=count(DISTINCT orders.order_no)*80
), campaign_evidence AS (
  SELECT refunded.customer_no, reservation.campaign_no, counts.order_count, counts.refunded_count,
         string_agg(DISTINCT refunded.order_no, ',' ORDER BY refunded.order_no) AS refunded_orders
  FROM refunded_orders refunded
  JOIN customer_counts counts ON counts.customer_no=refunded.customer_no
  JOIN plugin_idc_core_marketing_campaign_reservation reservation
    ON reservation.quote_reference=refunded.quote_no AND reservation.status_code='consumed'
  GROUP BY refunded.customer_no, reservation.campaign_no, counts.order_count, counts.refunded_count
  ORDER BY refunded.customer_no, reservation.campaign_no
  LIMIT ?
), candidates AS (
  SELECT evidence.customer_no, evidence.campaign_no, evidence.order_count,
         evidence.refunded_count, evidence.refunded_orders,
         md5('refund-arbitrage:'||evidence.customer_no||':'||evidence.campaign_no||':'||evidence.refunded_orders)||
         md5(evidence.refunded_orders||':'||evidence.order_count::TEXT||':'||evidence.refunded_count::TEXT) AS evidence_digest
  FROM campaign_evidence evidence
)
INSERT INTO plugin_idc_core_marketing_abuse_case
 (tenant_id,case_no,campaign_no,customer_no,signal_code,related_account_count,evidence_summary_code,
  evidence_digest,window_started_at,window_ended_at,observed_count)
SELECT 0,'mab_'||left(candidate.evidence_digest,32),candidate.campaign_no,candidate.customer_no,
       'refund_arbitrage',0,'refund_ratio_threshold',candidate.evidence_digest,
       CURRENT_TIMESTAMP-INTERVAL '30 days',CURRENT_TIMESTAMP,candidate.refunded_count
FROM candidates candidate
WHERE NOT EXISTS (
  SELECT 1 FROM plugin_idc_core_marketing_abuse_case existing
  WHERE existing.tenant_id=0 AND existing.customer_no=candidate.customer_no
    AND existing.campaign_no=candidate.campaign_no AND existing.signal_code='refund_arbitrage'
    AND existing.status_code IN ('pending_review','confirmed_abuse')
)
ON CONFLICT DO NOTHING`
