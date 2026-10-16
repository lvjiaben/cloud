// This file creates assisted orders inside the caller transaction and preserves
// the normal zero-amount settlement gate without withdrawing customer funds.
package commerce

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
)

func (s *assistedQuoteServiceImpl) checkoutDirectAssistedOrder(ctx context.Context, tx gdb.TX, requestNo string, actor int, key string) (*Checkout, error) {
	assisted, err := loadAssistedQuoteRequest(ctx, s.database, requestNo, true)
	if err != nil {
		return nil, err
	}
	if assisted == nil || assisted.StatusCode != string(assistedQuoteStatusDirectReady) || assisted.QuoteNo == nil {
		return nil, bizerr.NewCode(CodeAssistedQuoteConflict)
	}
	input, err := normalizeCheckoutRequestForSource(assisted.CustomerNo, "admin", CheckoutRequest{QuoteNo: *assisted.QuoteNo, IdempotencyKey: key})
	if err != nil {
		return nil, err
	}
	checkout, err := s.commerce.checkoutInTransaction(ctx, tx, checkoutAuthority{
		customerNo: assisted.CustomerNo, userID: actor, sourceCode: "admin", assistedRequestNo: &assisted.RequestNo,
		paymentTermCode: assisted.PaymentTermCode, paymentDueSeconds: assisted.PaymentDueSeconds,
		paymentDueAt:     assisted.ExpiresAt,
		contractOverride: contractpolicy.Rule{Policy: contractpolicy.Policy(assisted.ContractOverridePolicyCode), TemplateNo: assisted.ContractOverrideTemplateNo},
	}, input)
	if err != nil {
		return nil, err
	}
	if checkout == nil || checkout.OrderNo == "" {
		return nil, gerror.New("assisted checkout result is empty")
	}
	result, err := tx.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequest{TenantId: idccap.DefaultTenantID, RequestNo: requestNo, StatusCode: string(assistedQuoteStatusDirectReady), Version: assisted.Version}).
		Data(do.CommerceAssistedQuoteRequest{StatusCode: string(AssistedQuoteStatusOrdered), OrderNo: &checkout.OrderNo, Version: assisted.Version + 1}).Update()
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return nil, bizerr.NewCode(CodeAssistedQuoteConflict)
	}
	err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "commerce.assisted-quote.create", IdempotencyKey: key,
		OperationCode: "commerce.assisted-quote.create", TargetType: "assisted_quote_request", TargetNo: requestNo, CustomerNo: assisted.CustomerNo,
		ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "staff_assisted_order_created", RiskCode: "high", RequiredPermission: permissionAssistedQuoteCreate,
		CorrelationNo: checkout.OrderNo, BeforeSnapshot: map[string]any{}, AfterSnapshot: map[string]any{
			"orderNo": checkout.OrderNo, "statusCode": checkout.StatusCode, "currencyCode": assisted.CurrencyCode,
			"manualDiscountMinor": assisted.ManualDiscountMinor, "paymentDueAt": assisted.ExpiresAt, "reason": assisted.Reason,
		},
	})
	if err != nil {
		return nil, err
	}
	return checkout, nil
}
func (s *assistedQuoteServiceImpl) settleAssistedZeroCheckout(
	ctx context.Context,
	checkout *Checkout,
) error {
	if checkout == nil || checkout.TotalAmountMinor != 0 {
		return nil
	}
	settled, err := s.commerce.settlements.SettleOrder(ctx, checkout.OrderNo)
	if err != nil {
		return err
	}
	validBasis := settled != nil &&
		(settled.Basis == billing.SettlementBasisZeroAmount ||
			settled.Basis == billing.SettlementBasisTrial ||
			settled.Basis == billing.SettlementBasisFullDiscount)
	if !validBasis || settled.OrderNo != checkout.OrderNo || settled.InvoiceNo != checkout.InvoiceNo {
		return bizerr.NewCode(CodeAssistedQuoteUnavailable)
	}
	return nil
}
