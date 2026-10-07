// Package channel implements token, publication and idempotent order services for downstream channels.
package channel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type Service interface {
	AuthenticateToken(context.Context, string, string, string) (TokenCredential, error)
	CreateToken(context.Context, TokenInput) (TokenResult, error)
	ListTokens(context.Context, string) ([]TokenSummary, error)
	ListTokenPage(context.Context, string, PageQuery) (PageResult[TokenSummary], error)
	RevokeToken(context.Context, int64) error
	Catalog(context.Context, string) ([]CatalogItem, error)
	CatalogPage(context.Context, string, PageQuery) (PageResult[CatalogItem], error)
	Quote(context.Context, QuoteInput) (QuoteResult, error)
	CreateOrder(context.Context, OrderInput) (OrderResult, error)
	SettleOrder(context.Context, SettleInput) (OrderResult, error)
	CancelOrder(context.Context, CancelInput) (OrderResult, error)
	GetOrder(context.Context, int64, string) (Order, error)
	Lifecycle(context.Context, LifecycleInput) (LifecycleResult, error)
	RedeemLifecycleOutput(context.Context, string, string, string, channelcommercecap.ConfidentialOutputConsumer) error
	EnqueueWebhook(context.Context, WebhookEventInput) (WebhookDelivery, error)
	ClaimWebhookDeliveries(context.Context, int, time.Duration) ([]WebhookDelivery, error)
	MarkWebhookDelivered(context.Context, string, string, string) error
	ScheduleWebhookRetry(context.Context, string, string, string, string) error
	GetWebhookEvent(context.Context, string, string) (WebhookDelivery, error)
	Capabilities(context.Context) Capabilities
	Finance(context.Context, string, ...channelcommercecap.FinancePages) (channelcommercecap.FinanceOverview, error)
	RepayCredit(context.Context, CreditRepaymentInput) (channelcommercecap.RepayCreditResult, error)
	ListServices(context.Context, string, string) ([]channelcommercecap.ServiceItem, error)
	ListChannels(context.Context, PageQuery) (PageResult[ChannelSummary], error)
	SaveChannel(context.Context, ChannelInput) (ChannelSummary, error)
	SetChannelStatus(context.Context, string, string) (ChannelSummary, error)
	ListPublications(context.Context, string, PageQuery) (PageResult[PublicationSummary], error)
	SavePublication(context.Context, PublicationInput) (PublicationSummary, error)
	ListRequestAudit(context.Context, string, PageQuery) (PageResult[RequestAudit], error)
	Health(context.Context, string) (ChannelHealth, error)
	CreateOwnToken(context.Context, TokenInput) (TokenResult, error)
	ListOwnTokens(context.Context) ([]TokenSummary, error)
	RevokeOwnToken(context.Context, int64) error
}

const TokenScopeAll = "*"

var supportedTokenScopes = map[string]struct{}{
	TokenScopeAll:     {},
	"capability.read": {}, "catalog.read": {},
	"order.create": {}, "order.read": {}, "order.cancel": {},
	"service.renew": {}, "service.reconfigure": {}, "service.reinstall": {},
	"service.console": {}, "service.change_ip": {}, "service.terminate": {},
	"service.output": {},
	"service.read":   {}, "settlement.read": {}, "settlement.repay": {}, "webhook.read": {},
}

type tokenCredentialContextKey struct{}

// TokenCredential is the immutable identity established by channel-token
// middleware. It contains no token hash or raw credential.
type TokenCredential struct {
	TokenID   int64
	ChannelID string
}

// CredentialChannelID returns the middleware-established channel identity.
// Request payloads are deliberately not a fallback authority.
func CredentialChannelID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	credential, ok := ctx.Value(tokenCredentialContextKey{}).(TokenCredential)
	if !ok || credential.TokenID <= 0 {
		return ""
	}
	return strings.TrimSpace(credential.ChannelID)
}

func withTokenCredential(ctx context.Context, credential TokenCredential) context.Context {
	return context.WithValue(ctx, tokenCredentialContextKey{}, credential)
}

type TokenInput struct {
	ChannelID           string
	Scopes, SourceCIDRs []string
	ExpiresAt           *time.Time
}
type TokenResult struct {
	ID        int64
	Token     string
	ExpiresAt *time.Time
}
type TokenSummary struct {
	ID          int64
	ChannelID   string
	Scopes      []string
	SourceCIDRs []string
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}
type CatalogItem struct {
	ProductVersionID int64
	Currency         string
	Price            string
	Stock            int
}
type QuoteInput struct {
	ChannelID        string
	ProductVersionID int64
	Currency         string
}
type QuoteResult struct {
	ProductVersionID int64
	Currency         string
	Price            string
	TotalAmountMinor int64
	Stock            int
	Status           string
}

type DelegatedCustomerInput struct {
	CustomerType     string
	DisplayName      string
	CountryCode      string
	PreferredLocale  string
	Email            string
	PhoneCountryCode string
	Phone            string
}
type OrderInput struct {
	ChannelID, RequestID, ExternalCustomerID, OwnershipMode string
	ProductVersionID                                        int64
	Currency                                                string
	SettlementMethod                                        string
	DelegatedCustomer                                       *DelegatedCustomerInput
}
type OrderResult struct {
	ID       int64
	Status   string
	Replayed bool
}
type CancelInput struct {
	ChannelID string
	RequestID string
	OrderID   int64
}

