package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	lifecycleresourcespi "lina-plugin-idc-core/backend/cap/lifecycleresourcecap/spi"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	serveroperationspi "lina-plugin-idc-core/backend/cap/serveroperationresourcecap/spi"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/currency"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customeractivation"
	"lina-plugin-idc-core/backend/internal/service/notificationpolicy"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"lina-plugin-idc-core/backend/internal/service/renewal"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

type channelExternalActorResolver struct{}

func (channelExternalActorResolver) ResolveTrustedActor(context.Context) (audit.TrustedActor, error) {
	return audit.TrustedActor{Type: audit.ActorExternal, Reference: "plugin:idc-channel"}, nil
}

type channelCommerceRuntime struct {
	database        gdb.DB
	current         principal.Reader
	activation      customeractivation.Service
	orders          commerce.ChannelOrderService
	settlements     billing.FinancialSettlementService
	invoices        billing.InvoiceService
	catalog         catalog.CatalogReader
	pricing         salesmanagement.PriceRuleReader
	purchaseFiles   purchaseconfigurationfile.Service
	renewals        renewal.RuntimeService
	serverSource    serveroperationresourcecap.Source
	priced          *serveroperation.NativePriceBuilder
	lifecycleSource lifecycleresourcecap.Source
	audit           idccap.BusinessAuditWriter
}

type channelCustomerRow struct {
	CustomerNo       string `orm:"customer_no"`
	CustomerTypeCode string `orm:"customer_type_code"`
	StatusCode       string `orm:"status_code"`
	IdentityStatus   string `orm:"identity_status"`
	ActorUserID      int    `orm:"actor_user_id"`
}

type channelOrderRow struct {
	OrderNo                string `orm:"order_no"`
	InvoiceNo              string `orm:"invoice_no"`
	PayerCustomerNo        string `orm:"payer_customer_no"`
	ServiceOwnerCustomerNo string `orm:"service_owner_customer_no"`
	CurrencyCode           string `orm:"currency_code"`
	StatusCode             string `orm:"status_code"`
	TotalAmountMinor       int64  `orm:"total_amount_minor"`
	FinanciallySettled     bool   `orm:"financially_settled"`
}

func validateChannelCommerceSource(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) error {
	if ctx == nil || ctx.Err() != nil || services == nil || services.BizCtx() == nil ||
		services.Auth() == nil || services.Auth().Authz() == nil || services.Users() == nil ||
		services.Storage() == nil || callerPluginID != channelcommercecap.AllowedCallerPluginID {
		return gerror.New("channel commerce source caller is not allowed")
	}
	return nil
}

