// Package wechatpay wraps the official WeChat Pay APIv3 Go SDK.
package wechatpay

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	wxcore "github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/h5"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

const (
	extraCertificateSerial = "certificate_serial_no"
	extraPublicKeyID       = "wechatpay_public_key_id"
	extraAPIV3Key          = "api_v3_key"
)

type Adapter struct {
	httpClient *http.Client
	now        func() time.Time
}

var _ paymentcap.Adapter = (*Adapter)(nil)
var _ paymentcap.ConnectionVerifier = (*Adapter)(nil)

func New(client *http.Client) *Adapter {
	if client == nil {
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 15 * time.Second}
	}
	return &Adapter{httpClient: client, now: time.Now}
}
func (*Adapter) Provider() paymentcap.ProviderCode { return paymentcap.ProviderWechatPay }

// VerifyConnection performs a signed read-only order lookup. The official
// ORDER_NOT_EXIST response is the expected successful probe result: it proves
// that the merchant signature was accepted without creating a transaction.
func (a *Adapter) VerifyConnection(ctx context.Context, credentials paymentcap.Credentials) error {
	client, _, err := a.client(ctx, credentials)
	if err != nil {
		return err
	}
	mchid := credentials.MerchantID
	probe := "LINA_CONNECTION_CHECK_" + a.now().UTC().Format("20060102150405")
	_, _, err = (&native.NativeApiService{Client: client}).QueryOrderByOutTradeNo(
		ctx,
		native.QueryOrderByOutTradeNoRequest{OutTradeNo: &probe, Mchid: &mchid},
	)
	if err == nil || wxcore.IsAPIError(err, "ORDER_NOT_EXIST") || wxcore.IsAPIError(err, "ORDERNOTEXIST") {
		return nil
	}
	return err
}
func (*Adapter) ValidateCredentials(credentials paymentcap.Credentials) error {
	if credentials.MerchantID == "" || credentials.AppID == "" {
		return gerror.New("wechat pay merchant or app is missing")
	}
	if _, err := utils.LoadPrivateKey(string(credentials.PrivateKey)); err != nil {
		return gerror.New("wechat pay private key is invalid")
	}
	if _, err := utils.LoadPublicKey(string(credentials.PublicKey)); err != nil {
		return gerror.New("wechat pay public key is invalid")
	}
	if string(credentials.Extra[extraCertificateSerial]) == "" || string(credentials.Extra[extraPublicKeyID]) == "" || len(credentials.Extra[extraAPIV3Key]) != 32 {
		return gerror.New("wechat pay certificate or APIv3 key is invalid")
	}
	return nil
}
func (*Adapter) Capabilities() paymentcap.Capabilities {
	return paymentcap.Capabilities{
		Modes:      map[paymentcap.Mode]bool{paymentcap.ModeWechatNative: true, paymentcap.ModeWechatH5: true, paymentcap.ModeWechatJSAPI: true},
		Currencies: map[string]bool{"CNY": true},
		Operations: map[paymentcap.Operation]bool{paymentcap.OperationCreate: true, paymentcap.OperationCallback: true, paymentcap.OperationQuery: true, paymentcap.OperationClose: true, paymentcap.OperationRefund: true, paymentcap.OperationRefundCallback: true, paymentcap.OperationRefundQuery: true, paymentcap.OperationStatement: true},
	}
}

