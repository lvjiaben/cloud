package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type ipChangeCheckoutRow struct {
	QuoteNo, CustomerNo, ServiceNo, CurrencyCode, PolicyNo                            string
	ReservationNo, SnapshotNo, OldIPSetDigest, QuoteContentDigest                     string
	PoolNo, ChannelNo, EntitlementStatusCode, RuntimeStatusCode, ProviderTaskStatus   string
	IPQuantity, MaximumSnapshotAgeSeconds                                             int
	AmountMinor, PolicyVersion, CustomerSubjectVersion, CurrentCustomerSubjectVersion int64
	ExpectedServiceVersion, CurrentServiceVersion                                     int64
	ExpiresAt                                                                         time.Time
	RequireHighRiskReauth                                                             bool `orm:"require_high_risk_reauth"`
}

func (s *accessoryPricingImpl) CheckoutIPChangeCurrentCustomer(ctx context.Context,
	input IPChangeCheckoutInput) (*IPChangeCheckout, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	input.QuoteNo = strings.TrimSpace(input.QuoteNo)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) ||
		!validReference(input.QuoteNo, 40) || len(input.IdempotencyKey) < 8 ||
		len(input.IdempotencyKey) > 256 || input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	idempotencyDigest, _ := digestValue(struct{ CustomerNo, Key string }{subject.CustomerNo, input.IdempotencyKey})
	requestDigest, _ := digestValue(struct{ CustomerNo, QuoteNo string }{subject.CustomerNo, input.QuoteNo})
	if replay, replayErr := s.loadIPChangeCheckoutReplay(ctx, input.QuoteNo); replayErr != nil {
		return nil, bizerr.WrapCode(replayErr, CodeIPChangeUnavailable)
	} else if replay != nil {
		if replay.IdempotencyDigest != idempotencyDigest || replay.RequestDigest != requestDigest ||
			replay.ServiceNo == "" {
			return nil, bizerr.NewCode(CodeIPChangeConflict)
		}
		var quoteAmount struct {
			AmountMinor int64
			ExpiresAt   time.Time
		}
		if loadErr := s.database.GetScan(ctx, &quoteAmount, `SELECT amount_minor,expires_at
FROM plugin_idc_core_server_ip_change_quote
WHERE tenant_id=0 AND customer_no=? AND quote_no=?`, subject.CustomerNo, input.QuoteNo); loadErr != nil {
			return nil, bizerr.WrapCode(loadErr, CodeIPChangeUnavailable)
		}
		return projectIPChangeCheckout(replay, quoteAmount.AmountMinor, quoteAmount.ExpiresAt, true), nil
	}
	preflight, err := s.loadIPChangeCheckout(ctx, subject.CustomerNo, input.QuoteNo, false)
	if err != nil || preflight == nil {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	reservationRequest := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: preflight.ServiceNo,
			ChannelNo: preflight.ChannelNo, PoolNo: preflight.PoolNo},
		Quantity: preflight.IPQuantity, MaximumCapacityAgeSeconds: preflight.MaximumSnapshotAgeSeconds,
		ExpiresAt: preflight.ExpiresAt,
	}
	quoteIDValue, err := s.database.GetValue(ctx, `SELECT idempotency_digest
FROM plugin_idc_core_server_ip_change_quote WHERE tenant_id=0 AND quote_no=?`, input.QuoteNo)
	if err != nil || quoteIDValue.String() == "" {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	reservationRequest.IdempotencyKey = "ip-change:" + quoteIDValue.String()
	reservationRequest.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(reservationRequest)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	reservation, err := s.ipCapacity.ReserveIP(ctx, reservationRequest)
	if err != nil || reservation == nil || reservation.ReservationNo != preflight.ReservationNo ||
		reservation.Status != serveroperationresourcecap.IPReservationReserved ||
		reservation.Quantity != preflight.IPQuantity || reservation.CapacityObservationNo != preflight.SnapshotNo {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	parameters, err := json.Marshal(map[string]int{"ip_num": preflight.IPQuantity})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	confirmationBinding, err := accountOperationBinding(subject.CustomerNo, AccountCommand{
		ServiceNo: preflight.ServiceNo, Action: serveroperationresourcecap.ActionChangeIP,
		Parameters: parameters, IdempotencyKey: input.IdempotencyKey,
	}, parameters)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	if preflight.RequireHighRiskReauth {
		if _, err = s.recentAuth.Consume(ctx, input.ProofToken, confirmationBinding); err != nil {
			return nil, err
		}
	} else if input.ProofToken != "" {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	result, err := s.checkoutIPChangeTransaction(ctx, subject.CustomerNo, input.QuoteNo,
		input.IdempotencyKey, idempotencyDigest, requestDigest, ipChangeActor{Type: "customer",
			Reference: subject.CustomerNo, ReasonCode: "customer_ip_change_checkout"})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	return result, nil
}

func (s *accessoryPricingImpl) checkoutIPChangeTransaction(ctx context.Context, customerNo,
	quoteNo, idempotencyKey, idempotencyDigest, requestDigest string,
	actor ipChangeActor) (*IPChangeCheckout, error) {
	var result *IPChangeCheckout
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := s.loadIPChangeCheckout(ctx, customerNo, quoteNo, true)
		if loadErr != nil || row == nil {
			return bizerr.NewCode(CodeIPChangeUnavailable)
		}
		replay, loadErr := s.loadIPChangeCheckoutReplay(ctx, row.QuoteNo)
		if loadErr != nil {
			return loadErr
		}
		if replay != nil {
			if replay.IdempotencyDigest != idempotencyDigest || replay.RequestDigest != requestDigest {
				return bizerr.NewCode(CodeIPChangeConflict)
			}
			result = projectIPChangeCheckout(replay, row.AmountMinor, row.ExpiresAt, true)
			return nil
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if !row.ExpiresAt.After(now) || row.CustomerSubjectVersion != row.CurrentCustomerSubjectVersion ||
			row.ExpectedServiceVersion != row.CurrentServiceVersion {
			return bizerr.NewCode(CodeIPChangeConflict)
		}
		service := &serviceRow{ServiceNo: row.ServiceNo, CustomerNo: row.CustomerNo,
			CurrencyCode: row.CurrencyCode, PoolNo: row.PoolNo, ChannelNo: row.ChannelNo,
			EntitlementStatusCode: row.EntitlementStatusCode, RuntimeStatusCode: row.RuntimeStatusCode,
			ProviderTaskStatus: row.ProviderTaskStatus, Version: row.CurrentServiceVersion}
		if !allowedAction(service, serveroperationresourcecap.ActionChangeIP) {
			return bizerr.NewCode(CodeIPChangeUnavailable)
		}
		active, activeErr := s.database.GetValue(ctx, `SELECT EXISTS (
SELECT 1 FROM plugin_idc_core_server_operation_execution
 WHERE tenant_id=0 AND service_no=? AND status_code IN ('ready','pending','running','result_unknown')
UNION ALL SELECT 1 FROM plugin_idc_core_business_operation
 WHERE tenant_id=0 AND service_no=? AND status_code IN
 ('awaiting_financial_settlement','financially_settled','ready','executing','manual_review'))`, row.ServiceNo, row.ServiceNo)
		if activeErr != nil || active.Bool() {
			return bizerr.NewCode(CodeIPChangeConflict)
		}
		result, loadErr = s.persistIPChangeCheckout(ctx, row, idempotencyKey,
			idempotencyDigest, requestDigest, now, actor)
		return loadErr
	})
	return result, err
}

func (s *accessoryPricingImpl) loadIPChangeCheckout(ctx context.Context, customerNo,
	quoteNo string, lock bool) (*ipChangeCheckoutRow, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF service,customer"
	}
	var row *ipChangeCheckoutRow
	err := s.database.GetScan(ctx, &row, `SELECT quote.quote_no,quote.customer_no,quote.service_no,
quote.currency_code,quote.ip_quantity,quote.amount_minor,quote.policy_no,quote.policy_version,
quote.maximum_snapshot_age_seconds,quote.resource_reservation_no AS reservation_no,
quote.resource_snapshot_no AS snapshot_no,quote.old_ip_set_digest,quote.customer_subject_version,
quote.expected_service_version,quote.expires_at,commerce.content_digest AS quote_content_digest,
service.version AS current_service_version,customer.version AS current_customer_subject_version,
service.pool_no,COALESCE(service.channel_no,'') AS channel_no,service.entitlement_status_code,
service.runtime_status_code,service.provider_task_status_code,policy.require_high_risk_reauth
FROM plugin_idc_core_server_ip_change_quote quote
JOIN plugin_idc_core_commerce_quote_snapshot commerce
 ON commerce.tenant_id=quote.tenant_id AND commerce.quote_no=quote.quote_no
 AND commerce.customer_no=quote.customer_no AND commerce.purpose_code='ip_change'
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id=quote.tenant_id AND service.service_no=quote.service_no
 AND service.customer_no=quote.customer_no AND service.currency_code=quote.currency_code
JOIN plugin_idc_core_customer_subject customer
 ON customer.tenant_id=quote.tenant_id AND customer.customer_no=quote.customer_no
JOIN plugin_idc_core_server_operation_policy policy
 ON policy.tenant_id=quote.tenant_id
WHERE quote.tenant_id=0 AND quote.customer_no=? AND quote.quote_no=?`+suffix, customerNo, quoteNo)
	return row, err
}

