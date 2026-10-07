package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

const (
	maximumQuoteAssets      = 64
	defaultQuoteLifetime    = 15 * time.Minute
	maximumQuoteAdjustments = 4_096
)

var quoteDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var CodeQuoteNotAvailable = bizerr.MustDefine(
	"IDC_COMMERCE_QUOTE_NOT_AVAILABLE",
	"The cart cannot be quoted with its current product, inventory, or eligibility facts",
	gcode.CodeInvalidOperation,
)

type quoteCustomerFact struct {
	CustomerNo       string `orm:"customer_no"`
	CustomerTypeCode string `orm:"customer_type_code"`
	StatusCode       string `orm:"status_code"`
	SubjectVersion   int64  `orm:"subject_version"`
	LevelNo          string `orm:"level_no"`
	LevelVersion     int64  `orm:"level_version"`
}

type quoteCartGraph struct {
	cart            *entity.CommerceCart
	items           []entity.CommerceCartItem
	quantities      map[string]int
	priceSelections map[string][]catalog.ConfigurationPriceSelection
}

type quoteQuantitySnapshotRow struct {
	TenantID      int    `orm:"tenant_id"`
	QuoteItemNo   string `orm:"quote_item_no"`
	Quantity      int    `orm:"quantity"`
	ContentDigest string `orm:"content_digest"`
}

type quoteInventorySnapshotRow struct {
	QuoteItemNo      string
	VersionNo        string
	TrackingEnabled  bool
	InventoryVersion int64
	SellableQuantity int64
	ContentDigest    string
}

type quoteChargeSnapshotRow struct {
	TenantID                      int     `orm:"tenant_id"`
	QuoteChargeNo                 string  `orm:"quote_charge_no"`
	QuoteItemNo                   string  `orm:"quote_item_no"`
	CurrencyCode                  string  `orm:"currency_code"`
	LineNumber                    int     `orm:"line_number"`
	ChargeKindCode                string  `orm:"charge_kind_code"`
	ChargePhaseCode               string  `orm:"charge_phase_code"`
	ConfigurationSnapshotNo       *string `orm:"configuration_snapshot_no"`
	OptionNo                      *string `orm:"option_no"`
	ConfigurationPriceDimensionNo *string `orm:"configuration_price_dimension_no"`
	Quantity                      int     `orm:"quantity"`
	ChargeAmountMinor             int64   `orm:"charge_amount_minor"`
	SetupFeeMinor                 int64   `orm:"setup_fee_minor"`
	PricedAmountMinor             int64   `orm:"priced_amount_minor"`
	PriceSourceDigest             string  `orm:"price_source_digest"`
}

type calculatedQuoteLine struct {
	catalogLine   catalog.PurchaseQuoteLine
	cartItem      entity.CommerceCartItem
	quoteItemNo   string
	amounts       quoteItemAmounts
	adjustments   []QuoteAdjustment
	contentDigest string
}

type assistedQuoteBuild struct {
	requestNo     string
	requestDigest string
	createdBy     int
	sourceCode    string
}

// QuoteGuestCart persists a final owner-scoped quote without customer discounts.
func (s *serviceImpl) QuoteGuestCart(
	ctx context.Context,
	token string,
	request QuoteRequest,
) (*Quote, error) {
	digest, err := normalizeGuestToken(token)
	if err != nil {
		return nil, err
	}
	if len(request.AssetNumbers) != 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return s.quoteCart(ctx, cartOwner{guestDigest: digest}, request)
}

// QuoteAccountCart derives ownership exclusively from the active principal.
func (s *serviceImpl) QuoteAccountCart(ctx context.Context, request QuoteRequest) (*Quote, error) {
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	return s.quoteCart(ctx, cartOwner{customerNo: customerNo}, request)
}

