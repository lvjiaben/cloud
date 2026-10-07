// Package channel adapts the downstream channel service to HTTP.
package channel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	adminv1 "lina-plugin-idc-channel/backend/api/admin/v1"
	v1 "lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type Controller struct{ svc channel.Service }
type AdminController struct{ svc channel.Service }

func New(svc channel.Service) *Controller           { return &Controller{svc: svc} }
func NewAdmin(svc channel.Service) *AdminController { return &AdminController{svc: svc} }
func unixMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UnixMilli()
	return &milliseconds
}
func projectAdminChannel(value channel.ChannelSummary) *adminv1.ChannelSummary {
	return &adminv1.ChannelSummary{ChannelId: value.ChannelID, CustomerNo: value.CustomerNo, DisplayName: value.DisplayName, StatusCode: value.StatusCode, NewSalesEnabled: value.NewSalesEnabled, CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli()}
}
func (c *AdminController) ListChannels(ctx context.Context, req *adminv1.ListChannelsReq) (*adminv1.ListChannelsRes, error) {
	page, err := c.svc.ListChannels(ctx, channel.PageQuery{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.ChannelSummary, 0, len(page.List))
	for _, row := range page.List {
		items = append(items, projectAdminChannel(row))
	}
	return &adminv1.ListChannelsRes{List: items, PageMetadata: adminv1.PageMetadata{Page: page.Page, PageSize: page.PageSize, Total: page.Total}}, nil
}
func (c *AdminController) SaveChannel(ctx context.Context, req *adminv1.SaveChannelReq) (*adminv1.SaveChannelRes, error) {
	row, err := c.svc.SaveChannel(ctx, channel.ChannelInput{ChannelID: req.ChannelId, CustomerNo: req.CustomerNo, DisplayName: req.DisplayName})
	if err != nil {
		return nil, err
	}
	return &adminv1.SaveChannelRes{Channel: projectAdminChannel(row)}, nil
}
func (c *AdminController) SetChannelStatus(ctx context.Context, req *adminv1.SetChannelStatusReq) (*adminv1.SetChannelStatusRes, error) {
	row, err := c.svc.SetChannelStatus(ctx, req.ChannelId, req.Action)
	if err != nil {
		return nil, err
	}
	return &adminv1.SetChannelStatusRes{Channel: projectAdminChannel(row)}, nil
}
func projectAdminPublication(value channel.PublicationSummary) *adminv1.PublicationSummary {
	return &adminv1.PublicationSummary{ChannelId: value.ChannelID, ProductVersionId: value.ProductVersionID, Currency: value.Currency, PriceMinor: value.PriceMinor, Stock: value.Stock, Status: value.Status, PublicSlug: value.PublicSlug, ConfigurationRevision: value.ConfigurationRevision, BillingTermToken: value.BillingTermToken, PurchaseCycles: value.PurchaseCycles, Quantity: value.Quantity}
}
func (c *AdminController) ListPublications(ctx context.Context, req *adminv1.ListPublicationsReq) (*adminv1.ListPublicationsRes, error) {
	page, err := c.svc.ListPublications(ctx, req.ChannelId, channel.PageQuery{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.PublicationSummary, 0, len(page.List))
	for _, row := range page.List {
		items = append(items, projectAdminPublication(row))
	}
	return &adminv1.ListPublicationsRes{List: items, PageMetadata: adminv1.PageMetadata{Page: page.Page, PageSize: page.PageSize, Total: page.Total}}, nil
}
func (c *AdminController) SavePublication(ctx context.Context, req *adminv1.SavePublicationReq) (*adminv1.SavePublicationRes, error) {
	values := make([]channelcommercecap.PurchaseValue, len(req.Values))
	for index, value := range req.Values {
		values[index] = channelcommercecap.PurchaseValue{FieldToken: value.FieldToken, Type: value.Type, TextValue: value.TextValue, IntegerValue: value.IntegerValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue, OptionTokens: value.OptionTokens, TreeOptionTokens: value.TreeOptionTokens, AddressOptionTokens: value.AddressOptionTokens, FileTokens: value.FileTokens}
	}
	row, err := c.svc.SavePublication(ctx, channel.PublicationInput{ChannelID: req.ChannelId, ProductVersionID: req.ProductVersionId, Status: req.Status, Stock: req.Stock, Purchase: channelcommercecap.Purchase{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, BillingTermToken: req.BillingTermToken, CurrencyCode: req.Currency, PurchaseCycles: req.PurchaseCycles, Quantity: req.Quantity, Values: values}})
	if err != nil {
		return nil, err
	}
	return &adminv1.SavePublicationRes{Publication: projectAdminPublication(row)}, nil
}
func (c *AdminController) ListRequestAudit(ctx context.Context, req *adminv1.ListRequestAuditReq) (*adminv1.ListRequestAuditRes, error) {
	page, err := c.svc.ListRequestAudit(ctx, req.ChannelId, channel.PageQuery{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.RequestAudit, 0, len(page.List))
	for _, row := range page.List {
		item := &adminv1.RequestAudit{Id: row.ID, ChannelId: row.ChannelID, RequestId: row.RequestID,
			Operation: row.Operation, Status: row.Status, ResultCode: row.ResultCode,
			CreatedAt: row.CreatedAt.UnixMilli(), CompletedAt: unixMillis(row.CompletedAt)}
		items = append(items, item)
	}
	return &adminv1.ListRequestAuditRes{List: items, PageMetadata: adminv1.PageMetadata{Page: page.Page, PageSize: page.PageSize, Total: page.Total}}, nil
}
func (c *AdminController) Health(ctx context.Context, req *adminv1.HealthReq) (*adminv1.HealthRes, error) {
	row, err := c.svc.Health(ctx, req.ChannelId)
	if err != nil {
		return nil, err
	}
	result := &adminv1.HealthRes{ChannelId: row.ChannelID, StatusCode: row.StatusCode, NewSalesEnabled: row.NewSalesEnabled, ActiveTokenCount: row.ActiveTokenCount, PublicationCount: row.PublicationCount, PendingWebhookCount: row.PendingWebhookCount}
	result.LastRequestAt = unixMillis(row.LastRequestAt)
	return result, nil
}
func (c *AdminController) Finance(ctx context.Context, req *adminv1.FinanceReq) (*adminv1.FinanceRes, error) {
	row, err := c.svc.Finance(ctx, req.ChannelId, channelcommercecap.FinancePages{
		Invoices:   channelcommercecap.FinancePage{Page: req.InvoicePage, PageSize: req.InvoicePageSize},
		Statements: channelcommercecap.FinancePage{Page: req.StatementPage, PageSize: req.StatementPageSize},
		Repayments: channelcommercecap.FinancePage{Page: req.RepaymentPage, PageSize: req.RepaymentPageSize},
	})
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.FinanceCurrency, 0, len(row.Currencies))
	for _, item := range row.Currencies {
		items = append(items, &adminv1.FinanceCurrency{CurrencyCode: item.CurrencyCode, BalanceMinor: item.BalanceMinor, HeldBalanceMinor: item.HeldBalanceMinor, AvailableBalanceMinor: item.AvailableBalanceMinor, CreditLimitMinor: item.CreditLimitMinor, CreditAvailableMinor: item.CreditAvailableMinor, ReceivableMinor: item.ReceivableMinor, PaidMinor: item.PaidMinor, RefundedMinor: item.RefundedMinor, ServiceCostMinor: item.ServiceCostMinor})
	}
	invoices := make([]*adminv1.FinanceInvoice, 0, len(row.Invoices))
	for _, item := range row.Invoices {
		invoices = append(invoices, &adminv1.FinanceInvoice{InvoiceNo: item.InvoiceNo, OrderNo: item.OrderNo,
			CurrencyCode: item.CurrencyCode, Status: item.Status, TotalAmountMinor: item.TotalAmountMinor,
			PaidAmountMinor: item.PaidAmountMinor, CreditAmountMinor: item.CreditAmountMinor,
			RefundedAmountMinor: item.RefundedAmountMinor, CreatedAt: item.CreatedAt.UnixMilli()})
	}
	statements := make([]*adminv1.CreditStatement, 0, len(row.CreditStatements))
	for _, item := range row.CreditStatements {
		statements = append(statements, &adminv1.CreditStatement{StatementNo: item.StatementNo,
			CurrencyCode: item.CurrencyCode, Status: item.Status, TotalMinor: item.TotalMinor,
			PaidMinor: item.PaidMinor, OutstandingMinor: item.OutstandingMinor,
			IssuedAt: item.IssuedAt.UnixMilli(), PaymentDueAt: item.PaymentDueAt.UnixMilli()})
	}
	repayments := make([]*adminv1.CreditRepayment, 0, len(row.CreditRepayments))
	for _, item := range row.CreditRepayments {
		repayments = append(repayments, &adminv1.CreditRepayment{RepaymentNo: item.RepaymentNo,
			StatementNo: item.StatementNo, CurrencyCode: item.CurrencyCode, Status: item.Status,
			AmountMinor: item.AmountMinor, AllocatedMinor: item.AllocatedMinor,
			CreatedAt: item.CreatedAt.UnixMilli()})
	}
	return &adminv1.FinanceRes{Currencies: items, Invoices: invoices,
		InvoiceTotal: row.InvoiceTotal, StatementTotal: row.StatementTotal, RepaymentTotal: row.RepaymentTotal,
		CreditStatements: statements, CreditRepayments: repayments}, nil
}
func (c *AdminController) Capabilities(ctx context.Context, _ *adminv1.CapabilitiesReq) (*adminv1.CapabilitiesRes, error) {
	o := c.svc.Capabilities(ctx)
	return &adminv1.CapabilitiesRes{GenericAPI: o.GenericAPI, WebhookDelivery: o.WebhookDelivery, Adapters: o.Adapters, Operations: o.Operations}, nil
}
func (c *AdminController) Catalog(ctx context.Context, r *adminv1.CatalogReq) (*adminv1.CatalogRes, error) {
	page, err := c.svc.CatalogPage(ctx, r.ChannelId, channel.PageQuery{Page: r.Page, PageSize: r.PageSize})
	if err != nil {
		return nil, err
	}
	out := make([]*adminv1.CatalogItem, 0, len(page.List))
	for _, item := range page.List {
		out = append(out, &adminv1.CatalogItem{ProductVersionId: item.ProductVersionID, Currency: item.Currency, Price: item.Price, Stock: item.Stock})
	}
	return &adminv1.CatalogRes{List: out, PageMetadata: adminv1.PageMetadata{Page: page.Page, PageSize: page.PageSize, Total: page.Total}}, nil
}
func (c *AdminController) CreateToken(ctx context.Context, r *adminv1.CreateTokenReq) (*adminv1.CreateTokenRes, error) {
	var exp *time.Time
	if r.ExpiresAt != nil {
		t := time.UnixMilli(*r.ExpiresAt)
		exp = &t
	}
	out, e := c.svc.CreateToken(ctx, channel.TokenInput{ChannelID: r.ChannelId, Scopes: r.Scopes, SourceCIDRs: r.SourceCIDRs, ExpiresAt: exp})
	if e != nil {
		return nil, e
	}
	return &adminv1.CreateTokenRes{Id: out.ID, Token: out.Token, ExpiresAt: unixMillis(out.ExpiresAt)}, nil
}
func (c *AdminController) ListTokens(ctx context.Context, r *adminv1.ListTokensReq) (*adminv1.ListTokensRes, error) {
	page, err := c.svc.ListTokenPage(ctx, r.ChannelId, channel.PageQuery{Page: r.Page, PageSize: r.PageSize})
	if err != nil {
		return nil, err
	}
	out := make([]*adminv1.TokenSummary, 0, len(page.List))
	for _, item := range page.List {
		v := &adminv1.TokenSummary{Id: item.ID, ChannelId: item.ChannelID, Scopes: item.Scopes,
			SourceCIDRs: item.SourceCIDRs, CreatedAt: item.CreatedAt.UnixMilli(),
			ExpiresAt: unixMillis(item.ExpiresAt), RevokedAt: unixMillis(item.RevokedAt)}
		out = append(out, v)
	}
	return &adminv1.ListTokensRes{List: out, PageMetadata: adminv1.PageMetadata{Page: page.Page, PageSize: page.PageSize, Total: page.Total}}, nil
}
func (c *AdminController) RevokeToken(ctx context.Context, r *adminv1.RevokeTokenReq) (*adminv1.RevokeTokenRes, error) {
	e := c.svc.RevokeToken(ctx, r.Id)
	if e != nil {
		return nil, e
	}
	return &adminv1.RevokeTokenRes{Id: r.Id, Revoked: true}, nil
}
func (c *Controller) Catalog(ctx context.Context, r *v1.CatalogReq) (*v1.CatalogRes, error) {
	items, e := c.svc.Catalog(ctx, channel.CredentialChannelID(ctx))
	if e != nil {
		return nil, e
	}
	out := make([]*v1.CatalogItem, 0, len(items))
	for _, x := range items {
		y := x
		out = append(out, &v1.CatalogItem{ProductVersionId: y.ProductVersionID, Currency: y.Currency, Price: y.Price, Stock: y.Stock})
	}
	return &v1.CatalogRes{List: out}, nil
}
func (c *Controller) Quote(ctx context.Context, r *v1.QuoteReq) (*v1.QuoteRes, error) {
	o, e := c.svc.Quote(ctx, channel.QuoteInput{ChannelID: channel.CredentialChannelID(ctx), ProductVersionID: r.ProductVersionId, Currency: r.Currency})
	if e != nil {
		return nil, e
	}
	return &v1.QuoteRes{ProductVersionId: o.ProductVersionID, Currency: o.Currency, Price: o.Price, TotalAmountMinor: o.TotalAmountMinor, Stock: o.Stock, Status: o.Status}, nil
}
func (c *Controller) CreateOrder(ctx context.Context, r *v1.CreateOrderReq) (*v1.CreateOrderRes, error) {
	var delegated *channel.DelegatedCustomerInput
	if r.DelegatedCustomer != nil {
		delegated = &channel.DelegatedCustomerInput{
			CustomerType: r.DelegatedCustomer.CustomerType, DisplayName: r.DelegatedCustomer.DisplayName,
			CountryCode: r.DelegatedCustomer.CountryCode, PreferredLocale: r.DelegatedCustomer.PreferredLocale,
			Email: r.DelegatedCustomer.Email, PhoneCountryCode: r.DelegatedCustomer.PhoneCountryCode,
			Phone: r.DelegatedCustomer.Phone,
		}
	}
	o, e := c.svc.CreateOrder(ctx, channel.OrderInput{ChannelID: channel.CredentialChannelID(ctx), RequestID: r.RequestId, ExternalCustomerID: r.ExternalCustomerId, OwnershipMode: r.OwnershipMode, ProductVersionID: r.ProductVersionId, Currency: r.Currency, SettlementMethod: r.SettlementMethod, DelegatedCustomer: delegated})
	if e != nil {
		return nil, e
	}
	return &v1.CreateOrderRes{Id: o.ID, Status: o.Status, Replayed: o.Replayed}, nil
}
func (c *Controller) SettleOrder(ctx context.Context, r *v1.SettleOrderReq) (*v1.SettleOrderRes, error) {
	o, e := c.svc.SettleOrder(ctx, channel.SettleInput{ChannelID: channel.CredentialChannelID(ctx), RequestID: r.RequestId, OrderID: r.Id, SettlementMethod: r.SettlementMethod})
	if e != nil {
		return nil, e
	}
	return &v1.SettleOrderRes{Id: o.ID, Status: o.Status, Replayed: o.Replayed}, nil
}
func (c *Controller) CancelOrder(ctx context.Context, r *v1.CancelOrderReq) (*v1.CancelOrderRes, error) {
	o, e := c.svc.CancelOrder(ctx, channel.CancelInput{ChannelID: channel.CredentialChannelID(ctx), RequestID: r.RequestId, OrderID: r.Id})
	if e != nil {
		return nil, e
	}
	return &v1.CancelOrderRes{Id: o.ID, Status: o.Status, Replayed: o.Replayed}, nil
}
func (c *Controller) GetOrder(ctx context.Context, r *v1.GetOrderReq) (*v1.GetOrderRes, error) {
	o, e := c.svc.GetOrder(ctx, r.Id, channel.CredentialChannelID(ctx))
	if e != nil {
		return nil, e
	}
	return &v1.GetOrderRes{Id: o.ID, ChannelId: o.ChannelID, OwnershipMode: o.OwnershipMode, Currency: o.Currency, Status: o.Status, FinanciallySettled: o.FinanciallySettled, IdentityReady: o.IdentityReady, CoreOrderNo: o.CoreOrderNo, PayerCustomerNo: o.PayerCustomerNo, ServiceOwnerCustomerNo: o.ServiceOwnerCustomerNo, TotalAmountMinor: o.TotalAmountMinor}, nil
}
func (c *Controller) Finance(ctx context.Context, _ *v1.FinanceReq) (*v1.FinanceRes, error) {
	o, e := c.svc.Finance(ctx, channel.CredentialChannelID(ctx))
	if e != nil {
		return nil, e
	}
	items := make([]*v1.FinanceCurrency, 0, len(o.Currencies))
	for _, item := range o.Currencies {
		items = append(items, &v1.FinanceCurrency{CurrencyCode: item.CurrencyCode, BalanceMinor: item.BalanceMinor, HeldBalanceMinor: item.HeldBalanceMinor, AvailableBalanceMinor: item.AvailableBalanceMinor, CreditLimitMinor: item.CreditLimitMinor, CreditAvailableMinor: item.CreditAvailableMinor, ReceivableMinor: item.ReceivableMinor, PaidMinor: item.PaidMinor, RefundedMinor: item.RefundedMinor, ServiceCostMinor: item.ServiceCostMinor})
	}
	invoices := make([]*v1.FinanceInvoice, 0, len(o.Invoices))
	for _, item := range o.Invoices {
		invoices = append(invoices, &v1.FinanceInvoice{InvoiceNo: item.InvoiceNo, OrderNo: item.OrderNo,
			CurrencyCode: item.CurrencyCode, Status: item.Status, TotalAmountMinor: item.TotalAmountMinor,
			PaidAmountMinor: item.PaidAmountMinor, CreditAmountMinor: item.CreditAmountMinor,
			RefundedAmountMinor: item.RefundedAmountMinor, CreatedAt: item.CreatedAt.UnixMilli()})
	}
	statements := make([]*v1.CreditStatement, 0, len(o.CreditStatements))
	for _, item := range o.CreditStatements {
		statements = append(statements, &v1.CreditStatement{StatementNo: item.StatementNo,
			CurrencyCode: item.CurrencyCode, Status: item.Status, TotalMinor: item.TotalMinor,
			PaidMinor: item.PaidMinor, OutstandingMinor: item.OutstandingMinor,
			IssuedAt: item.IssuedAt.UnixMilli(), PaymentDueAt: item.PaymentDueAt.UnixMilli()})
	}
	repayments := make([]*v1.CreditRepayment, 0, len(o.CreditRepayments))
	for _, item := range o.CreditRepayments {
		repayments = append(repayments, &v1.CreditRepayment{RepaymentNo: item.RepaymentNo,
			StatementNo: item.StatementNo, CurrencyCode: item.CurrencyCode, Status: item.Status,
			AmountMinor: item.AmountMinor, AllocatedMinor: item.AllocatedMinor, CreatedAt: item.CreatedAt.UnixMilli()})
	}
	return &v1.FinanceRes{Currencies: items, Invoices: invoices,
		CreditStatements: statements, CreditRepayments: repayments}, nil
}

func (c *Controller) RepayCredit(ctx context.Context, req *v1.RepayCreditReq) (*v1.RepayCreditRes, error) {
	result, err := c.svc.RepayCredit(ctx, channel.CreditRepaymentInput{
		ChannelID: channel.CredentialChannelID(ctx), StatementNo: req.StatementNo,
		RequestID: req.RequestId, AmountMinor: req.AmountMinor,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RepayCreditRes{RepaymentNo: result.RepaymentNo, StatementNo: result.StatementNo,
		CurrencyCode: result.CurrencyCode, Status: result.Status, AmountMinor: result.AmountMinor,
		OutstandingMinor: result.OutstandingMinor, AvailableBalanceMinor: result.AvailableBalanceMinor,
		Replayed: result.Reused}, nil
}

func projectService(item channelcommercecap.ServiceItem) *v1.ServiceItem {
	return &v1.ServiceItem{ServiceNo: item.ServiceNo, ServiceOwnerCustomerNo: item.ServiceOwnerCustomerNo,
		OrderNo: item.OrderNo, ProductNo: item.ProductNo, VersionNo: item.VersionNo,
		CurrencyCode: item.CurrencyCode, Status: item.Status, RenewalMode: item.RenewalMode,
		StartedAt: item.StartedAt.UnixMilli(), ExpiresAt: unixMillis(item.ExpiresAt), UpdatedAt: item.UpdatedAt.UnixMilli()}
}

func (c *Controller) ListServices(ctx context.Context, _ *v1.ListServicesReq) (*v1.ListServicesRes, error) {
	rows, err := c.svc.ListServices(ctx, channel.CredentialChannelID(ctx), "")
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ServiceItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, projectService(row))
	}
	return &v1.ListServicesRes{List: items}, nil
}

func (c *Controller) GetService(ctx context.Context, req *v1.GetServiceReq) (*v1.GetServiceRes, error) {
	rows, err := c.svc.ListServices(ctx, channel.CredentialChannelID(ctx), req.ServiceNo)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, gerror.New("service not found")
	}
	return &v1.GetServiceRes{Service: projectService(rows[0])}, nil
}
func (c *Controller) Lifecycle(ctx context.Context, r *v1.LifecycleReq) (*v1.LifecycleRes, error) {
	o, e := c.svc.Lifecycle(ctx, channel.LifecycleInput{
		ChannelID: channel.CredentialChannelID(ctx), ServiceNo: r.ServiceNo, RequestID: r.RequestId,
		Operation: r.Operation, Parameters: r.Parameters, Confirmed: r.Confirmed,
	})
	if e != nil {
		return nil, e
	}
	return &v1.LifecycleRes{OperationNo: o.OperationNo, QuoteNo: o.QuoteNo, Status: o.Status,
		CurrencyCode: o.CurrencyCode, Direction: o.Direction, AmountMinor: o.AmountMinor,
		EffectiveAt: unixMillis(o.EffectiveAt), ExpiresAt: unixMillis(o.ExpiresAt), PaymentRequired: o.PaymentRequired,
		OutputAvailable: o.OutputAvailable, Replayed: o.Replayed}, nil
}

func (c *Controller) LifecycleOutput(ctx context.Context, r *v1.LifecycleOutputReq) (*v1.LifecycleOutputRes, error) {
	if !r.Confirmed {
		return nil, gerror.New("explicit confirmation is required")
	}
	response := g.RequestFromCtx(ctx).Response
	response.Header().Set("Cache-Control", "no-store, max-age=0")
	response.Header().Set("Pragma", "no-cache")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	err := c.svc.RedeemLifecycleOutput(ctx, channel.CredentialChannelID(ctx),
		r.ServiceNo, r.OperationNo, func(payload []byte) error {
			if !json.Valid(payload) {
				return gerror.New("confidential output is invalid")
			}
			response.Write([]byte(`{"code":0,"message":"OK","data":{"payload":`), payload, []byte(`}}`))
			return nil
		})
	if err != nil {
		return nil, err
	}
	return nil, nil
}
func (c *Controller) Capabilities(ctx context.Context, r *v1.CapabilitiesReq) (*v1.CapabilitiesRes, error) {
	o := c.svc.Capabilities(ctx)
	return &v1.CapabilitiesRes{GenericAPI: o.GenericAPI, WebhookDelivery: o.WebhookDelivery, Adapters: o.Adapters, Operations: o.Operations}, nil
}

func (c *Controller) GetWebhook(ctx context.Context, r *v1.GetWebhookReq) (*v1.GetWebhookRes, error) {
	event, err := c.svc.GetWebhookEvent(ctx, channel.CredentialChannelID(ctx), r.EventId)
	if err != nil {
		return nil, err
	}
	return &v1.GetWebhookRes{EventId: event.EventID, Version: event.Version, EventType: event.EventType, Payload: event.Payload, Signature: event.Signature, Status: event.Status, Attempts: event.Attempts}, nil
}
