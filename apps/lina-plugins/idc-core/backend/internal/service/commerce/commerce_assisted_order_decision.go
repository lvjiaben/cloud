package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

type normalizedAssistedQuoteDecision struct {
	AssistedQuoteDecisionInput
	requestDigest  string
	evidenceDigest string
}

func (s *assistedQuoteServiceImpl) DecideAssistedQuote(
	ctx context.Context,
	input AssistedQuoteDecisionInput,
) (*AssistedQuoteRequest, error) {
	normalized, err := normalizeAssistedQuoteDecision(input)
	if err != nil {
		return nil, err
	}
	var actor int
	if normalized.DecisionCode == AssistedQuoteDecisionApproved {
		actor, err = s.authorizer.AuthorizeAssistedQuoteApprove(ctx, normalized.RequestNo)
	} else {
		actor, err = s.authorizer.AuthorizeAssistedQuoteReject(ctx, normalized.RequestNo)
	}
	if err != nil {
		return nil, err
	}
	reused := false
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		request, loadErr := loadAssistedQuoteRequest(ctx, s.database, normalized.RequestNo, true)
		if loadErr != nil {
			return loadErr
		}
		if request == nil {
			return bizerr.NewCode(CodeAssistedQuoteNotAccessible)
		}
		if request.RequestedBy == actor {
			return bizerr.NewCode(CodeAssistedQuoteApprovalRequired)
		}
		evidenceDigest, evidenceErr := deriveAssistedDecisionEvidence(request, normalized, actor)
		if evidenceErr != nil {
			return evidenceErr
		}
		normalized.evidenceDigest = evidenceDigest
		if request.StatusCode != string(AssistedQuoteStatusPendingApproval) {
			if assistedQuoteDecisionReplayMatches(request, normalized, actor) {
				reused = true
				return nil
			}
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		now, nowErr := checkoutDatabaseNow(ctx, tx)
		if nowErr != nil {
			return nowErr
		}
		if request.ExpiresAt == nil || !request.ExpiresAt.After(now) {
			return bizerr.NewCode(CodeAssistedQuoteConflict)
		}
		if normalized.DecisionCode == AssistedQuoteDecisionRejected {
			return s.rejectAssistedQuote(ctx, tx, request, normalized, actor, now)
		}
		return s.approveAssistedQuote(ctx, tx, request, normalized, actor, now)
	})
	if err != nil {
		return nil, mapAssistedQuoteWriteError(err)
	}
	request, err := loadAssistedQuoteRequest(ctx, s.database, normalized.RequestNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "reload decided assisted quote"), CodeAssistedQuoteUnavailable)
	}
	if request == nil {
		return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
	}
	views, err := loadAssistedQuoteProjections(
		ctx, s.database, []entity.CommerceAssistedQuoteRequest{*request}, actor, reused,
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	return &views[0], nil
}