func (s *serviceImpl) quoteCart(
	ctx context.Context,
	owner cartOwner,
	request QuoteRequest,
) (*Quote, error) {
	assets, err := normalizeQuoteAssets(request.AssetNumbers)
	if err != nil || request.ExpectedCartVersion < 1 || s.quoteCatalog == nil {
		if s.quoteCatalog == nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		return nil, bizerr.NewCode(CodeInvalid)
	}
	quoteNo, err := newOpaqueNumber(s.random, "QTE_")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var result *Quote
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		graph, loadErr := s.lockQuoteCartGraph(ctx, owner, request.ExpectedCartVersion)
		if loadErr != nil {
			return loadErr
		}
		trialUses, trialErr := loadQuoteTrialUses(ctx, s.database, owner.customerNo, graph.items)
		if trialErr != nil {
			return trialErr
		}
		catalogInput, referenceErr := quoteCatalogInput(graph, trialUses)
		if referenceErr != nil {
			return referenceErr
		}
		catalogLines, assembleErr := s.quoteCatalog.AssemblePurchaseQuote(ctx, catalogInput)
		if assembleErr != nil {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		orderedCatalog, orderErr := orderQuoteCatalogLines(graph.items, catalogLines)
		if orderErr != nil {
			return orderErr
		}
		now, nowErr := quoteDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		customer, customerErr := loadQuoteCustomerFact(ctx, s.database, owner.customerNo)
		if customerErr != nil {
			return customerErr
		}
		calculated, calculateErr := s.prepareQuoteLines(orderedCatalog, graph.items)
		if calculateErr != nil {
			return calculateErr
		}
		adjustmentResult, adjustmentErr := s.evaluateQuoteAdjustments(
			ctx, quoteNo, graph.cart.CurrencyCode, now, customer, assets, calculated,
		)
		if adjustmentErr != nil {
			return adjustmentErr
		}
		if calculateErr = applyQuoteAdjustments(calculated, adjustmentResult); calculateErr != nil {
			return calculateErr
		}
		expiresAt, expiryErr := quoteExpiry(now, graph.cart, calculated, adjustmentResult)
		if expiryErr != nil {
			return expiryErr
		}
		result, calculateErr = s.persistQuote(
			ctx, quoteNo, graph, customer, now, expiresAt, calculated,
		)
		return calculateErr
	})
	if err != nil {
		if bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeConflict) ||
			bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeQuoteNotAvailable) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) lockQuoteCartGraph(
	ctx context.Context,
	owner cartOwner,
	expectedVersion int64,
) (*quoteCartGraph, error) {
	cart, err := lockOwnedCart(ctx, s.database, owner)
	if err != nil {
		return nil, err
	}
	if cart == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if cart.Version != expectedVersion {
		return nil, bizerr.NewCode(CodeConflict)
	}
	items := make([]entity.CommerceCartItem, 0)
	if err = s.database.GetScan(ctx, &items, lockQuoteCartItemsSQL, cart.CartNo, maximumCartItems+1); err != nil {
		return nil, gerror.Wrap(err, "lock quote cart items")
	}
	if len(items) == 0 || len(items) > maximumCartItems {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	type cartQuantityRow struct {
		CartItemNo string `orm:"cart_item_no"`
		Quantity   int    `orm:"quantity"`
	}
	quantityRows := make([]cartQuantityRow, 0, len(items))
	if err = s.database.GetScan(ctx, &quantityRows, `SELECT quantity.cart_item_no,quantity.quantity FROM plugin_idc_core_commerce_cart_item_quantity quantity
JOIN plugin_idc_core_commerce_cart_item item ON item.tenant_id=quantity.tenant_id AND item.cart_item_no=quantity.cart_item_no
WHERE item.tenant_id=0 AND item.cart_no=? ORDER BY item.sort_order,item.id LIMIT ?`, cart.CartNo, maximumCartItems+1); err != nil || len(quantityRows) != len(items) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	quantities := make(map[string]int, len(quantityRows))
	for _, row := range quantityRows {
		if row.Quantity < 1 || row.Quantity > 1000 {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		quantities[row.CartItemNo] = row.Quantity
	}
	for _, item := range items {
		if item.StatusCode != CartItemStatusActive {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	references := make([]catalog.PurchaseConfigurationReference, 0, len(items))
	for _, item := range items {
		references = append(references, catalog.PurchaseConfigurationReference{
			ReferenceNo: item.CartItemNo, ProductNo: item.ProductNo,
			VersionNo: item.VersionNo, BillingRuleNo: item.BillingRuleNo,
		})
	}
	states, err := s.catalog.RevalidatePurchaseConfigurations(ctx, references)
	if err != nil || len(states) != len(items) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	stateIndex := make(map[string]string, len(states))
	for _, state := range states {
		if state.IssueCode != "" {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		if _, duplicate := stateIndex[state.ReferenceNo]; duplicate {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		stateIndex[state.ReferenceNo] = state.IssueCode
	}
	for _, item := range items {
		if _, found := stateIndex[item.CartItemNo]; !found {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	snapshotNos := make([]string, 0, len(items))
	for _, item := range items {
		snapshotNos = append(snapshotNos, item.ConfigurationSnapshotNo)
	}
	type selectionRow struct {
		ConfigurationSnapshotNo string  `orm:"configuration_snapshot_no"`
		SelectionDigest         string  `orm:"selection_digest"`
		SelectedOptionCount     int     `orm:"selected_option_count"`
		DimensionNo             *string `orm:"dimension_no"`
		Quantity                *int    `orm:"quantity"`
	}
	var selectionRows []selectionRow
	if err = s.database.GetScan(ctx, &selectionRows, `SELECT snapshot.configuration_snapshot_no,snapshot.selection_digest,snapshot.selected_option_count,
selection.dimension_no,selection.quantity
FROM plugin_idc_core_commerce_configuration_snapshot snapshot
LEFT JOIN plugin_idc_core_commerce_configuration_price_selection selection
  ON selection.tenant_id=snapshot.tenant_id
 AND selection.configuration_snapshot_no=snapshot.configuration_snapshot_no
WHERE snapshot.tenant_id=0 AND snapshot.configuration_snapshot_no IN(?)
ORDER BY snapshot.configuration_snapshot_no,selection.dimension_no LIMIT ?`, snapshotNos, len(items)*128+len(items)+1); err != nil || len(selectionRows) > len(items)*128+len(items) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	priceSelections := make(map[string][]catalog.ConfigurationPriceSelection, len(items))
	selectionDigests := make(map[string]string, len(items))
	selectionCounts := make(map[string]int, len(items))
	for _, row := range selectionRows {
		if _, exists := priceSelections[row.ConfigurationSnapshotNo]; !exists {
			priceSelections[row.ConfigurationSnapshotNo] = []catalog.ConfigurationPriceSelection{}
		}
		selectionDigests[row.ConfigurationSnapshotNo] = row.SelectionDigest
		selectionCounts[row.ConfigurationSnapshotNo] = row.SelectedOptionCount
		if row.DimensionNo != nil && row.Quantity != nil {
			priceSelections[row.ConfigurationSnapshotNo] = append(priceSelections[row.ConfigurationSnapshotNo], catalog.ConfigurationPriceSelection{
				DimensionNo: *row.DimensionNo, Quantity: *row.Quantity,
			})
		}
	}
	for _, snapshotNo := range snapshotNos {
		selections, found := priceSelections[snapshotNo]
		if !found || len(selections) != selectionCounts[snapshotNo] {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		digest, digestErr := catalog.ConfigurationPriceSelectionDigest(selections)
		if digestErr != nil || digest != selectionDigests[snapshotNo] {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	return &quoteCartGraph{cart: cart, items: items, quantities: quantities, priceSelections: priceSelections}, nil
}

func quoteCatalogInput(
	graph *quoteCartGraph,
	authoritativeTrialUses map[string]int,
) (catalog.PurchaseQuoteInput, error) {
	input := catalog.PurchaseQuoteInput{
		CurrencyCode:           graph.cart.CurrencyCode,
		Lines:                  make([]catalog.PurchaseQuoteLineInput, 0, len(graph.items)),
		AuthoritativeTrialUses: authoritativeTrialUses,
	}
	for _, item := range graph.items {
		quantity := graph.quantities[item.CartItemNo]
		if quantity < 1 || quantity > 1000 {
			return catalog.PurchaseQuoteInput{}, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		line := catalog.PurchaseQuoteLineInput{
			ReferenceNo: item.CartItemNo, ProductNo: item.ProductNo,
			VersionNo: item.VersionNo, BillingRuleNo: item.BillingRuleNo,
			Quantity: quantity, PurchaseCycles: item.PurchaseCycles,
			PriceSelections: append([]catalog.ConfigurationPriceSelection(nil), graph.priceSelections[item.ConfigurationSnapshotNo]...),
		}
		input.Lines = append(input.Lines, line)
	}
	return input, nil
}

type quoteTrialUsageRow struct {
	ProductNo        string `orm:"product_no"`
	HeldQuantity     int    `orm:"held_quantity"`
	ConsumedQuantity int    `orm:"consumed_quantity"`
}

func loadQuoteTrialUses(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	items []entity.CommerceCartItem,
) (map[string]int, error) {
	if customerNo == "" {
		return nil, nil
	}
	productSet := make(map[string]struct{}, len(items))
	for _, item := range items {
		productSet[item.ProductNo] = struct{}{}
	}
	productNos := sortedCommerceKeys(productSet)
	result := make(map[string]int, len(productNos))
	for _, productNo := range productNos {
		result[productNo] = 0
	}
	rows := make([]quoteTrialUsageRow, 0, len(productNos))
	if err := database.Model(dao.TrialQualificationScope.Table()).Ctx(ctx).
		Fields(
			dao.TrialQualificationScope.Columns().ProductNo,
			dao.TrialQualificationScope.Columns().HeldQuantity,
			dao.TrialQualificationScope.Columns().ConsumedQuantity,
		).
		Where(do.TrialQualificationScope{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		}).
		WhereIn(dao.TrialQualificationScope.Columns().ProductNo, productNos).
		OrderAsc(dao.TrialQualificationScope.Columns().ProductNo).
		Limit(len(productNos) + 1).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "load authoritative trial usage")
	}
	if len(rows) > len(productNos) {
		return nil, gerror.New("trial usage projection exceeds requested products")
	}
	for _, row := range rows {
		if _, requested := result[row.ProductNo]; !requested || row.HeldQuantity < 0 ||
			row.ConsumedQuantity < 0 || row.HeldQuantity+row.ConsumedQuantity > 100 {
			return nil, gerror.New("trial usage projection is invalid")
		}
		result[row.ProductNo] = row.HeldQuantity + row.ConsumedQuantity
	}
	return result, nil
}

func sortedCommerceKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func orderQuoteCatalogLines(
	items []entity.CommerceCartItem,
	lines []catalog.PurchaseQuoteLine,
) ([]catalog.PurchaseQuoteLine, error) {
	if len(items) != len(lines) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	index := make(map[string]catalog.PurchaseQuoteLine, len(lines))
	for _, line := range lines {
		if _, duplicate := index[line.ReferenceNo]; duplicate {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		index[line.ReferenceNo] = line
	}
	ordered := make([]catalog.PurchaseQuoteLine, 0, len(items))
	for _, item := range items {
		line, found := index[item.CartItemNo]
		if !found {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		ordered = append(ordered, line)
	}
	return ordered, nil
}

func (s *serviceImpl) prepareQuoteLines(
	lines []catalog.PurchaseQuoteLine,
	items []entity.CommerceCartItem,
) ([]calculatedQuoteLine, error) {
	result := make([]calculatedQuoteLine, 0, len(lines))
	for index, line := range lines {
		quoteItemNo, err := newOpaqueNumber(s.random, "QIT_")
		if err != nil {
			return nil, err
		}
		amounts, err := calculateQuoteItemAmounts(line, 0)
		if err != nil {
			return nil, err
		}
		result = append(result, calculatedQuoteLine{
			catalogLine: line, cartItem: items[index], quoteItemNo: quoteItemNo,
			amounts: amounts, adjustments: []QuoteAdjustment{},
		})
	}
	return result, nil
}

func (s *serviceImpl) evaluateQuoteAdjustments(
	ctx context.Context,
	quoteNo string,
	currency string,
	now time.Time,
	customer *quoteCustomerFact,
	assets []string,
	lines []calculatedQuoteLine,
) (*QuoteAdjustmentResult, error) {
	if customer == nil {
		return &QuoteAdjustmentResult{Adjustments: []QuoteAdjustment{}}, nil
	}
	input := QuoteAdjustmentInput{
		QuoteNo: quoteNo, CustomerNo: customer.CustomerNo, CurrencyCode: currency,
		PurchasePhase: marketing.PurchasePhaseNewPurchase, CycleNumber: 1,
		IssuedAt: now, AssetNumbers: append([]string(nil), assets...),
		Lines: make([]QuoteAdjustmentLineInput, 0, len(lines)),
	}
	for _, line := range lines {
		input.Lines = append(input.Lines, QuoteAdjustmentLineInput{
			ReferenceNo: line.catalogLine.ReferenceNo, ProductNo: line.catalogLine.ProductNo,
			CategoryNo:    line.catalogLine.CategoryNo,
			SubtotalMinor: line.amounts.subtotal,
		})
	}
	result, err := s.adjustments.EvaluateAndReserveQuote(ctx, input)
	if err != nil || result == nil {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return result, nil
}

func applyQuoteAdjustments(lines []calculatedQuoteLine, result *QuoteAdjustmentResult) error {
	if result == nil || len(result.Adjustments) > maximumQuoteAdjustments {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	index := make(map[string]int, len(lines))
	for lineIndex := range lines {
		index[lines[lineIndex].catalogLine.ReferenceNo] = lineIndex
	}
	discounts := make([]int64, len(lines))
	for _, adjustment := range result.Adjustments {
		lineIndex, found := index[adjustment.ReferenceNo]
		if !found || !validQuoteAdjustment(adjustment) {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		var err error
		discounts[lineIndex], err = safeQuoteAmountAdd(discounts[lineIndex], adjustment.AmountMinor)
		if err != nil || discounts[lineIndex] > lines[lineIndex].amounts.subtotal {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		lines[lineIndex].adjustments = append(lines[lineIndex].adjustments, adjustment)
	}
	for index := range lines {
		amounts, err := calculateQuoteItemAmounts(lines[index].catalogLine, discounts[index])
		if err != nil {
			return err
		}
		lines[index].amounts = amounts
	}
	return nil
}

func validQuoteAdjustment(value QuoteAdjustment) bool {
	validType := value.TypeCode == QuoteAdjustmentCampaignDiscount || value.TypeCode == QuoteAdjustmentCoupon ||
		value.TypeCode == QuoteAdjustmentVoucher || value.TypeCode == QuoteAdjustmentCustomerPrice ||
		value.TypeCode == QuoteAdjustmentAgentPrice || value.TypeCode == QuoteAdjustmentManualDiscount
	return validType && value.AmountMinor > 0 && value.SourceReference == strings.TrimSpace(value.SourceReference) &&
		len(value.SourceReference) >= 1 && len(value.SourceReference) <= 128 &&
		quoteDigestPattern.MatchString(value.SourceVersionDigest) &&
		(value.ReservationReference == "" ||
			(value.ReservationReference == strings.TrimSpace(value.ReservationReference) &&
				len(value.ReservationReference) <= 128))
}

func quoteExpiry(
	now time.Time,
	cart *entity.CommerceCart,
	lines []calculatedQuoteLine,
	adjustments *QuoteAdjustmentResult,
) (time.Time, error) {
	expiresAt := now.Add(defaultQuoteLifetime)
	if cart.ExpiresAt == nil {
		return time.Time{}, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	expiresAt = earlierQuoteTime(expiresAt, *cart.ExpiresAt)
	if adjustments != nil && adjustments.ExpiresAt != nil {
		expiresAt = earlierQuoteTime(expiresAt, *adjustments.ExpiresAt)
	}
	if !expiresAt.After(now) {
		return time.Time{}, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return expiresAt, nil
}

func earlierQuoteTime(left, right time.Time) time.Time {
	if right.Before(left) {
		return right
	}
	return left
}

func (s *serviceImpl) persistQuote(
	ctx context.Context,
	quoteNo string,
	graph *quoteCartGraph,
	customer *quoteCustomerFact,
	issuedAt time.Time,
	expiresAt time.Time,
	lines []calculatedQuoteLine,
) (*Quote, error) {
	quote, header, itemRows, quantityRows, inventoryRows, chargeRows, adjustmentRows, err := s.buildQuoteRows(
		quoteNo, graph, customer, issuedAt, expiresAt, lines, nil,
	)
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Model(dao.CommerceQuoteSnapshot.Table()).Ctx(ctx).Data(header).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert quote snapshot")
	}
	if _, err = s.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(itemRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert quote item snapshots")
	}
	if _, err = s.database.Model("plugin_idc_core_commerce_quote_item_quantity_snapshot").Ctx(ctx).Data(quantityRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert generic quote quantities")
	}
	if _, err = s.database.Model("plugin_idc_core_commerce_quote_item_inventory_snapshot").Ctx(ctx).Data(inventoryRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert generic quote inventory snapshots")
	}
	if len(chargeRows) > 0 {
		if _, err = s.database.Model(dao.CommerceQuoteChargeSnapshot.Table()).Ctx(ctx).Data(chargeRows).Insert(); err != nil {
			return nil, gerror.Wrap(err, "insert quote charge snapshots")
		}
	}
	if len(adjustmentRows) > 0 {
		if _, err = s.database.Model(dao.CommerceQuoteAdjustmentSnapshot.Table()).Ctx(ctx).Data(adjustmentRows).Insert(); err != nil {
			return nil, gerror.Wrap(err, "insert quote adjustment snapshots")
		}
	}
	updated, err := s.database.Exec(ctx, attachLatestQuoteSQL,
		quoteNo, graph.cart.CartNo, graph.cart.Version,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "attach latest cart quote")
	}
	affected, err := updated.RowsAffected()
	if err != nil || affected != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	quote.CartVersion = graph.cart.Version + 1
	return quote, nil
}

func (s *serviceImpl) persistAssistedQuote(
	ctx context.Context,
	quoteNo string,
	graph *quoteCartGraph,
	customer *quoteCustomerFact,
	issuedAt time.Time,
	expiresAt time.Time,
	lines []calculatedQuoteLine,
	assisted assistedQuoteBuild,
) (*Quote, error) {
	quote, header, itemRows, quantityRows, inventoryRows, chargeRows, adjustmentRows, err := s.buildQuoteRows(
		quoteNo, graph, customer, issuedAt, expiresAt, lines, &assisted,
	)
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Model(dao.CommerceQuoteSnapshot.Table()).Ctx(ctx).Data(header).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert assisted quote snapshot")
	}
	if _, err = s.database.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Data(itemRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert assisted quote item snapshots")
	}
	if _, err = s.database.Model("plugin_idc_core_commerce_quote_item_quantity_snapshot").Ctx(ctx).Data(quantityRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert assisted generic quote quantities")
	}
	if _, err = s.database.Model("plugin_idc_core_commerce_quote_item_inventory_snapshot").Ctx(ctx).Data(inventoryRows).Insert(); err != nil {
		return nil, gerror.Wrap(err, "insert assisted generic quote inventory snapshots")
	}
	if len(chargeRows) > 0 {
		if _, err = s.database.Model(dao.CommerceQuoteChargeSnapshot.Table()).Ctx(ctx).Data(chargeRows).Insert(); err != nil {
			return nil, gerror.Wrap(err, "insert assisted quote charge snapshots")
		}
	}
	if len(adjustmentRows) > 0 {
		if _, err = s.database.Model(dao.CommerceQuoteAdjustmentSnapshot.Table()).Ctx(ctx).Data(adjustmentRows).Insert(); err != nil {
			return nil, gerror.Wrap(err, "insert assisted quote adjustment snapshots")
		}
	}
	return quote, nil
}

func (s *serviceImpl) buildQuoteRows(
	quoteNo string,
	graph *quoteCartGraph,
	customer *quoteCustomerFact,
	issuedAt time.Time,
	expiresAt time.Time,
	lines []calculatedQuoteLine,
	assisted *assistedQuoteBuild,
) (*Quote, do.CommerceQuoteSnapshot, []do.CommerceQuoteItemSnapshot,
	[]quoteQuantitySnapshotRow, []quoteInventorySnapshotRow, []quoteChargeSnapshotRow, []do.CommerceQuoteAdjustmentSnapshot, error,
) {
	quoteCartNo := assistedQuoteCartNo(graph, assisted)
	quote := &Quote{
		QuoteNo: quoteNo, CartNo: quoteCartNo, CurrencyCode: graph.cart.CurrencyCode,
		IssuedAt: issuedAt, ExpiresAt: expiresAt, Items: make([]QuoteItem, 0, len(lines)),
	}
	itemRows := make([]do.CommerceQuoteItemSnapshot, 0, len(lines))
	quantityRows := make([]quoteQuantitySnapshotRow, 0, len(lines))
	inventoryRows := make([]quoteInventorySnapshotRow, 0, len(lines))
	chargeRows := make([]quoteChargeSnapshotRow, 0)
	adjustmentRows := make([]do.CommerceQuoteAdjustmentSnapshot, 0)
	for lineIndex := range lines {
		line := &lines[lineIndex]
		itemDigest, err := quoteContentDigest(struct {
			Catalog     catalog.PurchaseQuoteLine
			CartItemNo  string
			Amounts     quoteItemAmounts
			Adjustments []QuoteAdjustment
			Quantity    int
		}{line.catalogLine, line.cartItem.CartItemNo, line.amounts, line.adjustments, graph.quantities[line.cartItem.CartItemNo]})
		if err != nil {
			return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
		}
		quantity := graph.quantities[line.cartItem.CartItemNo]
		if quantity < 1 || quantity > 1000 {
			return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		quantityDigest, digestErr := quoteContentDigest(struct {
			QuoteItemNo string
			Quantity    int
		}{line.quoteItemNo, quantity})
		if digestErr != nil {
			return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, digestErr
		}
		quantityRows = append(quantityRows, quoteQuantitySnapshotRow{TenantID: idccap.DefaultTenantID, QuoteItemNo: line.quoteItemNo, Quantity: quantity, ContentDigest: quantityDigest})
		inventoryDigest, digestErr := quoteContentDigest(struct {
			QuoteItemNo      string
			VersionNo        string
			TrackingEnabled  bool
			InventoryVersion int64
			SellableQuantity int64
		}{line.quoteItemNo, line.catalogLine.VersionNo, line.catalogLine.InventoryTrackingEnabled, line.catalogLine.InventoryVersion, line.catalogLine.InventorySellableQuantity})
		if digestErr != nil {
			return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, digestErr
		}
		inventoryRows = append(inventoryRows, quoteInventorySnapshotRow{
			QuoteItemNo: line.quoteItemNo, VersionNo: line.catalogLine.VersionNo,
			TrackingEnabled:  line.catalogLine.InventoryTrackingEnabled,
			InventoryVersion: line.catalogLine.InventoryVersion,
			SellableQuantity: line.catalogLine.InventorySellableQuantity,
			ContentDigest:    inventoryDigest,
		})
		line.contentDigest = itemDigest
		var cartNo, cartItemNo *string
		if assisted == nil {
			cartValue, itemValue := graph.cart.CartNo, line.cartItem.CartItemNo
			cartNo, cartItemNo = &cartValue, &itemValue
		}
		itemRows = append(itemRows, do.CommerceQuoteItemSnapshot{
			TenantId: idccap.DefaultTenantID, QuoteItemNo: line.quoteItemNo, QuoteNo: quoteNo,
			CartNo: cartNo, CartItemNo: cartItemNo, LineNumber: lineIndex + 1,
			CurrencyCode: graph.cart.CurrencyCode, ProductNo: line.catalogLine.ProductNo,
			VersionNo: line.catalogLine.VersionNo, BillingRuleNo: line.catalogLine.BillingRuleNo,
			ConfigurationSnapshotNo: line.cartItem.ConfigurationSnapshotNo,
			ProductNameZhCn:         line.catalogLine.ProductNameZhCN, ProductNameEnUs: line.catalogLine.ProductNameEnUS,
			PurchaseCycles: line.catalogLine.PurchaseCycles,
			ChargeTypeCode: line.catalogLine.ChargeTypeCode, PeriodUnitCode: line.catalogLine.PeriodUnitCode,
			TrialLimitQuantity: line.catalogLine.TrialLimitQuantity,
			PriorTrialUses:     line.catalogLine.PriorTrialUses,
			PeriodCount:        line.catalogLine.PeriodCount, RequiresContract: line.catalogLine.RequiresContract,
			ContractPolicyCode:    line.catalogLine.ContractPolicyCode,
			ContractTemplateNo:    nullableCommerceContractTemplate(line.catalogLine.ContractTemplateNo),
			FulfillmentModeCode:   line.catalogLine.FulfillmentModeCode,
			RiskDecisionCode:      line.catalogLine.RiskDecisionCode,
			InventoryDecisionCode: line.catalogLine.InventoryDecisionCode,
			RouteDecisionDigest:   line.catalogLine.RouteDecisionDigest,
			TaxPolicyNo:           line.catalogLine.TaxPolicyNo, PriceIncludesTax: line.catalogLine.PriceIncludesTax,
			TaxTreatmentCode:     line.catalogLine.TaxTreatmentCode,
			TaxRateBasisPoints:   line.catalogLine.TaxRateBasisPoints,
			TaxCategoryCode:      line.catalogLine.TaxCategoryCode,
			RoundingScopeCode:    line.catalogLine.RoundingScopeCode,
			RoundingModeCode:     line.catalogLine.RoundingModeCode,
			RoundingQuantumMinor: line.catalogLine.RoundingQuantumMinor,
			SetupSubtotalMinor:   line.amounts.setupSubtotal, PeriodSubtotalMinor: line.amounts.periodSubtotal,
			SubtotalMinor: line.amounts.subtotal, DiscountMinor: line.amounts.discount,
			NetAmountMinor: line.amounts.net, TaxAmountMinor: line.amounts.tax,
			TotalAmountMinor: line.amounts.total, ContentDigest: itemDigest,
		})
		for chargeIndex, charge := range line.catalogLine.Charges {
			chargeNo, numberErr := newOpaqueNumber(s.random, "QCH_")
			if numberErr != nil {
				return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, numberErr
			}
			phase := "periodic"
			if line.catalogLine.PeriodUnitCode == string(catalog.PeriodUnitOnce) || charge.ChargeAmountMinor == 0 {
				phase = "setup"
			}
			var configurationSnapshotNo, optionNo, dimensionNo *string
			if charge.OptionNo != "" {
				configuration, option := line.cartItem.ConfigurationSnapshotNo, charge.OptionNo
				configurationSnapshotNo = &configuration
				optionNo = &option
			}
			if charge.ConfigurationPriceDimensionNo != "" {
				configuration, dimension := line.cartItem.ConfigurationSnapshotNo, charge.ConfigurationPriceDimensionNo
				configurationSnapshotNo = &configuration
				dimensionNo = &dimension
			}
			chargeRows = append(chargeRows, quoteChargeSnapshotRow{
				TenantID: idccap.DefaultTenantID, QuoteChargeNo: chargeNo, QuoteItemNo: line.quoteItemNo,
				CurrencyCode: graph.cart.CurrencyCode, LineNumber: chargeIndex + 1,
				ChargeKindCode: charge.KindCode, ChargePhaseCode: phase,
				ConfigurationSnapshotNo: configurationSnapshotNo, OptionNo: optionNo,
				ConfigurationPriceDimensionNo: dimensionNo,
				Quantity:                      charge.Quantity, ChargeAmountMinor: charge.ChargeAmountMinor,
				SetupFeeMinor: charge.SetupFeeMinor, PricedAmountMinor: charge.PricedAmountMinor,
				PriceSourceDigest: charge.PriceSourceDigest,
			})
		}
		views := make([]QuoteAdjustmentView, 0, len(line.adjustments))
		for adjustmentIndex, adjustment := range line.adjustments {
			adjustmentNo, numberErr := newOpaqueNumber(s.random, "QAD_")
			if numberErr != nil {
				return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, numberErr
			}
			contentDigest, digestErr := quoteContentDigest(adjustment)
			if digestErr != nil {
				return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, digestErr
			}
			var reservation *string
			if adjustment.ReservationReference != "" {
				value := adjustment.ReservationReference
				reservation = &value
			}
			adjustmentRows = append(adjustmentRows, do.CommerceQuoteAdjustmentSnapshot{
				TenantId: idccap.DefaultTenantID, QuoteAdjustmentNo: adjustmentNo,
				QuoteItemNo: line.quoteItemNo, CurrencyCode: graph.cart.CurrencyCode,
				ApplicationOrder: adjustmentIndex + 1, AdjustmentTypeCode: adjustment.TypeCode,
				SourceReference:      adjustment.SourceReference,
				SourceVersionDigest:  adjustment.SourceVersionDigest,
				ReservationReference: reservation, AmountMinor: adjustment.AmountMinor,
				ContentDigest: contentDigest,
			})
			views = append(views, QuoteAdjustmentView{
				TypeCode: adjustment.TypeCode, SourceReference: adjustment.SourceReference,
				AmountMinor: adjustment.AmountMinor,
			})
		}
		quote.SubtotalMinor, err = safeQuoteAmountAdd(quote.SubtotalMinor, line.amounts.subtotal)
		if err == nil {
			quote.DiscountMinor, err = safeQuoteAmountAdd(quote.DiscountMinor, line.amounts.discount)
		}
		if err == nil {
			quote.NetAmountMinor, err = safeQuoteAmountAdd(quote.NetAmountMinor, line.amounts.net)
		}
		if err == nil {
			quote.TaxAmountMinor, err = safeQuoteAmountAdd(quote.TaxAmountMinor, line.amounts.tax)
		}
		if err == nil {
			quote.TotalAmountMinor, err = safeQuoteAmountAdd(quote.TotalAmountMinor, line.amounts.total)
		}
		if err != nil {
			return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
		}
		quote.Items = append(quote.Items, QuoteItem{
			QuoteItemNo: line.quoteItemNo, CartItemNo: quoteCartItemNo(line, assisted),
			ProductNameZhCN: line.catalogLine.ProductNameZhCN, ProductNameEnUS: line.catalogLine.ProductNameEnUS,
			PublicSlug: line.catalogLine.PublicSlug, ConfigurationRequired: line.catalogLine.ConfigurationRequired,
			Quantity: quantity, PurchaseCycles: line.catalogLine.PurchaseCycles,
			FulfillmentModeCode:   line.catalogLine.FulfillmentModeCode,
			RiskDecisionCode:      line.catalogLine.RiskDecisionCode,
			InventoryDecisionCode: line.catalogLine.InventoryDecisionCode,
			SetupSubtotalMinor:    line.amounts.setupSubtotal, PeriodSubtotalMinor: line.amounts.periodSubtotal,
			SubtotalMinor: line.amounts.subtotal, DiscountMinor: line.amounts.discount,
			NetAmountMinor: line.amounts.net, TaxAmountMinor: line.amounts.tax,
			TotalAmountMinor: line.amounts.total, Adjustments: views,
		})
	}
	inputDigest, err := quoteContentDigest(struct {
		CartNo     string
		Version    int64
		Currency   string
		Items      []entity.CommerceCartItem
		Quantities map[string]int
	}{graph.cart.CartNo, graph.cart.Version, graph.cart.CurrencyCode, graph.items, graph.quantities})
	if err != nil {
		return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
	}
	if assisted != nil {
		inputDigest = assisted.requestDigest
	}
	pricingFacts := make([]struct {
		ReferenceNo          string
		Charges              []catalog.PurchaseQuoteCharge
		TaxPolicyNo          string
		PriceIncludesTax     bool
		TaxTreatmentCode     string
		TaxRateBasisPoints   *int
		RoundingScopeCode    string
		RoundingModeCode     string
		RoundingQuantumMinor int64
	}, 0, len(lines))
	for _, line := range lines {
		pricingFacts = append(pricingFacts, struct {
			ReferenceNo          string
			Charges              []catalog.PurchaseQuoteCharge
			TaxPolicyNo          string
			PriceIncludesTax     bool
			TaxTreatmentCode     string
			TaxRateBasisPoints   *int
			RoundingScopeCode    string
			RoundingModeCode     string
			RoundingQuantumMinor int64
		}{
			ReferenceNo: line.catalogLine.ReferenceNo, Charges: line.catalogLine.Charges,
			TaxPolicyNo: line.catalogLine.TaxPolicyNo, PriceIncludesTax: line.catalogLine.PriceIncludesTax,
			TaxTreatmentCode:     line.catalogLine.TaxTreatmentCode,
			TaxRateBasisPoints:   line.catalogLine.TaxRateBasisPoints,
			RoundingScopeCode:    line.catalogLine.RoundingScopeCode,
			RoundingModeCode:     line.catalogLine.RoundingModeCode,
			RoundingQuantumMinor: line.catalogLine.RoundingQuantumMinor,
		})
	}
	pricingDigest, err := quoteContentDigest(pricingFacts)
	if err != nil {
		return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
	}
	inventoryFacts := make([]struct{ ReferenceNo, RouteDigest string }, 0, len(lines))
	for _, line := range lines {
		inventoryFacts = append(inventoryFacts, struct{ ReferenceNo, RouteDigest string }{
			line.catalogLine.ReferenceNo, line.catalogLine.RouteDecisionDigest,
		})
	}
	inventoryDigest, err := quoteContentDigest(inventoryFacts)
	if err != nil {
		return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
	}
	eligibilityDigest, err := quoteEligibilityDigest(customer, lines)
	if err != nil {
		return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
	}
	contentDigest, err := quoteContentDigest(struct {
		Quote       *Quote
		ItemDigests []do.CommerceQuoteItemSnapshot
		Adjustments []do.CommerceQuoteAdjustmentSnapshot
	}{quote, itemRows, adjustmentRows})
	if err != nil {
		return nil, do.CommerceQuoteSnapshot{}, nil, nil, nil, nil, nil, err
	}
	var cartNo *string
	if assisted == nil {
		value := graph.cart.CartNo
		cartNo = &value
	}
	source := "guest"
	createdBy := 0
	var customerNo *string
	var customerVersion *int64
	if customer != nil {
		source = "customer"
		customerNo, customerVersion = &customer.CustomerNo, &customer.SubjectVersion
	}
	if assisted != nil {
		source = assisted.sourceCode
		if source == "" {
			source = "staff"
		}
		createdBy = assisted.createdBy
	}
	header := do.CommerceQuoteSnapshot{
		TenantId: idccap.DefaultTenantID, QuoteNo: quoteNo, CartNo: cartNo,
		CustomerNo: customerNo, CustomerSubjectVersion: customerVersion,
		PurposeCode: "new_purchase", SourceCode: source, CurrencyCode: graph.cart.CurrencyCode,
		ItemCount: len(lines), SubtotalMinor: quote.SubtotalMinor,
		DiscountMinor: quote.DiscountMinor, NetAmountMinor: quote.NetAmountMinor,
		TaxAmountMinor: quote.TaxAmountMinor, TotalAmountMinor: quote.TotalAmountMinor,
		InputDigest: inputDigest, PricingRuleDigest: pricingDigest,
		EligibilityDigest: eligibilityDigest, InventoryDigest: inventoryDigest,
		ContentDigest: contentDigest, IssuedAt: &issuedAt, ExpiresAt: &expiresAt, CreatedBy: createdBy,
	}
	return quote, header, itemRows, quantityRows, inventoryRows, chargeRows, adjustmentRows, nil
}

func assistedQuoteCartNo(graph *quoteCartGraph, assisted *assistedQuoteBuild) string {
	if assisted != nil {
		return ""
	}
	return graph.cart.CartNo
}

func nullableCommerceContractTemplate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func quoteCartItemNo(line *calculatedQuoteLine, assisted *assistedQuoteBuild) string {
	if assisted != nil {
		return ""
	}
	return line.cartItem.CartItemNo
}

func quoteEligibilityDigest(customer *quoteCustomerFact, lines []calculatedQuoteLine) (string, error) {
	type adjustmentFact struct {
		ReferenceNo, TypeCode, SourceReference, SourceVersionDigest string
		AmountMinor                                                 int64
	}
	adjustments := make([]adjustmentFact, 0)
	for _, line := range lines {
		for _, adjustment := range line.adjustments {
			adjustments = append(adjustments, adjustmentFact{
				ReferenceNo: adjustment.ReferenceNo, TypeCode: adjustment.TypeCode,
				SourceReference:     adjustment.SourceReference,
				SourceVersionDigest: adjustment.SourceVersionDigest,
				AmountMinor:         adjustment.AmountMinor,
			})
		}
	}
	if customer == nil {
		return quoteContentDigest(struct {
			Customer    string
			Agent       string
			Adjustments []adjustmentFact
		}{"guest", "none", adjustments})
	}
	return quoteContentDigest(struct {
		Customer    quoteCustomerFact
		Agent       string
		Adjustments []adjustmentFact
	}{*customer, "none", adjustments})
}

func quoteContentDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func loadQuoteCustomerFact(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
) (*quoteCustomerFact, error) {
	if customerNo == "" {
		return nil, nil
	}
	rows := make([]quoteCustomerFact, 0, 2)
	if err := database.GetScan(ctx, &rows, quoteCustomerFactSQL, customerNo, 2); err != nil {
		return nil, gerror.Wrap(err, "load quote customer qualification")
	}
	if len(rows) != 1 || rows[0].StatusCode != "active" || rows[0].SubjectVersion < 1 ||
		(rows[0].LevelNo != "" && rows[0].LevelVersion < 1) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return &rows[0], nil
}

func quoteDatabaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "load quote database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("quote database time is empty")
	}
	return now, nil
}

func normalizeQuoteAssets(values []string) ([]string, error) {
	if len(values) > maximumQuoteAssets {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != strings.TrimSpace(value) || value == "" || len(value) > 128 {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

const lockQuoteCartItemsSQL = `
SELECT id, tenant_id, cart_item_no, cart_no, product_no, version_no,
       billing_rule_no, configuration_snapshot_no,
       purchase_cycles, status_code, issue_code, sort_order, version,
       removed_at, created_at, updated_at
FROM plugin_idc_core_commerce_cart_item
WHERE tenant_id = 0 AND cart_no = ? AND status_code <> 'removed'
ORDER BY sort_order ASC, id ASC
LIMIT ?
FOR UPDATE`

const quoteCustomerFactSQL = `
SELECT subject.customer_no, subject.customer_type_code, subject.status_code,
       subject.version AS subject_version,
       COALESCE(level_assignment.level_no, '') AS level_no,
       COALESCE(level.version, 0) AS level_version
FROM plugin_idc_core_customer_subject subject
LEFT JOIN plugin_idc_core_customer_level_assignment level_assignment
    ON level_assignment.tenant_id = subject.tenant_id
   AND level_assignment.customer_no = subject.customer_no
   AND level_assignment.status_code = 'active'
LEFT JOIN plugin_idc_core_customer_level level
    ON level.tenant_id = level_assignment.tenant_id
   AND level.level_no = level_assignment.level_no AND level.status_code = 'active'
WHERE subject.tenant_id = 0 AND subject.customer_no = ?
ORDER BY level_assignment.id DESC
LIMIT ?
FOR SHARE OF subject`

const attachLatestQuoteSQL = `
UPDATE plugin_idc_core_commerce_cart
SET latest_quote_no = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND cart_no = ? AND status_code = 'active'
  AND expires_at > CURRENT_TIMESTAMP AND version = ?`
