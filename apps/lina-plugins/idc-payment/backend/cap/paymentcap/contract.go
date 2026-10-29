// Package paymentcap defines the payment plugin's provider-neutral adapter contract.
package paymentcap

import (
	"context"
	"net"
	"net/url"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

type ProviderCode string
type Mode string
type Operation string

const (
	ProviderAlipay    ProviderCode = "alipay"
	ProviderWechatPay ProviderCode = "wechatpay"
	ProviderEPay      ProviderCode = "epay"
	ProviderUnionPay  ProviderCode = "unionpay"

	ModeAlipayPage   Mode = "alipay_page"
	ModeAlipayWAP    Mode = "alipay_wap"
	ModeWechatH5     Mode = "wechat_h5"
	ModeWechatNative Mode = "wechat_native"
	ModeWechatJSAPI  Mode = "wechat_jsapi"
	ModeEPayCashier  Mode = "epay_cashier"
	ModeEPayAlipay   Mode = "epay_alipay"
	ModeEPayWechat   Mode = "epay_wechat"
	ModeUnionPayWeb  Mode = "unionpay_web"
	ModeUnionPayWAP  Mode = "unionpay_wap"

	OperationCreate         Operation = "create"
	OperationCallback       Operation = "callback"
	OperationQuery          Operation = "query"
	OperationClose          Operation = "close"
	OperationRefund         Operation = "refund"
	OperationRefundCallback Operation = "refund_callback"
	OperationRefundQuery    Operation = "refund_query"
	OperationStatement      Operation = "statement"
)

var (
	ErrUnsupported     = gerror.New("payment capability is not verified")
	businessRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)
	currencyPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
)

// Capabilities are frozen when a channel is verified. Runtime code may not
// infer an unavailable operation from another provider or product mode.
type Capabilities struct {
	Modes      map[Mode]bool
	Currencies map[string]bool
	Operations map[Operation]bool
}

func (c Capabilities) Supports(mode Mode, currency string, operation Operation) bool {
	return c.Modes[mode] && c.Currencies[currency] && c.Operations[operation]
}

// Credentials are supplied only for one adapter call after resolving an
// opaque secret handle. Adapters must not persist, return, or log these bytes.
type Credentials struct {
	MerchantID string
	AppID      string
	Secret     []byte
	PrivateKey []byte
	PublicKey  []byte
	Extra      map[string][]byte
}

type CreateRequest struct {
	AttemptNo   string
	InvoiceNo   string
	Mode        Mode
	Currency    string
	AmountMinor int64
	Subject     string
	NotifyURL   string
	ReturnURL   string
	ClientIP    string
	OpenID      string
	ExpiresAt   time.Time
}

type Redirect struct {
	Method string
	URL    string
	Fields map[string]string
}

type CallbackRequest struct {
	Values  map[string]string
	Body    []byte
	Headers map[string]string
}

type VerifiedResult struct {
	ProviderEventReference string
	ExternalTradeReference string
	AttemptNo              string
	Currency               string
	AmountMinor            int64
	Status                 string
	EvidenceDigest         string
}

type QueryRequest struct{ AttemptNo, ExternalTradeReference string }
type CloseRequest struct{ AttemptNo, ExternalTradeReference string }
type RefundRequest struct {
	RefundAttemptNo, ExternalTradeReference, Currency string
	AmountMinor, TotalAmountMinor                     int64
	NotifyURL                                         string
}

// RefundQueryRequest identifies a provider refund and its original transaction.
type RefundQueryRequest struct {
	RefundAttemptNo, OriginalTradeReference, ExternalRefundReference string
}
type StatementRequest struct{ BusinessDate time.Time }
type StatementFile struct {
	DownloadURL, HashType, HashValue, EvidenceDigest string
}

type Adapter interface {
	Provider() ProviderCode
	Capabilities() Capabilities
	Create(context.Context, Credentials, CreateRequest) (*Redirect, error)
	VerifyCallback(context.Context, Credentials, CallbackRequest) (*VerifiedResult, error)
	Query(context.Context, Credentials, QueryRequest) (*VerifiedResult, error)
	Close(context.Context, Credentials, CloseRequest) error
	Refund(context.Context, Credentials, RefundRequest) (*VerifiedResult, error)
	VerifyRefundCallback(context.Context, Credentials, CallbackRequest) (*VerifiedResult, error)
	QueryRefund(context.Context, Credentials, RefundQueryRequest) (*VerifiedResult, error)
	Statement(context.Context, Credentials, StatementRequest) (*StatementFile, error)
}

// EndpointBoundAdapter is implemented only by providers whose documented
// cashier endpoint is merchant-configurable. The returned adapter is scoped
// to one verified channel and must not mutate the shared registry instance.
type EndpointBoundAdapter interface {
	WithEndpoint(string) (Adapter, error)
}

type CredentialValidator interface {
	ValidateCredentials(Credentials) error
}

// ConnectionVerifier performs one bounded, read-only upstream request. A
// result proves only the provider-specific checks documented by its adapter.
// EPay can prove endpoint reachability and credential shape only. This method
// must never create a charge or mutate an order.
type ConnectionVerifier interface {
	VerifyConnection(context.Context, Credentials) error
}

func ValidateCreateRequest(input CreateRequest) error {
	if !businessRefPattern.MatchString(input.AttemptNo) ||
		!businessRefPattern.MatchString(input.InvoiceNo) ||
		!currencyPattern.MatchString(input.Currency) || input.AmountMinor <= 0 ||
		input.Subject == "" || input.ExpiresAt.IsZero() {
		return gerror.New("invalid payment request")
	}
	for index, raw := range []string{input.NotifyURL, input.ReturnURL} {
		parsed, err := url.ParseRequestURI(raw)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return gerror.New("payment callback or return URL is invalid")
		}
		if parsed.Scheme == "https" {
			continue
		}
		// Browser returns may use the configured local development customer
		// origin. Server notifications always require HTTPS.
		address := net.ParseIP(parsed.Hostname())
		if index != 1 || parsed.Scheme != "http" || parsed.Hostname() != "localhost" && (address == nil || !address.IsLoopback()) {
			return gerror.New("payment notification requires HTTPS and browser returns require HTTPS or loopback HTTP")
		}
	}
	return nil
}

func ValidateRefundQueryRequest(input RefundQueryRequest) error {
	if !businessRefPattern.MatchString(input.RefundAttemptNo) ||
		len(input.OriginalTradeReference) > 256 || len(input.ExternalRefundReference) > 256 {
		return gerror.New("invalid refund query request")
	}
	return nil
}

func CloneCapabilities(value Capabilities) Capabilities {
	result := Capabilities{Modes: map[Mode]bool{}, Currencies: map[string]bool{}, Operations: map[Operation]bool{}}
	for key, enabled := range value.Modes {
		result.Modes[key] = enabled
	}
	for key, enabled := range value.Currencies {
		result.Currencies[key] = enabled
	}
	for key, enabled := range value.Operations {
		result.Operations[key] = enabled
	}
	return result
}
