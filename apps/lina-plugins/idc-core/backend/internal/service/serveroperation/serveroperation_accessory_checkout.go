// This file checks out frozen accessory quotes for an existing server. Paid
// accessories retain their own operation path and never reserve new-product stock.

package serveroperation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type accessoryCheckoutRow struct {
	PolicyNo                                                                            string
	PolicyVersion                                                                       int64
	QuoteNo, CustomerNo, ServiceNo, CurrencyCode, ActionCode, AccessoryKind, Parameters string
	PoolNo, ChannelNo, EntitlementStatusCode, RuntimeStatusCode                         string
	ProviderTaskStatus, RequestDigest, QuoteContentDigest                               string
	PeriodUnitCode, BaselineObjectNos, BaselineSnapshotNo                               string
	TotalAmountMinor, SubtotalMinor, NetAmountMinor, TaxAmountMinor                     int64
	ChargedQuantity, PeriodCount                                                        int
	CustomerSubjectVersion, CurrentCustomerSubjectVersion                               int64
	ExpectedServiceVersion, CurrentServiceVersion                                       int64
	TrafficSuspended                                                                    bool
	ExpiresAt, BaselineObservedAt                                                       time.Time
}

func (s *accessoryPricingImpl) CheckoutCurrentCustomer(ctx context.Context,
	input AccessoryCheckoutInput) (*AccessoryCheckout, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return s.checkoutAccessory(ctx, subject.CustomerNo, input, "customer", subject.CustomerNo, "", "customer_accessory_checkout")
}

func (s *accessoryPricingImpl) CheckoutForStaff(ctx context.Context, customerNo string,
	input AccessoryCheckoutInput, actorID int, permission string) (*AccessoryCheckout, error) {
	if !validReference(customerNo, 40) || actorID <= 0 || !idccap.IsCatalogedAdminPermission(permission) {
		return nil, bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return s.checkoutAccessory(ctx, customerNo, input, "staff", strconv.Itoa(actorID), permission,
		"staff_accessory_checkout")
}

func (s *accessoryPricingImpl) checkoutAccessory(ctx context.Context, customerNo string,
	input AccessoryCheckoutInput, actorType, actorReference, requiredPermission, reasonCode string) (*AccessoryCheckout, error) {
	input.QuoteNo = strings.TrimSpace(input.QuoteNo)
	if !validReference(input.QuoteNo, 40) || len(input.IdempotencyKey) < 8 ||
		len(input.IdempotencyKey) > 256 || input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	idempotencyDigest, err := digestValue(struct{ CustomerNo, Key string }{customerNo, input.IdempotencyKey})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	requestDigest, err := digestValue(struct{ CustomerNo, QuoteNo string }{customerNo, input.QuoteNo})
	if err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	var result *AccessoryCheckout
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := s.lockAccessoryCheckout(ctx, customerNo, input.QuoteNo)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeAccessoryQuoteUnavailable)
		}
		existing, loadErr := s.loadAccessoryCheckoutReplay(ctx, row.QuoteNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.IdempotencyDigest != idempotencyDigest || existing.RequestDigest != requestDigest {
				return bizerr.NewCode(CodeAccessoryPriceConflict)
			}
			result = projectAccessoryCheckout(existing, row.TotalAmountMinor, row.ExpiresAt, true)
			return nil
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if !row.ExpiresAt.After(now) || row.CustomerSubjectVersion != row.CurrentCustomerSubjectVersion ||
			row.ExpectedServiceVersion != row.CurrentServiceVersion {
			return bizerr.NewCode(CodeAccessoryPriceConflict)
		}
		action := serveroperationresourcecap.Action(row.ActionCode)
		service := &serviceRow{ServiceNo: row.ServiceNo, CustomerNo: row.CustomerNo,
			CurrencyCode: row.CurrencyCode, PoolNo: row.PoolNo, ChannelNo: row.ChannelNo,
			EntitlementStatusCode: row.EntitlementStatusCode, RuntimeStatusCode: row.RuntimeStatusCode,
			TrafficSuspended: row.TrafficSuspended, ProviderTaskStatus: row.ProviderTaskStatus,
			Version: row.CurrentServiceVersion}
		if row.ChannelNo == "" || !allowedAction(service, action) {
			return bizerr.NewCode(CodeAccessoryQuoteUnavailable)
		}
		active, activeErr := s.database.GetValue(ctx, `SELECT EXISTS (
SELECT 1 FROM plugin_idc_core_server_operation_execution
WHERE tenant_id=0 AND service_no=?
 AND status_code IN ('ready','pending','running','result_unknown')
UNION ALL
SELECT 1 FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND service_no=? AND operation_type_code='addon' AND status_code IN
('awaiting_financial_settlement','financially_settled','manual_review'))`, row.ServiceNo, row.ServiceNo)
		if activeErr != nil {
			return activeErr
		}
		if active.Bool() {
			return bizerr.NewCode(CodeAccessoryPriceConflict)
		}
		if loadErr = s.reserveTrafficPackage(ctx, row); loadErr != nil {
			return loadErr
		}
		result, loadErr = s.persistAccessoryCheckout(ctx, row, input.IdempotencyKey,
			idempotencyDigest, requestDigest, actorType, actorReference, requiredPermission, reasonCode, now)
		return loadErr
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeAccessoryQuoteUnavailable)
	}
	return result, nil
}