type ipChangeCheckoutReplay struct {
	OperationNo, OrderNo, InvoiceNo, ServiceNo, CurrencyCode, StatusCode string
	IdempotencyDigest, RequestDigest                                     string
}

func (s *accessoryPricingImpl) loadIPChangeCheckoutReplay(ctx context.Context,
	quoteNo string) (*ipChangeCheckoutReplay, error) {
	var row *ipChangeCheckoutReplay
	err := s.database.GetScan(ctx, &row, `SELECT operation_no,COALESCE(order_no,'') AS order_no,
COALESCE(invoice_no,'') AS invoice_no,service_no,currency_code,status_code,
idempotency_digest,request_digest
FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND operation_type_code='ip_change' AND quote_no=?`, quoteNo)
	return row, err
}

func projectIPChangeCheckout(row *ipChangeCheckoutReplay, amount int64,
	due time.Time, reused bool) *IPChangeCheckout {
	return &IPChangeCheckout{OperationNo: row.OperationNo, OrderNo: row.OrderNo,
		InvoiceNo: row.InvoiceNo, ServiceNo: row.ServiceNo, Status: row.StatusCode,
		AmountMinor: amount, CurrencyCode: row.CurrencyCode, PaymentDueAt: due.UTC(),
		PaymentRequired: amount > 0, Reused: reused}
}