func normalizeAssistedQuoteDecision(
	input AssistedQuoteDecisionInput,
) (normalizedAssistedQuoteDecision, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.DecisionReason = strings.TrimSpace(input.DecisionReason)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validAssistedIdentifier(input.RequestNo) ||
		(input.DecisionCode != AssistedQuoteDecisionApproved &&
			input.DecisionCode != AssistedQuoteDecisionRejected) ||
		utf8.RuneCountInString(input.DecisionReason) < 2 ||
		utf8.RuneCountInString(input.DecisionReason) > 1000 ||
		!utf8.ValidString(input.DecisionReason) ||
		!checkoutIdempotencyPattern.MatchString(input.IdempotencyKey) {
		return normalizedAssistedQuoteDecision{}, bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	idempotencyDigest := sha256.Sum256([]byte(
		"assisted-quote-decision/v1\x00" + input.IdempotencyKey,
	))
	requestDigest, err := quoteContentDigest(struct {
		RequestNo, DecisionCode, DecisionReason, IdempotencyDigest string
	}{
		input.RequestNo, string(input.DecisionCode), input.DecisionReason,
		hex.EncodeToString(idempotencyDigest[:]),
	})
	if err != nil {
		return normalizedAssistedQuoteDecision{}, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	return normalizedAssistedQuoteDecision{
		AssistedQuoteDecisionInput: input, requestDigest: requestDigest,
	}, nil
}

func deriveAssistedDecisionEvidence(
	request *entity.CommerceAssistedQuoteRequest,
	input normalizedAssistedQuoteDecision,
	actor int,
) (string, error) {
	if request == nil || request.RequestNo == "" || request.RequestDigest == "" ||
		request.RequestedBy <= 0 || actor <= 0 {
		return "", gerror.New("assisted quote decision evidence source is invalid")
	}
	return quoteContentDigest(struct {
		RequestNo, RequestDigest, DecisionCode, DecisionRequestDigest string
		RequestedBy, DecisionBy                                       int
	}{
		request.RequestNo, request.RequestDigest, string(input.DecisionCode), input.requestDigest,
		request.RequestedBy, actor,
	})
}

func assistedQuoteDecisionReplayMatches(
	request *entity.CommerceAssistedQuoteRequest,
	input normalizedAssistedQuoteDecision,
	actor int,
) bool {
	return request.DecisionCode != nil && *request.DecisionCode == string(input.DecisionCode) &&
		request.DecisionBy != nil && *request.DecisionBy == actor &&
		request.DecisionReason != nil && *request.DecisionReason == input.DecisionReason &&
		request.DecisionRequestDigest != nil && *request.DecisionRequestDigest == input.requestDigest &&
		request.DecisionEvidenceDigest != nil && *request.DecisionEvidenceDigest == input.evidenceDigest
}

func (s *assistedQuoteServiceImpl) rejectAssistedQuote(
	ctx context.Context,
	tx gdb.TX,
	request *entity.CommerceAssistedQuoteRequest,
	input normalizedAssistedQuoteDecision,
	actor int,
	now time.Time,
) error {
	result, err := tx.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequest{
			TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo,
			StatusCode: string(AssistedQuoteStatusPendingApproval), Version: request.Version,
		}).Data(do.CommerceAssistedQuoteRequest{
		StatusCode: string(AssistedQuoteStatusRejected), DecisionCode: decisionStringPointer(input.DecisionCode),
		DecisionBy: &actor, DecisionAt: &now, DecisionReason: &input.DecisionReason,
		DecisionRequestDigest:  &input.requestDigest,
		DecisionEvidenceDigest: &input.evidenceDigest,
		Version:                request.Version + 1, UpdatedAt: &now,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "reject assisted quote request")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return bizerr.NewCode(CodeAssistedQuoteConflict)
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "commerce.assisted-quote.decision",
		IdempotencyKey:   input.IdempotencyKey,
		OperationCode:    "commerce.assisted-quote.reject", TargetType: "assisted_quote_request",
		TargetNo: request.RequestNo, CustomerNo: request.CustomerNo,
		ResultCode: "rejected", ReasonCode: "independent_staff_rejection",
		RiskCode: "high", RequiredPermission: permissionAssistedQuoteReject,
		BeforeSnapshot: map[string]any{"statusCode": request.StatusCode, "version": request.Version},
		AfterSnapshot: map[string]any{
			"statusCode": AssistedQuoteStatusRejected, "version": request.Version + 1,
			"decisionReason": input.DecisionReason, "evidenceDigest": input.evidenceDigest,
		},
	})
}

