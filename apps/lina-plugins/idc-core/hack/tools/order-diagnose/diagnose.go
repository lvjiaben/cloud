// This file owns the fixed-budget HTTP diagnostic workflow. It consumes public
// local API projections only, with no database access, writes, or supplier calls.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	maxTasks    = 3
	maxBody     = 1024 * 1024
	coreAPI     = "/x/idc-core/api/v1/admin/"
	resourceAPI = "/x/idc-resource/api/v1/admin/resource/tasks/"
)

// Only opaque identifiers, machine codes, and decimal integer money are emitted.
var (
	referencePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	codePattern         = regexp.MustCompile(`^[A-Za-z0-9_.:-]{0,96}$`)
	moneyPattern        = regexp.MustCompile(`^-?[0-9]{1,40}$`)
	resourceTaskPattern = regexp.MustCompile(`^task_[a-f0-9]{32}$`)
)

type diagnosticClient struct {
	base, token string
	http        *http.Client
}

// report contains no raw API maps, user identity fields, notes, or credentials.
type report struct {
	OrderNo     string       `json:"orderNo"`
	OrderStatus string       `json:"orderStatus"`
	Invoice     *invoice     `json:"invoice,omitempty"`
	Tasks       []taskReport `json:"tasks"`
	TaskTotal   int          `json:"taskTotal"`
	Requests    int          `json:"requests"`
	Complete    bool         `json:"complete"`
	Conclusion  string       `json:"conclusion"`
	Issues      []string     `json:"issues"`
}

type invoice struct {
	StatusCode       string `json:"statusCode"`
	CurrencyCode     string `json:"currencyCode"`
	TotalAmountMinor string `json:"totalAmountMinor"`
	PaidAmountMinor  string `json:"paidAmountMinor"`
}

type execution struct {
	ResourceTaskNo string `json:"resourceTaskNo"`
	StatusCode     string `json:"statusCode"`
	PhaseCode      string `json:"phaseCode"`
	LastResultCode string `json:"lastResultCode"`
	AttemptCount   int    `json:"attemptCount"`
}

type taskReport struct {
	IntentNo   string          `json:"intentNo"`
	StatusCode string          `json:"statusCode"`
	Execution  *execution      `json:"execution,omitempty"`
	Resource   *resourceDetail `json:"resource,omitempty"`
}

type resourceDetail struct {
	Task *struct {
		TaskNo        string `json:"taskNo"`
		Status        string `json:"status"`
		ResultUnknown bool   `json:"resultUnknown"`
	} `json:"task"`
	Upstream *struct {
		ExternalTaskRef     string `json:"externalTaskRef"`
		ExternalResourceRef string `json:"externalResourceRef"`
		StatusCode          string `json:"statusCode"`
	} `json:"upstream"`
}