type SettleInput struct {
	ChannelID        string
	RequestID        string
	OrderID          int64
	SettlementMethod string
}
type Order struct {
	ID                                                   int64
	ChannelID, OwnershipMode, Currency, Status           string
	CoreOrderNo, PayerCustomerNo, ServiceOwnerCustomerNo string
	TotalAmountMinor                                     int64
	FinanciallySettled, IdentityReady                    bool
}

// LifecycleInput is the stable downstream contract for operations that are
// owned by the resource and core service layers. The channel validates the
// operation name and request envelope, but never pretends to execute an
// operation when those owners are not connected.
type LifecycleInput struct {
	ChannelID  string
	ServiceNo  string
	RequestID  string
	Operation  string
	Parameters json.RawMessage
	Confirmed  bool
}

type LifecycleResult struct {
	OperationNo     string
	QuoteNo         string
	Status          string
	CurrencyCode    string
	Direction       string
	AmountMinor     int64
	EffectiveAt     *time.Time
	ExpiresAt       *time.Time
	PaymentRequired bool
	OutputAvailable bool
	Replayed        bool
}
type CreditRepaymentInput struct {
	ChannelID   string
	StatementNo string
	RequestID   string
	AmountMinor int64
}
type Capabilities struct {
	GenericAPI      bool
	WebhookDelivery bool
	Adapters        map[string]string
	Operations      map[string]string
}
type service struct {
	db     gdb.DB
	bizCtx bizctxcap.Service
	core   channelcommercecap.Runtime
}

func New(db gdb.DB, core channelcommercecap.Runtime, contexts ...bizctxcap.Service) Service {
	var bizCtx bizctxcap.Service
	if len(contexts) > 0 {
		bizCtx = contexts[0]
	}
	return &service{db: db, bizCtx: bizCtx, core: core}
}

func (s *service) actorID(ctx context.Context) int64 {
	if s.bizCtx == nil {
		return 0
	}
	current := s.bizCtx.Current(ctx)
	if current.ActingUserID > 0 {
		return int64(current.ActingUserID)
	}
	if current.UserID > 0 {
		return int64(current.UserID)
	}
	return 0
}

func (s *service) CreateToken(ctx context.Context, in TokenInput) (TokenResult, error) {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	if strings.TrimSpace(in.ChannelID) == "" {
		return TokenResult{}, gerror.New("channel id is required")
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return TokenResult{}, gerror.New("token expiry must be in the future")
	}
	if len(in.Scopes) == 0 || len(in.Scopes) > 32 || len(in.SourceCIDRs) > 32 {
		return TokenResult{}, gerror.New("token scope or source limit is too large")
	}
	seenScopes := make(map[string]struct{}, len(in.Scopes))
	for i, scope := range in.Scopes {
		in.Scopes[i] = strings.TrimSpace(scope)
		if _, supported := supportedTokenScopes[in.Scopes[i]]; !supported {
			return TokenResult{}, gerror.New("token scope is invalid")
		}
		if _, duplicate := seenScopes[in.Scopes[i]]; duplicate {
			return TokenResult{}, gerror.New("token scope is duplicated")
		}
		seenScopes[in.Scopes[i]] = struct{}{}
	}
	for i, source := range in.SourceCIDRs {
		in.SourceCIDRs[i] = strings.TrimSpace(source)
		if _, _, err := net.ParseCIDR(in.SourceCIDRs[i]); err != nil {
			return TokenResult{}, gerror.New("token source restriction is invalid")
		}
	}
	if _, err := s.requireChannel(ctx, in.ChannelID, true); err != nil {
		return TokenResult{}, err
	}
	buf := make([]byte, 32)
	if _, e := rand.Read(buf); e != nil {
		return TokenResult{}, e
	}
	plain := hex.EncodeToString(buf)
	h := sha256.Sum256([]byte(plain))
	scopes, err := json.Marshal(in.Scopes)
	if err != nil {
		return TokenResult{}, err
	}
	cidrs, err := json.Marshal(in.SourceCIDRs)
	if err != nil {
		return TokenResult{}, err
	}
	row := struct {
		ChannelID   string     `json:"channel_id"`
		TokenHash   string     `json:"token_hash"`
		Scopes      string     `json:"scopes"`
		SourceCIDRs string     `json:"source_cidrs"`
		RateLimit   int        `json:"rate_limit"`
		ExpiresAt   *time.Time `json:"expires_at"`
		CreatedBy   int64      `json:"created_by"`
	}{in.ChannelID, hex.EncodeToString(h[:]), string(scopes), string(cidrs), 60, in.ExpiresAt, s.actorID(ctx)}
	r, e := s.db.Model("plugin_idc_channel_token").Ctx(ctx).Data(row).InsertAndGetId()
	if e != nil {
		return TokenResult{}, e
	}
	return TokenResult{ID: r, Token: plain, ExpiresAt: in.ExpiresAt}, nil
}