func (s *accessoryPricingImpl) persistIPChangeCheckout(ctx context.Context,
	row *ipChangeCheckoutRow, idempotencyKey, idempotencyDigest, requestDigest string,
	now time.Time, actor ipChangeActor) (*IPChangeCheckout, error) {
	operationNo, _ := deterministicReference("ipo_", row.QuoteNo)
	nextVersion := row.CurrentServiceVersion + 1
	status, orderNo, invoiceNo := "ready", "", ""
	financial := row.AmountMinor > 0
	var priceSnapshot any
	if financial {
		status = "awaiting_financial_settlement"
		orderNo, _ = deterministicReference("ior_", row.QuoteNo)
		invoiceNo, _ = deterministicReference("iin_", row.QuoteNo)
		priceNo, _ := deterministicReference("ips_", row.QuoteNo)
		priceSnapshot = priceNo
		accessoryRow := &accessoryCheckoutRow{QuoteNo: row.QuoteNo, CustomerNo: row.CustomerNo,
			ServiceNo: row.ServiceNo, CurrencyCode: row.CurrencyCode, ActionCode: "ip_change",
			PoolNo: row.PoolNo, ChannelNo: row.ChannelNo, RuntimeStatusCode: row.RuntimeStatusCode,
			QuoteContentDigest: row.QuoteContentDigest, TotalAmountMinor: row.AmountMinor,
			SubtotalMinor: row.AmountMinor, NetAmountMinor: row.AmountMinor, TaxAmountMinor: 0,
			ExpiresAt: row.ExpiresAt}
		// The immutable quote item remains authoritative for its internal tax split.
		var amounts struct{ SubtotalMinor, NetAmountMinor, TaxAmountMinor int64 }
		if err := s.database.GetScan(ctx, &amounts, `SELECT subtotal_minor,net_amount_minor,tax_amount_minor
FROM plugin_idc_core_commerce_quote_snapshot WHERE tenant_id=0 AND quote_no=?`, row.QuoteNo); err != nil {
			return nil, err
		}
		accessoryRow.SubtotalMinor, accessoryRow.NetAmountMinor, accessoryRow.TaxAmountMinor =
			amounts.SubtotalMinor, amounts.NetAmountMinor, amounts.TaxAmountMinor
		if err := s.persistAccessoryOrderAndInvoice(ctx, accessoryRow, orderNo, invoiceNo,
			idempotencyDigest, requestDigest, now); err != nil {
			return nil, err
		}
	}
	parameters, err := json.Marshal(map[string]int{"ip_num": row.IPQuantity})
	if err != nil {
		return nil, err
	}
	contentDigest, err := digestValue(struct {
		OperationNo, QuoteNo, ServiceNo, ReservationNo string
		Quantity, ExpectedVersion                      int64
		Financial                                      bool
	}{operationNo, row.QuoteNo, row.ServiceNo, row.ReservationNo,
		int64(row.IPQuantity), nextVersion, financial})
	if err != nil {
		return nil, err
	}
	var orderValue, invoiceValue any
	if financial {
		orderValue, invoiceValue = orderNo, invoiceNo
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
(tenant_id,operation_no,operation_type_code,customer_no,service_no,currency_code,price_snapshot_no,
quote_no,order_no,invoice_no,status_code,financial_settlement_required,execution_mode_code,
resource_owner_code,pool_no,idempotency_digest,request_digest,content_digest,expected_service_version,
last_error_code,version,created_at,updated_at)
VALUES (0,?,'ip_change',?,?,?,?,?,?,?,?,?,'unresolved','idc-resource',?,?,?,?,?,'',1,?,?)`,
		operationNo, row.CustomerNo, row.ServiceNo, row.CurrencyCode, priceSnapshot, row.QuoteNo,
		orderValue, invoiceValue, status, financial, row.PoolNo, idempotencyDigest, requestDigest,
		contentDigest, nextVersion, now, now); err != nil {
		return nil, gerror.Wrap(err, "insert IP-change business operation")
	}
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_intent
(tenant_id,operation_no,quote_no,service_no,resource_reservation_no,ip_quantity,old_ip_set_digest,
reservation_status_code,created_at,updated_at)
VALUES (0,?,?,?,?,?,?,'reserved',?,?)`, operationNo, row.QuoteNo, row.ServiceNo,
		row.ReservationNo, row.IPQuantity, row.OldIPSetDigest, now, now); err != nil {
		return nil, gerror.Wrap(err, "insert IP-change intent")
	}
	if !financial {
		if err = s.persistFreeIPChangeExecution(ctx, row, operationNo, nextVersion, parameters, now); err != nil {
			return nil, err
		}
	}
	if err = insertEvent(ctx, s.database, operationNo, row.ServiceNo, 1, nil, status,
		"server_ip_change.checkout_created", actor.Type, actor.Reference, contentDigest, now); err != nil {
		return nil, err
	}
	if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_service_instance
SET last_business_operation_no=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND customer_no=? AND version=?`, operationNo, now,
		row.ServiceNo, row.CustomerNo, row.CurrentServiceVersion); err != nil {
		return nil, bizerr.NewCode(CodeIPChangeConflict)
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "server.ip_change.checkout", IdempotencyKey: idempotencyKey,
		OperationCode: "server.ip_change.checkout", TargetType: "server_service",
		TargetNo: row.ServiceNo, CustomerNo: row.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded,
		ReasonCode: actor.ReasonCode, RiskCode: "high", CorrelationNo: operationNo,
		RequiredPermission: actor.Permission,
		BeforeSnapshot: map[string]any{"serviceVersion": row.CurrentServiceVersion,
			"oldIPSetDigest": row.OldIPSetDigest},
		AfterSnapshot: map[string]any{"serviceVersion": nextVersion,
			"ipQuantity": row.IPQuantity, "financialSettlementRequired": financial,
			"amountMinor": row.AmountMinor, "reservationNo": row.ReservationNo},
	}); err != nil {
		return nil, err
	}
	replay := &ipChangeCheckoutReplay{OperationNo: operationNo, OrderNo: orderNo,
		InvoiceNo: invoiceNo, ServiceNo: row.ServiceNo, CurrencyCode: row.CurrencyCode,
		StatusCode: status, IdempotencyDigest: idempotencyDigest, RequestDigest: requestDigest}
	return projectIPChangeCheckout(replay, row.AmountMinor, row.ExpiresAt, false), nil
}

type ipChangeActor struct {
	Type, Reference, ReasonCode, Permission string
}

func (s *accessoryPricingImpl) persistFreeIPChangeExecution(ctx context.Context,
	row *ipChangeCheckoutRow, operationNo string, expectedVersion int64,
	parameters []byte, now time.Time) error {
	request := serveroperationresourcecap.TaskRequest{OperationNo: operationNo,
		ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo,
		Action: serveroperationresourcecap.ActionChangeIP, ExpectedServiceVersion: expectedVersion,
		Parameters: parameters}
	requestDigest, err := serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		return err
	}
	executionNo, _ := deterministicReference("iox_", operationNo)
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_operation_execution
(tenant_id,execution_no,operation_no,service_no,action_code,resource_owner_code,channel_no,pool_no,
expected_service_version,original_runtime_status_code,parameters,secret_bindings,status_code,
request_digest,version,created_at,updated_at)
VALUES (0,?,?,?,'ip_change','idc-resource',?,?,?,?,?::JSONB,'[]'::JSONB,'ready',?,1,?,?)`,
		executionNo, operationNo, row.ServiceNo, row.ChannelNo, row.PoolNo, expectedVersion,
		row.RuntimeStatusCode, string(parameters), requestDigest, now, now)
	return err
}
