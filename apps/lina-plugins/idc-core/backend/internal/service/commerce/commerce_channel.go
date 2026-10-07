package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type ChannelOrderInput struct {
	ChannelID              string
	ExternalCustomerID     string
	OwnershipMode          string
	PayerCustomerNo        string
	ServiceOwnerCustomerNo string
	IdempotencyKey         string
	Purchase               catalog.PurchaseConfigurationInput
	Quantity               int
}

type ChannelOrderService interface {
	Quote(context.Context, string, catalog.PurchaseConfigurationInput, int) (*Quote, error)
	Checkout(context.Context, ChannelOrderInput) (*Checkout, error)
	Cancel(context.Context, string, string, string, string) (*OrderCancellation, error)
}

type channelOrderService struct {
	database  gdb.DB
	catalog   catalog.PurchaseConfigurationResolver
	assembler catalog.PurchaseQuoteAssembler
	commerce  *serviceImpl
}

func NewChannelOrderService(
	database gdb.DB,
	catalogResolver catalog.PurchaseConfigurationResolver,
	commerceService Service,
) (ChannelOrderService, error) {
	commerceImpl, ok := commerceService.(*serviceImpl)
	assembler, hasAssembler := catalogResolver.(catalog.PurchaseQuoteAssembler)
	if database == nil || catalogResolver == nil || !ok || commerceImpl == nil ||
		!hasAssembler || assembler == nil {
		return nil, gerror.New("channel order service requires database, catalog, and commerce services")
	}
	return &channelOrderService{
		database: database, catalog: catalogResolver, assembler: assembler, commerce: commerceImpl,
	}, nil
}

