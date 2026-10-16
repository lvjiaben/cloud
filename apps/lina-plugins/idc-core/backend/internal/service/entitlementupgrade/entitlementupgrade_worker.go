// This file reconciles paid upgrades with immutable Provider tasks. Unknown results
// remain held for review; configuration and expiry change only after proven success.

package entitlementupgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"reflect"
	"time"
)

// Run processes a fixed page; SQL locks make concurrent schedulers replay-safe.
func (s *serviceImpl) Run(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	var ids []struct{ UpgradeNo, CustomerNo string }
	err := s.database.GetScan(ctx, &ids, `SELECT u.upgrade_no,u.customer_no FROM plugin_idc_core_entitlement_upgrade u
 JOIN plugin_idc_core_billing_invoice i ON i.tenant_id=u.tenant_id AND i.invoice_no=u.invoice_no
 WHERE u.tenant_id=0 AND u.status_code IN ('pending_payment','ready','executing','retry_wait') AND u.available_at<=CURRENT_TIMESTAMP
	AND ((i.status_code='paid' AND i.paid_amount_minor=i.total_amount_minor)
 OR (u.status_code='pending_payment' AND u.quote_expires_at<=CURRENT_TIMESTAMP AND i.status_code IN ('pending_payment','overdue')
 AND i.paid_amount_minor=0 AND i.credit_amount_minor=0 AND i.waived_amount_minor=0)) ORDER BY u.available_at,u.id LIMIT ?`, limit)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err = s.process(ctx, id.UpgradeNo, id.CustomerNo); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (s *serviceImpl) process(ctx context.Context, no, customer string) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.load(ctx, no, customer)
		if err != nil {
			return err
		}
		if row == nil || row.StatusCode == statusSucceeded || row.StatusCode == statusManual || row.StatusCode == statusCancelled {
			return nil
		}
		if row.StatusCode == statusPending && !row.QuoteExpiresAt.After(time.Now()) {
			// The billing owner serializes cancellation against an external payment.
			// Only an untouched invoice may expire; a proven paid invoice still executes.
			var invoice *entity.BillingInvoice
			if err = s.database.Model(do.BillingInvoice{}).Ctx(ctx).Where(do.BillingInvoice{TenantId: 0, InvoiceNo: row.InvoiceNo, CustomerNo: customer}).LockUpdate().Scan(&invoice); err != nil {
				return err
			}
			if invoice != nil && invoice.PaidAmountMinor == 0 && invoice.TotalAmountMinor > 0 {
				if _, err = s.invoices.CancelUnpaid(ctx, row.InvoiceNo); err != nil {
					return err
				}
				return s.mark(ctx, row, statusCancelled, "quote_expired", nil)
			}
		}
		state, err := s.state(ctx, row.EntitlementNo, customer)
		if err != nil || state.Version != row.SourceVersion || !state.ExpiresAt.Equal(*row.PeriodExpiresAt) {
			return s.mark(ctx, row, statusManual, "service_revision_changed", nil)
		}
		var target *targetEvidence
		target, err = decodeTarget(row)
		if err != nil {
			return err
		}
		request := serveroperationresourcecap.TaskRequest{OperationNo: fmt.Sprintf("%s.%d", no, row.AttemptCount), ServiceNo: row.EntitlementNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo, Action: serveroperationresourcecap.ActionResize, ExpectedServiceVersion: row.SourceVersion, Parameters: json.RawMessage(row.ProviderParameters), RequestDigest: row.ContentDigest}
		var task *serveroperationresourcecap.Task
		if row.ProviderTaskNo == "" {
			// Revalidate the frozen target against current Provider capabilities before dispatch.
			checked, e := s.compiler.CompileUpgrade(ctx, serveroperationresourcecap.UpgradeConfigurationRequest{ServiceNo: row.EntitlementNo, CustomerNo: customer, ProductConfiguration: json.RawMessage(state.ProductConfiguration), CurrentConfiguration: json.RawMessage(state.CurrentConfiguration), TargetConfiguration: json.RawMessage(target.TypedValues)})
			if e != nil || checked == nil || checked.Route.ChannelNo != row.ChannelNo || checked.Route.PoolNo != row.PoolNo || !sameParameters(checked.Parameters, []byte(row.ProviderParameters)) {
				return s.mark(ctx, row, statusManual, "provider_capability_changed", nil)
			}
			task, err = s.source.CreateTask(ctx, request)
		} else {
			task, err = s.source.ReadTask(ctx, serveroperationresourcecap.TaskReadRequest{ProviderTaskNo: row.ProviderTaskNo, Request: request})
		}
		if err != nil {
			return s.mark(ctx, row, statusRetry, "provider_temporarily_unavailable", nil)
		}
		if task == nil || serveroperationresourcecap.ValidateTask(*task) != nil {
			return s.mark(ctx, row, statusManual, "provider_result_invalid", nil)
		}
		if task.Status == serveroperationresourcecap.TaskStatusSucceeded {
			if err = s.apply(ctx, row, target); err != nil {
				return err
			}
			return s.mark(ctx, row, statusSucceeded, "", task)
		}
		if task.Status == serveroperationresourcecap.TaskStatusFailed {
			// Each confirmed failure remains in the resource task history. A fresh request
			// identity is allowed at most three times; ambiguous results never redispatch.
			if row.AttemptCount < 2 {
				next := row.AttemptCount + 1
				due := time.Now().UTC().Add(time.Minute * time.Duration(next))
				_, err = s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{UpgradeNo: no, CustomerNo: customer}).Data(do.EntitlementUpgrade{StatusCode: statusRetry, ProviderTaskNo: "", AttemptCount: next, AvailableAt: &due, LastErrorCode: task.FailureCode}).Update()
				return err
			}
			return s.mark(ctx, row, statusManual, task.FailureCode, task)
		}
		if task.Status == serveroperationresourcecap.TaskStatusResultUnknown || task.Status == serveroperationresourcecap.TaskStatusManualReview {
			return s.mark(ctx, row, statusManual, task.FailureCode, task)
		}
		return s.mark(ctx, row, statusExecuting, "", task)
	})
}

