// Package epay implements only the locally documented submit.php and MD5 notification contract.
package epay

import (
	"context"
	"crypto/md5" // #nosec G501 -- the upstream EPay protocol explicitly requires MD5.
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
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

const DefaultEndpoint = "https://www.ezfp.cn/submit.php"

type Adapter struct {
	endpoint string
	client   *http.Client
}

var _ paymentcap.Adapter = (*Adapter)(nil)
var _ paymentcap.ConnectionVerifier = (*Adapter)(nil)

func New(endpoint string) (*Adapter, error) {
	return NewWithClient(endpoint, nil)
}

func NewWithClient(endpoint string, client *http.Client) (*Adapter, error) {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasSuffix(parsed.Path, "/submit.php") {
		return nil, gerror.New("EPay endpoint must be an HTTPS submit.php URL")
	}
	if client == nil {
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 15 * time.Second}
	}
	return &Adapter{endpoint: parsed.String(), client: client}, nil
}

func (*Adapter) Provider() paymentcap.ProviderCode { return paymentcap.ProviderEPay }

func (a *Adapter) WithEndpoint(endpoint string) (paymentcap.Adapter, error) {
	return NewWithClient(endpoint, a.client)
}

// VerifyConnection checks the merchant-controlled submit.php endpoint without
// submitting a signed order. EPay's locally frozen contract has no harmless
// credential-verification API, so this proves endpoint reachability only; the
// UI states that limitation explicitly.
func (a *Adapter) VerifyConnection(ctx context.Context, credentials paymentcap.Credentials) error {
	if err := a.ValidateCredentials(credentials); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
	if response.StatusCode < 200 || response.StatusCode >= 500 {
		return gerror.Newf("EPay endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (*Adapter) ValidateCredentials(credentials paymentcap.Credentials) error {
	if strings.TrimSpace(credentials.MerchantID) == "" || len(credentials.Secret) == 0 {
		return gerror.New("epay merchant or secret is missing")
	}
	return nil
}

func (*Adapter) Capabilities() paymentcap.Capabilities {
	return paymentcap.Capabilities{
		Modes:      map[paymentcap.Mode]bool{paymentcap.ModeEPayCashier: true, paymentcap.ModeEPayAlipay: true, paymentcap.ModeEPayWechat: true},
		Currencies: map[string]bool{"CNY": true},
		Operations: map[paymentcap.Operation]bool{
			paymentcap.OperationCreate:   true,
			paymentcap.OperationCallback: true,
		},
	}
}

func (a *Adapter) Create(_ context.Context, credentials paymentcap.Credentials, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
	if a == nil || strings.TrimSpace(credentials.MerchantID) == "" || len(credentials.Secret) == 0 {
		return nil, gerror.New("EPay merchant or secret is missing")
	}
	if err := paymentcap.ValidateCreateRequest(input); err != nil {
		return nil, err
	}
	if input.Currency != "CNY" || !a.Capabilities().Modes[input.Mode] {
		return nil, paymentcap.ErrUnsupported
	}
	fields := map[string]string{
		"pid":          credentials.MerchantID,
		"out_trade_no": input.AttemptNo,
		"notify_url":   input.NotifyURL,
		"return_url":   input.ReturnURL,
		"name":         truncateUTF8Bytes(input.Subject, 127),
		"money":        formatCNY(input.AmountMinor),
		"param":        input.InvoiceNo,
		"sign_type":    "MD5",
	}
	switch input.Mode {
	case paymentcap.ModeEPayAlipay:
		fields["type"] = "alipay"
	case paymentcap.ModeEPayWechat:
		fields["type"] = "wxpay"
	}
	fields["sign"] = sign(fields, credentials.Secret)
	return &paymentcap.Redirect{Method: "POST", URL: a.endpoint, Fields: fields}, nil
}

func (*Adapter) VerifyCallback(_ context.Context, credentials paymentcap.Credentials, input paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	if strings.TrimSpace(credentials.MerchantID) == "" || len(credentials.Secret) == 0 {
		return nil, gerror.New("EPay merchant or secret is missing")
	}
	values := input.Values
	if values["pid"] != credentials.MerchantID || values["sign_type"] != "MD5" || values["trade_status"] != "TRADE_SUCCESS" {
		return nil, gerror.New("EPay callback merchant or status mismatch")
	}
	expected := sign(values, credentials.Secret)
	provided := strings.ToLower(strings.TrimSpace(values["sign"]))
	if len(expected) != len(provided) || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return nil, gerror.New("EPay callback signature is invalid")
	}
	amount, err := parseCNY(values["money"])
	if err != nil {
		return nil, err
	}
	if !businessReference(values["out_trade_no"]) || strings.TrimSpace(values["trade_no"]) == "" {
		return nil, gerror.New("EPay callback is missing a valid trade reference")
	}
	evidence := sha256.Sum256([]byte(canonical(values, false)))
	return &paymentcap.VerifiedResult{
		ProviderEventReference: values["trade_no"],
		ExternalTradeReference: values["trade_no"],
		AttemptNo:              values["out_trade_no"], Currency: "CNY", AmountMinor: amount,
		Status: "succeeded", EvidenceDigest: hex.EncodeToString(evidence[:]),
	}, nil
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

func sign(values map[string]string, secret []byte) string {
	content := canonical(values, true) + string(secret)
	digest := md5.Sum([]byte(content)) // #nosec G401 -- compatibility with the documented EPay protocol.
	return hex.EncodeToString(digest[:])
}

func canonical(values map[string]string, omitSignature bool) string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if value == "" || key == "sign" || key == "sign_type" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, "&")
}

func formatCNY(minor int64) string {
	return strconv.FormatInt(minor/100, 10) + "." + strconv.FormatInt((minor%100)/10, 10) + strconv.FormatInt(minor%10, 10)
}

func parseCNY(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, gerror.New("EPay callback amount format is invalid")
	}
	if len(parts) == 1 {
		parts = append(parts, "00")
	}
	if len(parts[1]) == 1 {
		parts[1] += "0"
	}
	if len(parts[1]) != 2 {
		return 0, gerror.New("EPay callback amount must have at most two decimal places")
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || yuan < 0 {
		return 0, gerror.New("EPay callback amount format is invalid")
	}
	cent, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || cent < 0 || cent > 99 {
		return 0, gerror.New("EPay callback amount format is invalid")
	}
	if yuan > (1<<63-1-cent)/100 {
		return 0, gerror.New("EPay callback amount is out of range")
	}
	amount := yuan*100 + cent
	if amount <= 0 {
		return 0, gerror.New("EPay callback amount must be positive")
	}
	return amount, nil
}

func truncateUTF8Bytes(value string, limit int) string {
	value = strings.TrimSpace(value)
	for len([]byte(value)) > limit {
		value = string([]rune(value)[:len([]rune(value))-1])
	}
	return value
}

func businessReference(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (index > 0 && strings.ContainsRune("_.:-", r)) {
			continue
		}
		return false
	}
	return true
}