func (a *Adapter) Create(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
	if err := paymentcap.ValidateCreateRequest(input); err != nil {
		return nil, err
	}
	if input.Currency != "CNY" {
		return nil, paymentcap.ErrUnsupported
	}
	client, privateKey, err := a.client(ctx, credentials)
	if err != nil {
		return nil, err
	}
	description := truncate(input.Subject, 127)
	mchid := credentials.MerchantID
	appid := credentials.AppID
	amount := input.AmountMinor
	currency := "CNY"
	attempt := input.AttemptNo
	notifyURL := input.NotifyURL
	expires := input.ExpiresAt
	switch input.Mode {
	case paymentcap.ModeWechatNative:
		response, _, callErr := (&native.NativeApiService{Client: client}).Prepay(ctx, native.PrepayRequest{Appid: &appid, Mchid: &mchid, Description: &description, OutTradeNo: &attempt, TimeExpire: &expires, NotifyUrl: &notifyURL, Amount: &native.Amount{Total: &amount, Currency: &currency}})
		if callErr != nil {
			return nil, callErr
		}
		if response == nil || response.CodeUrl == nil || *response.CodeUrl == "" {
			return nil, gerror.New("WeChat Pay did not return a QR code")
		}
		return &paymentcap.Redirect{Method: "QR", URL: *response.CodeUrl, Fields: map[string]string{}}, nil
	case paymentcap.ModeWechatH5:
		if strings.TrimSpace(input.ClientIP) == "" {
			return nil, gerror.New("WeChat Pay H5 request is missing the client network address")
		}
		sceneType := "Wap"
		appName := "LinaPro IDC"
		appURL := input.ReturnURL
		response, _, callErr := (&h5.H5ApiService{Client: client}).Prepay(ctx, h5.PrepayRequest{Appid: &appid, Mchid: &mchid, Description: &description, OutTradeNo: &attempt, TimeExpire: &expires, NotifyUrl: &notifyURL, Amount: &h5.Amount{Total: &amount, Currency: &currency}, SceneInfo: &h5.SceneInfo{PayerClientIp: &input.ClientIP, H5Info: &h5.H5Info{Type: &sceneType, AppName: &appName, AppUrl: &appURL}}})
		if callErr != nil {
			return nil, callErr
		}
		if response == nil || response.H5Url == nil || *response.H5Url == "" {
			return nil, gerror.New("WeChat Pay did not return an H5 URL")
		}
		return &paymentcap.Redirect{Method: "GET", URL: *response.H5Url, Fields: map[string]string{"return_url": input.ReturnURL}}, nil
	case paymentcap.ModeWechatJSAPI:
		if strings.TrimSpace(input.OpenID) == "" {
			return nil, gerror.New("WeChat Pay JSAPI request is missing the current OpenID")
		}
		response, _, callErr := (&jsapi.JsapiApiService{Client: client}).Prepay(ctx, jsapi.PrepayRequest{Appid: &appid, Mchid: &mchid, Description: &description, OutTradeNo: &attempt, TimeExpire: &expires, NotifyUrl: &notifyURL, Amount: &jsapi.Amount{Total: &amount, Currency: &currency}, Payer: &jsapi.Payer{Openid: &input.OpenID}})
		if callErr != nil {
			return nil, callErr
		}
		if response == nil || response.PrepayId == nil || *response.PrepayId == "" {
			return nil, gerror.New("WeChat Pay did not return a prepay reference")
		}
		nonce, err := utils.GenerateNonce()
		if err != nil {
			return nil, err
		}
		timestamp := strconv.FormatInt(a.now().Unix(), 10)
		packageValue := "prepay_id=" + *response.PrepayId
		source := appid + "\n" + timestamp + "\n" + nonce + "\n" + packageValue + "\n"
		signature, err := utils.SignSHA256WithRSA(source, privateKey)
		if err != nil {
			return nil, err
		}
		return &paymentcap.Redirect{Method: "JSAPI", Fields: map[string]string{"appId": appid, "timeStamp": timestamp, "nonceStr": nonce, "package": packageValue, "signType": "RSA", "paySign": signature}}, nil
	default:
		return nil, paymentcap.ErrUnsupported
	}
}