func (s *accessoryPricingImpl) lockAccessoryCheckout(ctx context.Context, customerNo, quoteNo string) (*accessoryCheckoutRow, error) {
	var row *accessoryCheckoutRow
	err := s.database.GetScan(ctx, &row, `SELECT quote.quote_no,quote.customer_no,quote.service_no,quote.policy_no,quote.policy_version,
	quote.currency_code,quote.action_code,quote.parameters::TEXT AS parameters,quote.request_digest,
quote.content_digest AS quote_content_digest,quote.total_amount_minor,quote.subtotal_minor,
quote.net_amount_minor,quote.tax_amount_minor,quote.customer_subject_version,
quote.expected_service_version,quote.expires_at,quote.accessory_kind,quote.charged_quantity,
quote.period_unit_code,quote.period_count,quote.baseline_object_nos::TEXT AS baseline_object_nos,
quote.baseline_snapshot_no,quote.baseline_observed_at,service.version AS current_service_version,
customer.version AS current_customer_subject_version,service.pool_no,
COALESCE(service.channel_no,'') AS channel_no,service.entitlement_status_code,
service.runtime_status_code,service.provider_task_status_code,
EXISTS (SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension
 WHERE suspension.tenant_id=service.tenant_id AND suspension.service_no=service.service_no
 AND suspension.active) AS traffic_suspended
FROM plugin_idc_core_server_accessory_quote quote
JOIN plugin_idc_core_commerce_quote_snapshot commerce
 ON commerce.tenant_id=quote.tenant_id AND commerce.quote_no=quote.quote_no
 AND commerce.customer_no=quote.customer_no AND commerce.currency_code=quote.currency_code
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id=quote.tenant_id AND service.service_no=quote.service_no
 AND service.customer_no=quote.customer_no AND service.currency_code=quote.currency_code
JOIN plugin_idc_core_customer_subject customer
 ON customer.tenant_id=quote.tenant_id AND customer.customer_no=quote.customer_no
WHERE quote.tenant_id=0 AND quote.customer_no=? AND quote.quote_no=?
 AND commerce.purpose_code='addon' FOR UPDATE OF service,customer`, customerNo, quoteNo)
	if err != nil {
		return nil, gerror.Wrap(err, "lock accessory checkout")
	}
	return row, nil
}

type accessoryCheckoutReplay struct {
	OperationNo, OrderNo, InvoiceNo, ServiceNo, CurrencyCode, StatusCode string
	IdempotencyDigest, RequestDigest                                     string
}

func (s *accessoryPricingImpl) loadAccessoryCheckoutReplay(ctx context.Context, quoteNo string) (*accessoryCheckoutReplay, error) {
	var row *accessoryCheckoutReplay
	if err := s.database.GetScan(ctx, &row, `SELECT operation_no,COALESCE(order_no,'') AS order_no,
COALESCE(invoice_no,'') AS invoice_no,service_no,currency_code,status_code,idempotency_digest,request_digest
FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND operation_type_code='addon' AND quote_no=?`, quoteNo); err != nil {
		return nil, gerror.Wrap(err, "load accessory checkout replay")
	}
	return row, nil
}

func projectAccessoryCheckout(row *accessoryCheckoutReplay, amount int64, due time.Time, reused bool) *AccessoryCheckout {
	return &AccessoryCheckout{OperationNo: row.OperationNo, OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo,
		ServiceNo: row.ServiceNo, Status: row.StatusCode, AmountMinor: amount,
		CurrencyCode: row.CurrencyCode, PaymentDueAt: due.UTC(), Reused: reused}
}

