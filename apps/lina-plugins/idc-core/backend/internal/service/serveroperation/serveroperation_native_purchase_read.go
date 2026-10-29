// A lost checkout response must be recoverable from the immutable request even
// when the provider is offline, the quote has expired, or execution is pending.
package serveroperation

import (
	"context"
	"strings"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

func (s *nativeCommercialService) ReadPurchase(ctx context.Context, input servercommercialcap.PurchaseReadRequest) (*servercommercialcap.Purchase, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(input.QuoteNo, 40) || !validReference(input.EntitlementNo, 40) || !validReference(input.CustomerNo, 40) || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 || strings.TrimSpace(input.IdempotencyKey) != input.IdempotencyKey {
		return nil, PublicError(ErrInvalidInput)
	}
	intent, frozen, _, err := s.readNativeCheckoutIntent(ctx, servercommercialcap.CheckoutRequest{QuoteNo: input.QuoteNo, Target: servercommercialcap.TargetSnapshot{CustomerNo: input.CustomerNo}})
	if err != nil {
		return nil, PublicError(err)
	}
	if frozen.EntitlementNo != input.EntitlementNo || frozen.CustomerNo != input.CustomerNo {
		return nil, PublicError(ErrOperationConflict)
	}
	if err = s.authorizeNativeTarget(ctx, input.CustomerNo, intent.Action); err != nil {
		return nil, err
	}
	owned, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND entitlement_owner_id='idc-resource')`, input.EntitlementNo, input.CustomerNo)
	if err != nil {
		return nil, PublicError(err)
	}
	if !owned.Bool() {
		return nil, PublicError(ErrOperationConflict)
	}
	var rows []struct {
		OperationNo, OrderNo, InvoiceNo, StatusCode, LastErrorCode, OperationTypeCode string
		IdempotencyDigest, RequestDigest                                              string
		ExpectedSubjectVersion                                                        int64
	}
	err = s.database.GetScan(ctx, &rows, `SELECT op.operation_no,COALESCE(op.order_no,'') AS order_no,COALESCE(op.invoice_no,'') AS invoice_no,
op.status_code,op.last_error_code,op.operation_type_code,op.idempotency_digest,op.request_digest,op.expected_subject_version
FROM plugin_idc_core_business_operation op
JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=op.tenant_id AND e.entitlement_no=op.subject_no AND e.customer_no=op.customer_no AND e.entitlement_owner_id='idc-resource'
WHERE op.tenant_id=0 AND op.subject_type_code='entitlement' AND op.service_no IS NULL AND op.resource_owner_code='idc-resource'
AND op.quote_no=? AND op.subject_no=? AND op.customer_no=? AND op.currency_code=? AND op.operation_type_code IN ('addon','resize','ip_change') LIMIT 2`, input.QuoteNo, input.EntitlementNo, input.CustomerNo, intent.CurrencyCode)
	if err != nil {
		return nil, PublicError(err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, PublicError(ErrOperationConflict)
	}
	row := rows[0]
	idempotency, request, err := nativePurchaseReadDigests(input, row.OperationTypeCode)
	if err != nil {
		return nil, PublicError(err)
	}
	if row.IdempotencyDigest != idempotency || row.RequestDigest != request || row.ExpectedSubjectVersion != frozen.EntitlementVersion || (row.OperationTypeCode != "addon" && row.OperationTypeCode != string(intent.Action)) {
		return nil, PublicError(ErrOperationConflict)
	}
	return &servercommercialcap.Purchase{OperationNo: row.OperationNo, QuoteNo: input.QuoteNo, EntitlementNo: input.EntitlementNo, OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo, Status: row.StatusCode, LastErrorCode: row.LastErrorCode, CurrencyCode: intent.CurrencyCode, TotalAmountMinor: intent.TotalAmountMinor, Reused: true}, nil
}

func nativePurchaseReadDigests(input servercommercialcap.PurchaseReadRequest, kind string) (string, string, error) {
	var key, request any
	if kind == "addon" {
		key = struct{ CustomerNo, Key string }{input.CustomerNo, "native-checkout:" + input.IdempotencyKey}
		request = struct{ CustomerNo, QuoteNo string }{input.CustomerNo, input.QuoteNo}
	} else {
		key = struct{ Customer, Key string }{input.CustomerNo, "native-priced-checkout:" + input.IdempotencyKey}
		request = struct{ Customer, Quote string }{input.CustomerNo, input.QuoteNo}
	}
	idempotency, err := digestValue(key)
	if err != nil {
		return "", "", err
	}
	digest, err := digestValue(request)
	return idempotency, digest, err
}
