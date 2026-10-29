// Package alipay implements the official mainland Alipay website payment contract.
package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

const Gateway = "https://openapi.alipay.com/gateway.do"

type Adapter struct {
	client  *http.Client
	gateway string
	now     func() time.Time
}

var _ paymentcap.Adapter = (*Adapter)(nil)
var _ paymentcap.ConnectionVerifier = (*Adapter)(nil)

func New(client *http.Client) *Adapter {
	if client == nil {
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 15 * time.Second}
	}
	return &Adapter{client: client, gateway: Gateway, now: time.Now}
}

func (*Adapter) Provider() paymentcap.ProviderCode { return paymentcap.ProviderAlipay }

// VerifyConnection asks Alipay for a deliberately nonexistent trade. Alipay
// signs business-error responses as well, so TRADE_NOT_EXIST proves that the
// application, merchant key and platform public key all passed the protocol
// boundary without creating a payment.
func (a *Adapter) VerifyConnection(ctx context.Context, credentials paymentcap.Credentials) error {
	probe := "LINA_CONNECTION_CHECK_" + a.now().UTC().Format("20060102150405")
	biz, err := marshalBusinessContent(map[string]string{"out_trade_no": probe})
	if err != nil {
		return err
	}
	var response struct {
		Code string `json:"code"`
	}
	_, err = a.invoke(ctx, credentials, "alipay.trade.query", biz, &response)
	if err == nil {
		return nil
	}
	upper := strings.ToUpper(err.Error())
	if strings.Contains(upper, "TRADE_NOT_EXIST") || strings.Contains(upper, "TRADE_NOT_FOUND") {
		return nil
	}
	return err
}
func (*Adapter) ValidateCredentials(credentials paymentcap.Credentials) error {
	if strings.TrimSpace(credentials.AppID) == "" {
		return gerror.New("alipay app is missing")
	}
	if _, err := parsePrivateKey(credentials.PrivateKey); err != nil {
		return err
	}
	_, err := parsePublicKey(credentials.PublicKey)
	return err
}
func (*Adapter) Capabilities() paymentcap.Capabilities {
	return paymentcap.Capabilities{
		Modes:      map[paymentcap.Mode]bool{paymentcap.ModeAlipayPage: true, paymentcap.ModeAlipayWAP: true},
		Currencies: map[string]bool{"CNY": true},
		Operations: map[paymentcap.Operation]bool{paymentcap.OperationCreate: true, paymentcap.OperationCallback: true, paymentcap.OperationQuery: true, paymentcap.OperationClose: true, paymentcap.OperationRefund: true, paymentcap.OperationRefundQuery: true, paymentcap.OperationStatement: true},
	}
}

func (a *Adapter) Create(_ context.Context, credentials paymentcap.Credentials, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
	if err := paymentcap.ValidateCreateRequest(input); err != nil {
		return nil, err
	}
	if input.Currency != "CNY" || (input.Mode != paymentcap.ModeAlipayPage && input.Mode != paymentcap.ModeAlipayWAP) {
		return nil, paymentcap.ErrUnsupported
	}
	method, product := "alipay.trade.page.pay", "FAST_INSTANT_TRADE_PAY"
	if input.Mode == paymentcap.ModeAlipayWAP {
		method = "alipay.trade.wap.pay"
		product = "QUICK_WAP_WAY"
	}
	biz, err := marshalBusinessContent(map[string]any{"out_trade_no": input.AttemptNo, "total_amount": formatAmount(input.AmountMinor), "subject": truncate(input.Subject, 256), "product_code": product, "time_expire": input.ExpiresAt.Format("2006-01-02 15:04:05")})
	if err != nil {
		return nil, err
	}
	fields, err := a.signedFields(credentials, method, biz)
	if err != nil {
		return nil, err
	}
	fields["notify_url"] = input.NotifyURL
	fields["return_url"] = input.ReturnURL
	if err = a.signFields(credentials, fields); err != nil {
		return nil, err
	}
	return &paymentcap.Redirect{Method: "POST", URL: a.gateway, Fields: fields}, nil
}

func (a *Adapter) VerifyCallback(_ context.Context, credentials paymentcap.Credentials, input paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	values := input.Values
	if values["app_id"] != credentials.AppID || (credentials.MerchantID != "" && values["seller_id"] != credentials.MerchantID) || (values["trade_status"] != "TRADE_SUCCESS" && values["trade_status"] != "TRADE_FINISHED") {
		return nil, gerror.New("alipay callback application, merchant, or status mismatch")
	}
	publicKey, err := parsePublicKey(credentials.PublicKey)
	if err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(values["sign"])
	if err != nil {
		return nil, gerror.New("alipay callback signature encoding is invalid")
	}
	digest := sha256.Sum256([]byte(canonicalCallback(values)))
	if err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, gerror.New("alipay callback signature is invalid")
	}
	amount, err := parseAmount(values["total_amount"])
	if err != nil {
		return nil, err
	}
	evidence := sha256.Sum256([]byte(canonical(values)))
	return &paymentcap.VerifiedResult{ProviderEventReference: first(values["notify_id"], values["trade_no"]), ExternalTradeReference: values["trade_no"], AttemptNo: values["out_trade_no"], Currency: "CNY", AmountMinor: amount, Status: "succeeded", EvidenceDigest: hex.EncodeToString(evidence[:])}, nil
}