func (s *accessoryPricingImpl) persistAccessoryCheckout(ctx context.Context, row *accessoryCheckoutRow,
	idempotencyKey, idempotencyDigest, requestDigest, actorType, actorReference,
	requiredPermission, reasonCode string, now time.Time) (*AccessoryCheckout, error) {
	operationNo, _ := deterministicReference("aop_", row.QuoteNo)
	nextVersion := row.CurrentServiceVersion + 1
	status := "ready"
	financial := row.TotalAmountMinor > 0
	orderNo, invoiceNo, priceNo := "", "", ""
	if financial {
		status = "awaiting_financial_settlement"
		orderNo, _ = deterministicReference("aor_", row.QuoteNo)
		invoiceNo, _ = deterministicReference("ain_", row.QuoteNo)
		priceNo, _ = deterministicReference("aps_", row.QuoteNo)
		if err := s.persistAccessoryOrderAndInvoice(ctx, row, orderNo, invoiceNo,
			idempotencyDigest, requestDigest, now); err != nil {
			return nil, err
		}
	}
	operationContent, err := digestValue(struct {
		OperationNo, QuoteNo, OrderNo, InvoiceNo, ServiceNo, Action, Parameters string
		Financial                                                               bool
		ExpectedVersion                                                         int64
	}{operationNo, row.QuoteNo, orderNo, invoiceNo, row.ServiceNo, row.ActionCode,
		row.Parameters, financial, nextVersion})
	if err != nil {
		return nil, err
	}
	var price, order, invoice any
	if financial {
		price, order, invoice = priceNo, orderNo, invoiceNo
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
(tenant_id,operation_no,operation_type_code,customer_no,service_no,currency_code,price_snapshot_no,
quote_no,order_no,invoice_no,status_code,financial_settlement_required,execution_mode_code,
resource_owner_code,pool_no,idempotency_digest,request_digest,content_digest,expected_service_version,
last_error_code,version,created_at,updated_at)
VALUES (0,?,'addon',?,?,?,?,?,?,?, ?,?,'unresolved','idc-resource',?,?,?,?,?,'',1,?,?)`,
		operationNo, row.CustomerNo, row.ServiceNo, row.CurrencyCode, price, row.QuoteNo, order, invoice,
		status, financial, row.PoolNo, idempotencyDigest, requestDigest, operationContent, nextVersion, now, now); err != nil {
		return nil, gerror.Wrap(err, "insert accessory business operation")
	}
	if row.ActionCode == string(serveroperationresourcecap.ActionSnapshotCreate) ||
		row.ActionCode == string(serveroperationresourcecap.ActionBackupCreate) {
		if err = s.persistAccessoryEntitlementIntent(ctx, row, operationNo, now); err != nil {
			return nil, err
		}
	}
	evidence, err := digestValue(struct{ OperationNo, QuoteDigest, Status string }{operationNo, row.QuoteContentDigest, status})
	if err != nil {
		return nil, err
	}
	if err = insertEvent(ctx, s.database, operationNo, row.ServiceNo, 1, nil, status,
		"server_accessory.checkout_created", actorType, actorReference, evidence, now); err != nil {
		return nil, err
	}
	if !financial {
		if err = s.persistFreeAccessoryExecution(ctx, row, operationNo, nextVersion, now); err != nil {
			return nil, err
		}
	}
	if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_service_instance
SET last_business_operation_no=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND customer_no=? AND version=?`, operationNo, now,
		row.ServiceNo, row.CustomerNo, row.CurrentServiceVersion); err != nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceConflict)
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "server.accessory.checkout", IdempotencyKey: idempotencyKey,
		OperationCode: "server.accessory.checkout", TargetType: "server_service", TargetNo: row.ServiceNo,
		CustomerNo: row.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: reasonCode,
		RiskCode: "medium", RequiredPermission: requiredPermission, CorrelationNo: operationNo,
		BeforeSnapshot: map[string]any{"serviceVersion": row.CurrentServiceVersion},
		AfterSnapshot: map[string]any{"serviceVersion": nextVersion, "action": row.ActionCode,
			"financialSettlementRequired": financial, "totalAmountMinor": row.TotalAmountMinor},
	}); err != nil {
		return nil, err
	}
	replay := &accessoryCheckoutReplay{OperationNo: operationNo, OrderNo: orderNo, InvoiceNo: invoiceNo,
		ServiceNo: row.ServiceNo, CurrencyCode: row.CurrencyCode, StatusCode: status,
		IdempotencyDigest: idempotencyDigest, RequestDigest: requestDigest}
	return projectAccessoryCheckout(replay, row.TotalAmountMinor, row.ExpiresAt, false), nil
}

