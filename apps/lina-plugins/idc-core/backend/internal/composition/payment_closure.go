package composition

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type PaymentClosureBridge struct {
	database  gdb.DB
	lifecycle commerce.OrderLifecycleService
}

func NewPaymentClosureBridge(database gdb.DB, lifecycle commerce.OrderLifecycleService) (*PaymentClosureBridge, error) {
	if database == nil || lifecycle == nil {
		return nil, gerror.New("payment closure bridge dependencies are required")
	}
	return &PaymentClosureBridge{database: database, lifecycle: lifecycle}, nil
}
func (b *PaymentClosureBridge) ClaimPaymentClosure(ctx context.Context) (*billingcap.PaymentClosureWork, error) {
	row, err := b.database.GetOne(ctx, `SELECT cancellation_no,order_no FROM plugin_idc_core_commerce_order_cancellation WHERE tenant_id=0 AND status_code='pending_payment_closure' ORDER BY requested_at,id LIMIT 1`)
	if err != nil {
		return nil, err
	}
	if row.IsEmpty() {
		return &billingcap.PaymentClosureWork{}, nil
	}
	result := &billingcap.PaymentClosureWork{Found: true, CancellationNo: row["cancellation_no"].String(), OrderNo: row["order_no"].String()}
	if err = billingcap.ValidatePaymentClosureWork(*result); err != nil {
		return nil, err
	}
	return result, nil
}
func (b *PaymentClosureBridge) ReportPaymentClosure(ctx context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	if err := billingcap.ValidateReportPaymentClosure(input); err != nil {
		return nil, err
	}
	result, err := b.lifecycle.FinalizePaymentClosure(ctx, commerce.PaymentClosureConfirmation{CancellationNo: input.CancellationNo, Outcome: commerce.PaymentClosureOutcome(input.OutcomeCode), Reference: input.Reference, EvidenceDigest: input.EvidenceDigest})
	if err != nil {
		return nil, err
	}
	return &billingcap.PaymentClosureReport{CancellationNo: result.CancellationNo, StatusCode: string(result.Status), OutcomeCode: input.OutcomeCode}, nil
}
