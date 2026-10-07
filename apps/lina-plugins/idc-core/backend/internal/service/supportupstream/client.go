// Package supportupstream implements the isolated legacy-finance ticket protocol.
package supportupstream

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	ResultAccepted              = "accepted"
	ResultDefinitiveNotReceived = "definitive_not_received"
	ResultUnknown               = "result_unknown"
	maxResponseBytes            = 2 << 20
	requestTimeout              = 30 * time.Second
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type Attachment struct {
	FileName, MediaType string
	Content             []byte
}
type CreateRequest struct {
	DepartmentID             int64
	HostID                   string
	Title, Content, Priority string
	Attachments              []Attachment
}
type Result struct {
	Classification, EvidenceCode, TicketReference string
	HTTPStatus                                    int
}
type Reply struct {
	Reference, Content, UserType string
	CreatedAt                    time.Time
	AttachmentURLs               []string
}
type Client interface {
	Create(context.Context, Credentials, CreateRequest) Result
	Detail(context.Context, Credentials, string) ([]Reply, Result)
}

type HTTPClient struct {
	base     *url.URL
	authMode string
	client   *http.Client
}

func New(baseURL, authMode string) (*HTTPClient, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("upstream ticket endpoint must be one HTTPS origin")
	}
	if authMode != "zjmf_api_login" && authMode != "resource_login" {
		return nil, errors.New("upstream ticket authentication mode is invalid")
	}
	client, err := secureHTTPClient(base)
	if err != nil {
		return nil, err
	}
	return &HTTPClient{base: base, authMode: authMode, client: client}, nil
}

func (c *HTTPClient) Create(ctx context.Context, credentials Credentials, input CreateRequest) Result {
	jwt, result := c.login(ctx, credentials)
	if result.Classification != "" {
		return result
	}
	attachmentRefs := make([]string, 0, len(input.Attachments))
	for _, attachment := range input.Attachments {
		ref, uploadResult := c.upload(ctx, jwt, attachment)
		if uploadResult.Classification != "" {
			return uploadResult
		}
		attachmentRefs = append(attachmentRefs, ref)
	}
	form := url.Values{"dptid": {strconv.FormatInt(input.DepartmentID, 10)}, "title": {input.Title}, "content": {input.Content}, "priority": {input.Priority}, "is_api": {"1"}}
	if input.HostID != "" {
		form.Set("hostid", input.HostID)
	}
	if len(attachmentRefs) > 0 {
		form.Set("attachment", strings.Join(attachmentRefs, ","))
	}
	body, status, err := c.request(ctx, http.MethodPost, "ticket/create", jwt, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return unknown("create_transport_unknown", status)
	}
	var envelope struct {
		Status int `json:"status"`
		Data   struct {
			TID any `json:"tid"`
		} `json:"data"`
		TID any `json:"tid"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return unknown("create_response_malformed", status)
	}
	if status >= 200 && status < 300 && envelope.Status == 200 {
		reference := scalar(envelope.Data.TID)
		if reference == "" {
			reference = scalar(envelope.TID)
		}
		if reference != "" {
			return Result{Classification: ResultAccepted, EvidenceCode: "create_accepted_with_ticket_reference", TicketReference: reference, HTTPStatus: status}
		}
		return unknown("create_success_without_ticket_reference", status)
	}
	if status >= 400 && status < 500 && status != 408 && status != 429 {
		return definitive("create_rejected", status)
	}
	return unknown("create_result_unknown", status)
}

func (c *HTTPClient) Detail(ctx context.Context, credentials Credentials, ticketReference string) ([]Reply, Result) {
	jwt, result := c.login(ctx, credentials)
	if result.Classification != "" {
		return nil, result
	}
	endpoint := "ticket/detail?" + url.Values{"tid": {ticketReference}}.Encode()
	body, status, err := c.request(ctx, http.MethodGet, endpoint, jwt, nil, "")
	if err != nil {
		return nil, unknown("detail_transport_unknown", status)
	}
	var envelope struct {
		Status int `json:"status"`
		Data   struct {
			List []struct {
				ID         any      `json:"id"`
				Type       string   `json:"type"`
				Content    string   `json:"content"`
				UserType   string   `json:"user_type"`
				AdminID    any      `json:"admin_id"`
				Admin      string   `json:"admin"`
				Time       int64    `json:"time"`
				Attachment []string `json:"attachment"`
			} `json:"list"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil, unknown("detail_response_malformed", status)
	}
	if status >= 200 && status < 300 && envelope.Status == 200 {
		replies := make([]Reply, 0, len(envelope.Data.List))
		for _, item := range envelope.Data.List {
			adminID := scalar(item.AdminID)
			if item.Type != "r" || ((adminID == "" || adminID == "0") && strings.TrimSpace(item.Admin) == "") {
				continue
			}
			reference := scalar(item.ID)
			if reference == "" {
				continue
			}
			replies = append(replies, Reply{Reference: reference, Content: item.Content, UserType: item.UserType, CreatedAt: time.Unix(item.Time, 0).UTC(), AttachmentURLs: append([]string(nil), item.Attachment...)})
		}
		return replies, Result{Classification: ResultAccepted, EvidenceCode: "detail_read_succeeded", HTTPStatus: status}
	}
	if status >= 400 && status < 500 && status != 408 && status != 429 {
		return nil, definitive("detail_rejected", status)
	}
	return nil, unknown("detail_result_unknown", status)
}