func (s *accessoryPricingImpl) persistAccessoryEntitlementIntent(ctx context.Context,
	row *accessoryCheckoutRow, operationNo string, now time.Time) error {
	entitlementNo, err := deterministicReference("aen_", operationNo)
	if err != nil {
		return err
	}
	var parameters struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(row.Parameters), &parameters) != nil || len(parameters.Name) > 128 {
		return bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	billingMode := "free"
	if row.ChargedQuantity > 0 {
		billingMode = "paid"
	}
	evidence, err := digestValue(struct {
		EntitlementNo, OperationNo, ServiceNo, Kind, BillingMode, BaselineSnapshot string
	}{entitlementNo, operationNo, row.ServiceNo, row.AccessoryKind, billingMode, row.BaselineSnapshotNo})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement
(tenant_id,entitlement_no,customer_no,service_no,create_operation_no,accessory_kind,requested_name,
billing_mode_code,status_code,auto_renew_enabled,currency_code,period_unit_code,period_count,
baseline_object_nos,baseline_snapshot_no,baseline_observed_at,version,created_at,updated_at)
VALUES (0,?,?,?,?,?,? ,?,'pending_binding',TRUE,?,?,?,?::JSONB,?,?,1,?,?)`,
		entitlementNo, row.CustomerNo, row.ServiceNo, operationNo, row.AccessoryKind, parameters.Name,
		billingMode, row.CurrencyCode, row.PeriodUnitCode, row.PeriodCount, row.BaselineObjectNos,
		row.BaselineSnapshotNo, row.BaselineObservedAt, now, now); err != nil {
		return gerror.Wrap(err, "insert accessory entitlement intent")
	}
	eventNo, _ := deterministicReference("aee_", entitlementNo+":created")
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,to_status_code,operation_no,evidence_digest,occurred_at)
VALUES (0,?,?,'intent_created','pending_binding',?,?,?)`, eventNo, entitlementNo, operationNo, evidence, now)
	return gerror.Wrap(err, "insert accessory entitlement event")
}

