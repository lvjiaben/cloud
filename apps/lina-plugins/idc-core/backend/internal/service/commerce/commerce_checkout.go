// This file owns the local-only atomic conversion from one immutable account
// quote into an order, receivable invoice, inventory holds, fulfillment units,
// and one transactional outbox event.

package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	maximumCheckoutFulfillmentUnits = 1000
)

var checkoutIdempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)

// CheckoutRequest identifies exactly one immutable account quote. The raw
// idempotency key is digested before persistence and never returned.
type CheckoutRequest struct {
	QuoteNo        string
	IdempotencyKey string
}

// Checkout is the bounded payment-page handoff projection. It contains no
// provider, payment-channel, credential, or internal inventory evidence.
type Checkout struct {
	OrderNo          string
	InvoiceNo        string
	QuoteNo          string
	StatusCode       string
	CurrencyCode     string
	ItemCount        int
	TotalAmountMinor int64
	RequiresContract bool
	PaymentDueAt     time.Time
	Reused           bool
}

type checkoutInput struct {
	QuoteNo           string
	idempotencyDigest string
	requestDigest     string
}

type checkoutAuthority struct {
	customerNo         string
	userID             int
	sourceCode         string
	assistedRequestNo  *string
	paymentTermCode    string
	paymentDueSeconds  int
	contractOverride   contractpolicy.Rule
	channelAttribution *channelCheckoutAttribution
}

type channelCheckoutAttribution struct {
	ChannelID              string
	ExternalCustomerID     string
	OwnershipMode          string
	ServiceOwnerCustomerNo string
}

type checkoutGenericInventoryFact struct {
	QuoteItemNo      string `orm:"quote_item_no"`
	VersionNo        string `orm:"version_no"`
	TrackingEnabled  bool   `orm:"tracking_enabled"`
	InventoryVersion int64  `orm:"inventory_version"`
	SellableQuantity int64  `orm:"sellable_quantity"`
	ContentDigest    string `orm:"content_digest"`
}

type checkoutRows struct {
	orderNo             string
	invoiceNo           string
	sourceCode          string
	statusCode          string
	paymentDueAt        time.Time
	requiresContract    bool
	order               do.CommerceOrder
	items               []do.CommerceOrderItem
	itemQuantities      []map[string]any
	itemInventories     []map[string]any
	invoice             do.BillingInvoice
	invoiceItems        []do.BillingInvoiceItem
	genericReservations []map[string]any
	reviews             []do.CommerceOrderReview
}