// AuthenticateToken resolves one high-entropy bearer token, enforces its
// channel scope, source network, expiry, revocation and database-authoritative
// fixed-minute rate limit, then returns only the safe channel identity.
func (s *service) AuthenticateToken(
	ctx context.Context,
	rawToken string,
	clientIP string,
	requiredScope string,
) (TokenCredential, error) {
	rawToken = strings.TrimSpace(rawToken)
	requiredScope = strings.TrimSpace(requiredScope)
	if s == nil || s.db == nil || len(rawToken) != 64 {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	if _, supported := supportedTokenScopes[requiredScope]; !supported || requiredScope == TokenScopeAll {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	hash := sha256.Sum256([]byte(rawToken))
	var row struct {
		ID          int64      `json:"id"`
		ChannelID   string     `json:"channel_id"`
		Scopes      string     `json:"scopes"`
		SourceCIDRs string     `json:"source_cidrs"`
		RateLimit   int        `json:"rate_limit"`
		ExpiresAt   *time.Time `json:"expires_at"`
		RevokedAt   *time.Time `json:"revoked_at"`
	}
	if err := s.db.Model("plugin_idc_channel_token").Ctx(ctx).
		Fields("id,channel_id,scopes,source_cidrs,rate_limit,expires_at,revoked_at").
		Where("token_hash=?", hex.EncodeToString(hash[:])).Scan(&row); err != nil {
		return TokenCredential{}, err
	}
	if row.ID <= 0 || strings.TrimSpace(row.ChannelID) == "" || row.RevokedAt != nil ||
		(row.ExpiresAt != nil && !row.ExpiresAt.After(time.Now())) || row.RateLimit <= 0 {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	account, err := s.requireChannel(ctx, row.ChannelID, false)
	if err != nil || account.StatusCode == "pending_approval" ||
		(account.StatusCode == "paused" && !pausedChannelScopeAllowed(requiredScope)) {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	var scopes []string
	if err := json.Unmarshal([]byte(row.Scopes), &scopes); err != nil || !tokenScopeAllows(scopes, requiredScope) {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	var sourceCIDRs []string
	if err := json.Unmarshal([]byte(row.SourceCIDRs), &sourceCIDRs); err != nil || !tokenSourceAllows(sourceCIDRs, clientIP) {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	var requestCount int
	if err := s.db.GetScan(ctx, &requestCount, `
WITH accepted AS (
    INSERT INTO plugin_idc_channel_token_rate_window (
        token_id, window_started_at, request_count, updated_at
    ) VALUES (?, date_trunc('minute', CURRENT_TIMESTAMP), 1, CURRENT_TIMESTAMP)
    ON CONFLICT (token_id) DO UPDATE SET
        window_started_at = EXCLUDED.window_started_at,
        request_count = CASE
            WHEN plugin_idc_channel_token_rate_window.window_started_at < EXCLUDED.window_started_at THEN 1
            ELSE plugin_idc_channel_token_rate_window.request_count + 1
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE plugin_idc_channel_token_rate_window.window_started_at < EXCLUDED.window_started_at
       OR plugin_idc_channel_token_rate_window.request_count < ?
    RETURNING request_count
)
SELECT request_count FROM accepted`, row.ID, row.RateLimit); err != nil {
		return TokenCredential{}, err
	}
	if requestCount <= 0 || requestCount > row.RateLimit {
		return TokenCredential{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	return TokenCredential{TokenID: row.ID, ChannelID: strings.TrimSpace(row.ChannelID)}, nil
}

func pausedChannelScopeAllowed(scope string) bool {
	switch scope {
	case "capability.read", "order.read", "service.read", "settlement.read", "settlement.repay", "webhook.read":
		return true
	default:
		return false
	}
}

func tokenScopeAllows(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == TokenScopeAll || scope == required {
			return true
		}
	}
	return false
}

func tokenSourceAllows(cidrs []string, clientIP string) bool {
	if len(cidrs) == 0 {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	for _, raw := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
func (s *service) RevokeToken(ctx context.Context, id int64) error {
	r, e := s.db.Model("plugin_idc_channel_token").Ctx(ctx).Data(struct {
		RevokedAt time.Time `json:"revoked_at"`
	}{time.Now()}).Where("id=? AND revoked_at IS NULL", id).Update()
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return gerror.New("token not found or already revoked")
	}
	return nil
}
func (s *service) ListTokens(ctx context.Context, channelID string) ([]TokenSummary, error) {
	channelID = strings.TrimSpace(channelID)
	model := s.db.Model("plugin_idc_channel_token").Ctx(ctx).Order("id DESC").Limit(200)
	if channelID != "" {
		model = model.Where("channel_id=?", channelID)
	}
	return scanTokenSummaries(model)
}

func scanTokenSummaries(model *gdb.Model) ([]TokenSummary, error) {
	model = model.Fields("id,channel_id,scopes,source_cidrs,expires_at,revoked_at,created_at")
	var rows []struct {
		ID          int64      `json:"id"`
		ChannelID   string     `json:"channel_id"`
		Scopes      string     `json:"scopes"`
		SourceCIDRs string     `json:"source_cidrs"`
		ExpiresAt   *time.Time `json:"expires_at"`
		RevokedAt   *time.Time `json:"revoked_at"`
		CreatedAt   time.Time  `json:"created_at"`
	}
	if err := model.Scan(&rows); err != nil {
		return nil, err
	}
	result := make([]TokenSummary, 0, len(rows))
	for _, row := range rows {
		var scopes, cidrs []string
		if row.Scopes != "" {
			if err := json.Unmarshal([]byte(row.Scopes), &scopes); err != nil {
				return nil, err
			}
		}
		if row.SourceCIDRs != "" {
			if err := json.Unmarshal([]byte(row.SourceCIDRs), &cidrs); err != nil {
				return nil, err
			}
		}
		result = append(result, TokenSummary{ID: row.ID, ChannelID: row.ChannelID, Scopes: scopes, SourceCIDRs: cidrs, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, CreatedAt: row.CreatedAt})
	}
	return result, nil
}
func (s *service) Catalog(ctx context.Context, channelID string) ([]CatalogItem, error) {
	if _, err := s.requireChannel(ctx, channelID, true); err != nil {
		return nil, err
	}
	var rows []struct {
		ProductVersionID int64  `json:"product_version_id"`
		Currency         string `json:"currency"`
		Price            string `json:"price"`
		Stock            int    `json:"stock"`
	}
	e := s.db.Model("plugin_idc_channel_publication").Ctx(ctx).Fields("product_version_id,currency,price,stock").Where("channel_id=? AND status='published'", channelID).Order("id ASC").Scan(&rows)
	if e != nil {
		return nil, e
	}
	out := make([]CatalogItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, CatalogItem{r.ProductVersionID, r.Currency, r.Price, r.Stock})
	}
	return out, nil
}
func (s *service) Quote(ctx context.Context, in QuoteInput) (QuoteResult, error) {
	in = normalizeQuoteInput(in)
	if in.ChannelID == "" || in.ProductVersionID <= 0 || in.Currency == "" {
		return QuoteResult{}, gerror.New("channel, product version and currency are required")
	}
	account, err := s.requireChannel(ctx, in.ChannelID, true)
	if err != nil {
		return QuoteResult{}, err
	}
	publication, err := s.loadPublication(ctx, in.ChannelID, in.ProductVersionID, in.Currency)
	if err != nil {
		return QuoteResult{}, err
	}
	quote, err := s.core.Quote(ctx, channelcommercecap.QuoteInput{
		PayerCustomerNo: account.CustomerNo, Purchase: publication.Purchase(),
	})
	if err != nil {
		return QuoteResult{}, err
	}
	if quote.CurrencyCode != publication.Currency || quote.TotalAmountMinor != publication.PriceMinor {
		return QuoteResult{}, gerror.New("published channel price is stale")
	}
	return QuoteResult{
		ProductVersionID: publication.ProductVersionID, Currency: publication.Currency,
		Price: publication.Price, TotalAmountMinor: quote.TotalAmountMinor,
		Stock: publication.Stock, Status: publication.Status,
	}, nil
}

func normalizeQuoteInput(in QuoteInput) QuoteInput {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	return in
}

func normalizeOrderInput(in OrderInput) OrderInput {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.ExternalCustomerID = strings.TrimSpace(in.ExternalCustomerID)
	in.OwnershipMode = strings.TrimSpace(in.OwnershipMode)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	in.SettlementMethod = strings.ToLower(strings.TrimSpace(in.SettlementMethod))
	return in
}

func validateOrderInput(in OrderInput) error {
	if in.ChannelID == "" || in.RequestID == "" || len(in.RequestID) < 8 || len(in.RequestID) > 120 ||
		in.ProductVersionID <= 0 || in.Currency == "" ||
		(in.SettlementMethod != "" && in.SettlementMethod != "deferred" && in.SettlementMethod != "balance" && in.SettlementMethod != "credit") {
		return gerror.New("channel, request, product version and currency are required")
	}
	if (in.OwnershipMode != "channel_owned" && in.OwnershipMode != "delegated_customer") ||
		(in.OwnershipMode == "channel_owned" && (in.ExternalCustomerID != "" || in.DelegatedCustomer != nil)) ||
		(in.OwnershipMode == "delegated_customer" && in.ExternalCustomerID == "") {
		return gerror.New("unsupported ownership mode")
	}
	return nil
}

func (s *service) CreateOrder(ctx context.Context, in OrderInput) (OrderResult, error) {
	in = normalizeOrderInput(in)
	if err := validateOrderInput(in); err != nil {
		return OrderResult{}, err
	}
	requestHash := requestFingerprint(in)
	existing, er := s.db.Model("plugin_idc_channel_request").Ctx(ctx).Where("channel_id=? AND request_id=?", in.ChannelID, in.RequestID).One()
	if er != nil {
		return OrderResult{}, er
	}
	if existing != nil {
		if existing["request_hash"].String() != requestHash {
			return OrderResult{}, gerror.New("request id already used with different payload")
		}
		var replay struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal([]byte(existing["response"].String()), &replay)
		return OrderResult{ID: replay.ID, Status: existing["status"].String(), Replayed: true}, nil
	}
	account, err := s.requireChannel(ctx, in.ChannelID, true)
	if err != nil {
		return OrderResult{}, err
	}
	publication, err := s.loadPublication(ctx, in.ChannelID, in.ProductVersionID, in.Currency)
	if err != nil {
		return OrderResult{}, err
	}
	serviceOwner := account.CustomerNo
	if in.OwnershipMode == "delegated_customer" {
		serviceOwner, err = s.resolveDelegatedCustomer(ctx, in)
		if err != nil {
			return OrderResult{}, err
		}
	}
	coreOrder, err := s.core.CreateOrder(ctx, channelcommercecap.CreateOrderInput{
		ChannelID: in.ChannelID, ExternalCustomerID: in.ExternalCustomerID,
		OwnershipMode: in.OwnershipMode, PayerCustomerNo: account.CustomerNo,
		ServiceOwnerCustomerNo: serviceOwner, IdempotencyKey: in.RequestID,
		SettlementMethod: in.SettlementMethod, Purchase: publication.Purchase(),
	})
	if err != nil {
		return OrderResult{}, err
	}
	if coreOrder.CurrencyCode != publication.Currency || coreOrder.TotalAmountMinor != publication.PriceMinor {
		return OrderResult{}, gerror.New("core order does not match the published channel price")
	}
	var id int64
	e := s.db.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		id, err = tx.Model("plugin_idc_channel_order").Ctx(txCtx).Data(g.Map{
			"channel_id": in.ChannelID, "external_customer_id": in.ExternalCustomerID,
			"ownership_mode": in.OwnershipMode, "product_version_id": in.ProductVersionID,
			"currency": in.Currency, "amount": publication.Price, "status": coreOrder.Status,
			"financially_settled": coreOrder.FinanciallySettled, "core_order_no": coreOrder.OrderNo,
			"core_invoice_no": coreOrder.InvoiceNo, "payer_customer_no": coreOrder.PayerCustomerNo,
			"service_owner_customer_no": coreOrder.ServiceOwnerCustomerNo,
			"total_amount_minor":        coreOrder.TotalAmountMinor, "identity_ready": coreOrder.IdentityReady,
			"updated_at": time.Now(),
		}).InsertAndGetId()
		if err != nil {
			return err
		}
		payload, marshalErr := json.Marshal(struct {
			ID int64 `json:"id"`
		}{id})
		if marshalErr != nil {
			return marshalErr
		}
		_, err = tx.Model("plugin_idc_channel_request").Ctx(txCtx).Data(struct {
			ChannelID   string `json:"channel_id"`
			RequestID   string `json:"request_id"`
			Operation   string `json:"operation"`
			RequestHash string `json:"request_hash"`
			Response    string `json:"response"`
			Status      string `json:"status"`
		}{in.ChannelID, in.RequestID, "create_order", requestHash, string(payload), coreOrder.Status}).Insert()
		return err
	})
	if e != nil {
		// A concurrent caller may have won the unique request key. Re-read it and
		// return a replay only when the payload is byte-for-byte equivalent.
		if row, readErr := s.db.Model("plugin_idc_channel_request").Ctx(ctx).Where("channel_id=? AND request_id=?", in.ChannelID, in.RequestID).One(); readErr == nil && row != nil {
			if row["request_hash"].String() != requestHash {
				return OrderResult{}, gerror.New("request id already used with different payload")
			}
			var replay struct {
				ID int64 `json:"id"`
			}
			if json.Unmarshal([]byte(row["response"].String()), &replay) == nil && replay.ID > 0 {
				return OrderResult{ID: replay.ID, Status: row["status"].String(), Replayed: true}, nil
			}
		}
		return OrderResult{}, e
	}
	return OrderResult{ID: id, Status: coreOrder.Status, Replayed: coreOrder.Reused}, nil
}

func (s *service) CancelOrder(ctx context.Context, in CancelInput) (OrderResult, error) {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.RequestID = strings.TrimSpace(in.RequestID)
	if in.ChannelID == "" || in.RequestID == "" || in.OrderID <= 0 {
		return OrderResult{}, gerror.New("channel, request and order are required")
	}
	requestHash := cancelFingerprint(in)
	if existing, err := s.db.Model("plugin_idc_channel_request").Ctx(ctx).Where("channel_id=? AND request_id=?", in.ChannelID, in.RequestID).One(); err != nil {
		return OrderResult{}, err
	} else if existing != nil {
		if existing["request_hash"].String() != requestHash {
			return OrderResult{}, gerror.New("request id already used with different payload")
		}
		var replay OrderResult
		if json.Unmarshal([]byte(existing["response"].String()), &replay) == nil {
			replay.Replayed = true
			return replay, nil
		}
		return OrderResult{}, gerror.New("request evidence is invalid")
	}
	local, err := s.localOrder(ctx, in.OrderID, in.ChannelID)
	if err != nil {
		return OrderResult{}, err
	}
	coreOrder, err := s.core.CancelOrder(ctx, channelcommercecap.CancelOrderInput{
		ChannelID: in.ChannelID, OrderNo: local.CoreOrderNo, IdempotencyKey: in.RequestID,
	})
	if err != nil {
		return OrderResult{}, err
	}
	var out = OrderResult{ID: in.OrderID, Status: coreOrder.Status, Replayed: coreOrder.Reused}
	err = s.db.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if err := syncLocalOrder(txCtx, tx, in.OrderID, coreOrder); err != nil {
			return err
		}
		payload, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return marshalErr
		}
		_, err = tx.Model("plugin_idc_channel_request").Ctx(txCtx).Data(g.Map{"channel_id": in.ChannelID, "request_id": in.RequestID, "operation": "cancel_order", "request_hash": requestHash, "response": string(payload), "status": "accepted"}).Insert()
		return err
	})
	if err != nil {
		if row, readErr := s.db.Model("plugin_idc_channel_request").Ctx(ctx).Where("channel_id=? AND request_id=?", in.ChannelID, in.RequestID).One(); readErr == nil && row != nil && row["request_hash"].String() == requestHash {
			var replay OrderResult
			if json.Unmarshal([]byte(row["response"].String()), &replay) == nil {
				replay.Replayed = true
				return replay, nil
			}
		}
		return OrderResult{}, err
	}
	return out, nil
}

func (s *service) SettleOrder(ctx context.Context, in SettleInput) (OrderResult, error) {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.SettlementMethod = strings.ToLower(strings.TrimSpace(in.SettlementMethod))
	if in.ChannelID == "" || in.OrderID <= 0 || len(in.RequestID) < 8 || len(in.RequestID) > 120 ||
		(in.SettlementMethod != "balance" && in.SettlementMethod != "credit") {
		return OrderResult{}, gerror.New("channel settlement request is invalid")
	}
	fingerprint := requestFingerprint(struct {
		OrderID          int64  `json:"order_id"`
		SettlementMethod string `json:"settlement_method"`
	}{in.OrderID, in.SettlementMethod})
	if replay, ok, err := s.replayOrderRequest(ctx, in.ChannelID, in.RequestID, fingerprint); err != nil || ok {
		return replay, err
	}
	local, err := s.localOrder(ctx, in.OrderID, in.ChannelID)
	if err != nil {
		return OrderResult{}, err
	}
	coreOrder, err := s.core.SettleOrder(ctx, channelcommercecap.SettleOrderInput{
		ChannelID: in.ChannelID, OrderNo: local.CoreOrderNo,
		IdempotencyKey: in.RequestID, SettlementMethod: in.SettlementMethod,
	})
	if err != nil {
		return OrderResult{}, err
	}
	out := OrderResult{ID: in.OrderID, Status: coreOrder.Status, Replayed: coreOrder.Reused}
	err = s.db.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if err := syncLocalOrder(txCtx, tx, in.OrderID, coreOrder); err != nil {
			return err
		}
		payload, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return marshalErr
		}
		_, insertErr := tx.Model("plugin_idc_channel_request").Ctx(txCtx).Data(g.Map{
			"channel_id": in.ChannelID, "request_id": in.RequestID, "operation": "settle_order",
			"request_hash": fingerprint, "response": string(payload), "status": coreOrder.Status,
			"result_code": "accepted", "completed_at": time.Now(),
		}).Insert()
		return insertErr
	})
	if err != nil {
		if replay, ok, readErr := s.replayOrderRequest(ctx, in.ChannelID, in.RequestID, fingerprint); readErr == nil && ok {
			return replay, nil
		}
		return OrderResult{}, err
	}
	return out, nil
}