func newChannelCommerceRuntime(
	ctx context.Context,
	services capability.Services,
	callerPluginID string,
) (channelcommercecap.Runtime, error) {
	if err := validateChannelCommerceSource(ctx, services, callerPluginID); err != nil {
		return nil, err
	}
	principalReader, err := principal.NewAuthorizedReader(services.BizCtx(), services.Auth().Authz())
	if err != nil {
		return nil, err
	}
	auditConfig, err := runtimeconfig.LoadAuditConfig()
	if err != nil {
		return nil, err
	}
	auditService, err := audit.NewWithTrustedActorResolver(channelExternalActorResolver{}, services.Auth().Authz(), auditConfig)
	if err != nil {
		return nil, err
	}
	auditWriter, err := audit.NewBusinessWriterAdapter(auditService)
	if err != nil {
		return nil, err
	}
	events, err := outbox.NewEnqueuer(g.DB(), outbox.Config{})
	if err != nil {
		return nil, err
	}
	creditUsage, err := billing.NewCreditUsageService(g.DB())
	if err != nil {
		return nil, err
	}
	settlements, err := billing.NewFinancialSettlementService(g.DB(), events, creditUsage)
	if err != nil {
		return nil, err
	}
	invoices, err := billing.NewInvoiceService(g.DB())
	if err != nil {
		return nil, err
	}
	scanner, err := newContentScanner(ctx)
	if err != nil {
		return nil, err
	}
	cursorKey, err := runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return nil, err
	}
	purchaseTargets, err := catalog.NewPublicPurchaseFileTargetResolver(g.DB(), cursorKey, supplierRuntimeCapabilityInvoker)
	clear(cursorKey)
	if err != nil {
		return nil, err
	}
	purchaseFiles, err := purchaseconfigurationfile.New(
		g.DB(), channelFilePrincipal{}, services.Storage(), purchaseTargets, scanner,
	)
	if err != nil {
		return nil, err
	}
	cursorKey, err = runtimeconfig.LoadCatalogCursorKey()
	if err != nil {
		return nil, err
	}
	catalogReader, err := newChannelCurrencyCatalogReader(
		g.DB(), cursorKey, supplierRuntimeCapabilityInvoker, purchaseFiles,
	)
	clear(cursorKey)
	if err != nil {
		return nil, err
	}
	billingLabels, err := loadPublicBillingLabelResources()
	if err != nil {
		return nil, err
	}
	if err = catalog.BindPublicBillingLabelResources(catalogReader, billingLabels); err != nil {
		return nil, err
	}
	purchaseResolver, err := catalog.PurchaseResolverFromPublicReader(catalogReader)
	if err != nil {
		return nil, err
	}
	adjustments, err := newChannelQuoteAdjustments(ctx, services)
	if err != nil {
		return nil, err
	}
	commerceService, err := commerce.NewWithQuoteAdjustments(
		g.DB(), purchaseResolver, channelFilePrincipal{}, adjustments, settlements, nil,
	)
	if err != nil {
		return nil, err
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(catalogReader)
	if err != nil {
		return nil, err
	}
	if err = bindChannelCheckoutCapabilities(commerceService, g.DB(), freezer, purchaseFiles); err != nil {
		return nil, err
	}
	channelOrders, err := commerce.NewChannelOrderService(g.DB(), purchaseResolver, commerceService)
	if err != nil {
		return nil, err
	}
	notifications, err := notificationpolicy.NewEventSink(pluginID, events)
	if err != nil {
		return nil, err
	}
	lookupKey, err := runtimeconfig.LoadIdentityLookupKey()
	if err != nil {
		return nil, err
	}
	loginDirectory, err := buildCustomerLoginDirectory(services.BizCtx(), services.Users())
	if err != nil {
		return nil, err
	}
	activation, err := customeractivation.New(
		g.DB(), services.BizCtx(), principalReader, services.Auth().Authz(), services.Users(), loginDirectory,
		auditWriter, notifications, lookupKey, runtimeconfig.CustomerPublicURL,
	)
	clear(lookupKey)
	if err != nil {
		return nil, err
	}
	renewals, err := renewal.NewWithQuoteAdjustments(ctx, g.DB(), principalReader, catalogReader, adjustments)
	if err != nil {
		return nil, err
	}
	var serverSource serveroperationresourcecap.Source
	if factory := serveroperationspi.Freeze().Factory(); factory != nil {
		serverSource, err = factory(ctx)
		if err != nil {
			return nil, gerror.Wrap(err, "construct channel server operation source")
		}
	}
	priced, err := newChannelPricedBuilder(g.DB(), catalogReader, serverSource)
	if err != nil {
		return nil, err
	}
	var lifecycleSource lifecycleresourcecap.Source
	if factory := lifecycleresourcespi.Freeze().Factory(); factory != nil {
		lifecycleSource, err = factory(ctx)
		if err != nil {
			return nil, gerror.Wrap(err, "construct channel lifecycle source")
		}
	}
	pricing, err := salesmanagement.NewPriceRuleReader(g.DB())
	if err != nil {
		return nil, err
	}
	return &channelCommerceRuntime{
		pricing: pricing, purchaseFiles: purchaseFiles,
		database: g.DB(), current: principalReader, activation: activation,
		orders: channelOrders, settlements: settlements, invoices: invoices,
		catalog:  catalogReader,
		renewals: renewals, serverSource: serverSource, priced: priced, lifecycleSource: lifecycleSource,
		audit: auditWriter,
	}, nil
}

// newChannelPricedBuilder composes optional resource pricing only at startup;
// channel commerce remains usable for ordinary products without that source.
func newChannelPricedBuilder(database gdb.DB, reader catalog.CatalogReader, source serveroperationresourcecap.Source) (*serveroperation.NativePriceBuilder, error) {
	if source == nil {
		return nil, nil
	}
	return serveroperation.NewNativePriceBuilder(database, reader, source)
}