// newDiagnosticClient accepts only explicit loopback targets and disables
// proxies and redirects, so a local response cannot forward a bearer token.
func newDiagnosticClient(base, token, session string) (*diagnosticClient, error) {
	u, err := url.Parse(base)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("base-url must be a local API origin without credentials, path, query or fragment")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("diagnostics only allow local loopback addresses")
	}
	token, err = diagnosticToken(token, session)
	if err != nil {
		return nil, err
	}
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("local HTTP transport configuration is invalid")
	}
	transport := defaultTransport.Clone()
	transport.Proxy = nil
	return &diagnosticClient{base: strings.TrimRight(base, "/"), token: token, http: &http.Client{
		Transport: transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// diagnosticToken loads only the selected credential source and never includes
// its file contents or filesystem error details in an error message.
func diagnosticToken(token, session string) (string, error) {
	if session != "" {
		f, openErr := os.Open(session)
		if openErr != nil {
			return "", fmt.Errorf("could not read the selected session file")
		}
		var value struct {
			AccessToken string `json:"accessToken"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(f, maxBody)).Decode(&value)
		closeErr := f.Close()
		if decodeErr != nil || closeErr != nil {
			return "", fmt.Errorf("the selected session file is invalid")
		}
		token = value.AccessToken
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, "\r\n\t ") || len(token) > 16384 {
		return "", fmt.Errorf("LINA_DIAG_TOKEN or a valid --session-file is required; automatic login is never attempted")
	}
	return token, nil
}

// get never includes URLs, bodies, headers, or server error messages in errors.
func (c *diagnosticClient) get(ctx context.Context, path string, target any, r *report) error {
	if r.Requests >= 8 {
		return fmt.Errorf("request_budget_exceeded")
	}
	r.Requests++
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("request_invalid")
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("network_or_timeout")
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return fmt.Errorf("response_read_failed")
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("http_%d", response.StatusCode)
	}
	if len(body) > maxBody {
		return fmt.Errorf("response_too_large")
	}
	var envelope struct {
		Code json.RawMessage `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return fmt.Errorf("response_invalid")
	}
	if string(envelope.Code) != "0" {
		return fmt.Errorf("api_rejected")
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" || json.Unmarshal(envelope.Data, target) != nil {
		return fmt.Errorf("data_invalid")
	}
	return nil
}

// diagnose uses at most eight GETs: order, a bounded task page, and up to three
// intent and three resource details. It never scans unrelated order queues.
func (c *diagnosticClient) diagnose(ctx context.Context, order string) *report {
	r := &report{OrderNo: order, Complete: true, Tasks: []taskReport{}, Issues: []string{}, Conclusion: "Only locally saved facts are summarized; no upstream query or provisioning was performed"}
	issue := func(label string, err error) { r.Complete = false; r.Issues = append(r.Issues, label+":"+err.Error()) }
	var detail struct {
		Order *struct {
			Summary *struct {
				OrderNo    string `json:"orderNo"`
				StatusCode string `json:"statusCode"`
			} `json:"summary"`
			Invoice *invoice `json:"invoice"`
		} `json:"order"`
	}
	if err := c.get(ctx, coreAPI+"commerce/orders/"+url.PathEscape(order), &detail, r); err != nil {
		issue("order", err)
		return r
	}
	if detail.Order == nil || detail.Order.Summary == nil || detail.Order.Summary.OrderNo != order {
		issue("order", fmt.Errorf("identity_mismatch"))
		return r
	}
	r.OrderStatus = machineCode(detail.Order.Summary.StatusCode)
	if !validCodes(detail.Order.Summary.StatusCode) {
		issue("order", fmt.Errorf("code_invalid"))
	}
	if value := detail.Order.Invoice; value != nil {
		if !validCodes(value.StatusCode, value.CurrencyCode) {
			issue("invoice", fmt.Errorf("code_invalid"))
		}
		if !moneyPattern.MatchString(value.PaidAmountMinor) || !moneyPattern.MatchString(value.TotalAmountMinor) {
			issue("invoice", fmt.Errorf("amount_invalid"))
		} else {
			r.Invoice = &invoice{StatusCode: machineCode(value.StatusCode), CurrencyCode: machineCode(value.CurrencyCode), PaidAmountMinor: value.PaidAmountMinor, TotalAmountMinor: value.TotalAmountMinor}
		}
	}
	var page struct {
		Tasks []struct {
			IntentNo            string `json:"intentNo"`
			OrderNo             string `json:"orderNo"`
			StatusCode          string `json:"statusCode"`
			FulfillmentModeCode string `json:"fulfillmentModeCode"`
		} `json:"tasks"`
		Total      int    `json:"total"`
		NextCursor string `json:"nextCursor"`
	}
	if err := c.get(ctx, coreAPI+"product-fulfillment/tasks?orderNo="+url.QueryEscape(order)+"&limit=3", &page, r); err != nil {
		issue("fulfillment", err)
		return r
	}
	r.TaskTotal = page.Total
	if page.Total < len(page.Tasks) {
		issue("fulfillment", fmt.Errorf("count_inconsistent"))
	}
	if len(page.Tasks) > maxTasks || page.Total > maxTasks || page.NextCursor != "" {
		issue("fulfillment", fmt.Errorf("task_limit_exceeded"))
	}
	seen := make(map[string]*resourceDetail)
	for i, item := range page.Tasks {
		if i >= maxTasks {
			break
		}
		if item.OrderNo != order || !referencePattern.MatchString(item.IntentNo) {
			issue("fulfillment", fmt.Errorf("identity_mismatch"))
			continue
		}
		entry := taskReport{IntentNo: item.IntentNo, StatusCode: machineCode(item.StatusCode)}
		if !validCodes(item.StatusCode) {
			issue("intent", fmt.Errorf("code_invalid"))
		}
		var value struct {
			Task *struct {
				IntentNo  string     `json:"intentNo"`
				OrderNo   string     `json:"orderNo"`
				Execution *execution `json:"execution"`
			} `json:"task"`
		}
		if err := c.get(ctx, coreAPI+"product-fulfillment/tasks/"+url.PathEscape(item.IntentNo), &value, r); err != nil {
			issue("intent", err)
			r.Tasks = append(r.Tasks, entry)
			continue
		}
		if value.Task == nil || value.Task.IntentNo != item.IntentNo || value.Task.OrderNo != order {
			issue("intent", fmt.Errorf("identity_mismatch"))
			r.Tasks = append(r.Tasks, entry)
			continue
		}
		if e := value.Task.Execution; e != nil {
			if !validCodes(e.StatusCode, e.PhaseCode, e.LastResultCode) {
				issue("intent", fmt.Errorf("code_invalid"))
			}
			entry.Execution = &execution{StatusCode: machineCode(e.StatusCode), PhaseCode: machineCode(e.PhaseCode), LastResultCode: machineCode(e.LastResultCode), AttemptCount: e.AttemptCount}
			if err := c.loadResource(ctx, e.ResourceTaskNo, &entry, r, seen); err != nil {
				issue("resource", err)
			}
		}
		if value.Task.Execution == nil && item.StatusCode == "processing" && (item.FulfillmentModeCode == "payment_confirmed_plugin" || item.FulfillmentModeCode == "approval_then_plugin") {
			issue("intent", fmt.Errorf("execution_unavailable"))
		}
		r.Tasks = append(r.Tasks, entry)
	}
	return r
}

// loadResource follows one authorized local reference and caches its result,
// including failures, so repeated intent references do not repeat GETs.
func (c *diagnosticClient) loadResource(ctx context.Context, ref string, entry *taskReport, r *report, seen map[string]*resourceDetail) error {
	if ref == "" {
		return nil
	}
	if !resourceTaskPattern.MatchString(ref) {
		return fmt.Errorf("reference_invalid")
	}
	entry.Execution.ResourceTaskNo = ref
	if cached, ok := seen[ref]; ok {
		entry.Resource = cached
		return nil
	}
	seen[ref] = nil
	var value struct {
		Item *resourceDetail `json:"item"`
	}
	if err := c.get(ctx, resourceAPI+url.PathEscape(ref), &value, r); err != nil {
		return err
	}
	if value.Item == nil || value.Item.Task == nil || value.Item.Task.TaskNo != ref {
		return fmt.Errorf("identity_mismatch")
	}
	if !validResourceProjection(value.Item) {
		return fmt.Errorf("projection_invalid")
	}
	entry.Resource = value.Item
	seen[ref] = value.Item
	if value.Item.Task.ResultUnknown || value.Item.Task.Status == "result_unknown" {
		r.Conclusion = "The upstream result is unknown; verify existing resources and do not provision again based on this report"
	}
	return nil
}

// validCodes marks malformed projected values as incomplete evidence.
func validCodes(values ...string) bool {
	for _, value := range values {
		if !codePattern.MatchString(value) {
			return false
		}
	}
	return true
}

// validResourceProjection rejects malformed normalized references before use.
func validResourceProjection(value *resourceDetail) bool {
	if !validCodes(value.Task.Status) {
		return false
	}
	if value.Upstream == nil {
		return true
	}
	for _, ref := range []string{value.Upstream.ExternalTaskRef, value.Upstream.ExternalResourceRef} {
		if ref != "" && !referencePattern.MatchString(ref) {
			return false
		}
	}
	return validCodes(value.Upstream.StatusCode)
}

// machineCode prevents arbitrary messages and multi-line content entering output.
func machineCode(value string) string {
	if !codePattern.MatchString(value) {
		return "invalid_code"
	}
	return value
}