func (c *HTTPClient) login(ctx context.Context, credentials Credentials) (string, Result) {
	if strings.TrimSpace(credentials.Username) == "" || credentials.Password == "" {
		return "", definitive("credentials_invalid", 0)
	}
	form := url.Values{"username": {credentials.Username}, "password": {credentials.Password}}
	if c.authMode == "resource_login" {
		form.Set("type", "agent")
	}
	body, status, err := c.request(ctx, http.MethodPost, c.authMode, "", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", unknown("login_transport_unknown", status)
	}
	var envelope struct {
		Status int    `json:"status"`
		JWT    string `json:"jwt"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return "", unknown("login_response_malformed", status)
	}
	if status >= 200 && status < 300 && envelope.Status == 200 && strings.TrimSpace(envelope.JWT) != "" {
		return envelope.JWT, Result{}
	}
	if status >= 400 && status < 500 && status != 408 && status != 429 {
		return "", definitive("login_rejected", status)
	}
	return "", unknown("login_result_unknown", status)
}

func (c *HTTPClient) upload(ctx context.Context, jwt string, attachment Attachment) (string, Result) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("file", path.Base(attachment.FileName))
	if err != nil {
		return "", definitive("attachment_invalid", 0)
	}
	if _, err = part.Write(attachment.Content); err != nil {
		return "", definitive("attachment_read_failed", 0)
	}
	if err = writer.Close(); err != nil {
		return "", definitive("attachment_invalid", 0)
	}
	body, status, err := c.request(ctx, http.MethodPost, "upload_image", jwt, &buffer, writer.FormDataContentType())
	if err != nil {
		return "", unknown("attachment_upload_unknown", status)
	}
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return "", unknown("attachment_response_malformed", status)
	}
	reference := findUploadReference(payload)
	if status >= 200 && status < 300 && reference != "" {
		return reference, Result{}
	}
	if status >= 400 && status < 500 && status != 408 && status != 429 {
		return "", definitive("attachment_rejected", status)
	}
	return "", unknown("attachment_result_unknown", status)
}

func (c *HTTPClient) request(ctx context.Context, method, endpoint, jwt string, body io.Reader, contentType string) ([]byte, int, error) {
	target := *c.base
	target.Path = strings.TrimRight(c.base.Path, "/") + "/" + strings.TrimLeft(strings.Split(endpoint, "?")[0], "/")
	if index := strings.Index(endpoint, "?"); index >= 0 {
		target.RawQuery = endpoint[index+1:]
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return nil, response.StatusCode, errors.New("upstream response invalid")
	}
	return data, response.StatusCode, nil
}

func secureHTTPClient(base *url.URL) (*http.Client, error) {
	host := base.Hostname()
	port := base.Port()
	if port == "" {
		port = "443"
	}
	if net.ParseIP(host) != nil {
		return nil, errors.New("literal IP endpoints are not allowed")
	}
	dialer := &publicDialer{host: host, port: port, resolver: net.DefaultResolver, dialer: &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}}
	transport := &http.Transport{Proxy: nil, DialContext: dialer.DialContext, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 10 * time.Second, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}}
	return &http.Client{Transport: transport, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

type publicDialer struct {
	host, port string
	resolver   interface {
		LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
	}
	dialer interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
}

func (d *publicDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || !strings.EqualFold(host, d.host) || port != d.port {
		return nil, errors.New("upstream dial target changed")
	}
	resolved, err := d.resolver.LookupIPAddr(ctx, d.host)
	if err != nil || len(resolved) == 0 {
		return nil, errors.New("upstream DNS unavailable")
	}
	for _, candidate := range resolved {
		parsed, ok := netip.AddrFromSlice(candidate.IP)
		if !ok || !publicAddress(parsed.Unmap()) {
			return nil, errors.New("upstream DNS contains a non-public address")
		}
	}
	var last error
	for _, candidate := range resolved {
		connection, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		last = dialErr
	}
	return nil, last
}
func publicAddress(address netip.Addr) bool {
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		network, _ := netip.ParsePrefix(prefix)
		if network.Contains(address) {
			return false
		}
	}
	return true
}
func scalar(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed > 0 && typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
	case json.Number:
		return string(typed)
	}
	return ""
}
func findUploadReference(value any) string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			return strings.TrimSpace(typed)
		}
	case []any:
		for _, item := range typed {
			if found := findUploadReference(item); found != "" {
				return found
			}
		}
	case map[string]any:
		for _, key := range []string{"url", "path", "file", "filename", "data"} {
			if found := findUploadReference(typed[key]); found != "" {
				return found
			}
		}
	}
	return ""
}
func definitive(code string, status int) Result {
	return Result{Classification: ResultDefinitiveNotReceived, EvidenceCode: code, HTTPStatus: status}
}
func unknown(code string, status int) Result {
	return Result{Classification: ResultUnknown, EvidenceCode: code, HTTPStatus: status}
}