func (s *assistedQuoteServiceImpl) approveAssistedQuote(
	ctx context.Context,
	tx gdb.TX,
	request *entity.CommerceAssistedQuoteRequest,
	input normalizedAssistedQuoteDecision,
	actor int,
	now time.Time,
) error {
	if _, err := lockAssistedQuoteCustomer(ctx, tx, request.CustomerNo); err != nil {
		return err
	}
	graph, err := s.loadAssistedApprovalGraph(ctx, s.database, request)
	if err != nil {
		return err
	}
	trialUses, err := loadQuoteTrialUses(ctx, s.database, request.CustomerNo, graph.items)
	if err != nil {
		return err
	}
	catalogInput, err := quoteCatalogInput(graph, trialUses)
	if err != nil {
		return err
	}
	lines, err := s.assembler.AssemblePurchaseQuote(ctx, catalogInput)
	if err != nil {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	ordered, err := orderQuoteCatalogLines(graph.items, lines)
	if err != nil {
		return err
	}
	calculated, err := s.commerce.prepareQuoteLines(ordered, graph.items)
	if err != nil {
		return err
	}
	if err = applyAssistedManualDiscount(
		calculated, request.ManualDiscountMinor, request.RequestNo,
		request.RequestDigest, input.evidenceDigest,
	); err != nil {
		return err
	}
	expiresAt := earlierQuoteTime(now.Add(defaultQuoteLifetime), *request.ExpiresAt)
	if !expiresAt.After(now) {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	customerFact, err := loadQuoteCustomerFact(ctx, s.database, request.CustomerNo)
	if err != nil {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	if customerFact == nil {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	quoteNo, err := newOpaqueNumber(s.random, "QTE_")
	if err != nil {
		return err
	}
	if _, err = s.commerce.persistAssistedQuote(
		ctx, quoteNo, graph, customerFact, now, expiresAt, calculated,
		assistedQuoteBuild{requestNo: request.RequestNo, requestDigest: request.RequestDigest, createdBy: actor},
	); err != nil {
		return err
	}
	result, err := tx.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequest{
			TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo,
			StatusCode: string(AssistedQuoteStatusPendingApproval), Version: request.Version,
		}).Data(do.CommerceAssistedQuoteRequest{
		StatusCode: string(AssistedQuoteStatusApproved), DecisionCode: decisionStringPointer(input.DecisionCode),
		DecisionBy: &actor, DecisionAt: &now, DecisionReason: &input.DecisionReason,
		DecisionRequestDigest:  &input.requestDigest,
		DecisionEvidenceDigest: &input.evidenceDigest, QuoteNo: &quoteNo,
		Version: request.Version + 1, UpdatedAt: &now,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "approve assisted quote request")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return bizerr.NewCode(CodeAssistedQuoteConflict)
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "commerce.assisted-quote.decision",
		IdempotencyKey:   input.IdempotencyKey,
		OperationCode:    "commerce.assisted-quote.approve", TargetType: "assisted_quote_request",
		TargetNo: request.RequestNo, CustomerNo: request.CustomerNo,
		ResultCode: "approved", ReasonCode: "independent_staff_approval",
		RiskCode: "high", RequiredPermission: permissionAssistedQuoteApprove,
		CorrelationNo:  quoteNo,
		BeforeSnapshot: map[string]any{"statusCode": request.StatusCode, "version": request.Version},
		AfterSnapshot: map[string]any{
			"statusCode": AssistedQuoteStatusApproved, "version": request.Version + 1,
			"quoteNo": quoteNo, "manualDiscountMinor": request.ManualDiscountMinor,
			"decisionReason": input.DecisionReason, "evidenceDigest": input.evidenceDigest,
		},
	})
}

func (s *assistedQuoteServiceImpl) loadAssistedApprovalGraph(
	ctx context.Context,
	database gdb.DB,
	request *entity.CommerceAssistedQuoteRequest,
) (*quoteCartGraph, error) {
	items := make([]entity.CommerceAssistedQuoteRequestItem, 0, request.ItemCount)
	if err := database.Model(dao.CommerceAssistedQuoteRequestItem.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequestItem{
			TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo,
		}).OrderAsc(dao.CommerceAssistedQuoteRequestItem.Columns().LineNumber).
		Limit(maximumAssistedQuoteItems + 1).Scan(&items); err != nil {
		return nil, gerror.Wrap(err, "load assisted approval items")
	}
	if len(items) == 0 || len(items) > maximumAssistedQuoteItems || len(items) != request.ItemCount {
		return nil, gerror.New("assisted approval item count is inconsistent")
	}
	for index, item := range items {
		if item.LineNumber != index+1 || item.CurrencyCode != request.CurrencyCode {
			return nil, gerror.New("assisted approval item ordering is inconsistent")
		}
	}
	references := make([]catalog.PurchaseConfigurationReference, 0, len(items))
	cartItems := make([]entity.CommerceCartItem, 0, len(items))
	type assistedQuantityRow struct {
		RequestItemNo string `orm:"request_item_no"`
		Quantity      int    `orm:"quantity"`
	}
	quantityRows := make([]assistedQuantityRow, 0, len(items))
	if err := s.database.GetScan(ctx, &quantityRows, `SELECT quantity.request_item_no,quantity.quantity
FROM plugin_idc_core_commerce_assisted_quote_item_quantity quantity
JOIN plugin_idc_core_commerce_assisted_quote_request_item item ON item.tenant_id=quantity.tenant_id AND item.request_item_no=quantity.request_item_no
WHERE item.tenant_id=0 AND item.request_no=? ORDER BY item.line_number,item.id LIMIT ?`, request.RequestNo, maximumAssistedQuoteItems+1); err != nil || len(quantityRows) != len(items) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	quantities := make(map[string]int, len(quantityRows))
	for _, row := range quantityRows {
		if row.Quantity < 1 || row.Quantity > 1000 {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		quantities[row.RequestItemNo] = row.Quantity
	}
	for _, item := range items {
		references = append(references, catalog.PurchaseConfigurationReference{
			ReferenceNo: item.RequestItemNo, ProductNo: item.ProductNo,
			VersionNo: item.VersionNo, BillingRuleNo: item.BillingRuleNo,
		})
		cartItems = append(cartItems, entity.CommerceCartItem{
			CartItemNo: item.RequestItemNo, CartNo: request.RequestNo,
			ProductNo: item.ProductNo, VersionNo: item.VersionNo,
			BillingRuleNo: item.BillingRuleNo, ConfigurationSnapshotNo: item.ConfigurationSnapshotNo,
			PurchaseCycles: item.PurchaseCycles,
		})
	}
	states, err := s.catalog.RevalidatePurchaseConfigurations(ctx, references)
	if err != nil {
		return nil, err
	}
	if len(states) != len(references) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	stateIndex := make(map[string]struct{}, len(states))
	for _, state := range states {
		if state.IssueCode != "" {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		if _, duplicate := stateIndex[state.ReferenceNo]; duplicate {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		stateIndex[state.ReferenceNo] = struct{}{}
	}
	for _, reference := range references {
		if _, exists := stateIndex[reference.ReferenceNo]; !exists {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	return &quoteCartGraph{
		cart: &entity.CommerceCart{
			CartNo: request.RequestNo, CurrencyCode: request.CurrencyCode,
			Version: request.Version, ExpiresAt: request.ExpiresAt,
		},
		items: cartItems, quantities: quantities,
	}, nil
}

func applyAssistedManualDiscount(
	lines []calculatedQuoteLine,
	discount int64,
	requestNo string,
	requestDigest string,
	evidenceDigest string,
) error {
	weights := make([]int64, len(lines))
	for index := range lines {
		weights[index] = lines[index].amounts.subtotal
	}
	allocated, err := allocateQuoteAmount(discount, weights)
	if err != nil {
		return err
	}
	sourceDigest, err := quoteContentDigest(struct {
		RequestNo, RequestDigest, EvidenceDigest string
	}{requestNo, requestDigest, evidenceDigest})
	if err != nil {
		return err
	}
	adjustments := make([]QuoteAdjustment, 0, len(lines))
	for index, amount := range allocated {
		if amount == 0 {
			continue
		}
		adjustments = append(adjustments, QuoteAdjustment{
			ReferenceNo: lines[index].catalogLine.ReferenceNo,
			TypeCode:    QuoteAdjustmentManualDiscount, SourceReference: requestNo,
			SourceVersionDigest: sourceDigest, AmountMinor: amount,
		})
	}
	return applyQuoteAdjustments(lines, &QuoteAdjustmentResult{Adjustments: adjustments})
}

func decisionStringPointer(value AssistedQuoteDecision) *string {
	result := string(value)
	return &result
}