func (runtime *channelCommerceRuntime) CurrentCustomer(ctx context.Context) (*channelcommercecap.CustomerSubject, error) {
	if runtime == nil || runtime.current == nil {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	audience, err := runtime.current.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" || audience.UserID < 1 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	row, err := runtime.loadCustomer(ctx, audience.CustomerNo)
	if err != nil || row.StatusCode != "active" || row.ActorUserID != audience.UserID {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return &channelcommercecap.CustomerSubject{
		CustomerNo: row.CustomerNo, CustomerTypeCode: row.CustomerTypeCode,
		Status: row.StatusCode, IdentityStatus: row.IdentityStatus, ActorUserID: row.ActorUserID,
	}, nil
}

func (runtime *channelCommerceRuntime) ValidatePayer(
	ctx context.Context,
	customerNo string,
) (*channelcommercecap.CustomerSubject, error) {
	// Issuing a credential is not a purchase. Keep identity evidence available
	// to transaction owners without requiring verification at authorization time.
	row, err := runtime.loadCustomer(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	if row.StatusCode != string(customer.StatusActive) || row.ActorUserID < 1 {
		return nil, bizerr.NewCode(customer.CodeStateDenied)
	}
	return &channelcommercecap.CustomerSubject{
		CustomerNo: row.CustomerNo, CustomerTypeCode: row.CustomerTypeCode,
		Status: row.StatusCode, IdentityStatus: row.IdentityStatus, ActorUserID: row.ActorUserID,
	}, nil
}

func (runtime *channelCommerceRuntime) CreateDelegatedCustomer(
	ctx context.Context,
	input channelcommercecap.DelegatedCustomerInput,
) (*channelcommercecap.DelegatedCustomer, error) {
	invitation, err := runtime.activation.CreateForChannel(ctx, input.ChannelID, customeractivation.CreateInput{
		CustomerType: input.CustomerType, DisplayName: input.DisplayName, CountryCode: input.CountryCode,
		PreferredLocale: input.PreferredLocale, Email: input.Email, PhoneCountryCode: input.PhoneCountryCode,
		Phone: input.Phone, IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &channelcommercecap.DelegatedCustomer{
		CustomerNo: invitation.CustomerNo, Status: "pending_activation", IdentityStatus: "unverified",
		InvitationNo: invitation.InvitationNo, InvitationExpiry: invitation.ExpiresAt, Reused: invitation.Reused,
	}, nil
}

func (runtime *channelCommerceRuntime) Quote(
	ctx context.Context,
	input channelcommercecap.QuoteInput,
) (*channelcommercecap.Quote, error) {
	ctx, contextErr := runtime.fileCustomerContext(ctx, input.PayerCustomerNo)
	if contextErr != nil {
		return nil, contextErr
	}
	purchase, quantity, err := channelPurchase(input.Purchase)
	if err != nil {
		return nil, err
	}
	var quote *commerce.Quote
	if input.IdempotencyKey != "" {
		quote, err = runtime.orders.CreateQuote(ctx, input.ChannelID, input.PayerCustomerNo, input.IdempotencyKey, purchase, quantity)
	} else {
		quote, err = runtime.orders.Quote(ctx, input.PayerCustomerNo, purchase, quantity)
	}
	if err != nil {
		return nil, err
	}
	return &channelcommercecap.Quote{
		QuoteNo: quote.QuoteNo, CurrencyCode: quote.CurrencyCode, SubtotalMinor: quote.SubtotalMinor,
		DiscountMinor: quote.DiscountMinor, TaxAmountMinor: quote.TaxAmountMinor,
		TotalAmountMinor: quote.TotalAmountMinor, ExpiresAt: quote.ExpiresAt,
	}, nil
}

// CreateOrder commits the core checkout and same-currency balance payment together.
// A rejected payment rolls back the quote, order, invoice, reservations and outbox.
func (runtime *channelCommerceRuntime) CreateOrder(
	ctx context.Context,
	input channelcommercecap.CreateOrderInput,
) (*channelcommercecap.Order, error) {
	if input.OwnershipMode != "channel_owned" || input.ExternalCustomerID != "" ||
		input.PayerCustomerNo != input.ServiceOwnerCustomerNo ||
		(input.SettlementMethod != "" && input.SettlementMethod != "balance") {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	ctx, contextErr := runtime.fileCustomerContext(ctx, input.PayerCustomerNo)
	if contextErr != nil {
		return nil, contextErr
	}
	var purchase catalog.PurchaseConfigurationInput
	var quantity int
	var err error
	if input.QuoteNo == "" {
		purchase, quantity, err = channelPurchase(input.Purchase)
	}
	if err != nil {
		return nil, err
	}
	var result *channelcommercecap.Order
	err = runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		checkout, checkoutErr := runtime.orders.Checkout(ctx, commerce.ChannelOrderInput{
			QuoteNo:   input.QuoteNo,
			ChannelID: input.ChannelID, OwnershipMode: "channel_owned", PayerCustomerNo: input.PayerCustomerNo,
			ServiceOwnerCustomerNo: input.PayerCustomerNo, IdempotencyKey: input.IdempotencyKey,
			Purchase: purchase, Quantity: quantity,
		})
		if checkoutErr != nil {
			return checkoutErr
		}
		if checkout.TotalAmountMinor == 0 {
			if _, settleErr := runtime.settlements.SettleOrder(ctx, checkout.OrderNo); settleErr != nil {
				return settleErr
			}
		} else if _, settleErr := runtime.settle(ctx, input.PayerCustomerNo, checkout.InvoiceNo, checkout.OrderNo, input.IdempotencyKey, "balance"); settleErr != nil {
			return settleErr
		}
		var projectErr error
		result, projectErr = runtime.projectOrder(ctx, input.ChannelID, checkout.OrderNo, checkout.Reused)
		return projectErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (runtime *channelCommerceRuntime) SettleOrder(
	ctx context.Context,
	input channelcommercecap.SettleOrderInput,
) (*channelcommercecap.Order, error) {
	row, err := runtime.loadOrder(ctx, input.ChannelID, input.OrderNo)
	if err != nil {
		return nil, err
	}
	owner, err := runtime.loadCustomer(ctx, row.ServiceOwnerCustomerNo)
	if err != nil || owner.StatusCode != "active" || owner.IdentityStatus != "verified" {
		return nil, bizerr.NewCode(billing.CodeDirectoryNotAccessible)
	}
	if _, err = runtime.requirePayer(ctx, row.PayerCustomerNo); err != nil {
		return nil, err
	}
	if !row.FinanciallySettled {
		if _, err = runtime.settle(ctx, row.PayerCustomerNo, row.InvoiceNo, row.OrderNo, input.IdempotencyKey, input.SettlementMethod); err != nil {
			return nil, err
		}
	}
	return runtime.projectOrder(ctx, input.ChannelID, input.OrderNo, row.FinanciallySettled)
}

func (runtime *channelCommerceRuntime) GetOrder(
	ctx context.Context,
	channelID string,
	orderNo string,
) (*channelcommercecap.Order, error) {
	return runtime.projectOrder(ctx, channelID, orderNo, false)
}

func (runtime *channelCommerceRuntime) CancelOrder(
	ctx context.Context,
	input channelcommercecap.CancelOrderInput,
) (*channelcommercecap.Order, error) {
	row, err := runtime.loadOrder(ctx, input.ChannelID, input.OrderNo)
	if err != nil {
		return nil, err
	}
	cancelled, err := runtime.orders.Cancel(ctx, input.ChannelID, row.PayerCustomerNo, input.OrderNo, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return runtime.projectOrder(ctx, input.ChannelID, cancelled.OrderNo, cancelled.Reused)
}

func (runtime *channelCommerceRuntime) FinanceOverview(
	ctx context.Context,
	channelID string,
	payer string,
	requestedPages ...channelcommercecap.FinancePages,
) (*channelcommercecap.FinanceOverview, error) {
	pages, err := normalizeChannelFinancePages(requestedPages)
	if err != nil {
		return nil, err
	}
	channelID = strings.TrimSpace(channelID)
	payer = strings.TrimSpace(payer)
	if payer == "" {
		return &channelcommercecap.FinanceOverview{Currencies: []channelcommercecap.FinanceCurrency{}}, nil
	}
	if _, err := runtime.requirePayer(ctx, payer); err != nil {
		return nil, err
	}
	var rows []channelcommercecap.FinanceCurrency
	err = runtime.database.GetScan(ctx, &rows, `
WITH latest_balance AS (
    SELECT DISTINCT ON (account.account_no) account.currency_code,snapshot.closing_balance_minor
    FROM plugin_idc_core_billing_ledger_account account
    LEFT JOIN plugin_idc_core_billing_balance_snapshot snapshot ON snapshot.account_no=account.account_no AND snapshot.tenant_id=account.tenant_id
    WHERE account.tenant_id=0 AND account.customer_no=? AND account.account_type_code='customer_balance'
    ORDER BY account.account_no,snapshot.sequence_no DESC NULLS LAST
), balance AS (
    SELECT currency_code,COALESCE(SUM(closing_balance_minor),0)::BIGINT AS balance_minor FROM latest_balance GROUP BY currency_code
), held AS (
    SELECT currency_code,COALESCE(SUM(amount_minor-consumed_amount_minor-released_amount_minor),0)::BIGINT AS held_balance_minor
    FROM plugin_idc_core_billing_fund_hold WHERE tenant_id=0 AND customer_no=? AND status_code IN ('active','partially_consumed') GROUP BY currency_code
), credit AS (
    SELECT currency_code,COALESCE(SUM(total_limit_minor),0)::BIGINT AS credit_limit_minor,COALESCE(SUM(available_limit_minor),0)::BIGINT AS credit_available_minor
    FROM plugin_idc_core_credit_account WHERE tenant_id=0 AND customer_no=? AND status_code IN ('active','frozen') GROUP BY currency_code
), invoices AS (
    SELECT invoice.currency_code,
      COALESCE(SUM(invoice.total_amount_minor-invoice.paid_amount_minor-invoice.credit_amount_minor-invoice.waived_amount_minor),0)::BIGINT AS receivable_minor,
      COALESCE(SUM(invoice.paid_amount_minor),0)::BIGINT AS paid_minor,
      COALESCE(SUM(invoice.refunded_amount_minor),0)::BIGINT AS refunded_minor
    FROM plugin_idc_core_billing_invoice invoice
    JOIN plugin_idc_core_channel_order_attribution attribution ON attribution.order_no=invoice.order_no AND attribution.tenant_id=invoice.tenant_id
    WHERE invoice.tenant_id=0 AND attribution.channel_id=? AND attribution.payer_customer_no=? GROUP BY invoice.currency_code
), costs AS (
    SELECT item.currency_code,COALESCE(SUM(item.total_cost_minor),0)::BIGINT AS service_cost_minor
    FROM plugin_idc_core_commerce_order_item item
    JOIN plugin_idc_core_channel_order_attribution attribution ON attribution.order_no=item.order_no AND attribution.tenant_id=item.tenant_id
    WHERE item.tenant_id=0 AND attribution.channel_id=? AND attribution.payer_customer_no=? AND item.total_cost_minor IS NOT NULL GROUP BY item.currency_code
), currencies AS (
    SELECT currency_code FROM balance UNION SELECT currency_code FROM held UNION SELECT currency_code FROM credit UNION SELECT currency_code FROM invoices UNION SELECT currency_code FROM costs
)
SELECT currencies.currency_code,
 COALESCE(balance.balance_minor,0)::BIGINT AS balance_minor,
 COALESCE(held.held_balance_minor,0)::BIGINT AS held_balance_minor,
 (COALESCE(balance.balance_minor,0)-COALESCE(held.held_balance_minor,0))::BIGINT AS available_balance_minor,
 COALESCE(credit.credit_limit_minor,0)::BIGINT AS credit_limit_minor,
 COALESCE(credit.credit_available_minor,0)::BIGINT AS credit_available_minor,
 COALESCE(invoices.receivable_minor,0)::BIGINT AS receivable_minor,
 COALESCE(invoices.paid_minor,0)::BIGINT AS paid_minor,
 COALESCE(invoices.refunded_minor,0)::BIGINT AS refunded_minor,
 COALESCE(costs.service_cost_minor,0)::BIGINT AS service_cost_minor
FROM currencies
LEFT JOIN balance USING(currency_code) LEFT JOIN held USING(currency_code) LEFT JOIN credit USING(currency_code)
LEFT JOIN invoices USING(currency_code) LEFT JOIN costs USING(currency_code)
ORDER BY currencies.currency_code`, payer, payer, payer, channelID, payer, channelID, payer)
	if err != nil {
		return nil, err
	}
	result, err := runtime.loadChannelFinanceLists(ctx, channelID, payer, pages)
	if err != nil {
		return nil, err
	}
	result.Currencies = rows
	return result, nil
}

// loadChannelFinanceLists projects bounded rows and counts using identical owner predicates.
func (runtime *channelCommerceRuntime) loadChannelFinanceLists(ctx context.Context, channelID, payer string, pages channelcommercecap.FinancePages) (*channelcommercecap.FinanceOverview, error) {
	invoiceCount, err := runtime.database.GetValue(ctx, `SELECT COUNT(*)
FROM plugin_idc_core_billing_invoice invoice
JOIN plugin_idc_core_channel_order_attribution attribution
  ON attribution.tenant_id=invoice.tenant_id AND attribution.order_no=invoice.order_no
WHERE invoice.tenant_id=0 AND attribution.channel_id=? AND attribution.payer_customer_no=?`, channelID, payer)
	if err != nil {
		return nil, err
	}
	statementCount, err := runtime.database.GetValue(ctx, `SELECT COUNT(*)
FROM plugin_idc_core_credit_statement WHERE tenant_id=0 AND customer_no=?`, payer)
	if err != nil {
		return nil, err
	}
	repaymentCount, err := runtime.database.GetValue(ctx, `SELECT COUNT(*)
FROM plugin_idc_core_credit_repayment WHERE tenant_id=0 AND customer_no=?`, payer)
	if err != nil {
		return nil, err
	}
	var invoices []channelcommercecap.FinanceInvoice
	if err = runtime.database.GetScan(ctx, &invoices, `
SELECT invoice.invoice_no,invoice.order_no,invoice.currency_code,invoice.status_code,
       invoice.total_amount_minor,invoice.paid_amount_minor,invoice.credit_amount_minor,
       invoice.refunded_amount_minor,invoice.created_at
FROM plugin_idc_core_billing_invoice invoice
JOIN plugin_idc_core_channel_order_attribution attribution
  ON attribution.tenant_id=invoice.tenant_id AND attribution.order_no=invoice.order_no
WHERE invoice.tenant_id=0 AND attribution.channel_id=? AND attribution.payer_customer_no=?
ORDER BY invoice.created_at DESC,invoice.id DESC LIMIT ? OFFSET ?`, channelID, payer, pages.Invoices.PageSize, (pages.Invoices.Page-1)*pages.Invoices.PageSize); err != nil {
		return nil, err
	}
	var statements []channelcommercecap.CreditStatement
	if err = runtime.database.GetScan(ctx, &statements, `
SELECT statement_no,currency_code,status_code AS status,total_amount_minor AS total_minor,
       paid_amount_minor AS paid_minor,outstanding_amount_minor AS outstanding_minor,
       issued_at,payment_due_at
FROM plugin_idc_core_credit_statement
WHERE tenant_id=0 AND customer_no=?
ORDER BY issued_at DESC,id DESC LIMIT ? OFFSET ?`, payer, pages.Statements.PageSize, (pages.Statements.Page-1)*pages.Statements.PageSize); err != nil {
		return nil, err
	}
	var repayments []channelcommercecap.CreditRepayment
	if err = runtime.database.GetScan(ctx, &repayments, `
SELECT repayment.repayment_no,COALESCE(allocation.statement_no,'') AS statement_no,
       repayment.currency_code,repayment.status_code AS status,repayment.amount_minor,
       repayment.allocated_amount_minor AS allocated_minor,repayment.created_at
FROM plugin_idc_core_credit_repayment repayment
LEFT JOIN LATERAL (
    SELECT statement_no FROM plugin_idc_core_credit_repayment_allocation allocation
    WHERE allocation.tenant_id=repayment.tenant_id AND allocation.repayment_no=repayment.repayment_no
      AND allocation.statement_no IS NOT NULL
    ORDER BY allocation.id LIMIT 1
) allocation ON TRUE
WHERE repayment.tenant_id=0 AND repayment.customer_no=?
ORDER BY repayment.created_at DESC,repayment.id DESC LIMIT ? OFFSET ?`, payer, pages.Repayments.PageSize, (pages.Repayments.Page-1)*pages.Repayments.PageSize); err != nil {
		return nil, err
	}
	return &channelcommercecap.FinanceOverview{
		Invoices: invoices, CreditStatements: statements, CreditRepayments: repayments,
		InvoiceTotal: invoiceCount.Int(), StatementTotal: statementCount.Int(), RepaymentTotal: repaymentCount.Int(),
	}, nil
}

// normalizeChannelFinancePages bounds owner queries before any database access.
func normalizeChannelFinancePages(requested []channelcommercecap.FinancePages) (channelcommercecap.FinancePages, error) {
	const maximumSize = 100
	const maximumPage = 1000000
	pages := channelcommercecap.FinancePages{}
	if len(requested) > 1 {
		return pages, bizerr.NewCode(billing.CodeDirectoryInvalid)
	}
	if len(requested) == 1 {
		pages = requested[0]
	}
	for _, page := range []*channelcommercecap.FinancePage{&pages.Invoices, &pages.Statements, &pages.Repayments} {
		if page.Page == 0 {
			page.Page = 1
		}
		if page.PageSize == 0 {
			page.PageSize = 10
			if len(requested) == 0 {
				page.PageSize = maximumSize
			}
		}
		if page.Page < 1 || page.Page > maximumPage || page.PageSize < 1 || page.PageSize > maximumSize {
			return channelcommercecap.FinancePages{}, bizerr.NewCode(billing.CodeDirectoryInvalid)
		}
	}
	return pages, nil
}

func (runtime *channelCommerceRuntime) RepayCredit(
	ctx context.Context,
	input channelcommercecap.RepayCreditInput,
) (*channelcommercecap.RepayCreditResult, error) {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.PayerCustomerNo = strings.TrimSpace(input.PayerCustomerNo)
	input.StatementNo = strings.TrimSpace(input.StatementNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.ChannelID == "" || input.StatementNo == "" || input.AmountMinor <= 0 ||
		len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 120 {
		return nil, bizerr.NewCode(billing.CodeCreditInvalid)
	}
	if _, err := runtime.requirePayer(ctx, input.PayerCustomerNo); err != nil {
		return nil, err
	}
	var owner string
	if err := runtime.database.GetScan(ctx, &owner, `SELECT customer_no
FROM plugin_idc_core_credit_statement
WHERE tenant_id=0 AND statement_no=? AND customer_no=?`, input.StatementNo, input.PayerCustomerNo); err != nil {
		return nil, err
	}
	if owner != input.PayerCustomerNo {
		return nil, bizerr.NewCode(billing.CodeDirectoryNotAccessible)
	}
	snapshotDigest, err := channelDigest(struct {
		Kind, ChannelID, CustomerNo, StatementNo, RequestNo string
		AmountMinor                                         int64
	}{"channel_credit_balance_repayment", input.ChannelID, input.PayerCustomerNo,
		input.StatementNo, input.IdempotencyKey, input.AmountMinor})
	if err != nil {
		return nil, err
	}
	evidenceDigest, err := channelDigest(struct{ Kind, SnapshotDigest string }{
		"channel_approved_balance_repayment", snapshotDigest,
	})
	if err != nil {
		return nil, err
	}
	usage, err := billing.NewCreditUsageService(runtime.database)
	if err != nil {
		return nil, err
	}
	repayment, err := usage.RepayCreditStatementFromBalance(ctx, billing.CreditBalanceRepaymentInput{
		StatementNo: input.StatementNo, AmountMinor: input.AmountMinor,
		SourceReference:      "channel:" + input.ChannelID + ":" + input.IdempotencyKey,
		SourceSnapshotDigest: snapshotDigest, ConfirmationEvidenceDigest: evidenceDigest,
	})
	if err != nil {
		return nil, err
	}
	if repayment == nil || repayment.Repayment == nil || repayment.Repayment.CustomerNo != input.PayerCustomerNo {
		return nil, bizerr.NewCode(billing.CodeDirectoryNotAccessible)
	}
	return &channelcommercecap.RepayCreditResult{
		RepaymentNo: repayment.Repayment.RepaymentNo, StatementNo: repayment.Repayment.StatementNo,
		CurrencyCode: repayment.Repayment.CurrencyCode, Status: repayment.Repayment.StatusCode,
		AmountMinor:           repayment.Repayment.AmountMinor,
		OutstandingMinor:      repayment.Repayment.StatementOutstandingMinor,
		AvailableBalanceMinor: repayment.AvailableBalanceMinor, Reused: repayment.Reused,
	}, nil
}

func (runtime *channelCommerceRuntime) ListServices(
	ctx context.Context,
	input channelcommercecap.ServiceListInput,
) ([]channelcommercecap.ServiceItem, error) {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.PayerCustomerNo = strings.TrimSpace(input.PayerCustomerNo)
	input.ServiceNo = strings.TrimSpace(input.ServiceNo)
	if input.ChannelID == "" {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	if _, err := runtime.requirePayer(ctx, input.PayerCustomerNo); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT entitlement.entitlement_no AS service_no,
       entitlement.customer_no AS service_owner_customer_no,entitlement.order_no,
       entitlement.product_no,entitlement.version_no,entitlement.currency_code,
       entitlement.status_code AS status,entitlement.renewal_mode_code AS renewal_mode,
       entitlement.started_at,entitlement.expires_at,entitlement.updated_at
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_channel_order_attribution attribution
  ON attribution.tenant_id=entitlement.tenant_id AND attribution.order_no=entitlement.order_no
WHERE entitlement.tenant_id=0 AND attribution.channel_id=? AND attribution.payer_customer_no=?`
	arguments := []any{input.ChannelID, input.PayerCustomerNo}
	if input.ServiceNo != "" {
		query += " AND entitlement.entitlement_no=?"
		arguments = append(arguments, input.ServiceNo)
	}
	if input.OrderNo != "" {
		query += " AND entitlement.order_no=?"
		arguments = append(arguments, input.OrderNo)
	}
	if input.AfterServiceNo != "" {
		query += " AND entitlement.entitlement_no>?"
		arguments = append(arguments, input.AfterServiceNo)
	}
	query += " ORDER BY entitlement.entitlement_no ASC LIMIT ?"
	arguments = append(arguments, limit)
	items := make([]channelcommercecap.ServiceItem, 0)
	if err := runtime.database.GetScan(ctx, &items, query, arguments...); err != nil {
		return nil, err
	}
	return items, nil
}

func channelDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (runtime *channelCommerceRuntime) settle(
	ctx context.Context,
	payerCustomerNo string,
	invoiceNo string,
	orderNo string,
	idempotencyKey string,
	method string,
) (bool, error) {
	if strings.TrimSpace(method) != "balance" {
		return false, bizerr.NewCode(billing.CodeDirectoryInvalid)
	}
	if _, err := runtime.requirePayer(ctx, payerCustomerNo); err != nil {
		return false, err
	}
	var owned bool
	ownership, err := runtime.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=? AND order_no=? AND customer_no=?)`, invoiceNo, orderNo, payerCustomerNo)
	if err != nil {
		return false, err
	}
	owned = ownership.Bool()
	if !owned {
		return false, bizerr.NewCode(billing.CodeDirectoryNotAccessible)
	}
	digest := sha256.Sum256([]byte(payerCustomerNo + ":" + invoiceNo + ":" + idempotencyKey))
	fingerprint := hex.EncodeToString(digest[:])
	paid, err := runtime.invoices.PayFromBalance(ctx, billing.InvoiceBalancePaymentInput{InvoiceNo: invoiceNo, SourceReference: "channel:" + fingerprint, SourceSnapshotDigest: fingerprint})
	if err != nil {
		return false, err
	}
	if paid == nil || paid.Invoice == nil || paid.Posting == nil || paid.Invoice.CustomerNo != payerCustomerNo || paid.Invoice.OrderNo != orderNo || paid.Posting.CustomerNo != payerCustomerNo || paid.Invoice.CurrencyCode != paid.Posting.CurrencyCode {
		return false, bizerr.NewCode(billing.CodeDirectoryIntegrity)
	}
	if _, err = runtime.settlements.SettleOrder(ctx, orderNo); err != nil {
		return false, err
	}
	return paid.Reused, nil
}

func (runtime *channelCommerceRuntime) requirePayer(ctx context.Context, customerNo string) (*channelCustomerRow, error) {
	row, err := runtime.loadCustomer(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	if row.StatusCode != string(customer.StatusActive) || row.ActorUserID < 1 {
		return nil, bizerr.NewCode(customer.CodeStateDenied)
	}
	if row.IdentityStatus != string(identitycap.StatusVerified) {
		return nil, bizerr.NewCode(customer.CodeVerificationRequired)
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) loadCustomer(ctx context.Context, customerNo string) (*channelCustomerRow, error) {
	customerNo = strings.TrimSpace(customerNo)
	var row *channelCustomerRow
	err := runtime.database.GetScan(ctx, &row, `
SELECT subject.customer_no,subject.customer_type_code,subject.status_code,
 CASE WHEN subject.customer_type_code='enterprise' THEN enterprise.verification_status_code ELSE individual.identity_status_code END AS identity_status,
 COALESCE(binding.sys_user_id,0) AS actor_user_id
FROM plugin_idc_core_customer_subject subject
LEFT JOIN plugin_idc_core_customer_enterprise_profile enterprise ON enterprise.tenant_id=subject.tenant_id AND enterprise.customer_no=subject.customer_no
LEFT JOIN plugin_idc_core_customer_individual_profile individual ON individual.tenant_id=subject.tenant_id AND individual.customer_no=subject.customer_no
LEFT JOIN LATERAL (
  SELECT sys_user_id FROM plugin_idc_core_customer_identity_binding binding
  WHERE binding.tenant_id=subject.tenant_id AND binding.customer_no=subject.customer_no AND binding.status_code='active'
  ORDER BY binding.id LIMIT 1
) binding ON TRUE
WHERE subject.tenant_id=0 AND subject.customer_no=?`, customerNo)
	if err != nil {
		return nil, err
	}
	if row == nil || row.CustomerNo == "" {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) loadOrder(ctx context.Context, channelID, orderNo string) (*channelOrderRow, error) {
	var row *channelOrderRow
	err := runtime.database.GetScan(ctx, &row, `
SELECT orders.order_no,invoice.invoice_no,attribution.payer_customer_no,attribution.service_owner_customer_no,
 orders.currency_code,orders.status_code,orders.total_amount_minor,
 EXISTS(SELECT 1 FROM plugin_idc_core_financial_settlement settlement WHERE settlement.tenant_id=0 AND settlement.order_no=orders.order_no) AS financially_settled
FROM plugin_idc_core_channel_order_attribution attribution
JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=attribution.tenant_id AND orders.order_no=attribution.order_no
JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=orders.tenant_id AND invoice.order_no=orders.order_no AND invoice.source_code='order'
WHERE attribution.tenant_id=0 AND attribution.channel_id=? AND attribution.order_no=?`, strings.TrimSpace(channelID), strings.TrimSpace(orderNo))
	if err != nil {
		return nil, err
	}
	if row == nil || row.OrderNo == "" {
		return nil, bizerr.NewCode(commerce.CodeNotAccessible)
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) projectOrder(
	ctx context.Context,
	channelID string,
	orderNo string,
	reused bool,
) (*channelcommercecap.Order, error) {
	row, err := runtime.loadOrder(ctx, channelID, orderNo)
	if err != nil {
		return nil, err
	}
	owner, err := runtime.loadCustomer(ctx, row.ServiceOwnerCustomerNo)
	if err != nil {
		return nil, err
	}
	identityReady := owner.StatusCode == "active" && owner.IdentityStatus == "verified"
	status := row.StatusCode
	if !identityReady && !row.FinanciallySettled {
		status = "pending_identity"
	}
	return &channelcommercecap.Order{
		OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo, PayerCustomerNo: row.PayerCustomerNo,
		ServiceOwnerCustomerNo: row.ServiceOwnerCustomerNo, CurrencyCode: row.CurrencyCode,
		Status: status, TotalAmountMinor: row.TotalAmountMinor, FinanciallySettled: row.FinanciallySettled,
		IdentityReady: identityReady, Reused: reused,
	}, nil
}

func channelPurchase(input channelcommercecap.Purchase) (catalog.PurchaseConfigurationInput, int, error) {
	quantity := input.Quantity
	if quantity < 1 || quantity > 1000 {
		return catalog.PurchaseConfigurationInput{}, 0, bizerr.NewCode(commerce.CodeInvalid)
	}
	values := make([]catalog.PublicPurchaseConfigurationValue, len(input.Values))
	for index, value := range input.Values {
		values[index] = catalog.PublicPurchaseConfigurationValue{
			FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue,
			IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue,
			BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue,
			OptionTokens: append([]string(nil), value.OptionTokens...), TreeOptionTokens: append([]string(nil), value.TreeOptionTokens...),
			AddressOptionTokens: append([]string(nil), value.AddressOptionTokens...), FileTokens: append([]string(nil), value.FileTokens...),
		}
	}
	return catalog.PurchaseConfigurationInput{
		PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision,
		BillingTermToken: input.BillingTermToken, CurrencyCode: input.CurrencyCode,
		PurchaseCycles: input.PurchaseCycles, Values: values,
	}, quantity, nil
}

// BrowseCatalog exposes only published product projections, bounded by the
// catalog owner's cursor and page size rules. No cost or provider data escapes.
func (runtime *channelCommerceRuntime) BrowseCatalog(ctx context.Context, input channelcommercecap.CatalogQuery) (*channelcommercecap.CatalogPage, error) {
	page, err := runtime.catalog.Search(ctx, catalog.PublicCatalogSearchInput{NamePrefix: input.NamePrefix, Currency: input.Currency, Cursor: input.Cursor, Limit: input.Limit})
	if err != nil {
		return nil, err
	}
	result := &channelcommercecap.CatalogPage{Products: make([]channelcommercecap.CatalogProduct, 0, len(page.Products)), AvailableCurrencies: page.AvailableCurrencies, NextCursor: page.NextCursor}
	for _, row := range page.Products {
		result.Products = append(result.Products, channelcommercecap.CatalogProduct{PublicSlug: row.PublicSlug, NameZhCN: row.NameZhCN, NameEnUS: row.NameEnUS, SummaryZhCN: row.SummaryZhCN, SummaryEnUS: row.SummaryEnUS, Currencies: row.Currencies, Availability: row.Availability})
	}
	return result, nil
}

// newChannelCurrencyCatalogReader constructs a public reader with authoritative currency admission.
func newChannelCurrencyCatalogReader(database gdb.DB, key []byte, runtime catalog.PublicPurchaseConfigurationRuntime, files purchaseconfigurationfile.Service) (catalog.CatalogReader, error) {
	reader, err := catalog.NewPublicCatalogReaderConfigured(database, key, runtime, files)
	if err != nil {
		return nil, err
	}
	source, err := currency.NewReader(database)
	if err != nil {
		return nil, err
	}
	if err = catalog.BindCurrencies(reader, source); err != nil {
		return nil, err
	}
	return reader, nil
}

// bindChannelCheckoutCapabilities attaches currency and configuration admission before channel checkout routes.
func bindChannelCheckoutCapabilities(owner commerce.Service, database gdb.DB, freezer catalog.PurchaseConfigurationFreezer, files purchaseconfigurationfile.OrderFileConsumer) error {
	reader, err := currency.NewReader(database)
	if err != nil {
		return err
	}
	if err = commerce.BindCheckoutCurrencies(owner, reader); err != nil {
		return err
	}
	return commerce.BindStructuredConfigurationSnapshots(owner, freezer, files)
}
