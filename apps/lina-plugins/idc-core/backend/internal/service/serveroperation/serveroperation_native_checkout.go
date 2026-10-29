// Native checkout reserves a real entitlement's addon slot and merchant stock, then creates ordinary payable documents once.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// Checkout never authorizes a provider write; paid operations wait for the existing financial settlement consumer.
func (s *nativeCommercialService) Checkout(ctx context.Context, input servercommercialcap.CheckoutRequest) (*servercommercialcap.Purchase, error) {
	if !validReference(input.QuoteNo, 40) || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 || strings.TrimSpace(input.IdempotencyKey) != input.IdempotencyKey {
		return nil, ErrInvalidInput
	}
	priced, err := nativeHasPricedQuote(ctx, s.database, input.QuoteNo)
	if err != nil {
		return nil, err
	}
	if priced {
		return s.nativeCheckoutPriced(ctx, input)
	}
	var result *servercommercialcap.Purchase
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		core, err := s.readNativeCommercialTarget(ctx, input.Target, true)
		if err != nil {
			return err
		}
		quote, err := s.readNativeAccessoryQuote(ctx, core.CustomerNo, input.QuoteNo, "")
		if err != nil {
			return err
		}
		if quote == nil || quote.SubjectNo != core.EntitlementNo {
			return ErrOperationConflict
		}
		if err = s.authorizeNativeTarget(ctx, core.CustomerNo, quote.Action); err != nil {
			return err
		}
		idempotency, err := digestValue(struct{ CustomerNo, Key string }{core.CustomerNo, "native-checkout:" + input.IdempotencyKey})
		if err != nil {
			return err
		}
		request, err := digestValue(struct{ CustomerNo, QuoteNo string }{core.CustomerNo, input.QuoteNo})
		if err != nil {
			return err
		}
		var replays []struct{ OperationNo, OrderNo, InvoiceNo, StatusCode, IdempotencyDigest, RequestDigest string }
		if err = s.database.GetScan(ctx, &replays, `SELECT operation_no,COALESCE(order_no,'') AS order_no,COALESCE(invoice_no,'') AS invoice_no,status_code,idempotency_digest,request_digest FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND quote_no=? AND customer_no=? AND subject_type_code='entitlement' AND subject_no=? AND operation_type_code='addon' FOR UPDATE`, input.QuoteNo, core.CustomerNo, core.EntitlementNo); err != nil {
			return err
		}
		if len(replays) > 1 {
			return ErrOperationConflict
		}
		if len(replays) == 1 {
			row := replays[0]
			if row.IdempotencyDigest != idempotency || row.RequestDigest != request {
				return ErrOperationConflict
			}
			result = &servercommercialcap.Purchase{OperationNo: row.OperationNo, QuoteNo: quote.QuoteNo, EntitlementNo: core.EntitlementNo, OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo, Status: row.StatusCode, CurrencyCode: quote.CurrencyCode, TotalAmountMinor: quote.TotalAmountMinor, Reused: true}
			return nil
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		if err = s.validateNativeAccessoryCheckout(ctx, core, input, quote, now); err != nil {
			return err
		}
		result, err = s.persistNativeAccessoryCheckout(ctx, quote, core, input.IdempotencyKey, idempotency, request, now)
		return err
	})
	return result, err
}

func (s *nativeCommercialService) reserveNativeTrafficPackage(ctx context.Context, policyNo string) error {
	var rows []struct{ StockQuantity *int64 }
	if err := s.database.GetScan(ctx, &rows, `SELECT stock_quantity FROM plugin_idc_core_server_accessory_price_policy WHERE tenant_id=0 AND policy_no=?`, policyNo); err != nil {
		return err
	}
	if len(rows) != 1 {
		return ErrOperationConflict
	}
	if rows[0].StockQuantity == nil {
		return nil
	}
	var occupancy []struct{ Quantity int64 }
	if err := s.pricing.trafficPackageOccupancy(ctx, []string{policyNo}).Scan(&occupancy); err != nil {
		return err
	}
	if len(occupancy) > 1 {
		return ErrOperationConflict
	}
	if len(occupancy) == 1 && occupancy[0].Quantity >= *rows[0].StockQuantity || *rows[0].StockQuantity <= 0 {
		return ErrOperationConflict
	}
	return nil
}

