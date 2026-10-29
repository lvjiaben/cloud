// Native priced checkout admits one immutable entitlement intent. Paid orders
// wait for financial settlement; free and deferred changes never fabricate payment.

package serveroperation

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func (s *nativeCommercialService) nativeCheckoutPriced(ctx context.Context, input servercommercialcap.CheckoutRequest) (*servercommercialcap.Purchase, error) {
	if !validReference(input.QuoteNo, 40) || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 || input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) {
		return nil, PublicError(ErrInvalidInput)
	}
	initial, err := s.loadNativePricedQuote(ctx, input.QuoteNo, input.Target.CustomerNo, "")
	if err != nil {
		return nil, PublicError(err)
	}
	if initial == nil {
		return nil, PublicError(ErrOperationConflict)
	}
	if err = s.authorizeNativeTarget(ctx, input.Target.CustomerNo, initial.Price.Action); err != nil {
		return nil, err
	}
	var result *servercommercialcap.Purchase
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, err := s.readNativeCommercialTarget(ctx, input.Target, true)
		if err != nil {
			return err
		}
		q, err := s.loadNativePricedQuote(ctx, input.QuoteNo, target.CustomerNo, "")
		if err != nil {
			return err
		}
		if q == nil {
			return ErrOperationConflict
		}
		idempotency, err := digestValue(struct{ Customer, Key string }{target.CustomerNo, "native-priced-checkout:" + input.IdempotencyKey})
		if err != nil {
			return err
		}
		request, err := digestValue(struct{ Customer, Quote string }{target.CustomerNo, q.QuoteNo})
		if err != nil {
			return err
		}
		var replay *struct{ OperationNo, OrderNo, InvoiceNo, Status, IdempotencyDigest, RequestDigest string }
		if err = s.database.GetScan(ctx, &replay, `SELECT operation_no,COALESCE(order_no,'') AS order_no,COALESCE(invoice_no,'') AS invoice_no,status_code AS status,idempotency_digest,request_digest FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=? AND customer_no=? AND quote_no=? AND operation_type_code=?`, target.EntitlementNo, target.CustomerNo, q.QuoteNo, q.Price.Action); err != nil {
			return err
		}
		if replay != nil {
			if replay.IdempotencyDigest != idempotency || replay.RequestDigest != request {
				return ErrOperationConflict
			}
			result = &servercommercialcap.Purchase{OperationNo: replay.OperationNo, QuoteNo: q.QuoteNo, EntitlementNo: target.EntitlementNo, OrderNo: replay.OrderNo, InvoiceNo: replay.InvoiceNo, Status: replay.Status, CurrencyCode: target.CurrencyCode, TotalAmountMinor: q.Price.TotalAmountMinor, Reused: true}
			return nil
		}
		p := q.Price
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		if err = s.validateNativePricedCheckout(ctx, target, input, q, now); err != nil {
			return err
		}
		operationNo, err := deterministicReference("npo_", q.QuoteNo)
		if err != nil {
			return err
		}
		orderNo, invoiceNo, status := "", "", "ready"
		financial := p.TotalAmountMinor > 0
		var order, invoice, price any
		if financial {
			status = "awaiting_financial_settlement"
			orderNo, err = deterministicReference("npr_", q.QuoteNo)
			if err != nil {
				return err
			}
			invoiceNo, err = deterministicReference("npv_", q.QuoteNo)
			if err != nil {
				return err
			}
			priceNo, priceErr := deterministicReference("nps_", q.QuoteNo)
			if priceErr != nil {
				return priceErr
			}
			financialRow := &accessoryCheckoutRow{QuoteNo: q.QuoteNo, CustomerNo: target.CustomerNo, CurrencyCode: target.CurrencyCode, TotalAmountMinor: p.TotalAmountMinor, SubtotalMinor: p.SubtotalMinor, NetAmountMinor: p.NetAmountMinor, TaxAmountMinor: p.TaxAmountMinor, QuoteContentDigest: q.ContentDigest, ExpiresAt: p.ExpiresAt}
			// This existing writer only copies commerce quote facts into order/invoice
			// and non-stock snapshots; it does not access a legacy server or addon policy.
			if err = s.pricing.persistAccessoryOrderAndInvoice(ctx, financialRow, orderNo, invoiceNo, idempotency, request, now); err != nil {
				return err
			}
			order, invoice, price = orderNo, invoiceNo, priceNo
		}
		storedValue, err := s.database.GetValue(ctx, `SELECT provider_parameters::TEXT FROM plugin_idc_core_channel_service_quote WHERE tenant_id=0 AND quote_no=? AND source_code='native'`, q.QuoteNo)
		if err != nil {
			return err
		}
		storedParameters := storedValue.String()
		content, err := nativeAccessoryIntentDigest(operationNo, q.QuoteNo, orderNo, invoiceNo, target.EntitlementNo, string(p.Action), storedParameters, financial, target.EntitlementVersion)
		if err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
   (tenant_id,operation_no,operation_type_code,customer_no,service_no,subject_type_code,subject_no,currency_code,price_snapshot_no,quote_no,order_no,invoice_no,status_code,financial_settlement_required,execution_mode_code,resource_owner_code,pool_no,idempotency_digest,request_digest,content_digest,expected_service_version,expected_subject_version,last_error_code,version)
		   VALUES(0,?,?,?,NULL,'entitlement',?,?,?,?,?,?,?,?,'unresolved','idc-resource',?,?,?,?,NULL,?,'',1)`, operationNo, p.Action, target.CustomerNo, target.EntitlementNo, target.CurrencyCode, price, q.QuoteNo, order, invoice, status, financial, target.PoolNo, idempotency, request, content, target.EntitlementVersion); err != nil {
			return err
		}
		if err = s.bindNativePricedIP(ctx, p, operationNo); err != nil {
			return err
		}
		if err = appendNativeAccessoryEvent(ctx, s.database, operationNo, target.EntitlementNo, 1, "", status, "native.priced.checkout", q.QuoteNo, q.ContentDigest); err != nil {
			return err
		}
		actor := s.pricing.bizCtx.Current(ctx)
		if err = s.pricing.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "native.server.priced.checkout", IdempotencyKey: operationNo, OperationCode: "server." + string(p.Action), TargetType: "service_entitlement", TargetNo: target.EntitlementNo, CustomerNo: target.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "approved_native_quote", RiskCode: "high", CorrelationNo: operationNo, BeforeSnapshot: map[string]any{"quoteNo": q.QuoteNo, "entitlementVersion": target.EntitlementVersion}, AfterSnapshot: map[string]any{"status": status, "amountMinor": p.TotalAmountMinor, "actor": strconv.Itoa(actor.UserID)}}); err != nil {
			return err
		}
		result = &servercommercialcap.Purchase{OperationNo: operationNo, QuoteNo: q.QuoteNo, EntitlementNo: target.EntitlementNo, OrderNo: orderNo, InvoiceNo: invoiceNo, Status: status, CurrencyCode: target.CurrencyCode, TotalAmountMinor: p.TotalAmountMinor}
		return nil
	})
	return result, PublicError(err)
}

// bindNativePricedIP transfers a quote reservation to the actual commercial
// intent in the same transaction. Subsequent payment delays cannot expire it.
func (s *nativeCommercialService) bindNativePricedIP(ctx context.Context, price *PreparedNativePrice, operationNo string) error {
	if price.Action != serveroperationresourcecap.ActionChangeIP {
		return nil
	}
	holder, ok := s.prices.source.(serveroperationresourcecap.IPCommercialHold)
	if !ok || nilServerOperationDependency(holder) || price.Reservation == nil {
		return ErrServiceUnavailable
	}
	held, err := holder.BindIPCommercial(ctx, serveroperationresourcecap.IPCommercialHoldRequest{ReservationNo: price.Reservation.ReservationNo, OperationNo: operationNo})
	if err != nil {
		return err
	}
	if held == nil || serveroperationresourcecap.ValidateIPReservation(*held) != nil || held.Status != serveroperationresourcecap.IPReservationCommercialHold || held.ReservationNo != price.Reservation.ReservationNo || held.CapacityObservationNo != price.ResourceSnapshotNo || held.Quantity != price.IPQuantity {
		return ErrOperationConflict
	}
	return nil
}

// validateNativePricedCheckout rechecks current owner, route, action and IP state
// under the already held parent lock before any payable document is written.
func (s *nativeCommercialService) validateNativePricedCheckout(ctx context.Context, target *nativeCommercialTarget, input servercommercialcap.CheckoutRequest, q *nativePricedQuote, now time.Time) error {
	p := q.Price
	if !p.ExpiresAt.After(now) || p.Context.EntitlementNo != target.EntitlementNo || p.Context.EntitlementVersion != target.EntitlementVersion || p.Context.CustomerVersion != target.CustomerVersion || p.Context.PriceConfigurationNo != target.PriceConfigurationNo || p.Context.StructuredConfigurationNo != target.StructuredConfigurationNo || p.Context.CurrencyCode != target.CurrencyCode || q.Target.LocalResourceNo != input.Target.LocalResourceNo || q.Target.ChannelNo != input.Target.ChannelNo || q.Target.PoolNo != input.Target.PoolNo || !nativeSnapshotAllows(input.Target, p.Action) {
		return ErrOperationConflict
	}
	busy, err := s.nativeCommercialBusy(ctx, target.EntitlementNo, "")
	if err != nil {
		return err
	}
	if busy {
		return ErrOperationConflict
	}
	route := serveroperationresourcecap.CapabilityRoute{ServiceNo: target.LocalResourceNo, ChannelNo: target.ChannelNo, PoolNo: target.PoolNo}
	if err = s.pricing.parameters.ValidateParameters(ctx, route, p.Action, p.Parameters); err != nil {
		return PublicError(err)
	}
	if p.Action == serveroperationresourcecap.ActionChangeIP {
		addresses, addressErr := currentIPAddresses(input.Target.Items)
		if addressErr != nil || len(addresses) != p.IPQuantity || nativePriceHash("channel-ip-old/v1", target.EntitlementNo, strings.Join(addresses, ",")) != p.OldIPSetDigest {
			return ErrOperationConflict
		}
		if err = s.prices.RevalidateIP(ctx, p); err != nil {
			return PublicError(err)
		}
	}
	return nil
}
