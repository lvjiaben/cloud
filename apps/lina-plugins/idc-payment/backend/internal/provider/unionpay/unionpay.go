// Package unionpay implements the verified UnionPay ACP 5.1.0 web cashier
// boundary. The first release exposes create and signed callback handling;
// query, close, refund and statements remain closed until the merchant-facing
// workflow can persist the original UnionPay transaction timestamp.
package unionpay

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
	"encoding/pem"
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

const (
	DefaultEndpoint = "https://gateway.95516.com/gateway/api/frontTransReq.do"
	TestEndpoint    = "https://gateway.test.95516.com/gateway/api/frontTransReq.do"
	extraCertID     = "certificate_serial_no"
)

type Adapter struct {
	endpoint string
	client   *http.Client
	now      func() time.Time
}

var _ paymentcap.Adapter = (*Adapter)(nil)
var _ paymentcap.CredentialValidator = (*Adapter)(nil)
var _ paymentcap.ConnectionVerifier = (*Adapter)(nil)
var _ paymentcap.EndpointBoundAdapter = (*Adapter)(nil)

func New(endpoint string, client *http.Client) (*Adapter, error) {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultEndpoint
	}
	if !AllowedEndpoint(endpoint) {
		return nil, gerror.New("UnionPay endpoint must be an official ACP web cashier URL")
	}
	if client == nil {
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 15 * time.Second}
	}
	return &Adapter{endpoint: endpoint, client: client, now: time.Now}, nil
}

func AllowedEndpoint(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Host != "gateway.95516.com" && parsed.Host != "gateway.test.95516.com") {
		return false
	}
	return parsed.Path == "/gateway/api/frontTransReq.do"
}

func (*Adapter) Provider() paymentcap.ProviderCode { return paymentcap.ProviderUnionPay }

func (a *Adapter) WithEndpoint(endpoint string) (paymentcap.Adapter, error) {
	return New(endpoint, a.client)
}

func (*Adapter) ValidateCredentials(credentials paymentcap.Credentials) error {
	if strings.TrimSpace(credentials.MerchantID) == "" || strings.TrimSpace(string(credentials.Extra[extraCertID])) == "" {
		return gerror.New("UnionPay merchant or signing certificate serial number is missing")
	}
	if _, err := parsePrivateKey(credentials.PrivateKey); err != nil {
		return err
	}
	if _, err := parsePublicKey(credentials.PublicKey); err != nil {
		return err
	}
	return nil
}

func (*Adapter) Capabilities() paymentcap.Capabilities {
	return paymentcap.Capabilities{
		Modes: map[paymentcap.Mode]bool{
			paymentcap.ModeUnionPayWeb: true,
			paymentcap.ModeUnionPayWAP: true,
		},
		Currencies: map[string]bool{"CNY": true},
		Operations: map[paymentcap.Operation]bool{
			paymentcap.OperationCreate:   true,
			paymentcap.OperationCallback: true,
		},
	}
}

func (a *Adapter) Create(_ context.Context, credentials paymentcap.Credentials, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
	if err := a.ValidateCredentials(credentials); err != nil {
		return nil, err
	}
	if err := paymentcap.ValidateCreateRequest(input); err != nil || input.Currency != "CNY" {
		return nil, gerror.New("UnionPay payment request is invalid")
	}
	channelType := "07"
	if input.Mode == paymentcap.ModeUnionPayWAP {
		channelType = "08"
	} else if input.Mode != paymentcap.ModeUnionPayWeb {
		return nil, paymentcap.ErrUnsupported
	}
	fields := map[string]string{
		"version":      "5.1.0",
		"encoding":     "UTF-8",
		"signMethod":   "01",
		"txnType":      "01",
		"txnSubType":   "01",
		"bizType":      "000201",
		"channelType":  channelType,
		"accessType":   "0",
		"merId":        credentials.MerchantID,
		"orderId":      unionOrderID(input.AttemptNo),
		"txnTime":      a.now().UTC().Format("20060102150405"),
		"txnAmt":       strconv.FormatInt(input.AmountMinor, 10),
		"currencyCode": "156",
		"frontUrl":     input.ReturnURL,
		"backUrl":      input.NotifyURL,
		"reqReserved":  input.AttemptNo,
	}
	if err := sign(fields, credentials); err != nil {
		return nil, err
	}
	return &paymentcap.Redirect{Method: http.MethodPost, URL: a.endpoint, Fields: fields}, nil
}

