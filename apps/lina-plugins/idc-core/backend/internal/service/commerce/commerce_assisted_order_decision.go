// This file builds an immutable authoritative quote for direct staff assistance.
// Historic decision facts are read-only; new orders never fabricate approval.
package commerce

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"time"
)

func (s *assistedQuoteServiceImpl) prepareAssistedOrderQuote(
	ctx context.Context,
	tx gdb.TX,
	request *entity.CommerceAssistedQuoteRequest,
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
		request.RequestDigest, request.RequestDigest,
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
	quote, err := s.commerce.persistAssistedQuote(
		ctx, quoteNo, graph, customerFact, now, expiresAt, calculated,
		assistedQuoteBuild{requestNo: request.RequestNo, requestDigest: request.RequestDigest, createdBy: actor},
	)
	if err != nil {
		return err
	}
	if err = s.freezeAssistedConfigurations(ctx, request, graph, quote); err != nil {
		return err
	}
	result, err := tx.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequest{TenantId: idccap.DefaultTenantID, RequestNo: request.RequestNo, StatusCode: string(assistedQuoteStatusDirectReady), Version: request.Version}).
		Data(do.CommerceAssistedQuoteRequest{QuoteNo: &quoteNo, Version: request.Version + 1}).Update()
	if err != nil {
		return gerror.Wrap(err, "store direct assisted quote")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return bizerr.NewCode(CodeAssistedQuoteConflict)
	}
	return nil
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