func requestFingerprint(in any) string {
	b, err := json.Marshal(in)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func cancelFingerprint(in CancelInput) string {
	b, err := json.Marshal(struct {
		OrderID int64 `json:"order_id"`
	}{in.OrderID})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}
func (s *service) GetOrder(ctx context.Context, id int64, channelID string) (Order, error) {
	channelID = strings.TrimSpace(channelID)
	if id <= 0 || channelID == "" {
		return Order{}, gerror.New("order not found")
	}
	// Channel scoping is part of the lookup, so guessing an order id from a
	// different channel produces the same not-found result and never leaks
	// whether that order exists.
	local, err := s.localOrder(ctx, id, channelID)
	if err != nil {
		return Order{}, err
	}
	coreOrder, err := s.coreOrder(ctx, channelID, local.CoreOrderNo)
	if err != nil {
		return Order{}, err
	}
	_, err = s.db.Model("plugin_idc_channel_order").Ctx(ctx).Data(g.Map{
		"status": coreOrder.Status, "financially_settled": coreOrder.FinanciallySettled,
		"identity_ready": coreOrder.IdentityReady, "updated_at": time.Now(),
	}).Where("id=? AND channel_id=?", id, channelID).Update()
	if err != nil {
		return Order{}, err
	}
	return projectLocalOrder(local, coreOrder), nil
}

// Lifecycle exposes the versioned lifecycle envelope while keeping customer
// ownership, settlement, business-operation and Provider-task truth in core.
// This edge validates the channel state and never accepts a caller-supplied
// payer or service owner.
func (s *service) Lifecycle(ctx context.Context, in LifecycleInput) (LifecycleResult, error) {
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.ServiceNo = strings.TrimSpace(in.ServiceNo)
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.Operation = strings.ToLower(strings.TrimSpace(in.Operation))
	if in.ChannelID == "" || in.ServiceNo == "" || in.RequestID == "" ||
		len(in.RequestID) < 8 || len(in.RequestID) > 120 || in.RequestID != strings.TrimSpace(in.RequestID) {
		return LifecycleResult{}, bizerr.NewCode(codeLifecycleInvalid)
	}
	switch in.Operation {
	case "renew", "reconfigure", "reinstall", "console", "change_ip", "terminate":
		account, channelErr := s.requireChannel(ctx, in.ChannelID, false)
		if channelErr != nil {
			return LifecycleResult{}, channelErr
		}
		if account.StatusCode != "approved" {
			return LifecycleResult{}, gerror.New("paused channel cannot submit lifecycle operations")
		}
		result, err := s.core.Lifecycle(ctx, channelcommercecap.LifecycleInput{
			ChannelID: in.ChannelID, ServiceNo: in.ServiceNo, Operation: in.Operation,
			IdempotencyKey: in.RequestID, Parameters: in.Parameters, Confirmed: in.Confirmed,
		})
		if err != nil {
			return LifecycleResult{}, err
		}
		return LifecycleResult{OperationNo: result.OperationNo, QuoteNo: result.QuoteNo,
			Status: result.Status, CurrencyCode: result.CurrencyCode, Direction: result.Direction,
			AmountMinor: result.AmountMinor, EffectiveAt: result.EffectiveAt, ExpiresAt: result.ExpiresAt,
			PaymentRequired: result.PaymentRequired, OutputAvailable: result.OutputAvailable,
			Replayed: result.Reused}, nil
	default:
		return LifecycleResult{}, bizerr.NewCode(codeLifecycleInvalid)
	}
}

func (s *service) RedeemLifecycleOutput(
	ctx context.Context,
	channelID string,
	serviceNo string,
	operationNo string,
	consume channelcommercecap.ConfidentialOutputConsumer,
) error {
	account, err := s.requireChannel(ctx, strings.TrimSpace(channelID), false)
	if err != nil || account.StatusCode != "approved" {
		return bizerr.NewCode(codeLifecycleInvalid)
	}
	return s.core.RedeemLifecycleOutput(ctx, account.ChannelID,
		strings.TrimSpace(serviceNo), strings.TrimSpace(operationNo), consume)
}

func (s *service) Capabilities(context.Context) Capabilities {
	return Capabilities{
		GenericAPI: true, WebhookDelivery: true,
		Adapters: map[string]string{"zjmf-finance": "disabled_unverified", "v10": "disabled_unverified", "whmcs": "disabled_unverified", "linapro": "generic_api_v1"},
		Operations: map[string]string{
			"catalog":      "enabled",
			"quote":        "enabled",
			"create_order": "enabled",
			"settle_order": "enabled_balance_credit",
			"cancel_order": "enabled_core_guarded",
			"renew":        "enabled_core_guarded",
			"reconfigure":  "enabled_core_guarded",
			"reinstall":    "enabled_core_guarded",
			"console":      "enabled_core_guarded",
			"change_ip":    "enabled_core_guarded",
			"terminate":    "enabled_core_guarded",
		},
	}
}

func (s *service) Finance(ctx context.Context, channelID string, pages ...channelcommercecap.FinancePages) (channelcommercecap.FinanceOverview, error) {
	account, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return channelcommercecap.FinanceOverview{}, err
	}
	result, err := s.core.FinanceOverview(ctx, strings.TrimSpace(channelID), account.CustomerNo, pages...)
	if err != nil {
		return channelcommercecap.FinanceOverview{}, err
	}
	return *result, nil
}