func (*Adapter) VerifyCallback(_ context.Context, credentials paymentcap.Credentials, input paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	if err := verify(input.Values, credentials); err != nil {
		return nil, err
	}
	if input.Values["merId"] != credentials.MerchantID || input.Values["respCode"] != "00" {
		return nil, gerror.New("UnionPay callback merchant or status mismatch")
	}
	amount, err := strconv.ParseInt(input.Values["txnAmt"], 10, 64)
	if err != nil || amount <= 0 || input.Values["currencyCode"] != "156" {
		return nil, gerror.New("UnionPay callback amount or currency is invalid")
	}
	attemptNo := strings.TrimSpace(input.Values["reqReserved"])
	queryID := strings.TrimSpace(input.Values["queryId"])
	if attemptNo == "" || queryID == "" {
		return nil, gerror.New("UnionPay callback is missing payment references")
	}
	digest := sha256.Sum256([]byte(canonical(input.Values)))
	return &paymentcap.VerifiedResult{
		ProviderEventReference: queryID,
		ExternalTradeReference: queryID,
		AttemptNo:              attemptNo,
		Currency:               "CNY",
		AmountMinor:            amount,
		Status:                 "succeeded",
		EvidenceDigest:         hex.EncodeToString(digest[:]),
	}, nil
}

// VerifyConnection sends one signed query for a deliberately nonexistent
// order. A signed 34 response (no such transaction) proves the merchant
// number, signing certificate and UnionPay verification key without charging.
func (a *Adapter) VerifyConnection(ctx context.Context, credentials paymentcap.Credentials) error {
	if err := a.ValidateCredentials(credentials); err != nil {
		return err
	}
	now := a.now().UTC()
	fields := map[string]string{
		"version":     "5.1.0",
		"encoding":    "UTF-8",
		"signMethod":  "01",
		"txnType":     "00",
		"txnSubType":  "00",
		"bizType":     "000000",
		"accessType":  "0",
		"channelType": "07",
		"merId":       credentials.MerchantID,
		"orderId":     "LINA" + now.Format("20060102150405"),
		"txnTime":     now.Format("20060102150405"),
	}
	if err := sign(fields, credentials); err != nil {
		return err
	}
	form := url.Values{}
	for key, value := range fields {
		form.Set(key, value)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, queryEndpoint(a.endpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode/100 != 2 {
		return gerror.New("UnionPay connection check did not return a usable response")
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return gerror.New("UnionPay connection response is invalid")
	}
	result := make(map[string]string, len(values))
	for key := range values {
		result[key] = values.Get(key)
	}
	if err = verify(result, credentials); err != nil {
		return err
	}
	if result["merId"] != credentials.MerchantID || (result["respCode"] != "34" && result["respCode"] != "00") {
		return gerror.New("UnionPay rejected the merchant or payment product configuration")
	}
	return nil
}

func (*Adapter) Query(context.Context, paymentcap.Credentials, paymentcap.QueryRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}
func (*Adapter) Close(context.Context, paymentcap.Credentials, paymentcap.CloseRequest) error {
	return paymentcap.ErrUnsupported
}
func (*Adapter) Refund(context.Context, paymentcap.Credentials, paymentcap.RefundRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}
func (*Adapter) VerifyRefundCallback(context.Context, paymentcap.Credentials, paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}
func (*Adapter) QueryRefund(context.Context, paymentcap.Credentials, paymentcap.RefundQueryRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}
func (*Adapter) Statement(context.Context, paymentcap.Credentials, paymentcap.StatementRequest) (*paymentcap.StatementFile, error) {
	return nil, paymentcap.ErrUnsupported
}

func sign(values map[string]string, credentials paymentcap.Credentials) error {
	privateKey, err := parsePrivateKey(credentials.PrivateKey)
	if err != nil {
		return err
	}
	values["certId"] = strings.TrimSpace(string(credentials.Extra[extraCertID]))
	firstDigest := sha256.Sum256([]byte(canonical(values)))
	secondDigest := sha256.Sum256([]byte(hex.EncodeToString(firstDigest[:])))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, secondDigest[:])
	if err != nil {
		return err
	}
	values["signature"] = base64.StdEncoding.EncodeToString(signature)
	return nil
}

func verify(values map[string]string, credentials paymentcap.Credentials) error {
	publicKey, err := parsePublicKey(credentials.PublicKey)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(values["signature"]))
	if err != nil || len(signature) == 0 {
		return gerror.New("UnionPay response signature is invalid")
	}
	firstDigest := sha256.Sum256([]byte(canonical(values)))
	secondDigest := sha256.Sum256([]byte(hex.EncodeToString(firstDigest[:])))
	if err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, secondDigest[:], signature); err != nil {
		return gerror.New("UnionPay response signature is invalid")
	}
	return nil
}

func canonical(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if key != "signature" && value != "" {
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

func parsePrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, gerror.New("UnionPay merchant private key is invalid")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, gerror.New("UnionPay merchant private key is invalid")
	}
	return key, nil
}

func parsePublicKey(raw []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, gerror.New("UnionPay platform public key is invalid")
	}
	if certificate, err := x509.ParseCertificate(block.Bytes); err == nil {
		if rsaKey, ok := certificate.PublicKey.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
	}
	if rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return rsaKey, nil
	}
	return nil, gerror.New("UnionPay platform public key is invalid")
}

func queryEndpoint(endpoint string) string {
	return strings.Replace(endpoint, "/frontTransReq.do", "/queryTrans.do", 1)
}

func unionOrderID(attemptNo string) string {
	digest := sha256.Sum256([]byte(attemptNo))
	return "L" + strings.ToUpper(hex.EncodeToString(digest[:16]))
}