func (s *accessoryPricingImpl) persistAccessoryOrderAndInvoice(ctx context.Context, row *accessoryCheckoutRow,
	orderNo, invoiceNo, idempotencyDigest, requestDigest string, now time.Time) error {
	var quoteItem *entity.CommerceQuoteItemSnapshot
	if err := s.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).
		Where(do.CommerceQuoteItemSnapshot{TenantId: 0, QuoteNo: row.QuoteNo}).Scan(&quoteItem); err != nil {
		return err
	}
	if quoteItem == nil {
		return bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	orderItemNo, _ := deterministicReference("aoi_", row.QuoteNo)
	invoiceItemNo, _ := deterministicReference("aii_", row.QuoteNo)
	itemDigest, err := digestValue(struct{ OrderNo, QuoteItemNo, QuoteDigest string }{
		orderNo, quoteItem.QuoteItemNo, quoteItem.ContentDigest})
	if err != nil {
		return err
	}
	order := do.CommerceOrder{TenantId: 0, OrderNo: orderNo, CustomerNo: row.CustomerNo,
		QuoteNo: row.QuoteNo, CurrencyCode: row.CurrencyCode, SourceCode: "portal",
		StatusCode: "pending_payment", RiskDecisionCode: "approved", ItemCount: 1,
		SubtotalMinor: row.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: row.NetAmountMinor,
		TaxAmountMinor: row.TaxAmountMinor, TotalAmountMinor: row.TotalAmountMinor,
		CheckoutIdempotencyDigest: idempotencyDigest, CheckoutRequestDigest: requestDigest,
		PaymentDueAt: &row.ExpiresAt, Version: 1, CreatedAt: &now, UpdatedAt: &now}
	item := do.CommerceOrderItem{TenantId: 0, OrderItemNo: orderItemNo, OrderNo: orderNo,
		QuoteNo: row.QuoteNo, QuoteItemNo: quoteItem.QuoteItemNo, LineNumber: 1,
		CurrencyCode: quoteItem.CurrencyCode, ProductNo: quoteItem.ProductNo, VersionNo: quoteItem.VersionNo,
		BillingRuleNo: quoteItem.BillingRuleNo, ConfigurationSnapshotNo: quoteItem.ConfigurationSnapshotNo,
		ProductNameZhCn: quoteItem.ProductNameZhCn, ProductNameEnUs: quoteItem.ProductNameEnUs,
		PurchaseCycles: 1,
		ChargeTypeCode: quoteItem.ChargeTypeCode, PeriodUnitCode: quoteItem.PeriodUnitCode,
		PeriodCount: quoteItem.PeriodCount, FulfillmentModeCode: "automatic",
		RiskDecisionCode:      "approved",
		InventoryDecisionCode: "manual_confirmation", RouteDecisionDigest: quoteItem.RouteDecisionDigest,
		TaxPolicyNo: quoteItem.TaxPolicyNo, PriceIncludesTax: true,
		TaxTreatmentCode: quoteItem.TaxTreatmentCode, TaxRateBasisPoints: quoteItem.TaxRateBasisPoints,
		TaxCategoryCode: quoteItem.TaxCategoryCode, RoundingScopeCode: quoteItem.RoundingScopeCode,
		RoundingModeCode: quoteItem.RoundingModeCode, RoundingQuantumMinor: quoteItem.RoundingQuantumMinor,
		SetupSubtotalMinor: quoteItem.SetupSubtotalMinor, PeriodSubtotalMinor: quoteItem.PeriodSubtotalMinor,
		SubtotalMinor: row.SubtotalMinor, DiscountMinor: 0, NetAmountMinor: row.NetAmountMinor,
		TaxAmountMinor: row.TaxAmountMinor, TotalAmountMinor: row.TotalAmountMinor,
		ContentDigest: itemDigest, CreatedAt: &now}
	invoiceContent, _ := digestValue(struct{ InvoiceNo, OrderNo, QuoteDigest string }{invoiceNo, orderNo, row.QuoteContentDigest})
	orderCopy := orderNo
	invoice := do.BillingInvoice{TenantId: 0, InvoiceNo: invoiceNo, CustomerNo: row.CustomerNo,
		OrderNo: &orderCopy, SourceCode: "order", CurrencyCode: row.CurrencyCode,
		StatusCode: "pending_payment", ItemCount: 1, SubtotalMinor: row.SubtotalMinor,
		DiscountMinor: 0, NetAmountMinor: row.NetAmountMinor, TaxAmountMinor: row.TaxAmountMinor,
		TotalAmountMinor: row.TotalAmountMinor, PaidAmountMinor: 0, RefundableAmountMinor: 0,
		RefundedAmountMinor: 0, SourceSnapshotDigest: row.QuoteContentDigest, ContentDigest: invoiceContent,
		PaymentDueAt: &row.ExpiresAt, IssuedAt: &now, Version: 1, CreatedAt: &now, UpdatedAt: &now}
	invoiceItemContent, _ := digestValue(struct{ InvoiceNo, OrderItemNo, ItemDigest string }{invoiceNo, orderItemNo, itemDigest})
	orderItemCopy := orderItemNo
	invoiceItem := do.BillingInvoiceItem{TenantId: 0, InvoiceItemNo: invoiceItemNo, InvoiceNo: invoiceNo,
		CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode, LineNumber: 1,
		ItemTypeCode: "order_item", OrderNo: &orderCopy, OrderItemNo: &orderItemCopy,
		SourceReference: orderItemNo, DescriptionZhCn: quoteItem.ProductNameZhCn,
		DescriptionEnUs: quoteItem.ProductNameEnUs, SubtotalMinor: row.SubtotalMinor,
		DiscountMinor: 0, NetAmountMinor: row.NetAmountMinor, TaxAmountMinor: row.TaxAmountMinor,
		TotalAmountMinor: row.TotalAmountMinor, RefundableCapMinor: row.TotalAmountMinor,
		ContentDigest: invoiceItemContent, CreatedAt: &now}
	quantityDigest, err := digestValue(struct {
		OrderItemNo, QuoteItemNo string
		Quantity                 int
	}{orderItemNo, quoteItem.QuoteItemNo, 1})
	if err != nil {
		return err
	}
	itemQuantity := map[string]any{
		"tenant_id": 0, "order_item_no": orderItemNo, "order_no": orderNo,
		"quote_item_no": quoteItem.QuoteItemNo, "quantity": 1, "content_digest": quantityDigest,
	}
	for _, insertion := range []struct {
		table string
		data  any
	}{
		{dao.CommerceOrder.Table(), order}, {dao.CommerceOrderItem.Table(), item},
		{"plugin_idc_core_commerce_order_item_quantity_snapshot", itemQuantity},
		{dao.BillingInvoice.Table(), invoice}, {dao.BillingInvoiceItem.Table(), invoiceItem},
	} {
		if _, err = s.database.Model(insertion.table).Ctx(ctx).Data(insertion.data).Insert(); err != nil {
			return gerror.Wrap(err, "insert accessory checkout financial rows")
		}
	}
	return s.persistAccessoryCheckoutInventory(ctx, quoteItem, orderItemNo)
}

