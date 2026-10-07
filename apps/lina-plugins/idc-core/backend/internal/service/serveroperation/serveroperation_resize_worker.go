package serveroperation

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type dueResizeRow struct {
	OperationNo, ServiceNo, CustomerNo, PoolNo, ChannelNo, RuntimeStatus, Parameters, SourceConfigurationNo string
	ExpectedServiceVersion, CurrentServiceVersion                                                           int64
}

func (w *workerImpl) materializeDueResizeDowngrades(ctx context.Context, limit int) error {
	for i := 0; i < limit; i++ {
		hadWork := false
		err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			var row *dueResizeRow
			if err := w.database.GetScan(ctx, &row, `SELECT operation.operation_no,operation.service_no,operation.customer_no,
operation.pool_no,COALESCE(service.channel_no,'') AS channel_no,service.runtime_status_code AS runtime_status,
quote.provider_parameters::TEXT AS parameters,quote.source_configuration_snapshot_no,
operation.expected_service_version,service.version AS current_service_version
FROM plugin_idc_core_server_resize_intent intent
JOIN plugin_idc_core_business_operation operation ON operation.tenant_id=intent.tenant_id AND operation.operation_no=intent.operation_no
JOIN plugin_idc_core_server_resize_quote quote ON quote.tenant_id=intent.tenant_id AND quote.quote_no=intent.quote_no
JOIN plugin_idc_core_service_instance service ON service.tenant_id=intent.tenant_id AND service.service_no=intent.service_no
WHERE intent.tenant_id=0 AND intent.direction_code='downgrade' AND intent.status_code='scheduled'
 AND intent.effective_at<=CURRENT_TIMESTAMP ORDER BY intent.effective_at,intent.id LIMIT 1
FOR UPDATE OF intent,operation,service SKIP LOCKED`); err != nil {
				return err
			}
			if row == nil {
				return nil
			}
			hadWork = true
			now, err := databaseNow(ctx, w.database)
			if err != nil {
				return err
			}
			if row.ChannelNo == "" || row.ExpectedServiceVersion != row.CurrentServiceVersion {
				return w.freezeDueResize(ctx, row.OperationNo, "server_resize.schedule_conflict", now)
			}
			current, err := w.database.GetValue(ctx, `SELECT configuration_snapshot_no FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND service_no=?`, row.ServiceNo)
			if err != nil {
				return err
			}
			if current.String() != row.SourceConfigurationNo {
				return w.freezeDueResize(ctx, row.OperationNo, "server_resize.configuration_changed", now)
			}
			request := serveroperationresourcecap.TaskRequest{OperationNo: row.OperationNo, ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo,
				Action: serveroperationresourcecap.ActionResize, ExpectedServiceVersion: row.CurrentServiceVersion, Parameters: []byte(row.Parameters)}
			digest, err := serveroperationresourcecap.TaskRequestDigest(request)
			if err != nil {
				return err
			}
			executionNo, _ := deterministicReference("rox_", row.OperationNo)
			if _, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_operation_execution
(tenant_id,execution_no,operation_no,service_no,action_code,resource_owner_code,channel_no,pool_no,expected_service_version,
original_runtime_status_code,parameters,secret_bindings,status_code,request_digest,version,created_at,updated_at)
VALUES (0,?,?,?,'resize','idc-resource',?,?,?,?,?::JSONB,'[]'::JSONB,'ready',?,1,?,?)`, executionNo, row.OperationNo, row.ServiceNo,
				row.ChannelNo, row.PoolNo, row.CurrentServiceVersion, row.RuntimeStatus, row.Parameters, digest, now, now); err != nil {
				return err
			}
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation SET status_code='ready',updated_at=?,version=version+1
WHERE tenant_id=0 AND operation_no=? AND status_code='quoted'`, now, row.OperationNo); err != nil {
				return err
			}
			if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent SET status_code='ready',updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code='scheduled'`, now, row.OperationNo); err != nil {
				return err
			}
			from := "quoted"
			return insertEvent(ctx, w.database, row.OperationNo, row.ServiceNo, 2, &from, "ready", "server_resize.scheduled_effective", "system", "server-operation-worker", digest, now)
		})
		if err != nil {
			return err
		}
		if !hadWork {
			break
		}
	}
	return nil
}

func (w *workerImpl) freezeDueResize(ctx context.Context, operationNo, code string, now time.Time) error {
	if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent SET status_code='manual_review',updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code='scheduled'`, now, operationNo); err != nil {
		return err
	}
	return execOne(ctx, w.database, `UPDATE plugin_idc_core_business_operation SET status_code='manual_review',last_error_code=?,updated_at=?,version=version+1
WHERE tenant_id=0 AND operation_no=? AND status_code='quoted'`, code, now, operationNo)
}