func (s *service) RepayCredit(
	ctx context.Context,
	input CreditRepaymentInput,
) (channelcommercecap.RepayCreditResult, error) {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.StatementNo = strings.TrimSpace(input.StatementNo)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.ChannelID == "" || input.StatementNo == "" || input.AmountMinor <= 0 ||
		len(input.RequestID) < 8 || len(input.RequestID) > 120 {
		return channelcommercecap.RepayCreditResult{}, gerror.New("credit repayment request is invalid")
	}
	account, err := s.requireChannel(ctx, input.ChannelID, false)
	if err != nil {
		return channelcommercecap.RepayCreditResult{}, err
	}
	result, err := s.core.RepayCredit(ctx, channelcommercecap.RepayCreditInput{
		ChannelID: input.ChannelID, PayerCustomerNo: account.CustomerNo,
		StatementNo: input.StatementNo, AmountMinor: input.AmountMinor,
		IdempotencyKey: input.RequestID,
	})
	if err != nil {
		return channelcommercecap.RepayCreditResult{}, err
	}
	return *result, nil
}

func (s *service) ListServices(
	ctx context.Context,
	channelID string,
	serviceNo string,
) ([]channelcommercecap.ServiceItem, error) {
	account, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	return s.core.ListServices(ctx, channelcommercecap.ServiceListInput{
		ChannelID: strings.TrimSpace(channelID), PayerCustomerNo: account.CustomerNo,
		ServiceNo: strings.TrimSpace(serviceNo), Limit: 100,
	})
}