func (a *Adapter) VerifyCallback(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	publicKey, publicKeyID, apiKey, err := callbackKeys(credentials)
	if err != nil {
		return nil, err
	}
	handler, err := notify.NewRSANotifyHandler(string(apiKey), verifiers.NewSHA256WithRSAPubkeyVerifier(publicKeyID, *publicKey))
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://merchant.invalid/callback", bytes.NewReader(input.Body))
	if err != nil {
		return nil, err
	}
	for key, value := range input.Headers {
		request.Header.Set(key, value)
	}
	var transaction payments.Transaction
	notice, err := handler.ParseNotifyRequest(ctx, request, &transaction)
	if err != nil {
		return nil, gerror.New("WeChat Pay callback signature or encryption is invalid")
	}
	if notice == nil || notice.EventType != "TRANSACTION.SUCCESS" || transaction.Mchid == nil || *transaction.Mchid != credentials.MerchantID || transaction.Appid == nil || *transaction.Appid != credentials.AppID || transaction.OutTradeNo == nil || transaction.TransactionId == nil || transaction.Amount == nil || transaction.Amount.Total == nil {
		return nil, gerror.New("WeChat Pay callback application, merchant, or trade mismatch")
	}
	currency := "CNY"
	if transaction.Amount.PayerCurrency != nil {
		currency = *transaction.Amount.PayerCurrency
	}
	digest := sha256.Sum256(input.Body)
	return &paymentcap.VerifiedResult{ProviderEventReference: notice.ID, ExternalTradeReference: *transaction.TransactionId, AttemptNo: *transaction.OutTradeNo, Currency: currency, AmountMinor: *transaction.Amount.Total, Status: "succeeded", EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}

func (a *Adapter) Query(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.QueryRequest) (*paymentcap.VerifiedResult, error) {
	client, _, err := a.client(ctx, credentials)
	if err != nil {
		return nil, err
	}
	mchid := credentials.MerchantID
	attempt := input.AttemptNo
	transaction, _, err := (&native.NativeApiService{Client: client}).QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{OutTradeNo: &attempt, Mchid: &mchid})
	if err != nil {
		return nil, err
	}
	return projectTransaction(transaction)
}
func (a *Adapter) Close(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.CloseRequest) error {
	client, _, err := a.client(ctx, credentials)
	if err != nil {
		return err
	}
	mchid := credentials.MerchantID
	attempt := input.AttemptNo
	_, err = (&native.NativeApiService{Client: client}).CloseOrder(ctx, native.CloseOrderRequest{OutTradeNo: &attempt, Mchid: &mchid})
	return err
}
func (a *Adapter) Refund(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.RefundRequest) (*paymentcap.VerifiedResult, error) {
	if input.AmountMinor <= 0 || input.TotalAmountMinor < input.AmountMinor || input.Currency != "CNY" {
		return nil, gerror.New("WeChat Pay refund amount is invalid")
	}
	client, _, err := a.client(ctx, credentials)
	if err != nil {
		return nil, err
	}
	currency := "CNY"
	response, _, err := (&refunddomestic.RefundsApiService{Client: client}).Create(ctx, refunddomestic.CreateRequest{TransactionId: &input.ExternalTradeReference, OutRefundNo: &input.RefundAttemptNo, NotifyUrl: &input.NotifyURL, Amount: &refunddomestic.AmountReq{Refund: &input.AmountMinor, Total: &input.TotalAmountMinor, Currency: &currency}})
	if err != nil {
		return nil, err
	}
	return projectRefund(response)
}
func (a *Adapter) VerifyRefundCallback(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	publicKey, publicKeyID, apiKey, err := callbackKeys(credentials)
	if err != nil {
		return nil, err
	}
	handler, err := notify.NewRSANotifyHandler(string(apiKey), verifiers.NewSHA256WithRSAPubkeyVerifier(publicKeyID, *publicKey))
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://merchant.invalid/refund-callback", bytes.NewReader(input.Body))
	if err != nil {
		return nil, err
	}
	for key, value := range input.Headers {
		request.Header.Set(key, value)
	}
	var refund refunddomestic.Refund
	notice, err := handler.ParseNotifyRequest(ctx, request, &refund)
	if err != nil {
		return nil, gerror.New("WeChat Pay refund callback signature or encryption is invalid")
	}
	if notice == nil || !strings.HasPrefix(notice.EventType, "REFUND.") {
		return nil, gerror.New("WeChat Pay refund callback event type mismatch")
	}
	result, err := projectRefund(&refund)
	if err != nil {
		return nil, err
	}
	if !wechatRefundEventMatchesResult(notice.EventType, result.Status) {
		return nil, gerror.New("WeChat Pay refund callback event and result mismatch")
	}
	digest := sha256.Sum256(input.Body)
	result.ProviderEventReference = notice.ID
	result.EvidenceDigest = hex.EncodeToString(digest[:])
	return result, nil
}