func (s *serviceImpl) mark(ctx context.Context, row *entity.EntitlementUpgrade, status, reason string, task *serveroperationresourcecap.Task) error {
	due := time.Now().UTC().Add(30 * time.Second)
	data := do.EntitlementUpgrade{StatusCode: status, LastErrorCode: reason, AvailableAt: &due}
	if task != nil {
		data.ProviderTaskNo = task.ProviderTaskNo
	}
	if status == statusSucceeded {
		now := time.Now().UTC()
		data.CompletedAt = &now
	}
	_, err := s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{UpgradeNo: row.UpgradeNo, CustomerNo: row.CustomerNo}).Data(data).Update()
	return err
}

func (s *serviceImpl) apply(ctx context.Context, row *entity.EntitlementUpgrade, target *targetEvidence) error {
	price := "UPS_" + row.IdempotencyDigest[:32]
	// The full-period equivalent is a new immutable effective price; historical
	// orders remain untouched and subsequent renewal reads its new selection set.
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
 (tenant_id,price_snapshot_no,subject_type_code,subject_no,customer_no,phase_code,source_quote_no,source_quote_item_no,
 product_no,version_no,billing_rule_no,configuration_snapshot_no,structured_configuration_snapshot_no,currency_code,purchase_cycles,period_unit_code,period_count,
 renewal_mode_code,tax_policy_no,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,
 component_count,subtotal_minor,net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
 VALUES(0,?,'entitlement',?,?,'new_purchase',?,?,?,?,?,?,?,?,?,?,?,'unresolved',?,false,?,?,?,?,?, ?,1,?,?,0,?,?,?)`, price, row.EntitlementNo, row.CustomerNo, target.QuoteNo, target.QuoteItemNo, target.ProductNo, target.VersionNo, target.BillingRuleNo, target.ConfigurationSnapshotNo, target.StructuredNo, row.CurrencyCode, target.Line.PurchaseCycles, target.Line.PeriodUnitCode, target.Line.PeriodCount, target.Line.TaxPolicyNo, target.Line.TaxTreatmentCode, target.Line.TaxRateBasisPoints, target.Line.TaxCategoryCode, target.Line.RoundingScopeCode, target.Line.RoundingModeCode, target.Line.RoundingQuantumMinor, target.PeriodNet, target.PeriodNet, target.PeriodNet, row.ContentDigest, row.ContentDigest)
	if err != nil {
		return err
	}
	// The effective period equivalent has one aggregate component; the exact
	// original base/configuration breakdown remains in the immutable target quote.
	if _, err = s.database.Model(do.ServicePriceComponentSnapshot{}).Ctx(ctx).Data(do.ServicePriceComponentSnapshot{
		TenantId: 0, ComponentSnapshotNo: "UPC_" + row.IdempotencyDigest[:32], PriceSnapshotNo: price,
		SubjectTypeCode: "entitlement", SubjectNo: row.EntitlementNo, LineNumber: 1,
		ComponentTypeCode: "base", ChargePhaseCode: "periodic", SourceReference: row.UpgradeNo,
		Quantity: 1, UnitAmountMinor: target.PeriodNet, AmountMinor: target.PeriodNet,
		SourceDigest: row.ContentDigest, ContentDigest: row.ContentDigest,
	}).Insert(); err != nil {
		return err
	}
	result, err := s.database.Model(do.ServiceEntitlement{}).Ctx(ctx).Where(do.ServiceEntitlement{TenantId: 0, EntitlementNo: row.EntitlementNo, CustomerNo: row.CustomerNo, Version: row.SourceVersion}).Data(do.ServiceEntitlement{CurrentPriceSnapshotNo: price, Version: row.SourceVersion + 1}).Update()
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	_, err = s.database.Model(do.EntitlementUpgrade{}).Ctx(ctx).Where(do.EntitlementUpgrade{UpgradeNo: row.UpgradeNo, CustomerNo: row.CustomerNo}).Data(do.EntitlementUpgrade{TargetPriceSnapshotNo: price, TargetConfigurationNo: target.StructuredNo}).Update()
	return err
}

// sameParameters compares JSON values rather than PostgreSQL JSONB formatting.
// Number strings retain integer precision; object key order is immaterial.
func sameParameters(left, right []byte) bool {
	var a, b any
	first, second := json.NewDecoder(bytes.NewReader(left)), json.NewDecoder(bytes.NewReader(right))
	first.UseNumber()
	second.UseNumber()
	return first.Decode(&a) == nil && second.Decode(&b) == nil && reflect.DeepEqual(a, b)
}