type channelAccountRow struct {
	ChannelID      string `orm:"channel_id"`
	CustomerNo     string `orm:"customer_no"`
	DisplayName    string `orm:"display_name"`
	StatusCode     string `orm:"status_code"`
	NewSalesEnable bool   `orm:"new_sales_enabled"`
}

type publicationRow struct {
	ProductVersionID      int64  `orm:"product_version_id"`
	Currency              string `orm:"currency"`
	Price                 string `orm:"price"`
	PriceMinor            int64  `orm:"price_minor"`
	Stock                 int    `orm:"stock"`
	Status                string `orm:"status"`
	PublicSlug            string `orm:"public_slug"`
	ConfigurationRevision string `orm:"configuration_revision"`
	BillingTermToken      string `orm:"billing_term_token"`
	PurchaseCycles        int    `orm:"purchase_cycles"`
	Quantity              int    `orm:"quantity"`
	PurchaseValues        string `orm:"purchase_values"`
}

func (row publicationRow) Purchase() channelcommercecap.Purchase {
	var values []channelcommercecap.PurchaseValue
	_ = json.Unmarshal([]byte(row.PurchaseValues), &values)
	return channelcommercecap.Purchase{
		PublicSlug: row.PublicSlug, ConfigurationRevision: row.ConfigurationRevision,
		BillingTermToken: row.BillingTermToken, CurrencyCode: row.Currency,
		PurchaseCycles: row.PurchaseCycles, Quantity: row.Quantity, Values: values,
	}
}