func wechatRefundEventMatchesResult(eventType, status string) bool {
	switch eventType {
	case "REFUND.SUCCESS":
		return status == "succeeded"
	case "REFUND.CLOSED":
		return status == "failed"
	case "REFUND.ABNORMAL":
		return status == "manual_review"
	default:
		return false
	}
}
func (a *Adapter) QueryRefund(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.RefundQueryRequest) (*paymentcap.VerifiedResult, error) {
	if paymentcap.ValidateRefundQueryRequest(input) != nil {
		return nil, gerror.New("WeChat Pay refund attempt reference is invalid")
	}
	client, _, err := a.client(ctx, credentials)
	if err != nil {
		return nil, err
	}
	response, _, err := (&refunddomestic.RefundsApiService{Client: client}).QueryByOutRefundNo(ctx, refunddomestic.QueryByOutRefundNoRequest{OutRefundNo: &input.RefundAttemptNo})
	if err != nil {
		return nil, err
	}
	return projectRefund(response)
}
func (a *Adapter) Statement(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.StatementRequest) (*paymentcap.StatementFile, error) {
	if input.BusinessDate.IsZero() {
		return nil, gerror.New("WeChat Pay statement date is invalid")
	}
	client, _, err := a.client(ctx, credentials)
	if err != nil {
		return nil, err
	}
	requestURL := "https://api.mch.weixin.qq.com/v3/bill/tradebill?bill_date=" + url.QueryEscape(input.BusinessDate.Format("2006-01-02")) + "&bill_type=ALL"
	result, err := client.Get(ctx, requestURL)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Response == nil || result.Response.Body == nil {
		return nil, gerror.New("WeChat Pay did not return a statement result")
	}
	defer result.Response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(result.Response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var response struct {
		HashType    string `json:"hash_type"`
		HashValue   string `json:"hash_value"`
		DownloadURL string `json:"download_url"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return nil, gerror.New("WeChat Pay statement result is invalid")
	}
	parsed, err := url.Parse(response.DownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, gerror.New("WeChat Pay statement URL is invalid")
	}
	digest := sha256.Sum256(raw)
	return &paymentcap.StatementFile{DownloadURL: response.DownloadURL, HashType: response.HashType, HashValue: response.HashValue, EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}

func (a *Adapter) client(ctx context.Context, credentials paymentcap.Credentials) (*wxcore.Client, *rsa.PrivateKey, error) {
	if credentials.MerchantID == "" || credentials.AppID == "" || len(credentials.PrivateKey) == 0 || len(credentials.PublicKey) == 0 {
		return nil, nil, gerror.New("WeChat Pay merchant, application, or key is missing")
	}
	privateKey, err := utils.LoadPrivateKey(string(credentials.PrivateKey))
	if err != nil {
		return nil, nil, gerror.New("WeChat Pay merchant private key is invalid")
	}
	publicKey, err := utils.LoadPublicKey(string(credentials.PublicKey))
	if err != nil {
		return nil, nil, gerror.New("WeChat Pay platform public key is invalid")
	}
	serial := string(credentials.Extra[extraCertificateSerial])
	publicKeyID := string(credentials.Extra[extraPublicKeyID])
	if serial == "" || publicKeyID == "" {
		return nil, nil, gerror.New("WeChat Pay certificate serial or platform public key ID is missing")
	}
	client, err := wxcore.NewClient(ctx, option.WithHTTPClient(a.httpClient), option.WithWechatPayPublicKeyAuthCipher(credentials.MerchantID, serial, privateKey, publicKeyID, publicKey))
	return client, privateKey, err
}
func callbackKeys(credentials paymentcap.Credentials) (*rsa.PublicKey, string, []byte, error) {
	publicKey, err := utils.LoadPublicKey(string(credentials.PublicKey))
	if err != nil {
		return nil, "", nil, gerror.New("WeChat Pay platform public key is invalid")
	}
	publicKeyID := string(credentials.Extra[extraPublicKeyID])
	apiKey := credentials.Extra[extraAPIV3Key]
	if publicKeyID == "" || len(apiKey) != 32 {
		return nil, "", nil, gerror.New("WeChat Pay platform public key ID or APIv3 key is missing")
	}
	return publicKey, publicKeyID, apiKey, nil
}
func projectTransaction(value *payments.Transaction) (*paymentcap.VerifiedResult, error) {
	if value == nil || value.OutTradeNo == nil || value.TransactionId == nil || value.Amount == nil || value.Amount.Total == nil || value.TradeState == nil {
		return nil, gerror.New("WeChat Pay response is missing transaction data")
	}
	currency := "CNY"
	if value.Amount.PayerCurrency != nil {
		currency = *value.Amount.PayerCurrency
	}
	status := "pending"
	switch *value.TradeState {
	case "SUCCESS":
		status = "succeeded"
	case "CLOSED", "PAYERROR", "REVOKED":
		status = "closed"
	case "REFUND":
		status = "refunding"
	}
	evidence := sha256.Sum256([]byte(value.String()))
	return &paymentcap.VerifiedResult{ProviderEventReference: *value.TransactionId, ExternalTradeReference: *value.TransactionId, AttemptNo: *value.OutTradeNo, Currency: currency, AmountMinor: *value.Amount.Total, Status: status, EvidenceDigest: hex.EncodeToString(evidence[:])}, nil
}
func projectRefund(value *refunddomestic.Refund) (*paymentcap.VerifiedResult, error) {
	if value == nil || value.RefundId == nil || value.OutRefundNo == nil || value.TransactionId == nil || value.Amount == nil || value.Amount.Refund == nil || value.Status == nil {
		return nil, gerror.New("WeChat Pay response is missing refund data")
	}
	status := "pending"
	switch *value.Status {
	case refunddomestic.STATUS_SUCCESS:
		status = "succeeded"
	case refunddomestic.STATUS_CLOSED:
		status = "failed"
	case refunddomestic.STATUS_ABNORMAL:
		status = "manual_review"
	}
	currency := "CNY"
	if value.Amount.Currency != nil {
		currency = *value.Amount.Currency
	}
	evidence := sha256.Sum256([]byte(value.String()))
	return &paymentcap.VerifiedResult{ProviderEventReference: *value.RefundId, ExternalTradeReference: *value.RefundId, AttemptNo: *value.OutRefundNo, Currency: currency, AmountMinor: *value.Amount.Refund, Status: status, EvidenceDigest: hex.EncodeToString(evidence[:])}, nil
}
func truncate(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return string(runes)
}
