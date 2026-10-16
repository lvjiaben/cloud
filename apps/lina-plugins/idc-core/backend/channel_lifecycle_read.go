// This file reads lifecycle progress without submitting or reconciling an
// operation. The original service attribution bounds every result to one AppID.
package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

// GetLifecycle returns only a persisted, customer-owned operation projection.
// Worker reconciliation remains the owner of status changes; GET is read-only.
func (runtime *channelCommerceRuntime) GetLifecycle(ctx context.Context, channelID, serviceNo, operationNo string) (*channelcommercecap.LifecycleResult, error) {
	channelID, serviceNo, operationNo = strings.TrimSpace(channelID), strings.TrimSpace(serviceNo), strings.TrimSpace(operationNo)
	if channelID == "" || serviceNo == "" || operationNo == "" || len(serviceNo) > 40 || len(operationNo) > 40 {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	var row *struct {
		OperationNo     string
		QuoteNo         string
		Status          string
		CurrencyCode    string
		AmountMinor     int64
		PaymentRequired bool
	}
	err := runtime.database.GetScan(ctx, &row, `
SELECT operation.operation_no,COALESCE(operation.quote_no,'') AS quote_no,
 operation.status_code AS status,operation.currency_code,
 COALESCE(invoice.total_amount_minor,0) AS amount_minor,
 (operation.status_code='awaiting_financial_settlement') AS payment_required
FROM plugin_idc_core_business_operation operation
JOIN plugin_idc_core_service_entitlement entitlement
 ON entitlement.tenant_id=operation.tenant_id AND entitlement.entitlement_no=operation.subject_no
JOIN plugin_idc_core_channel_order_attribution attribution
 ON attribution.tenant_id=entitlement.tenant_id AND attribution.order_no=entitlement.order_no
LEFT JOIN plugin_idc_core_billing_invoice invoice
 ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
WHERE operation.tenant_id=0 AND operation.subject_type_code='entitlement'
 AND attribution.channel_id=? AND operation.subject_no=? AND operation.operation_no=?
 AND operation.customer_no=attribution.service_owner_customer_no
 AND operation.payer_customer_no=attribution.payer_customer_no`, channelID, serviceNo, operationNo)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(commerce.CodeNotAccessible)
	}
	return &channelcommercecap.LifecycleResult{OperationNo: row.OperationNo, QuoteNo: row.QuoteNo, Status: row.Status, CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor, PaymentRequired: row.PaymentRequired}, nil
}

// ReadRenewalByRequest reads the exact digest used by ConfirmForChannel. It
// never retries a mutation and returns no foreign operation existence signal.
func (runtime *channelCommerceRuntime) ReadRenewalByRequest(ctx context.Context, channelID, serviceNo, requestID string) (*channelcommercecap.RenewalRequestResult, error) {
	if channelID == "" || serviceNo == "" || len(serviceNo) > 40 || len(requestID) < 8 || len(requestID) > 120 || strings.TrimSpace(requestID) != requestID {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	digest := sha256.Sum256([]byte(requestID + ":confirm"))
	var result *channelcommercecap.RenewalRequestResult
	err := runtime.database.GetScan(ctx, &result, `SELECT TRUE AS found,operation.operation_no,operation.status_code AS status,
operation.currency_code,COALESCE(invoice.total_amount_minor,0) AS amount_minor,entitlement.expires_at AS current_expires_at
FROM plugin_idc_core_business_operation operation
JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.tenant_id=operation.tenant_id AND entitlement.entitlement_no=operation.subject_no
JOIN plugin_idc_core_channel_order_attribution attribution ON attribution.tenant_id=entitlement.tenant_id AND attribution.order_no=entitlement.order_no
LEFT JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
WHERE operation.tenant_id=0 AND operation.operation_type_code='renewal' AND operation.subject_type_code='entitlement'
AND attribution.channel_id=? AND operation.subject_no=? AND operation.idempotency_digest=?
AND operation.customer_no=attribution.service_owner_customer_no AND operation.payer_customer_no=attribution.payer_customer_no`, channelID, serviceNo, hex.EncodeToString(digest[:]))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &channelcommercecap.RenewalRequestResult{}, nil
	}
	return result, nil
}