type localOrderRow struct {
	ID                     int64  `orm:"id"`
	ChannelID              string `orm:"channel_id"`
	OwnershipMode          string `orm:"ownership_mode"`
	Currency               string `orm:"currency"`
	CoreOrderNo            string `orm:"core_order_no"`
	PayerCustomerNo        string `orm:"payer_customer_no"`
	ServiceOwnerCustomerNo string `orm:"service_owner_customer_no"`
}

func (s *service) requireChannel(ctx context.Context, channelID string, newSales bool) (*channelAccountRow, error) {
	row, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if row.StatusCode == "closed" ||
		(newSales && (row.StatusCode != "approved" || !row.NewSalesEnable)) {
		return nil, gerror.New("channel is not available")
	}
	return row, nil
}

func (s *service) loadChannel(ctx context.Context, channelID string) (*channelAccountRow, error) {
	channelID = strings.TrimSpace(channelID)
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || channelID == "" {
		return nil, gerror.New("channel is not available")
	}
	var row *channelAccountRow
	if err := s.db.Model("plugin_idc_channel_account").Ctx(ctx).
		Fields("channel_id,customer_no,display_name,status_code,new_sales_enabled").
		Where("channel_id=?", channelID).Scan(&row); err != nil {
		return nil, err
	}
	if row == nil || row.ChannelID == "" {
		return nil, gerror.New("channel is not available")
	}
	return row, nil
}