func (s *nativeCommercialService) persistNativeAccessoryCheckout(ctx context.Context, quote *nativeAccessoryQuoteRecord, target *nativeCommercialTarget, key, idempotency, request string, now time.Time) (*servercommercialcap.Purchase, error) {
	operation, err := deterministicReference("nop_", quote.QuoteNo)
	if err != nil {
		return nil, err
	}
	financial := quote.TotalAmountMinor > 0
	status, order, invoice := "ready", "", ""
	var priceValue, orderValue, invoiceValue any
	if financial {
		status = "awaiting_financial_settlement"
		order, err = deterministicReference("aor_", quote.QuoteNo)
		if err != nil {
			return nil, err
		}
		invoice, err = deterministicReference("ain_", quote.QuoteNo)
		if err != nil {
			return nil, err
		}
		price, priceErr := deterministicReference("aps_", quote.QuoteNo)
		if priceErr != nil {
			return nil, priceErr
		}
		priceValue, orderValue, invoiceValue = price, order, invoice
		row := &accessoryCheckoutRow{QuoteNo: quote.QuoteNo, CustomerNo: quote.CustomerNo, CurrencyCode: quote.CurrencyCode, QuoteContentDigest: quote.ContentDigest, SubtotalMinor: quote.SubtotalMinor, NetAmountMinor: quote.NetAmountMinor, TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.TotalAmountMinor, ExpiresAt: quote.ExpiresAt}
		if err = s.pricing.persistAccessoryOrderAndInvoice(ctx, row, order, invoice, idempotency, request, now); err != nil {
			return nil, err
		}
	}
	content, err := nativeAccessoryIntentDigest(operation, quote.QuoteNo, order, invoice, target.EntitlementNo, string(quote.Action), quote.Parameters, financial, target.EntitlementVersion)
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
 (tenant_id,operation_no,operation_type_code,customer_no,service_no,subject_type_code,subject_no,currency_code,price_snapshot_no,
 quote_no,order_no,invoice_no,status_code,financial_settlement_required,execution_mode_code,resource_owner_code,pool_no,
 idempotency_digest,request_digest,content_digest,expected_subject_version,version)
 VALUES(0,?,'addon',?,NULL,'entitlement',?,?,?,?,?,?,?,?, 'unresolved','idc-resource',?,?,?,?,?,1)`, operation, target.CustomerNo, target.EntitlementNo, target.CurrencyCode, priceValue, quote.QuoteNo, orderValue, invoiceValue, status, financial, target.PoolNo, idempotency, request, content, target.EntitlementVersion); err != nil {
		return nil, err
	}
	if quote.Kind == serveroperationresourcecap.AccessoryKindSnapshot || quote.Kind == serveroperationresourcecap.AccessoryKindBackup {
		if err = s.persistNativeAccessoryEntitlement(ctx, quote, operation); err != nil {
			return nil, err
		}
	}
	if err = appendNativeAccessoryEvent(ctx, s.database, operation, target.EntitlementNo, 1, "", status, "server_accessory.checkout_created", target.CustomerNo, content); err != nil {
		return nil, err
	}
	if err = s.pricing.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "server.native.accessory.checkout", IdempotencyKey: key, OperationCode: "server.accessory.checkout", TargetType: "service_entitlement", TargetNo: target.EntitlementNo, CustomerNo: target.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "native_accessory_checkout", RiskCode: "medium", CorrelationNo: operation, BeforeSnapshot: map[string]any{"entitlementVersion": target.EntitlementVersion}, AfterSnapshot: map[string]any{"action": quote.Action, "financialSettlementRequired": financial, "totalAmountMinor": quote.TotalAmountMinor}}); err != nil {
		return nil, err
	}
	return &servercommercialcap.Purchase{OperationNo: operation, QuoteNo: quote.QuoteNo, EntitlementNo: target.EntitlementNo, OrderNo: order, InvoiceNo: invoice, Status: status, CurrencyCode: target.CurrencyCode, TotalAmountMinor: quote.TotalAmountMinor}, nil
}

func (s *nativeCommercialService) persistNativeAccessoryEntitlement(ctx context.Context, quote *nativeAccessoryQuoteRecord, operation string) error {
	no, err := deterministicReference("aen_", operation)
	if err != nil {
		return err
	}
	var parameters struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(quote.Parameters), &parameters) != nil {
		return ErrInvalidInput
	}
	mode := "free"
	if quote.ChargedQuantity > 0 {
		mode = "paid"
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement
 (tenant_id,entitlement_no,customer_no,service_no,subject_type_code,subject_no,create_operation_no,accessory_kind,requested_name,billing_mode_code,status_code,auto_renew_enabled,currency_code,period_unit_code,period_count,baseline_object_nos,baseline_snapshot_no,baseline_observed_at,version)
 VALUES(0,?,?,NULL,'entitlement',?,?,?,?,?,'pending_binding',TRUE,?,?,?,?::JSONB,?,?,1)`, no, quote.CustomerNo, quote.SubjectNo, operation, quote.Kind, parameters.Name, mode, quote.CurrencyCode, quote.PeriodUnitCode, quote.PeriodCount, quote.BaselineJSON, quote.BaselineSnapshotNo, quote.BaselineObservedAt); err != nil {
		return err
	}
	event, err := deterministicReference("aee_", no+":created")
	if err != nil {
		return err
	}
	evidence, err := digestValue(struct{ EntitlementNo, OperationNo, SubjectNo string }{no, operation, quote.SubjectNo})
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event(tenant_id,event_no,entitlement_no,event_type_code,to_status_code,operation_no,evidence_digest,occurred_at) VALUES(0,?,?,'intent_created','pending_binding',?,?,CURRENT_TIMESTAMP)`, event, no, operation, evidence)
	return err
}

// validateNativeAccessoryCheckout checks frozen money, subject revision and merchant occupancy while the parent stays locked.
func (s *nativeCommercialService) validateNativeAccessoryCheckout(ctx context.Context, core *nativeCommercialTarget, input servercommercialcap.CheckoutRequest, quote *nativeAccessoryQuoteRecord, now time.Time) error {
	if !quote.ExpiresAt.After(now) || quote.ExpectedSubjectVersion != core.EntitlementVersion || quote.CustomerSubjectVersion != core.CustomerVersion || quote.CurrencyCode != core.CurrencyCode || !nativeSnapshotAllows(input.Target, quote.Action) {
		return ErrOperationConflict
	}
	var frozen servercommercialcap.TargetSnapshot
	if json.Unmarshal([]byte(quote.NativeTargetSnapshot), &frozen) != nil || frozen.LocalResourceNo != input.Target.LocalResourceNo || frozen.ChannelNo != input.Target.ChannelNo || frozen.PoolNo != input.Target.PoolNo {
		return ErrOperationConflict
	}
	busy, err := s.nativeCommercialBusy(ctx, core.EntitlementNo, "")
	if err != nil {
		return err
	}
	if busy {
		return ErrOperationConflict
	}
	if _, err := s.database.GetValue(ctx, `SELECT policy_no FROM plugin_idc_core_server_accessory_price_policy WHERE tenant_id=0 AND policy_no=? FOR UPDATE`, quote.PolicyNo); err != nil {
		return err
	}
	packageNo := ""
	if quote.Kind == serveroperationresourcecap.AccessoryKindTraffic {
		packageNo = quote.PolicyNo
	}
	policy, err := s.nativeAccessoryPolicy(ctx, core, quote.Kind, packageNo)
	if err != nil {
		return err
	}
	if policy.PolicyNo != quote.PolicyNo || policy.PolicyVersion != quote.PolicyVersion {
		return ErrOperationConflict
	}
	existing, reserved, err := nativeAccessoryOccupancy(ctx, s.database, input.Target, quote.Kind)
	if err != nil {
		return err
	}
	if existing != quote.ExistingObjectCount || reserved != quote.ReservedCreateCount {
		return ErrOperationConflict
	}
	if quote.Kind == serveroperationresourcecap.AccessoryKindTraffic {
		if err = s.reserveNativeTrafficPackage(ctx, quote.PolicyNo); err != nil {
			return err
		}
	}
	return nil
}