// CheckoutAccountQuote derives ownership exclusively from the active customer
// principal and never performs network I/O. Safe replay returns the first order.
func (s *serviceImpl) CheckoutAccountQuote(
	ctx context.Context,
	request CheckoutRequest,
) (*Checkout, error) {
	audience, err := s.currentCheckoutAudience(ctx)
	if err != nil {
		return nil, err
	}
	input, err := normalizeCheckoutRequest(audience.CustomerNo, request)
	if err != nil {
		return nil, err
	}
	var result *Checkout
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var checkoutErr error
		authority := checkoutAuthority{
			customerNo: audience.CustomerNo, userID: audience.UserID,
			sourceCode: "portal", paymentTermCode: "standard_prepay",
			contractOverride: contractpolicy.Rule{Policy: contractpolicy.PolicyInherit},
		}
		result, checkoutErr = s.checkoutInTransaction(ctx, tx, authority, input)
		return checkoutErr
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeNotAccessible) ||
			bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeCapacityExceeded) ||
			bizerr.Is(err, CodeQuoteNotAvailable) || bizerr.Is(err, CodeContractPolicyInvalid) ||
			bizerr.Is(err, CodeContractTemplateUnavailable) ||
			bizerr.Is(err, CodeContractTemplateConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if result.TotalAmountMinor == 0 {
		settled, settleErr := s.settlements.SettleOrder(ctx, result.OrderNo)
		if settleErr != nil {
			return nil, settleErr
		}
		if settled == nil || settled.OrderNo != result.OrderNo ||
			settled.InvoiceNo != result.InvoiceNo ||
			(settled.Basis != billing.SettlementBasisZeroAmount &&
				settled.Basis != billing.SettlementBasisTrial &&
				settled.Basis != billing.SettlementBasisFullDiscount) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	return result, nil
}

func (s *serviceImpl) currentCheckoutAudience(ctx context.Context) (*principal.Audience, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" || audience.UserID < 1 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return audience, nil
}

func normalizeCheckoutRequest(customerNo string, request CheckoutRequest) (checkoutInput, error) {
	return normalizeCheckoutRequestForSource(customerNo, "portal", request)
}

func normalizeCheckoutRequestForSource(
	customerNo string,
	sourceCode string,
	request CheckoutRequest,
) (checkoutInput, error) {
	if request.QuoteNo != strings.TrimSpace(request.QuoteNo) || request.QuoteNo == "" ||
		len(request.QuoteNo) > 40 || request.IdempotencyKey != strings.TrimSpace(request.IdempotencyKey) ||
		!checkoutIdempotencyPattern.MatchString(request.IdempotencyKey) ||
		(sourceCode != "portal" && sourceCode != "admin" && sourceCode != "channel") {
		return checkoutInput{}, bizerr.NewCode(CodeInvalid)
	}
	idempotencyMaterial := request.IdempotencyKey
	if sourceCode == "admin" || sourceCode == "channel" {
		idempotencyMaterial = sourceCode + "\x00" + request.IdempotencyKey
	}
	idempotencyDigest := sha256.Sum256([]byte(idempotencyMaterial))
	requestDigest, err := quoteContentDigest(struct {
		CustomerNo string `json:"customerNo"`
		QuoteNo    string `json:"quoteNo"`
		SourceCode string `json:"sourceCode"`
	}{customerNo, request.QuoteNo, sourceCode})
	if err != nil {
		return checkoutInput{}, gerror.Wrap(err, "digest checkout request")
	}
	return checkoutInput{
		QuoteNo:           request.QuoteNo,
		idempotencyDigest: hex.EncodeToString(idempotencyDigest[:]),
		requestDigest:     requestDigest,
	}, nil
}

func (s *serviceImpl) checkoutInTransaction(
	ctx context.Context,
	tx gdb.TX,
	authority checkoutAuthority,
	input checkoutInput,
) (*Checkout, error) {
	if err := lockCheckoutIdempotency(ctx, tx, authority.customerNo, input.idempotencyDigest); err != nil {
		return nil, err
	}
	existing, err := loadCheckoutByIdempotency(
		ctx, tx, authority.customerNo, input.idempotencyDigest, true,
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.CheckoutRequestDigest != input.requestDigest || existing.QuoteNo != input.QuoteNo {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return projectPersistedCheckout(ctx, tx, existing, true)
	}

	quote, err := lockCheckoutQuote(
		ctx, tx, authority.customerNo, input.QuoteNo, authority.sourceCode,
	)
	if err != nil {
		return nil, err
	}
	if existing, err = loadCheckoutByQuote(ctx, tx, quote.QuoteNo); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	now, err := checkoutDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	if quote.ExpiresAt == nil || !quote.ExpiresAt.After(now) || quote.IssuedAt == nil ||
		quote.CustomerSubjectVersion == nil {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	cart, err := lockCheckoutCartForAuthority(ctx, tx, quote, authority, now)
	if err != nil {
		return nil, err
	}
	customer, err := lockCheckoutCustomer(ctx, tx, authority.customerNo, *quote.CustomerSubjectVersion)
	if err != nil {
		return nil, err
	}
	items, err := loadCheckoutQuoteItems(ctx, tx, quote)
	if err != nil {
		return nil, err
	}
	quantities, err := loadCheckoutQuoteItemQuantities(ctx, tx, quote, items)
	if err != nil {
		return nil, err
	}
	inventoryFacts, err := loadCheckoutQuoteItemInventories(ctx, tx, quote, items)
	if err != nil {
		return nil, err
	}
	_, manualReview, _, err := checkoutRequirements(items, quantities, inventoryFacts)
	if err != nil {
		return nil, err
	}
	contractDecision, err := s.resolveCheckoutContractDecision(ctx, customer, items, authority)
	if err != nil {
		return nil, err
	}
	if err = lockAndValidateGenericCheckoutInventory(ctx, tx, items, quantities, inventoryFacts); err != nil {
		return nil, err
	}

	paymentWindow, err := checkoutPaymentWindowForAuthority(ctx, tx, authority)
	if err != nil {
		return nil, err
	}
	paymentDueAt := now.Add(paymentWindow)
	rows, err := s.buildCheckoutRows(
		quote, items, authority, input, now, paymentDueAt,
		manualReview, contractDecision, quantities, inventoryFacts,
	)
	if err != nil {
		return nil, err
	}
	if err = insertCheckoutRows(ctx, tx, rows); err != nil {
		return nil, err
	}
	if err = insertChannelCheckoutAttribution(ctx, tx, rows.orderNo, authority); err != nil {
		return nil, err
	}
	if s.structuredConfiguration != nil {
		if err = s.structuredConfiguration.bindCheckout(ctx, tx, authority, items, rows); err != nil {
			return nil, err
		}
	}
	if err = s.reserveCheckoutTrialQualifications(
		ctx, tx, authority.customerNo, quote.CurrencyCode, rows, items, quantities, now,
	); err != nil {
		return nil, err
	}
	if err = s.consumeCheckoutReservationEvidence(ctx, tx, authority.customerNo, quote.QuoteNo); err != nil {
		return nil, err
	}
	if cart != nil {
		if err = closeCheckedOutCart(ctx, tx, cart, now); err != nil {
			return nil, err
		}
	}
	if err = s.enqueueCheckoutCreated(ctx, rows, quote, now); err != nil {
		return nil, err
	}
	return projectCheckoutRows(rows, quote, false)
}

func lockCheckoutCartForAuthority(
	ctx context.Context,
	tx gdb.TX,
	quote *entity.CommerceQuoteSnapshot,
	authority checkoutAuthority,
	now time.Time,
) (*entity.CommerceCart, error) {
	if authority.sourceCode == "portal" {
		if quote.CartNo == nil {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		return lockCheckoutCart(ctx, tx, quote, authority.customerNo, now)
	}
	if authority.sourceCode == "channel" && quote.CartNo == nil && authority.assistedRequestNo == nil && authority.channelAttribution != nil {
		return nil, nil
	}
	if authority.sourceCode != "admin" || quote.CartNo != nil || authority.assistedRequestNo == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return nil, nil
}

func insertChannelCheckoutAttribution(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
	authority checkoutAuthority,
) error {
	if authority.sourceCode != "channel" {
		if authority.channelAttribution != nil {
			return bizerr.NewCode(CodeInvalid)
		}
		return nil
	}
	attribution := authority.channelAttribution
	if attribution == nil || strings.TrimSpace(attribution.ChannelID) == "" ||
		len(attribution.ChannelID) > 80 || strings.TrimSpace(attribution.ServiceOwnerCustomerNo) == "" ||
		len(attribution.ServiceOwnerCustomerNo) > 40 || len(attribution.ExternalCustomerID) > 160 ||
		(attribution.OwnershipMode != "channel_owned" && attribution.OwnershipMode != "delegated_customer") ||
		(attribution.OwnershipMode == "channel_owned" &&
			(attribution.ServiceOwnerCustomerNo != authority.customerNo || attribution.ExternalCustomerID != "")) ||
		(attribution.OwnershipMode == "delegated_customer" &&
			(attribution.ServiceOwnerCustomerNo == authority.customerNo || strings.TrimSpace(attribution.ExternalCustomerID) == "")) {
		return bizerr.NewCode(CodeInvalid)
	}
	_, err := tx.Exec(`INSERT INTO plugin_idc_core_channel_order_attribution(
tenant_id,order_no,channel_id,external_customer_id,ownership_mode,payer_customer_no,service_owner_customer_no
) VALUES(0,?,?,?,?,?,?)`, orderNo, attribution.ChannelID, attribution.ExternalCustomerID,
		attribution.OwnershipMode, authority.customerNo, attribution.ServiceOwnerCustomerNo)
	return gerror.Wrap(err, "insert channel order attribution")
}

func checkoutPaymentWindowForAuthority(
	ctx context.Context,
	tx gdb.TX,
	authority checkoutAuthority,
) (time.Duration, error) {
	if authority.sourceCode != "admin" {
		return loadCheckoutPaymentWindow(ctx, tx)
	}
	validTerm := authority.paymentTermCode == "standard_prepay" ||
		authority.paymentTermCode == "extended_prepay"
	validWindow := authority.paymentDueSeconds >= 300 && authority.paymentDueSeconds <= 604800
	validStandard := authority.paymentTermCode != "standard_prepay" || authority.paymentDueSeconds <= 86400
	validExtended := authority.paymentTermCode != "extended_prepay" || authority.paymentDueSeconds >= 86401
	if !validTerm || !validWindow || !validStandard || !validExtended {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	return time.Duration(authority.paymentDueSeconds) * time.Second, nil
}

func (s *serviceImpl) consumeCheckoutReservationEvidence(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	quoteNo string,
) error {
	var evidence struct {
		ReservationCount int `orm:"reservation_count"`
	}
	if err := tx.Ctx(ctx).GetScan(&evidence, `
SELECT COUNT(*) FILTER (WHERE a.reservation_reference IS NOT NULL) AS reservation_count
FROM plugin_idc_core_commerce_quote_adjustment_snapshot a
JOIN plugin_idc_core_commerce_quote_item_snapshot i
  ON i.quote_item_no = a.quote_item_no
WHERE a.tenant_id = 0 AND i.tenant_id = 0 AND i.quote_no = ?`, quoteNo); err != nil {
		return gerror.Wrap(err, "count checkout quote adjustments")
	}
	if evidence.ReservationCount == 0 {
		return nil
	}
	if !s.hasReservationFinalizer {
		return gerror.New("checkout quote reservation finalizer is unavailable")
	}
	if err := s.reservations.ConsumeQuote(ctx, customerNo, quoteNo); err != nil {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return nil
}

func lockCheckoutIdempotency(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	idempotencyDigest string,
) error {
	value, err := tx.Ctx(ctx).GetValue(
		`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`,
		"idc-checkout:"+customerNo+":"+idempotencyDigest,
	)
	if err != nil {
		return gerror.Wrap(err, "lock checkout idempotency scope")
	}
	_ = value
	return nil
}

func checkoutDatabaseNow(ctx context.Context, tx gdb.TX) (time.Time, error) {
	value, err := tx.Ctx(ctx).GetValue(`SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "load checkout database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("checkout database time is empty")
	}
	return now, nil
}

func loadCheckoutPaymentWindow(ctx context.Context, tx gdb.TX) (time.Duration, error) {
	var policy entity.CommerceCheckoutPolicy
	if err := tx.Ctx(ctx).GetScan(&policy, `
SELECT payment_validity_seconds
FROM plugin_idc_core_commerce_checkout_policy
WHERE tenant_id = 0 AND policy_no = 'default'`); err != nil {
		return 0, gerror.Wrap(err, "load checkout payment validity policy")
	}
	if policy.PaymentValiditySeconds < 300 || policy.PaymentValiditySeconds > 86400 {
		return 0, gerror.New("checkout payment validity policy is unavailable")
	}
	return time.Duration(policy.PaymentValiditySeconds) * time.Second, nil
}

func loadCheckoutByIdempotency(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	idempotencyDigest string,
	lock bool,
) (*entity.CommerceOrder, error) {
	var row *entity.CommerceOrder
	model := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		CheckoutIdempotencyDigest: idempotencyDigest,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load checkout by idempotency scope")
	}
	return row, nil
}

func loadCheckoutByQuote(ctx context.Context, tx gdb.TX, quoteNo string) (*entity.CommerceOrder, error) {
	var row *entity.CommerceOrder
	if err := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{
		TenantId: idccap.DefaultTenantID, QuoteNo: quoteNo,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load checkout by quote")
	}
	return row, nil
}

func lockCheckoutQuote(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	quoteNo string,
	orderSource string,
) (*entity.CommerceQuoteSnapshot, error) {
	var quote *entity.CommerceQuoteSnapshot
	if err := tx.Model(dao.CommerceQuoteSnapshot.Table()).Ctx(ctx).Where(do.CommerceQuoteSnapshot{
		TenantId: idccap.DefaultTenantID, QuoteNo: quoteNo,
	}).LockUpdate().Scan(&quote); err != nil {
		return nil, gerror.Wrap(err, "lock checkout quote")
	}
	expectedQuoteSource := "customer"
	if orderSource == "admin" {
		expectedQuoteSource = "staff"
	} else if orderSource == "channel" {
		expectedQuoteSource = "channel"
	}
	if quote == nil || quote.CustomerNo == nil || *quote.CustomerNo != customerNo ||
		quote.SourceCode != expectedQuoteSource || quote.PurposeCode != "new_purchase" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return quote, nil
}

func lockCheckoutCart(
	ctx context.Context,
	tx gdb.TX,
	quote *entity.CommerceQuoteSnapshot,
	customerNo string,
	now time.Time,
) (*entity.CommerceCart, error) {
	var cart *entity.CommerceCart
	if err := tx.Model(dao.CommerceCart.Table()).Ctx(ctx).Where(do.CommerceCart{
		TenantId: idccap.DefaultTenantID, CartNo: *quote.CartNo,
	}).LockUpdate().Scan(&cart); err != nil {
		return nil, gerror.Wrap(err, "lock checkout cart")
	}
	if cart == nil || cart.CustomerNo == nil || *cart.CustomerNo != customerNo ||
		cart.StatusCode != "active" || cart.LatestQuoteNo == nil || *cart.LatestQuoteNo != quote.QuoteNo ||
		cart.CurrencyCode != quote.CurrencyCode || cart.ExpiresAt == nil || !cart.ExpiresAt.After(now) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return cart, nil
}

func lockCheckoutCustomer(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	quotedVersion int64,
) (*entity.CustomerSubject, error) {
	var customer *entity.CustomerSubject
	if err := tx.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	}).LockUpdate().Scan(&customer); err != nil {
		return nil, gerror.Wrap(err, "lock checkout customer")
	}
	if customer == nil || customer.StatusCode != "active" || customer.Version != quotedVersion {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return customer, nil
}

func (s *serviceImpl) resolveCheckoutContractDecision(
	ctx context.Context,
	customer *entity.CustomerSubject,
	items []entity.CommerceQuoteItemSnapshot,
	authority checkoutAuthority,
) (contractpolicy.Decision, error) {
	if customer == nil {
		return contractpolicy.Decision{}, bizerr.NewCode(CodeContractPolicyInvalid)
	}
	customerType := contracttemplatecap.CustomerType(customer.CustomerTypeCode)
	enterpriseRule := contractpolicy.Rule{Policy: contractpolicy.PolicyNone}
	if customerType == contracttemplatecap.CustomerTypeEnterprise {
		enterpriseRule = contractpolicy.Rule{
			Policy:     contractpolicy.Policy(customer.EnterpriseContractPolicyCode),
			TemplateNo: customer.EnterpriseContractTemplateNo,
		}
	}
	productRules := make([]contractpolicy.Rule, 0, len(items))
	for _, item := range items {
		productRules = append(productRules, contractpolicy.Rule{
			Policy: contractpolicy.Policy(item.ContractPolicyCode), TemplateNo: item.ContractTemplateNo,
		})
	}
	decision, err := s.contractPolicy.Resolve(ctx, contractpolicy.ResolveInput{
		CustomerType: customerType, EnterpriseRule: enterpriseRule,
		ProductRules: productRules, OrderOverride: authority.contractOverride,
	})
	if err == nil {
		return decision, nil
	}
	return contractpolicy.Decision{}, mapContractPolicyResolveError(err)
}

func mapContractPolicyResolveError(err error) error {
	switch {
	case errors.Is(err, contractpolicy.ErrTemplateConflict):
		return bizerr.WrapCode(err, CodeContractTemplateConflict)
	case errors.Is(err, contracttemplatecap.ErrSourceUnavailable),
		errors.Is(err, contracttemplatecap.ErrTemplateUnavailable):
		return bizerr.WrapCode(err, CodeContractTemplateUnavailable)
	default:
		return bizerr.WrapCode(err, CodeContractPolicyInvalid)
	}
}

func loadCheckoutQuoteItems(
	ctx context.Context,
	tx gdb.TX,
	quote *entity.CommerceQuoteSnapshot,
) ([]entity.CommerceQuoteItemSnapshot, error) {
	items := make([]entity.CommerceQuoteItemSnapshot, 0, quote.ItemCount)
	err := tx.Model(dao.CommerceQuoteItemSnapshot.Table()).Ctx(ctx).Where(do.CommerceQuoteItemSnapshot{
		TenantId: idccap.DefaultTenantID, QuoteNo: quote.QuoteNo,
	}).OrderAsc(dao.CommerceQuoteItemSnapshot.Columns().LineNumber).
		Limit(maximumCartItems + 1).Scan(&items)
	if err != nil {
		return nil, gerror.Wrap(err, "load checkout quote items")
	}
	if len(items) == 0 || len(items) > maximumCartItems || len(items) != quote.ItemCount {
		return nil, gerror.New("checkout quote item count is inconsistent")
	}
	var subtotal, discount, net, tax, total int64
	for index, item := range items {
		if item.LineNumber != index+1 || item.CurrencyCode != quote.CurrencyCode {
			return nil, gerror.New("checkout quote item ordering is inconsistent")
		}
		var err error
		if subtotal, err = safeQuoteAmountAdd(subtotal, item.SubtotalMinor); err != nil {
			return nil, err
		}
		if discount, err = safeQuoteAmountAdd(discount, item.DiscountMinor); err != nil {
			return nil, err
		}
		if net, err = safeQuoteAmountAdd(net, item.NetAmountMinor); err != nil {
			return nil, err
		}
		if tax, err = safeQuoteAmountAdd(tax, item.TaxAmountMinor); err != nil {
			return nil, err
		}
		if total, err = safeQuoteAmountAdd(total, item.TotalAmountMinor); err != nil {
			return nil, err
		}
	}
	if subtotal != quote.SubtotalMinor || discount != quote.DiscountMinor || net != quote.NetAmountMinor ||
		tax != quote.TaxAmountMinor || total != quote.TotalAmountMinor {
		return nil, gerror.New("checkout quote totals are inconsistent")
	}
	return items, nil
}

func checkoutRequirements(
	items []entity.CommerceQuoteItemSnapshot,
	quantities map[string]int,
	inventoryFactSets ...map[string]checkoutGenericInventoryFact,
) (int, bool, bool, error) {
	var inventoryFacts map[string]checkoutGenericInventoryFact
	if len(inventoryFactSets) > 0 {
		inventoryFacts = inventoryFactSets[0]
	}
	totalUnits := 0
	manualReview := false
	requiresContract := false
	trialItems := 0
	for _, item := range items {
		quantity := quantities[item.QuoteItemNo]
		if quantity < 1 || totalUnits > maximumCheckoutFulfillmentUnits-quantity {
			return 0, false, false, bizerr.NewCode(CodeCapacityExceeded)
		}
		totalUnits += quantity
		if item.ChargeTypeCode == "trial" {
			trialItems++
			if !validCheckoutTrialItem(item) {
				return 0, false, false, bizerr.NewCode(CodeQuoteNotAvailable)
			}
		} else if item.TrialLimitQuantity != 0 || item.PriorTrialUses != 0 {
			return 0, false, false, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		requiresContract = requiresContract || item.RequiresContract
		if item.InventoryDecisionCode != "available" {
			return 0, false, false, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		manualReview = manualReview || item.RiskDecisionCode == "manual_review" ||
			item.FulfillmentModeCode == "manual_review"
		if fact, generic := inventoryFacts[item.QuoteItemNo]; generic && fact.InventoryVersion > 0 {
			if fact.VersionNo != item.VersionNo || fact.SellableQuantity < 0 ||
				(fact.TrackingEnabled && fact.SellableQuantity < int64(quantity)) {
				return 0, false, false, bizerr.NewCode(CodeQuoteNotAvailable)
			}
			continue
		}
		return 0, false, false, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	if trialItems != 0 && trialItems != len(items) {
		return 0, false, false, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return totalUnits, manualReview, requiresContract, nil
}

func validCheckoutTrialItem(item entity.CommerceQuoteItemSnapshot) bool {
	return item.TrialLimitQuantity >= 1 && item.TrialLimitQuantity <= 100 &&
		item.PriorTrialUses >= 0 && item.PriorTrialUses < item.TrialLimitQuantity &&
		item.PurchaseCycles == 1 && item.SetupSubtotalMinor == 0 &&
		item.PeriodSubtotalMinor == 0 && item.SubtotalMinor == 0 && item.DiscountMinor == 0 &&
		item.NetAmountMinor == 0 && item.TaxAmountMinor == 0 && item.TotalAmountMinor == 0
}

func loadCheckoutQuoteItemInventories(
	ctx context.Context,
	tx gdb.TX,
	quote *entity.CommerceQuoteSnapshot,
	items []entity.CommerceQuoteItemSnapshot,
) (map[string]checkoutGenericInventoryFact, error) {
	rows := make([]checkoutGenericInventoryFact, 0, len(items))
	if err := tx.Ctx(ctx).GetScan(&rows, `SELECT inventory.quote_item_no,inventory.version_no,
inventory.tracking_enabled,inventory.inventory_version,inventory.sellable_quantity,inventory.content_digest
FROM plugin_idc_core_commerce_quote_item_inventory_snapshot inventory
JOIN plugin_idc_core_commerce_quote_item_snapshot item ON item.quote_item_no=inventory.quote_item_no
WHERE item.tenant_id=0 AND item.quote_no=? ORDER BY item.line_number,item.id LIMIT ?`, quote.QuoteNo, maximumCartItems+1); err != nil {
		return nil, gerror.Wrap(err, "load generic quote inventory snapshots")
	}
	if len(rows) != len(items) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	result := make(map[string]checkoutGenericInventoryFact, len(rows))
	for _, row := range rows {
		if row.QuoteItemNo == "" || row.VersionNo == "" || row.InventoryVersion < 0 || row.SellableQuantity < 0 || !quoteDigestPattern.MatchString(row.ContentDigest) {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		if _, duplicate := result[row.QuoteItemNo]; duplicate {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		result[row.QuoteItemNo] = row
	}
	return result, nil
}

func lockAndValidateGenericCheckoutInventory(
	ctx context.Context,
	tx gdb.TX,
	items []entity.CommerceQuoteItemSnapshot,
	quantities map[string]int,
	facts map[string]checkoutGenericInventoryFact,
) error {
	requested := make(map[string]int64)
	expectedVersion := make(map[string]int64)
	for _, item := range items {
		fact := facts[item.QuoteItemNo]
		if fact.InventoryVersion == 0 || !fact.TrackingEnabled {
			continue
		}
		requested[fact.VersionNo] += int64(quantities[item.QuoteItemNo])
		if existing, found := expectedVersion[fact.VersionNo]; found && existing != fact.InventoryVersion {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		expectedVersion[fact.VersionNo] = fact.InventoryVersion
	}
	if len(requested) == 0 {
		return nil
	}
	versions := make([]string, 0, len(requested))
	for versionNo := range requested {
		versions = append(versions, versionNo)
	}
	sort.Strings(versions)
	rows := make([]struct {
		VersionNo        string `orm:"version_no"`
		TrackingEnabled  bool   `orm:"tracking_enabled"`
		SellableQuantity int64  `orm:"sellable_quantity"`
		Version          int64  `orm:"version"`
	}, 0, len(versions))
	if err := tx.Ctx(ctx).GetScan(&rows, `SELECT version_no,tracking_enabled,sellable_quantity,version
FROM plugin_idc_core_catalog_product_version_inventory
WHERE version_no=ANY(string_to_array(?,',')) ORDER BY version_no FOR UPDATE`, strings.Join(versions, ",")); err != nil {
		return gerror.Wrap(err, "lock generic product inventory")
	}
	if len(rows) != len(versions) {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	available := make(map[string]int64, len(rows))
	for _, row := range rows {
		if !row.TrackingEnabled || row.Version != expectedVersion[row.VersionNo] {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		available[row.VersionNo] = row.SellableQuantity
	}
	reserved := make([]struct {
		VersionNo string `orm:"version_no"`
		Quantity  int64  `orm:"quantity"`
	}, 0, len(versions))
	if err := tx.Ctx(ctx).GetScan(&reserved, `SELECT version_no,COALESCE(SUM(reserved_quantity),0) quantity
FROM plugin_idc_core_commerce_inventory_reservation
WHERE version_no=ANY(string_to_array(?,',')) AND status_code IN('held','committed')
GROUP BY version_no`, strings.Join(versions, ",")); err != nil {
		return gerror.Wrap(err, "load generic product inventory reservations")
	}
	for _, row := range reserved {
		available[row.VersionNo] -= row.Quantity
	}
	for versionNo, quantity := range requested {
		if quantity < 1 || available[versionNo] < quantity {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	return nil
}

func loadCheckoutQuoteItemQuantities(ctx context.Context, tx gdb.TX, quote *entity.CommerceQuoteSnapshot, items []entity.CommerceQuoteItemSnapshot) (map[string]int, error) {
	type row struct {
		QuoteItemNo string `orm:"quote_item_no"`
		Quantity    int    `orm:"quantity"`
	}
	rows := make([]row, 0, len(items))
	if err := tx.GetScan(&rows, `SELECT quantity.quote_item_no,quantity.quantity
FROM plugin_idc_core_commerce_quote_item_quantity_snapshot quantity
JOIN plugin_idc_core_commerce_quote_item_snapshot item ON item.tenant_id=quantity.tenant_id AND item.quote_item_no=quantity.quote_item_no
WHERE item.tenant_id=0 AND item.quote_no=? ORDER BY item.line_number,item.id LIMIT ?`, quote.QuoteNo, maximumCartItems+1); err != nil {
		return nil, gerror.Wrap(err, "load generic quote quantities")
	}
	if len(rows) != len(items) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	result := make(map[string]int, len(rows))
	for _, row := range rows {
		if row.Quantity < 1 || row.Quantity > 1000 {
			return nil, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		result[row.QuoteItemNo] = row.Quantity
	}
	return result, nil
}

func (s *serviceImpl) buildCheckoutRows(
	quote *entity.CommerceQuoteSnapshot,
	items []entity.CommerceQuoteItemSnapshot,
	authority checkoutAuthority,
	input checkoutInput,
	now time.Time,
	paymentDueAt time.Time,
	manualReview bool,
	contractDecision contractpolicy.Decision,
	quantities map[string]int,
	inventoryFacts map[string]checkoutGenericInventoryFact,
) (checkoutRows, error) {
	orderNo, err := newOpaqueNumber(s.random, "ORD_")
	if err != nil {
		return checkoutRows{}, err
	}
	invoiceNo, err := newOpaqueNumber(s.random, "INV_")
	if err != nil {
		return checkoutRows{}, err
	}
	status := "pending_payment"
	if manualReview {
		status = "manual_review"
	}
	reviews := make([]do.CommerceOrderReview, 0, 1)
	if checkoutRiskDecision(items) == "manual_review" {
		reviewNo, reviewErr := newOpaqueNumber(s.random, "REV_")
		if reviewErr != nil {
			return checkoutRows{}, reviewErr
		}
		reviews = append(reviews, do.CommerceOrderReview{
			TenantId: idccap.DefaultTenantID, ReviewNo: reviewNo, OrderNo: orderNo,
			CustomerNo: authority.customerNo, ReviewTypeCode: "risk", StatusCode: "pending",
			ReasonCode: "", RequestedAt: &now, CreatedAt: &now, UpdatedAt: &now,
		})
	}
	order := do.CommerceOrder{
		TenantId: idccap.DefaultTenantID, OrderNo: orderNo, CustomerNo: authority.customerNo,
		QuoteNo: quote.QuoteNo, CurrencyCode: quote.CurrencyCode, SourceCode: authority.sourceCode,
		StatusCode: status, RiskDecisionCode: checkoutRiskDecision(items),
		RequiresContract: contractDecision.RequiresContract, ItemCount: len(items),
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: quote.DiscountMinor,
		NetAmountMinor: quote.NetAmountMinor, TaxAmountMinor: quote.TaxAmountMinor,
		TotalAmountMinor:          quote.TotalAmountMinor,
		CheckoutIdempotencyDigest: input.idempotencyDigest,
		CheckoutRequestDigest:     input.requestDigest, PaymentDueAt: &paymentDueAt,
		AssistedRequestNo: authority.assistedRequestNo, PaymentTermCode: authority.paymentTermCode,
		ContractOverridePolicyCode:   string(contractDecision.OrderOverride.Policy),
		EnterpriseContractPolicyCode: string(contractDecision.EnterpriseRule.Policy),
		ContractDecisionSourceCode:   string(contractDecision.Source),
		Version:                      1, CreatedBy: authority.userID, CreatedAt: &now, UpdatedAt: &now,
	}
	if contractDecision.OrderOverride.TemplateNo != "" {
		order.ContractOverrideTemplateNo = contractDecision.OrderOverride.TemplateNo
	}
	if contractDecision.EnterpriseRule.TemplateNo != "" {
		order.EnterpriseContractTemplateNo = contractDecision.EnterpriseRule.TemplateNo
	}
	if contractDecision.RequiresContract {
		order.ContractTemplateNo = contractDecision.Template.TemplateNo
		order.ContractTemplateVersionNo = contractDecision.Template.VersionNo
		order.ContractTemplateName = contractDecision.Template.Name
		order.ContractLanguageCode = contractDecision.Template.LanguageCode
		order.ContractScopeCode = contractDecision.Template.ScopeCode
		order.ContractSigningOrderCode = contractDecision.Template.SigningOrderCode
	}
	orderItems := make([]do.CommerceOrderItem, 0, len(items))
	itemQuantities := make([]map[string]any, 0, len(items))
	itemInventories := make([]map[string]any, 0, len(items))
	invoiceItems := make([]do.BillingInvoiceItem, 0, len(items))
	genericReservations := make([]map[string]any, 0, len(items))
	for _, item := range items {
		quantity := quantities[item.QuoteItemNo]
		if quantity < 1 {
			return checkoutRows{}, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		orderItemNo, numberErr := newOpaqueNumber(s.random, "ORI_")
		if numberErr != nil {
			return checkoutRows{}, numberErr
		}
		itemDigest, digestErr := quoteContentDigest(struct {
			OrderNo            string `json:"orderNo"`
			QuoteItemNo        string `json:"quoteItemNo"`
			QuoteContentDigest string `json:"quoteContentDigest"`
		}{orderNo, item.QuoteItemNo, item.ContentDigest})
		if digestErr != nil {
			return checkoutRows{}, digestErr
		}
		orderItems = append(orderItems, checkoutOrderItem(orderNo, orderItemNo, quote, item, quantity, itemDigest, now))
		quantityDigest, digestErr := quoteContentDigest(struct {
			OrderItemNo, QuoteItemNo string
			Quantity                 int
		}{orderItemNo, item.QuoteItemNo, quantity})
		if digestErr != nil {
			return checkoutRows{}, digestErr
		}
		itemQuantities = append(itemQuantities, map[string]any{"tenant_id": idccap.DefaultTenantID, "order_item_no": orderItemNo, "order_no": orderNo, "quote_item_no": item.QuoteItemNo, "quantity": quantity, "content_digest": quantityDigest})
		inventoryFact, hasInventoryFact := inventoryFacts[item.QuoteItemNo]
		if !hasInventoryFact {
			return checkoutRows{}, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		inventoryDigest, digestErr := quoteContentDigest(struct {
			OrderItemNo      string
			QuoteItemNo      string
			VersionNo        string
			TrackingEnabled  bool
			InventoryVersion int64
			SellableQuantity int64
		}{orderItemNo, item.QuoteItemNo, inventoryFact.VersionNo, inventoryFact.TrackingEnabled, inventoryFact.InventoryVersion, inventoryFact.SellableQuantity})
		if digestErr != nil {
			return checkoutRows{}, digestErr
		}
		itemInventories = append(itemInventories, map[string]any{
			"order_item_no": orderItemNo, "quote_item_no": item.QuoteItemNo,
			"version_no": inventoryFact.VersionNo, "tracking_enabled": inventoryFact.TrackingEnabled,
			"inventory_version": inventoryFact.InventoryVersion, "sellable_quantity": inventoryFact.SellableQuantity,
			"content_digest": inventoryDigest,
		})
		if inventoryFact.InventoryVersion > 0 && inventoryFact.TrackingEnabled {
			value, reserveErr := newOpaqueNumber(s.random, "RSV_")
			if reserveErr != nil {
				return checkoutRows{}, reserveErr
			}
			reservationDigest, digestErr := quoteContentDigest(struct {
				ReservationNo, OrderItemNo, VersionNo string
				InventoryVersion                      int64
				Quantity                              int
			}{value, orderItemNo, inventoryFact.VersionNo, inventoryFact.InventoryVersion, quantity})
			if digestErr != nil {
				return checkoutRows{}, digestErr
			}
			genericReservations = append(genericReservations, map[string]any{
				"tenant_id": idccap.DefaultTenantID, "reservation_no": value,
				"order_no": orderNo, "order_item_no": orderItemNo,
				"version_no": inventoryFact.VersionNo, "inventory_version": inventoryFact.InventoryVersion,
				"reserved_quantity": quantity, "status_code": "held", "release_reason_code": "",
				"content_digest": reservationDigest, "hold_expires_at": &paymentDueAt,
				"version": 1, "created_at": &now, "updated_at": &now,
			})
		}
		invoiceItemNo, invoiceItemErr := newOpaqueNumber(s.random, "IVI_")
		if invoiceItemErr != nil {
			return checkoutRows{}, invoiceItemErr
		}
		invoiceItemDigest, digestErr := quoteContentDigest(struct {
			InvoiceNo   string `json:"invoiceNo"`
			OrderItemNo string `json:"orderItemNo"`
			ItemDigest  string `json:"itemDigest"`
		}{invoiceNo, orderItemNo, itemDigest})
		if digestErr != nil {
			return checkoutRows{}, digestErr
		}
		orderNoCopy, orderItemNoCopy := orderNo, orderItemNo
		invoiceItems = append(invoiceItems, do.BillingInvoiceItem{
			TenantId: idccap.DefaultTenantID, InvoiceItemNo: invoiceItemNo,
			InvoiceNo: invoiceNo, CustomerNo: authority.customerNo, CurrencyCode: quote.CurrencyCode,
			LineNumber: item.LineNumber, ItemTypeCode: "order_item",
			OrderNo: &orderNoCopy, OrderItemNo: &orderItemNoCopy, SourceReference: orderItemNo,
			DescriptionZhCn: item.ProductNameZhCn, DescriptionEnUs: item.ProductNameEnUs,
			SubtotalMinor: item.SubtotalMinor, DiscountMinor: item.DiscountMinor,
			NetAmountMinor: item.NetAmountMinor, TaxAmountMinor: item.TaxAmountMinor,
			TotalAmountMinor: item.TotalAmountMinor, RefundableCapMinor: item.TotalAmountMinor,
			ContentDigest: invoiceItemDigest, CreatedAt: &now,
		})
	}
	invoiceDigest, err := quoteContentDigest(struct {
		InvoiceNo        string `json:"invoiceNo"`
		OrderNo          string `json:"orderNo"`
		QuoteDigest      string `json:"quoteDigest"`
		InvoiceItemCount int    `json:"invoiceItemCount"`
	}{invoiceNo, orderNo, quote.ContentDigest, len(invoiceItems)})
	if err != nil {
		return checkoutRows{}, err
	}
	orderNoCopy := orderNo
	invoice := do.BillingInvoice{
		TenantId: idccap.DefaultTenantID, InvoiceNo: invoiceNo,
		CustomerNo: authority.customerNo, OrderNo: &orderNoCopy, SourceCode: "order",
		CurrencyCode: quote.CurrencyCode, StatusCode: "pending_payment", ItemCount: len(items),
		SubtotalMinor: quote.SubtotalMinor, DiscountMinor: quote.DiscountMinor,
		NetAmountMinor: quote.NetAmountMinor, TaxAmountMinor: quote.TaxAmountMinor,
		TotalAmountMinor: quote.TotalAmountMinor, PaidAmountMinor: 0,
		RefundableAmountMinor: 0, RefundedAmountMinor: 0,
		SourceSnapshotDigest: quote.ContentDigest, ContentDigest: invoiceDigest,
		PaymentDueAt: &paymentDueAt, IssuedAt: &now, Version: 1,
		CreatedAt: &now, UpdatedAt: &now,
	}
	return checkoutRows{
		orderNo: orderNo, invoiceNo: invoiceNo, sourceCode: authority.sourceCode, statusCode: status,
		paymentDueAt: paymentDueAt, requiresContract: contractDecision.RequiresContract,
		order: order, items: orderItems, itemQuantities: itemQuantities, itemInventories: itemInventories,
		invoice: invoice, invoiceItems: invoiceItems,
		genericReservations: genericReservations, reviews: reviews,
	}, nil
}

func checkoutRiskDecision(items []entity.CommerceQuoteItemSnapshot) string {
	for _, item := range items {
		if item.RiskDecisionCode == "manual_review" {
			return "manual_review"
		}
	}
	return "approved"
}

func checkoutOrderItem(
	orderNo string,
	orderItemNo string,
	quote *entity.CommerceQuoteSnapshot,
	item entity.CommerceQuoteItemSnapshot,
	quantity int,
	contentDigest string,
	now time.Time,
) do.CommerceOrderItem {
	return do.CommerceOrderItem{
		TenantId: idccap.DefaultTenantID, OrderItemNo: orderItemNo, OrderNo: orderNo,
		QuoteNo: quote.QuoteNo, QuoteItemNo: item.QuoteItemNo, LineNumber: item.LineNumber,
		CurrencyCode: item.CurrencyCode, ProductNo: item.ProductNo, VersionNo: item.VersionNo,
		BillingRuleNo: item.BillingRuleNo, ConfigurationSnapshotNo: item.ConfigurationSnapshotNo,
		ProductNameZhCn: item.ProductNameZhCn,
		ProductNameEnUs: item.ProductNameEnUs,
		PurchaseCycles:  item.PurchaseCycles, ChargeTypeCode: item.ChargeTypeCode,
		TrialLimitQuantity: item.TrialLimitQuantity, PriorTrialUses: item.PriorTrialUses,
		PeriodUnitCode: item.PeriodUnitCode, PeriodCount: item.PeriodCount,
		RequiresContract: item.RequiresContract, FulfillmentModeCode: item.FulfillmentModeCode,
		ContractPolicyCode: item.ContractPolicyCode,
		ContractTemplateNo: nullableCommerceContractTemplate(item.ContractTemplateNo),
		RiskDecisionCode:   item.RiskDecisionCode, InventoryDecisionCode: item.InventoryDecisionCode,
		RouteDecisionDigest: item.RouteDecisionDigest, TaxPolicyNo: item.TaxPolicyNo,
		PriceIncludesTax: item.PriceIncludesTax, TaxTreatmentCode: item.TaxTreatmentCode,
		TaxRateBasisPoints: item.TaxRateBasisPoints, TaxCategoryCode: item.TaxCategoryCode,
		RoundingScopeCode: item.RoundingScopeCode, RoundingModeCode: item.RoundingModeCode,
		RoundingQuantumMinor: item.RoundingQuantumMinor,
		SetupSubtotalMinor:   item.SetupSubtotalMinor, PeriodSubtotalMinor: item.PeriodSubtotalMinor,
		SubtotalMinor: item.SubtotalMinor, DiscountMinor: item.DiscountMinor,
		NetAmountMinor: item.NetAmountMinor, TaxAmountMinor: item.TaxAmountMinor,
		TotalAmountMinor: item.TotalAmountMinor, ContentDigest: contentDigest, CreatedAt: &now,
	}
}

func insertCheckoutRows(ctx context.Context, tx gdb.TX, rows checkoutRows) error {
	if _, err := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).Data(rows.order).Insert(); err != nil {
		return gerror.Wrap(err, "insert checkout order")
	}
	if _, err := tx.Model(dao.CommerceOrderItem.Table()).Ctx(ctx).Data(rows.items).Insert(); err != nil {
		return gerror.Wrap(err, "insert checkout order items")
	}
	if _, err := tx.Model("plugin_idc_core_commerce_order_item_quantity_snapshot").Ctx(ctx).Data(rows.itemQuantities).Insert(); err != nil {
		return gerror.Wrap(err, "insert generic order item quantities")
	}
	if _, err := tx.Model("plugin_idc_core_commerce_order_item_inventory_snapshot").Ctx(ctx).Data(rows.itemInventories).Insert(); err != nil {
		return gerror.Wrap(err, "insert generic order item inventory snapshots")
	}
	if _, err := tx.Model(dao.BillingInvoice.Table()).Ctx(ctx).Data(rows.invoice).Insert(); err != nil {
		return gerror.Wrap(err, "insert checkout invoice")
	}
	if _, err := tx.Model(dao.BillingInvoiceItem.Table()).Ctx(ctx).Data(rows.invoiceItems).Insert(); err != nil {
		return gerror.Wrap(err, "insert checkout invoice items")
	}
	if len(rows.genericReservations) > 0 {
		if _, err := tx.Model(dao.CommerceInventoryReservation.Table()).Ctx(ctx).Data(rows.genericReservations).Insert(); err != nil {
			return gerror.Wrap(err, "insert generic product inventory reservations")
		}
	}
	if len(rows.reviews) > 0 {
		if _, err := tx.Model(dao.CommerceOrderReview.Table()).Ctx(ctx).Data(rows.reviews).Insert(); err != nil {
			return gerror.Wrap(err, "insert checkout order reviews")
		}
	}
	return nil
}

func closeCheckedOutCart(ctx context.Context, tx gdb.TX, cart *entity.CommerceCart, now time.Time) error {
	columns := dao.CommerceCart.Columns()
	result, err := tx.Model(dao.CommerceCart.Table()).Ctx(ctx).Where(do.CommerceCart{
		TenantId: idccap.DefaultTenantID, CartNo: cart.CartNo, StatusCode: "active", Version: cart.Version,
	}).Data(do.CommerceCart{
		StatusCode: "checked_out", ClosedAt: &now,
		Version: gdb.Raw(columns.Version + " + 1"), UpdatedAt: &now,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "close checked out cart")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read checked out cart result")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) enqueueCheckoutCreated(
	ctx context.Context,
	rows checkoutRows,
	quote *entity.CommerceQuoteSnapshot,
	now time.Time,
) error {
	orderNo := rows.orderNo
	invoiceNo := rows.invoiceNo
	payload, err := json.Marshal(struct {
		OrderNo          string `json:"orderNo"`
		InvoiceNo        string `json:"invoiceNo"`
		CustomerNo       string `json:"customerNo"`
		CurrencyCode     string `json:"currencyCode"`
		StatusCode       string `json:"statusCode"`
		TotalAmountMinor int64  `json:"totalAmountMinor"`
	}{
		OrderNo: orderNo, InvoiceNo: invoiceNo, CustomerNo: *quote.CustomerNo,
		CurrencyCode: quote.CurrencyCode, StatusCode: rows.statusCode,
		TotalAmountMinor: quote.TotalAmountMinor,
	})
	if err != nil {
		return gerror.Wrap(err, "encode checkout outbox payload")
	}
	metadata, err := json.Marshal(struct {
		Source string `json:"source"`
	}{Source: rows.sourceCode})
	if err != nil {
		return gerror.Wrap(err, "encode checkout outbox metadata")
	}
	_, err = s.outbox.Enqueue(ctx, outbox.EnqueueInput{
		EventType: "commerce.order.created", EventVersion: 1, Topic: "commerce.order.created",
		AggregateType: "commerce_order", AggregateNo: orderNo, CorrelationNo: quote.QuoteNo,
		DeduplicationKey: "commerce-order-created:" + orderNo,
		Payload:          payload, Metadata: metadata, AvailableAt: now,
	})
	if err != nil {
		return gerror.Wrap(err, "enqueue checkout created event")
	}
	return nil
}

func projectCheckoutRows(
	rows checkoutRows,
	quote *entity.CommerceQuoteSnapshot,
	reused bool,
) (*Checkout, error) {
	if rows.paymentDueAt.IsZero() {
		return nil, gerror.New("checkout order has no payment due time")
	}
	return &Checkout{
		OrderNo: rows.orderNo, InvoiceNo: rows.invoiceNo,
		QuoteNo: quote.QuoteNo, StatusCode: rows.statusCode,
		CurrencyCode: quote.CurrencyCode, ItemCount: quote.ItemCount,
		TotalAmountMinor: quote.TotalAmountMinor,
		RequiresContract: rows.requiresContract, PaymentDueAt: rows.paymentDueAt,
		Reused: reused,
	}, nil
}

func projectPersistedCheckout(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	reused bool,
) (*Checkout, error) {
	var invoice *entity.BillingInvoice
	if err := tx.Model(dao.BillingInvoice.Table()).Ctx(ctx).Where(do.BillingInvoice{
		TenantId: idccap.DefaultTenantID, OrderNo: &order.OrderNo,
	}).Scan(&invoice); err != nil {
		return nil, gerror.Wrap(err, "load persisted checkout invoice")
	}
	if invoice == nil || invoice.CustomerNo != order.CustomerNo || invoice.CurrencyCode != order.CurrencyCode ||
		invoice.PaymentDueAt == nil || order.PaymentDueAt == nil || !invoice.PaymentDueAt.Equal(*order.PaymentDueAt) ||
		invoice.TotalAmountMinor != order.TotalAmountMinor || invoice.ItemCount != order.ItemCount {
		return nil, gerror.New("persisted checkout projection is inconsistent")
	}
	return &Checkout{
		OrderNo: order.OrderNo, InvoiceNo: invoice.InvoiceNo, QuoteNo: order.QuoteNo,
		StatusCode: order.StatusCode, CurrencyCode: order.CurrencyCode, ItemCount: order.ItemCount,
		TotalAmountMinor: order.TotalAmountMinor, RequiresContract: order.RequiresContract,
		PaymentDueAt: *order.PaymentDueAt, Reused: reused,
	}, nil
}