func (s *service) loadPublication(ctx context.Context, channelID string, productVersionID int64, currency string) (*publicationRow, error) {
	var row *publicationRow
	err := s.db.Model("plugin_idc_channel_publication").Ctx(ctx).
		Fields("product_version_id,currency,price,price_minor,stock,status,public_slug,configuration_revision,billing_term_token,purchase_cycles,quantity,purchase_values").
		Where("channel_id=? AND product_version_id=? AND currency=? AND status='published'", channelID, productVersionID, currency).Scan(&row)
	if err != nil {
		return nil, err
	}
	if row == nil || row.ProductVersionID <= 0 || row.PublicSlug == "" || row.ConfigurationRevision == "" || row.BillingTermToken == "" {
		return nil, gerror.New("published product not found")
	}
	var values []channelcommercecap.PurchaseValue
	if json.Unmarshal([]byte(row.PurchaseValues), &values) != nil {
		return nil, gerror.New("published product configuration is invalid")
	}
	return row, nil
}

func (s *service) resolveDelegatedCustomer(ctx context.Context, in OrderInput) (string, error) {
	var existing string
	if err := s.db.GetScan(ctx, &existing, `SELECT customer_no FROM plugin_idc_channel_external_customer WHERE channel_id=? AND external_customer_id=?`, in.ChannelID, in.ExternalCustomerID); err != nil {
		return "", err
	}
	if existing != "" {
		return existing, nil
	}
	if in.DelegatedCustomer == nil {
		return "", gerror.New("delegated customer details are required for the first order")
	}
	input := in.DelegatedCustomer
	created, err := s.core.CreateDelegatedCustomer(ctx, channelcommercecap.DelegatedCustomerInput{
		ChannelID: in.ChannelID, ExternalID: in.ExternalCustomerID,
		CustomerType: input.CustomerType, DisplayName: input.DisplayName, CountryCode: input.CountryCode,
		PreferredLocale: input.PreferredLocale, Email: input.Email, PhoneCountryCode: input.PhoneCountryCode,
		Phone: input.Phone, IdempotencyKey: in.RequestID,
	})
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO plugin_idc_channel_external_customer(channel_id,external_customer_id,customer_no,invitation_no,status_code,identity_status_code) VALUES(?,?,?,?,?,?) ON CONFLICT(channel_id,external_customer_id) DO NOTHING`, in.ChannelID, in.ExternalCustomerID, created.CustomerNo, created.InvitationNo, created.Status, created.IdentityStatus)
	if err != nil {
		return "", err
	}
	if err = s.db.GetScan(ctx, &existing, `SELECT customer_no FROM plugin_idc_channel_external_customer WHERE channel_id=? AND external_customer_id=?`, in.ChannelID, in.ExternalCustomerID); err != nil {
		return "", err
	}
	if existing != created.CustomerNo {
		return "", gerror.New("external customer id is already bound")
	}
	return existing, nil
}

func (s *service) localOrder(ctx context.Context, id int64, channelID string) (*localOrderRow, error) {
	var row *localOrderRow
	if err := s.db.Model("plugin_idc_channel_order").Ctx(ctx).
		Fields("id,channel_id,ownership_mode,currency,core_order_no,payer_customer_no,service_owner_customer_no").
		Where("id=? AND channel_id=?", id, channelID).Scan(&row); err != nil {
		return nil, err
	}
	if row == nil || row.CoreOrderNo == "" {
		return nil, gerror.New("order not found")
	}
	return row, nil
}

func (s *service) coreOrder(ctx context.Context, channelID, orderNo string) (*channelcommercecap.Order, error) {
	return s.core.GetOrder(ctx, channelID, orderNo)
}

func syncLocalOrder(ctx context.Context, tx gdb.TX, id int64, order *channelcommercecap.Order) error {
	result, err := tx.Model("plugin_idc_channel_order").Ctx(ctx).Data(g.Map{
		"status": order.Status, "financially_settled": order.FinanciallySettled,
		"identity_ready": order.IdentityReady, "updated_at": time.Now(),
	}).Where("id=? AND core_order_no=?", id, order.OrderNo).Update()
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return gerror.New("channel order projection changed concurrently")
	}
	return nil
}

func projectLocalOrder(local *localOrderRow, core *channelcommercecap.Order) Order {
	return Order{
		ID: local.ID, ChannelID: local.ChannelID, OwnershipMode: local.OwnershipMode,
		Currency: local.Currency, Status: core.Status, CoreOrderNo: core.OrderNo,
		PayerCustomerNo: core.PayerCustomerNo, ServiceOwnerCustomerNo: core.ServiceOwnerCustomerNo,
		TotalAmountMinor: core.TotalAmountMinor, FinanciallySettled: core.FinanciallySettled,
		IdentityReady: core.IdentityReady,
	}
}

func (s *service) replayOrderRequest(ctx context.Context, channelID, requestID, fingerprint string) (OrderResult, bool, error) {
	row, err := s.db.Model("plugin_idc_channel_request").Ctx(ctx).Where("channel_id=? AND request_id=?", channelID, requestID).One()
	if err != nil || row == nil {
		return OrderResult{}, false, err
	}
	if row["request_hash"].String() != fingerprint {
		return OrderResult{}, false, gerror.New("request id already used with different payload")
	}
	var result OrderResult
	if err = json.Unmarshal([]byte(row["response"].String()), &result); err != nil {
		return OrderResult{}, false, gerror.New("request evidence is invalid")
	}
	result.Replayed = true
	return result, true, nil
}