func (a *Adapter) Query(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.QueryRequest) (*paymentcap.VerifiedResult, error) {
	biz, err := marshalBusinessContent(map[string]string{"out_trade_no": input.AttemptNo})
	if err != nil {
		return nil, err
	}
	var response struct {
		Code        string `json:"code"`
		Msg         string `json:"msg"`
		TradeNo     string `json:"trade_no"`
		OutTradeNo  string `json:"out_trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
	}
	raw, err := a.invoke(ctx, credentials, "alipay.trade.query", biz, &response)
	if err != nil {
		return nil, err
	}
	status := "pending"
	if response.TradeStatus == "TRADE_SUCCESS" || response.TradeStatus == "TRADE_FINISHED" {
		status = "succeeded"
	} else if response.TradeStatus == "TRADE_CLOSED" {
		status = "closed"
	}
	amount, err := parseAmount(response.TotalAmount)
	if err != nil && status == "succeeded" {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &paymentcap.VerifiedResult{ProviderEventReference: response.TradeNo, ExternalTradeReference: response.TradeNo, AttemptNo: response.OutTradeNo, Currency: "CNY", AmountMinor: amount, Status: status, EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}

func (a *Adapter) Close(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.CloseRequest) error {
	biz, err := marshalBusinessContent(map[string]string{"out_trade_no": input.AttemptNo})
	if err != nil {
		return err
	}
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	_, err = a.invoke(ctx, credentials, "alipay.trade.close", biz, &response)
	return err
}

func (a *Adapter) Refund(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.RefundRequest) (*paymentcap.VerifiedResult, error) {
	if input.AmountMinor <= 0 {
		return nil, gerror.New("alipay refund amount must be positive")
	}
	biz, err := marshalBusinessContent(map[string]string{"trade_no": input.ExternalTradeReference, "refund_amount": formatAmount(input.AmountMinor), "out_request_no": input.RefundAttemptNo})
	if err != nil {
		return nil, err
	}
	var response struct {
		Code       string `json:"code"`
		Msg        string `json:"msg"`
		TradeNo    string `json:"trade_no"`
		OutTradeNo string `json:"out_trade_no"`
		RefundFee  string `json:"refund_fee"`
		FundChange string `json:"fund_change"`
	}
	raw, err := a.invoke(ctx, credentials, "alipay.trade.refund", biz, &response)
	if err != nil {
		return nil, err
	}
	status := "pending"
	if response.FundChange == "Y" {
		status = "succeeded"
	}
	amount, err := parseAmount(response.RefundFee)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &paymentcap.VerifiedResult{
		ProviderEventReference: response.TradeNo, ExternalTradeReference: input.RefundAttemptNo,
		AttemptNo: input.RefundAttemptNo, Currency: "CNY", AmountMinor: amount,
		Status: status, EvidenceDigest: hex.EncodeToString(digest[:]),
	}, nil
}

func (*Adapter) VerifyRefundCallback(context.Context, paymentcap.Credentials, paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}

func (a *Adapter) QueryRefund(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.RefundQueryRequest) (*paymentcap.VerifiedResult, error) {
	if err := paymentcap.ValidateRefundQueryRequest(input); err != nil {
		return nil, gerror.New("alipay refund attempt reference is invalid")
	}
	if strings.TrimSpace(input.OriginalTradeReference) == "" {
		return nil, gerror.New("alipay original trade reference is invalid")
	}
	biz, err := marshalBusinessContent(map[string]string{
		"trade_no": input.OriginalTradeReference, "out_request_no": input.RefundAttemptNo,
	})
	if err != nil {
		return nil, err
	}
	var response struct {
		TradeNo      string `json:"trade_no"`
		OutTradeNo   string `json:"out_trade_no"`
		OutRequestNo string `json:"out_request_no"`
		RefundAmount string `json:"refund_amount"`
		RefundStatus string `json:"refund_status"`
	}
	raw, err := a.invoke(ctx, credentials, "alipay.trade.fastpay.refund.query", biz, &response)
	if err != nil {
		return nil, err
	}
	amount, err := parseAmount(response.RefundAmount)
	if err != nil {
		return nil, err
	}
	status := "pending"
	if response.RefundStatus == "REFUND_SUCCESS" {
		status = "succeeded"
	}
	digest := sha256.Sum256(raw)
	return &paymentcap.VerifiedResult{
		ProviderEventReference: response.TradeNo,
		ExternalTradeReference: input.RefundAttemptNo,
		AttemptNo:              first(response.OutRequestNo, input.RefundAttemptNo), Currency: "CNY",
		AmountMinor: amount, Status: status, EvidenceDigest: hex.EncodeToString(digest[:]),
	}, nil
}

func (a *Adapter) Statement(ctx context.Context, credentials paymentcap.Credentials, input paymentcap.StatementRequest) (*paymentcap.StatementFile, error) {
	if input.BusinessDate.IsZero() {
		return nil, gerror.New("alipay statement date is invalid")
	}
	biz, err := marshalBusinessContent(map[string]string{"bill_type": "trade", "bill_date": input.BusinessDate.Format("2006-01-02")})
	if err != nil {
		return nil, err
	}
	var response struct {
		BillDownloadURL string `json:"bill_download_url"`
	}
	raw, err := a.invoke(ctx, credentials, "alipay.data.dataservice.bill.downloadurl.query", biz, &response)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(response.BillDownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, gerror.New("alipay statement URL is invalid")
	}
	digest := sha256.Sum256(raw)
	return &paymentcap.StatementFile{DownloadURL: response.BillDownloadURL, EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}

func marshalBusinessContent(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (a *Adapter) invoke(ctx context.Context, credentials paymentcap.Credentials, method, biz string, target any) ([]byte, error) {
	fields, err := a.signedFields(credentials, method, biz)
	if err != nil {
		return nil, err
	}
	if err = a.signFields(credentials, fields); err != nil {
		return nil, err
	}
	form := url.Values{}
	for key, value := range fields {
		form.Set(key, value)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.gateway, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode/100 != 2 {
		return nil, gerror.Newf("alipay API returned HTTP %d", response.StatusCode)
	}
	key := strings.ReplaceAll(method, ".", "_") + "_response"
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return nil, gerror.New("alipay response is not valid JSON")
	}
	body, ok := envelope[key]
	if !ok {
		return nil, gerror.New("alipay response is missing the operation result")
	}
	var encodedSignature string
	if err = json.Unmarshal(envelope["sign"], &encodedSignature); err != nil || encodedSignature == "" {
		return nil, gerror.New("alipay response is missing a signature")
	}
	publicKey, err := parsePublicKey(credentials.PublicKey)
	if err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.DecodeString(encodedSignature)
	if err != nil {
		return nil, gerror.New("alipay response signature encoding is invalid")
	}
	digest := sha256.Sum256(body)
	if err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, gerror.New("alipay response signature is invalid")
	}
	if err = json.Unmarshal(body, target); err != nil {
		return nil, err
	}
	var common struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
	}
	_ = json.Unmarshal(body, &common)
	if common.Code != "10000" {
		return nil, gerror.Newf("alipay rejected the request: %s", first(common.SubMsg, common.Msg, common.SubCode, common.Code))
	}
	return raw, nil
}

func (a *Adapter) signedFields(credentials paymentcap.Credentials, method, biz string) (map[string]string, error) {
	if credentials.AppID == "" || len(credentials.PrivateKey) == 0 {
		return nil, gerror.New("alipay application or private key is missing")
	}
	return map[string]string{"app_id": credentials.AppID, "method": method, "format": "JSON", "charset": "utf-8", "sign_type": "RSA2", "timestamp": a.now().Format("2006-01-02 15:04:05"), "version": "1.0", "biz_content": biz}, nil
}
func (a *Adapter) signFields(credentials paymentcap.Credentials, fields map[string]string) error {
	key, err := parsePrivateKey(credentials.PrivateKey)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(canonical(fields)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return err
	}
	fields["sign"] = base64.StdEncoding.EncodeToString(signature)
	return nil
}
func canonical(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if key != "sign" && value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, "&")
}
func canonicalCallback(values map[string]string) string {
	copyValues := make(map[string]string, len(values))
	for key, value := range values {
		if key != "sign" && key != "sign_type" {
			copyValues[key] = value
		}
	}
	return canonical(copyValues)
}
func parsePrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, gerror.New("alipay private key is invalid")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, gerror.New("alipay private key is invalid")
	}
	return key, nil
}
func parsePublicKey(raw []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, gerror.New("alipay public key is invalid")
	}
	if value, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := value.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, gerror.New("alipay public key is invalid")
	}
	return key, nil
}
func parseAmount(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) == 1 {
		parts = append(parts, "00")
	}
	if len(parts) != 2 || len(parts[1]) > 2 || parts[0] == "" {
		return 0, gerror.New("alipay amount format is invalid")
	}
	parts[1] += strings.Repeat("0", 2-len(parts[1]))
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || yuan < 0 {
		return 0, gerror.New("alipay amount format is invalid")
	}
	cent, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || cent < 0 || cent > 99 || yuan > (1<<63-1-cent)/100 {
		return 0, gerror.New("alipay amount is out of range")
	}
	return yuan*100 + cent, nil
}
func formatAmount(value int64) string {
	return strconv.FormatInt(value/100, 10) + "." + fmt.Sprintf("%02d", value%100)
}
func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown error"
}
