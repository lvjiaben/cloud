package commerce

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
)

func (s *assistedQuoteServiceImpl) CheckoutAssistedQuote(
	ctx context.Context,
	request AssistedQuoteCheckoutInput,
) (*AssistedQuoteRequest, error) {
	request.RequestNo = strings.TrimSpace(request.RequestNo)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if !validAssistedIdentifier(request.RequestNo) ||
		!checkoutIdempotencyPattern.MatchString(request.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	actor, err := s.authorizer.AuthorizeAssistedQuoteCheckout(ctx, request.RequestNo)
	if err != nil {
		return nil, err
	}
	reused := false
	var checkoutResult *Checkout
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		assisted, loadErr := loadAssistedQuoteRequest(ctx, s.database, request.RequestNo, true)
		if loadErr != nil {
			return loadErr
		}
		if assisted == nil {
			return bizerr.NewCode(CodeAssistedQuoteNotAccessible)
		}
		if assisted.StatusCode != string(AssistedQuoteStatusApproved) &&
			assisted.StatusCode != string(AssistedQuoteStatusOrdered) {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		if assisted.QuoteNo == nil {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		now, nowErr := checkoutDatabaseNow(ctx, tx)
		if nowErr != nil {
			return nowErr
		}
		if assisted.StatusCode == string(AssistedQuoteStatusApproved) &&
			(assisted.ExpiresAt == nil || !assisted.ExpiresAt.After(now)) {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		checkoutInput, normalizeErr := normalizeCheckoutRequestForSource(
			assisted.CustomerNo, "admin", CheckoutRequest{
				QuoteNo: *assisted.QuoteNo, IdempotencyKey: request.IdempotencyKey,
			},
		)
		if normalizeErr != nil {
			return normalizeErr
		}
		checkout, checkoutErr := s.commerce.checkoutInTransaction(ctx, tx, checkoutAuthority{
			customerNo: assisted.CustomerNo, userID: actor, sourceCode: "admin",
			assistedRequestNo: &assisted.RequestNo,
			paymentTermCode:   assisted.PaymentTermCode,
			paymentDueSeconds: assisted.PaymentDueSeconds,
			contractOverride: contractpolicy.Rule{
				Policy:     contractpolicy.Policy(assisted.ContractOverridePolicyCode),
				TemplateNo: assisted.ContractOverrideTemplateNo,
			},
		}, checkoutInput)
		if checkoutErr != nil {
			return checkoutErr
		}
		if checkout == nil || checkout.OrderNo == "" {
			return gerror.New("assisted checkout result is empty")
		}
		checkoutResult = checkout
		if assisted.StatusCode == string(AssistedQuoteStatusOrdered) {
			if assisted.OrderNo == nil || *assisted.OrderNo != checkout.OrderNo || !checkout.Reused {
				return bizerr.NewCode(CodeAssistedQuoteConflict)
			}
			reused = true
			return nil
		}
		result, updateErr := tx.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
			Where(do.CommerceAssistedQuoteRequest{
				TenantId: idccap.DefaultTenantID, RequestNo: assisted.RequestNo,
				StatusCode: string(AssistedQuoteStatusApproved), Version: assisted.Version,
			}).Data(do.CommerceAssistedQuoteRequest{
			StatusCode: string(AssistedQuoteStatusOrdered), OrderNo: &checkout.OrderNo,
			Version: assisted.Version + 1, UpdatedAt: &now,
		}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "mark assisted quote ordered")
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil || affected != 1 {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "commerce.assisted-quote.checkout",
			IdempotencyKey:   request.IdempotencyKey,
			OperationCode:    "commerce.assisted-quote.checkout", TargetType: "assisted_quote_request",
			TargetNo: assisted.RequestNo, CustomerNo: assisted.CustomerNo,
			ResultCode: "ordered", ReasonCode: "approved_assisted_quote_checkout",
			RiskCode: "high", RequiredPermission: permissionAssistedQuoteCheckout,
			CorrelationNo: checkout.OrderNo,
			BeforeSnapshot: map[string]any{
				"statusCode": assisted.StatusCode, "version": assisted.Version,
				"quoteNo": *assisted.QuoteNo,
			},
			AfterSnapshot: map[string]any{
				"statusCode": AssistedQuoteStatusOrdered, "version": assisted.Version + 1,
				"orderNo": checkout.OrderNo, "orderStatusCode": checkout.StatusCode,
				"paymentTermCode": assisted.PaymentTermCode,
			},
		})
	})
	if err != nil {
		return nil, mapAssistedQuoteWriteError(err)
	}
	if err = s.settleAssistedZeroCheckout(ctx, checkoutResult); err != nil {
		return nil, err
	}
	assisted, err := loadAssistedQuoteRequest(ctx, s.database, request.RequestNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "reload checked out assisted quote"), CodeAssistedQuoteUnavailable)
	}
	if assisted == nil {
		return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
	}
	views, err := loadAssistedQuoteProjections(
		ctx, s.database, []entity.CommerceAssistedQuoteRequest{*assisted}, actor, reused,
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	return &views[0], nil
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