// persistAccessoryCheckoutInventory freezes the non-stock nature of an addon in
// both commerce snapshots. The caller owns the checkout transaction and has
// already locked the quote's customer and server. An existing, different quote
// inventory fact is never overwritten or silently converted to untracked stock.
func (s *accessoryPricingImpl) persistAccessoryCheckoutInventory(
	ctx context.Context, quoteItem *entity.CommerceQuoteItemSnapshot, orderItemNo string,
) error {
	quoteDigest, err := digestValue(struct {
		QuoteItemNo      string
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
	}{quoteItem.QuoteItemNo, quoteItem.VersionNo, false, 0, 0})
	if err != nil {
		return err
	}
	orderDigest, err := digestValue(struct {
		OrderItemNo      string
		QuoteItemNo      string
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
	}{orderItemNo, quoteItem.QuoteItemNo, quoteItem.VersionNo, false, 0, 0})
	if err != nil {
		return err
	}
	quoteSnapshot := do.CommerceQuoteItemInventorySnapshot{
		QuoteItemNo: quoteItem.QuoteItemNo, VersionNo: quoteItem.VersionNo,
		TrackingEnabled: false, InventoryVersion: int64(0), SellableQuantity: int64(0),
		ContentDigest: quoteDigest,
	}
	if _, err = s.database.Model(dao.CommerceQuoteItemInventorySnapshot.Table()).Ctx(ctx).
		Data(quoteSnapshot).OnConflict(dao.CommerceQuoteItemInventorySnapshot.Columns().QuoteItemNo).
		InsertIgnore(); err != nil {
		return gerror.Wrap(err, "freeze accessory quote non-stock inventory")
	}
	// A primary-key lookup validates the full immutable fact before freezing
	// the order. The caller's transaction stays active, and the database forbids
	// changes/deletion of this quote snapshot, so it cannot change after validation.
	matching, err := s.database.Model(dao.CommerceQuoteItemInventorySnapshot.Table()).Ctx(ctx).
		Where(quoteSnapshot).Count()
	if err != nil {
		return gerror.Wrap(err, "validate accessory quote non-stock inventory")
	}
	if matching != 1 {
		return bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	inserted, err := s.database.Model(dao.CommerceOrderItemInventorySnapshot.Table()).Ctx(ctx).
		Data(do.CommerceOrderItemInventorySnapshot{
			OrderItemNo: orderItemNo, QuoteItemNo: quoteItem.QuoteItemNo, VersionNo: quoteItem.VersionNo,
			TrackingEnabled: false, InventoryVersion: int64(0), SellableQuantity: int64(0),
			ContentDigest: orderDigest,
		}).Insert()
	if err != nil {
		return gerror.Wrap(err, "freeze accessory order non-stock inventory")
	}
	affected, err := inserted.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count accessory inventory snapshot insertion")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeAccessoryQuoteUnavailable)
	}
	return nil
}

func (s *accessoryPricingImpl) persistFreeAccessoryExecution(ctx context.Context, row *accessoryCheckoutRow,
	operationNo string, expectedVersion int64, now time.Time) error {
	parameters := json.RawMessage(row.Parameters)
	request := serveroperationresourcecap.TaskRequest{OperationNo: operationNo, ServiceNo: row.ServiceNo,
		ChannelNo: row.ChannelNo, PoolNo: row.PoolNo, Action: serveroperationresourcecap.Action(row.ActionCode),
		ExpectedServiceVersion: expectedVersion, Parameters: parameters}
	requestDigest, err := serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		return err
	}
	executionNo, _ := deterministicReference("aox_", operationNo)
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_operation_execution
(tenant_id,execution_no,operation_no,service_no,action_code,resource_owner_code,channel_no,pool_no,
expected_service_version,original_runtime_status_code,parameters,secret_bindings,status_code,
request_digest,version,created_at,updated_at)
VALUES (0,?,?,?,?,'idc-resource',?,?,?,?,?::JSONB,'[]'::JSONB,'ready',?,1,?,?)`,
		executionNo, operationNo, row.ServiceNo, row.ActionCode, row.ChannelNo, row.PoolNo,
		expectedVersion, row.RuntimeStatusCode, row.Parameters, requestDigest, now, now)
	return err
}