func (s *channelOrderService) Quote(
	ctx context.Context,
	payerCustomerNo string,
	purchase catalog.PurchaseConfigurationInput,
	quantity int,
) (*Quote, error) {
	if strings.TrimSpace(payerCustomerNo) == "" || quantity < 1 || quantity > 1000 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	resolved, err := s.catalog.ResolvePurchaseConfiguration(ctx, purchase)
	if err != nil || resolved == nil || purchase.PurchaseCycles < resolved.MinimumPurchaseCycles {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return s.buildQuote(ctx, payerCustomerNo, purchase, resolved, quantity, "channel-preview", "", false)
}

func normalizeChannelOrderInput(input ChannelOrderInput) ChannelOrderInput {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.ExternalCustomerID = strings.TrimSpace(input.ExternalCustomerID)
	input.PayerCustomerNo = strings.TrimSpace(input.PayerCustomerNo)
	input.ServiceOwnerCustomerNo = strings.TrimSpace(input.ServiceOwnerCustomerNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	return input
}

func validateChannelOrderInput(input ChannelOrderInput) error {
	if input.ChannelID == "" || input.PayerCustomerNo == "" || input.ServiceOwnerCustomerNo == "" ||
		input.Quantity < 1 || input.Quantity > 1000 ||
		(input.OwnershipMode != "channel_owned" && input.OwnershipMode != "delegated_customer") ||
		(input.OwnershipMode == "channel_owned" &&
			(input.PayerCustomerNo != input.ServiceOwnerCustomerNo || input.ExternalCustomerID != "")) ||
		(input.OwnershipMode == "delegated_customer" &&
			(input.PayerCustomerNo == input.ServiceOwnerCustomerNo || input.ExternalCustomerID == "")) {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func (s *channelOrderService) Checkout(
	ctx context.Context,
	input ChannelOrderInput,
) (*Checkout, error) {
	input = normalizeChannelOrderInput(input)
	if err := validateChannelOrderInput(input); err != nil {
		return nil, err
	}
	quoteNo := deterministicChannelQuoteNo(input.ChannelID, input.PayerCustomerNo, input.IdempotencyKey)
	requestDigest, err := quoteContentDigest(struct {
		ChannelID, ExternalCustomerID, OwnershipMode string
		PayerCustomerNo, ServiceOwnerCustomerNo      string
		Purchase                                     catalog.PurchaseConfigurationInput
		Quantity                                     int
	}{input.ChannelID, input.ExternalCustomerID, input.OwnershipMode,
		input.PayerCustomerNo, input.ServiceOwnerCustomerNo, input.Purchase, input.Quantity})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var result *Checkout
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		checkoutInput, normalizeErr := normalizeCheckoutRequestForSource(
			input.PayerCustomerNo, "channel", CheckoutRequest{QuoteNo: quoteNo, IdempotencyKey: input.IdempotencyKey},
		)
		if normalizeErr != nil {
			return normalizeErr
		}
		if lockErr := lockCheckoutIdempotency(ctx, tx, input.PayerCustomerNo, checkoutInput.idempotencyDigest); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadCheckoutByIdempotency(ctx, tx, input.PayerCustomerNo, checkoutInput.idempotencyDigest, true)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.QuoteNo != quoteNo || existing.CheckoutRequestDigest != checkoutInput.requestDigest {
				return bizerr.NewCode(CodeConflict)
			}
			result, loadErr = projectPersistedCheckout(ctx, tx, existing, true)
			return loadErr
		}
		var quote *Quote
		var header *entity.CommerceQuoteSnapshot
		if loadErr = tx.Model("plugin_idc_core_commerce_quote_snapshot").Ctx(ctx).
			Where("tenant_id=0 AND quote_no=?", quoteNo).Scan(&header); loadErr != nil {
			return loadErr
		}
		if header == nil {
			resolved, resolveErr := s.catalog.ResolvePurchaseConfiguration(ctx, input.Purchase)
			if resolveErr != nil || resolved == nil || input.Purchase.PurchaseCycles < resolved.MinimumPurchaseCycles {
				return bizerr.NewCode(CodeQuoteNotAvailable)
			}
			quote, resolveErr = s.buildQuote(ctx, input.PayerCustomerNo, input.Purchase, resolved,
				input.Quantity, quoteNo, requestDigest, true)
			if resolveErr != nil {
				return resolveErr
			}
		} else if header.InputDigest != requestDigest || header.SourceCode != "channel" ||
			header.CustomerNo == nil || *header.CustomerNo != input.PayerCustomerNo {
			return bizerr.NewCode(CodeConflict)
		}
		_ = quote
		result, loadErr = s.commerce.checkoutInTransaction(ctx, tx, checkoutAuthority{
			customerNo: input.PayerCustomerNo, sourceCode: "channel",
			paymentTermCode:  "standard_prepay",
			contractOverride: contractpolicy.Rule{Policy: contractpolicy.PolicyInherit},
			channelAttribution: &channelCheckoutAttribution{
				ChannelID: input.ChannelID, ExternalCustomerID: input.ExternalCustomerID,
				OwnershipMode: input.OwnershipMode, ServiceOwnerCustomerNo: input.ServiceOwnerCustomerNo,
			},
		}, checkoutInput)
		return loadErr
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) ||
			bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeQuoteNotAvailable) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *channelOrderService) buildQuote(
	ctx context.Context,
	payerCustomerNo string,
	purchase catalog.PurchaseConfigurationInput,
	resolved *catalog.PurchaseConfiguration,
	quantity int,
	quoteNo string,
	requestDigest string,
	persist bool,
) (*Quote, error) {
	itemNo, err := newOpaqueNumber(s.commerce.random, "CHI_")
	if err != nil {
		return nil, err
	}
	snapshotNo := "channel-preview"
	if persist {
		numbers, numberErr := s.commerce.newSnapshotNumbers()
		if numberErr != nil {
			return nil, numberErr
		}
		snapshotNo, itemNo = numbers.snapshotNo, numbers.cartItemNo
		if insertErr := insertGenericPurchaseSnapshot(ctx, s.database, resolved, snapshotNo); insertErr != nil {
			return nil, insertErr
		}
	}
	item := entity.CommerceCartItem{
		CartItemNo: itemNo, CartNo: quoteNo, ProductNo: resolved.ProductNo,
		VersionNo: resolved.VersionNo, BillingRuleNo: resolved.BillingRuleNo,
		ConfigurationSnapshotNo: snapshotNo, PurchaseCycles: purchase.PurchaseCycles,
	}
	expiresAt := time.Now().Add(defaultQuoteLifetime)
	graph := &quoteCartGraph{
		cart:  &entity.CommerceCart{CartNo: quoteNo, CurrencyCode: resolved.CurrencyCode, Version: 1, ExpiresAt: &expiresAt},
		items: []entity.CommerceCartItem{item}, quantities: map[string]int{itemNo: quantity},
		priceSelections: map[string][]catalog.ConfigurationPriceSelection{
			snapshotNo: append([]catalog.ConfigurationPriceSelection(nil), resolved.PriceSelections...),
		},
	}
	states, err := s.catalog.RevalidatePurchaseConfigurations(ctx, []catalog.PurchaseConfigurationReference{{
		ReferenceNo: itemNo, ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo, BillingRuleNo: resolved.BillingRuleNo,
	}})
	if err != nil || len(states) != 1 || states[0].ReferenceNo != itemNo || states[0].IssueCode != "" {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	trialUses, err := loadQuoteTrialUses(ctx, s.database, payerCustomerNo, graph.items)
	if err != nil {
		return nil, err
	}
	catalogInput, err := quoteCatalogInput(graph, trialUses)
	if err != nil {
		return nil, err
	}
	lines, err := s.assembler.AssemblePurchaseQuote(ctx, catalogInput)
	if err != nil {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	ordered, err := orderQuoteCatalogLines(graph.items, lines)
	if err != nil {
		return nil, err
	}
	calculated, err := s.commerce.prepareQuoteLines(ordered, graph.items)
	if err != nil {
		return nil, err
	}
	if err = applyQuoteAdjustments(calculated, &QuoteAdjustmentResult{Adjustments: []QuoteAdjustment{}}); err != nil {
		return nil, err
	}
	now, err := quoteDatabaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	expiresAt = now.Add(defaultQuoteLifetime)
	graph.cart.ExpiresAt = &expiresAt
	customer, err := loadQuoteCustomerFact(ctx, s.database, payerCustomerNo)
	if err != nil || customer == nil {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	if !persist {
		quote := &Quote{QuoteNo: quoteNo, CurrencyCode: resolved.CurrencyCode, IssuedAt: now, ExpiresAt: expiresAt}
		for _, line := range calculated {
			quote.SubtotalMinor += line.amounts.subtotal
			quote.DiscountMinor += line.amounts.discount
			quote.NetAmountMinor += line.amounts.net
			quote.TaxAmountMinor += line.amounts.tax
			quote.TotalAmountMinor += line.amounts.total
		}
		return quote, nil
	}
	quote, err := s.commerce.persistAssistedQuote(ctx, quoteNo, graph, customer, now, expiresAt, calculated,
		assistedQuoteBuild{requestNo: quoteNo, requestDigest: requestDigest, sourceCode: "channel"})
	if err != nil {
		return nil, err
	}
	if s.commerce.structuredConfiguration != nil {
		if len(quote.Items) != 1 || quote.Items[0].QuoteItemNo == "" {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		if _, err = s.commerce.freezeQuoteItemConfiguration(ctx, &principal.Audience{
			Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
			CustomerNo: payerCustomerNo,
		}, FreezeQuoteItemConfigurationInput{
			QuoteItemNo: quote.Items[0].QuoteItemNo,
			Purchase: catalog.PublicPurchaseConfigurationInput{
				PublicSlug: purchase.PublicSlug, ConfigurationRevision: purchase.ConfigurationRevision,
				Values: purchase.Values,
			},
		}); err != nil {
			return nil, err
		}
	}
	return quote, nil
}

func (s *channelOrderService) Cancel(
	ctx context.Context,
	channelID string,
	payerCustomerNo string,
	orderNo string,
	idempotencyKey string,
) (*OrderCancellation, error) {
	channelID, payerCustomerNo = strings.TrimSpace(channelID), strings.TrimSpace(payerCustomerNo)
	orderNo, idempotencyKey = strings.TrimSpace(orderNo), strings.TrimSpace(idempotencyKey)
	requestDigest, err := orderCancellationRequestDigest(
		payerCustomerNo, orderNo, OrderCancellationSourceCustomer,
		OrderCancellationReasonCustomerRequested, idempotencyKey,
	)
	if err != nil || channelID == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *OrderCancellation
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		value, loadErr := tx.GetValue(`SELECT EXISTS(SELECT 1 FROM plugin_idc_core_channel_order_attribution WHERE tenant_id=0 AND order_no=? AND channel_id=? AND payer_customer_no=?)`, orderNo, channelID, payerCustomerNo)
		if loadErr != nil || !value.Bool() {
			return bizerr.NewCode(CodeNotAccessible)
		}
		order, loadErr := lockOrderForCustomerCancellation(ctx, tx, orderNo, payerCustomerNo)
		if loadErr != nil {
			return loadErr
		}
		result, loadErr = initiateOrderCancellation(ctx, tx, order, orderCancellationInput{
			orderNo: orderNo, source: OrderCancellationSourceCustomer,
			reason: OrderCancellationReasonCustomerRequested, requestDigest: requestDigest,
		}, cancellationDependencies{
			reverser:    s.commerce.reservationReverser,
			hasReverser: s.commerce.hasReservationReverser,
			outbox:      s.commerce.outbox,
		})
		return loadErr
	})
	if err != nil {
		return nil, normalizeOrderLifecycleError(err)
	}
	return result, nil
}

func deterministicChannelQuoteNo(parts ...string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("idc-core/channel-quote/v1"))
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return "QTE_" + hex.EncodeToString(hash.Sum(nil)[:16])
}