func (w *workerImpl) finishResizeSuccess(ctx context.Context, row *workRow, task *serveroperationresourcecap.Task,
	runtimeStatus, providerStatus string, now time.Time) error {
	var intent *struct {
		TargetConfigurationNo, TargetVersionNo, TargetBillingRuleNo, TargetPriceNo, CurrencyCode, CustomerNo, PeriodUnit string
		PeriodCount                                                                                                      int
		TargetRecurring, TargetSubtotal, TargetNet, TargetTax                                                            int64
		QuoteNo                                                                                                          string
	}
	if err := w.database.GetScan(ctx, &intent, `SELECT intent.target_configuration_snapshot_no AS target_configuration_no,intent.target_version_no,
intent.target_billing_rule_no,intent.target_price_snapshot_no AS target_price_no,quote.currency_code,quote.customer_no,
service.period_unit_code AS period_unit,service.period_count,quote.target_recurring_amount_minor AS target_recurring,
quote.target_subtotal_minor AS target_subtotal,quote.target_net_amount_minor AS target_net,quote.target_tax_amount_minor AS target_tax,
quote.quote_no FROM plugin_idc_core_server_resize_intent intent
JOIN plugin_idc_core_server_resize_quote quote ON quote.tenant_id=intent.tenant_id AND quote.quote_no=intent.quote_no
JOIN plugin_idc_core_service_instance service ON service.tenant_id=intent.tenant_id AND service.service_no=intent.service_no
WHERE intent.tenant_id=0 AND intent.operation_no=? FOR UPDATE OF intent`, row.OperationNo); err != nil {
		return err
	}
	if intent == nil {
		return ErrOperationConflict
	}
	var tax *struct {
		PolicyNo, Treatment, Category, Scope, Mode string
		Rate                                       *int
		Quantum                                    int64
	}
	if err := w.database.GetScan(ctx, &tax, `SELECT tax_policy_no AS policy_no,tax_treatment_code AS treatment,tax_rate_basis_points AS rate,
tax_category_code AS category,rounding_scope_code AS scope,rounding_mode_code AS mode,rounding_quantum_minor AS quantum
FROM plugin_idc_core_catalog_tax_policy WHERE tenant_id=0 AND version_no=? AND currency_code=? AND status_code='active'
ORDER BY id LIMIT 1`, intent.TargetVersionNo, intent.CurrencyCode); err != nil {
		return err
	}
	if tax == nil {
		return ErrOperationConflict
	}
	pricingDigest, _ := digestValue(struct {
		QuoteNo, Evidence string
		Amount            int64
	}{intent.QuoteNo, task.EvidenceDigest, intent.TargetRecurring})
	contentDigest, _ := digestValue(struct{ PriceNo, ConfigNo, Evidence string }{intent.TargetPriceNo, intent.TargetConfigurationNo, task.EvidenceDigest})
	componentCount := 1
	if intent.TargetTax > 0 {
		componentCount++
	}
	if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
(tenant_id,price_snapshot_no,service_no,customer_no,phase_code,product_no,version_no,billing_rule_no,configuration_snapshot_no,
currency_code,purchase_cycles,period_unit_code,period_count,renewal_mode_code,capability_evidence_no,capability_evidence_digest,
capability_evidence_observed_at,capability_evidence_expires_at,tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,
tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,component_count,subtotal_minor,discount_minor,net_amount_minor,
tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
SELECT 0,?,?,?,'resize_target',product_no,?,?,?,?,1,?,?,'provider_confirmed',?,?,?,?,?,TRUE,?,?,?,?,?,?,?, ?,0,?,?,?,?,?
FROM plugin_idc_core_catalog_product_version WHERE tenant_id=0 AND version_no=?`, intent.TargetPriceNo, row.ServiceNo, intent.CustomerNo,
		intent.TargetVersionNo, intent.TargetBillingRuleNo, intent.TargetConfigurationNo, intent.CurrencyCode, intent.PeriodUnit, intent.PeriodCount,
		task.CapabilityEvidenceNo, task.CapabilityEvidenceDigest, task.ObservedAt, task.ObservedAt.Add(24*time.Hour), tax.PolicyNo, tax.Treatment, tax.Rate,
		tax.Category, tax.Scope, tax.Mode, tax.Quantum, componentCount, intent.TargetSubtotal, intent.TargetNet, intent.TargetTax, intent.TargetRecurring,
		pricingDigest, contentDigest, intent.TargetVersionNo); err != nil {
		return gerror.Wrap(err, "insert resize renewal price baseline")
	}
	baseNo, _ := deterministicReference("rpm_", intent.TargetPriceNo+":base")
	if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(tenant_id,component_snapshot_no,price_snapshot_no,service_no,line_number,component_type_code,charge_phase_code,source_reference,
quantity,unit_amount_minor,amount_minor,source_digest,content_digest) VALUES (0,?,?,?,1,'base','periodic',?,1,?,?,?,?,?)`, baseNo,
		intent.TargetPriceNo, row.ServiceNo, intent.QuoteNo, intent.TargetNet, intent.TargetNet, pricingDigest, contentDigest); err != nil {
		return err
	}
	if intent.TargetTax > 0 {
		taxNo, _ := deterministicReference("rpm_", intent.TargetPriceNo+":tax")
		if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(tenant_id,component_snapshot_no,price_snapshot_no,service_no,line_number,component_type_code,charge_phase_code,source_reference,
quantity,unit_amount_minor,amount_minor,source_digest,content_digest) VALUES (0,?,?,?,2,'tax','total',?,1,?,?,?,?,?)`, taxNo, intent.TargetPriceNo,
			row.ServiceNo, tax.PolicyNo, intent.TargetTax, intent.TargetTax, pricingDigest, contentDigest); err != nil {
			return err
		}
	}
	if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_service_instance SET runtime_status_code=?,provider_task_status_code=?,
configuration_snapshot_no=?,version_no=?,billing_rule_no=?,current_price_snapshot_no=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND version=?`, runtimeStatus, providerStatus, intent.TargetConfigurationNo, intent.TargetVersionNo,
		intent.TargetBillingRuleNo, intent.TargetPriceNo, now, row.ServiceNo, row.ServiceVersion); err != nil {
		return err
	}
	return execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent SET status_code='succeeded',updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code IN ('executing','result_unknown')`, now, row.OperationNo)
}

func (w *workerImpl) finishResizeFailure(ctx context.Context, row *workRow, task *serveroperationresourcecap.Task, now time.Time) (bool, error) {
	var paid *struct {
		OrderNo, InvoiceNo, CustomerNo, CurrencyCode string
		Amount                                       int64
	}
	if err := w.database.GetScan(ctx, &paid, `SELECT COALESCE(operation.order_no,'') AS order_no,COALESCE(operation.invoice_no,'') AS invoice_no,
operation.customer_no,operation.currency_code,COALESCE(invoice.paid_amount_minor,0) AS amount
FROM plugin_idc_core_business_operation operation LEFT JOIN plugin_idc_core_billing_invoice invoice
 ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
WHERE operation.tenant_id=0 AND operation.operation_no=?`, row.OperationNo); err != nil {
		return false, err
	}
	if paid != nil && paid.OrderNo != "" && paid.Amount > 0 && task.Status == serveroperationresourcecap.TaskStatusFailed {
		handoffNo, _ := deterministicReference("rrh_", row.OperationNo)
		evidence := task.EvidenceDigest
		if evidence == "" {
			evidence, _ = digestValue(struct{ OperationNo, Failure string }{row.OperationNo, task.FailureCode})
		}
		if _, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_resize_refund_handoff
(tenant_id,handoff_no,operation_no,order_no,invoice_no,customer_no,currency_code,amount_minor,status_code,reason_code,evidence_digest,created_at,updated_at)
VALUES (0,?,?,?,?,?,?,?,'ready','server_resize.provider_failed',?,?,?) ON CONFLICT (operation_no) DO NOTHING`, handoffNo, row.OperationNo,
			paid.OrderNo, paid.InvoiceNo, paid.CustomerNo, paid.CurrencyCode, paid.Amount, evidence, now, now); err != nil {
			return false, err
		}
		if _, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='refunding',updated_at=?,version=version+1
WHERE tenant_id=0 AND operation_no=? AND status_code='failed'`, now, row.OperationNo); err != nil {
			return false, err
		}
		if _, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_commerce_order SET status_code='refund_pending',version=version+1,updated_at=?
WHERE tenant_id=0 AND order_no=? AND status_code IN ('paid','fulfilling','partially_fulfilled','fulfilled')`, now, paid.OrderNo); err != nil {
			return false, err
		}
		if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent SET status_code='refund_pending',refund_handoff_no=?,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code IN ('executing','result_unknown')`, handoffNo, now, row.OperationNo); err != nil {
			return false, err
		}
		return true, nil
	}
	status := "failed"
	if task.Status == serveroperationresourcecap.TaskStatusManualReview {
		status = "manual_review"
	}
	if err := execOne(ctx, w.database, `UPDATE plugin_idc_core_server_resize_intent SET status_code=?,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND status_code IN ('executing','result_unknown')`, status, now, row.OperationNo); err != nil {
		return false, err
	}
	return false, nil
}
