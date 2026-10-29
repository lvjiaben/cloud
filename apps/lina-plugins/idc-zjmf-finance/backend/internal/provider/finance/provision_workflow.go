package finance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type ProvisionIntent struct {
	DownstreamURL          string
	DownstreamID           string
	Parameters             json.RawMessage
	DownstreamTokenBinding string
}

type ClearOutcome struct {
	ExistingResourceRef string
	ExistingInvoiceRef  string
	ContinuePurchase    bool
}

func BuildClearCartCall(intent ProvisionIntent) (Call, map[string]string, error) {
	err := validateProvisionIntent(intent)
	if err != nil {
		return Call{}, nil, err
	}
	return Call{Method: http.MethodPost, Path: "/cart/clear", Form: downstreamForm(intent)}, downstreamSecretFields(intent), nil
}

func ClassifyClearOutcome(raw json.RawMessage) (ClearOutcome, error) {
	status := findString(raw, "status")
	hostID := findString(raw, "hostid")
	invoiceID := findString(raw, "invoiceid")
	domainStatus := findString(raw, "domainstatus")
	if status == "400" && positiveDecimal(hostID) && strings.EqualFold(domainStatus, "Active") {
		return ClearOutcome{ExistingResourceRef: hostID}, nil
	}
	if status != "200" {
		return ClearOutcome{}, errInvalidResponse
	}
	if invoiceID != "" {
		if !positiveDecimal(invoiceID) || !positiveDecimal(hostID) {
			return ClearOutcome{}, errInvalidResponse
		}
		return ClearOutcome{ExistingResourceRef: hostID, ExistingInvoiceRef: invoiceID}, nil
	}
	return ClearOutcome{ContinuePurchase: true}, nil
}

func BuildAddToCartCall(intent ProvisionIntent, currencyID string) (Call, map[string]string, error) {
	if err := validateProvisionIntent(intent); err != nil || !positiveDecimal(currencyID) {
		return Call{}, nil, errInvalidConfig
	}
	parameters, err := decodePurchaseParameters(intent.Parameters)
	if err != nil {
		return Call{}, nil, err
	}
	form := url.Values{
		"pid": {parameters.ProductID}, "billingcycle": {parameters.BillingCycle}, "host": {parameters.Hostname},
		"currencyid": {currencyID}, "qty": {"1"},
	}
	for optionID, value := range parameters.ConfigOptions {
		form.Set("configoption["+optionID+"]", value)
	}
	for fieldID, value := range parameters.CustomFields {
		form.Set("customfield["+fieldID+"]", value)
	}
	return Call{Method: http.MethodPost, Path: "/cart/add_to_shop", Form: form}, nil, nil
}

func BuildSettleCall(intent ProvisionIntent) (Call, map[string]string, error) {
	err := validateProvisionIntent(intent)
	if err != nil {
		return Call{}, nil, err
	}
	parameters, err := decodePurchaseParameters(intent.Parameters)
	if err != nil {
		return Call{}, nil, err
	}
	form := downstreamForm(intent)
	form.Set("cart_data[pid]", parameters.ProductID)
	form.Set("cart_data[billingcycle]", parameters.BillingCycle)
	form.Set("cart_data[host]", parameters.Hostname)
	form.Set("cart_data[qty]", "1")
	for optionID, value := range parameters.ConfigOptions {
		form.Set("cart_data[configoptions]["+optionID+"]", value)
	}
	for fieldID, value := range parameters.CustomFields {
		form.Set("cart_data[customfield]["+fieldID+"]", value)
	}
	return Call{Method: http.MethodPost, Path: "/cart/settle", Form: form}, downstreamSecretFields(intent), nil
}

func BuildApplyCreditCall(intent ProvisionIntent, invoiceID string) (Call, map[string]string, error) {
	err := validateProvisionIntent(intent)
	if err != nil || !positiveDecimal(invoiceID) {
		return Call{}, nil, errInvalidConfig
	}
	form := downstreamForm(intent)
	form.Set("invoiceid", invoiceID)
	form.Set("use_credit", "1")
	form.Set("enough", "0")
	return Call{Method: http.MethodPost, Path: "/apply_credit", Form: form}, downstreamSecretFields(intent), nil
}

func SettlementResult(requestID string, raw json.RawMessage) resourcecap.OperationResult {
	status := findString(raw, "status")
	hostID := nestedArrayFirst(raw, "hostid")
	if hostID == "" {
		hostID = findString(raw, "hostid")
	}
	now := timeNowUTC()
	// Native finance returns 200 with invoiceid and hostid for an unpaid order.
	// Only 1001 proves settlement; a created host reference is not payment evidence.
	if status == "1001" && positiveDecimal(hostID) {
		return resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: hostID, Evidence: []resourcecap.Evidence{{Kind: "upstream_settlement", Reference: requestID, Summary: "native finance confirmed settlement and returned a server reference", ObservedAt: now}}}
	}
	return failedResult(requestID, "")
}

type purchaseParameters struct {
	Workflow      string            `json:"workflow,omitempty"`
	ProductID     string            `json:"pid"`
	BillingCycle  string            `json:"billingcycle"`
	Hostname      string            `json:"host"`
	Quantity      int               `json:"qty"`
	ConfigOptions map[string]string `json:"configoption"`
	CustomFields  map[string]string `json:"customfield,omitempty"`
}

func decodePurchaseParameters(raw json.RawMessage) (purchaseParameters, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value purchaseParameters
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF || !positiveDecimal(value.ProductID) ||
		(value.Workflow != "" && value.Workflow != "serialized_cart_v1") || !safeCode.MatchString(value.BillingCycle) || value.Quantity != 1 || value.ConfigOptions == nil || len(value.ConfigOptions) > 128 || len(value.CustomFields) > 128 {
		return purchaseParameters{}, errInvalidConfig
	}
	for key, option := range value.ConfigOptions {
		if !positiveDecimal(key) || !safeCode.MatchString(option) {
			return purchaseParameters{}, errInvalidConfig
		}
	}
	for key, field := range value.CustomFields {
		if !positiveDecimal(key) || !utf8.ValidString(field) || len(field) > 8192 || strings.ContainsRune(field, '\x00') {
			return purchaseParameters{}, errInvalidConfig
		}
	}
	return value, nil
}

func validateProvisionIntent(intent ProvisionIntent) error {
	callback, err := url.Parse(intent.DownstreamURL)
	if err != nil || callback.Scheme != "https" || callback.Hostname() == "" || callback.User != nil || callback.RawQuery != "" || callback.Fragment != "" ||
		!positiveDecimal(intent.DownstreamID) || intent.DownstreamTokenBinding != "downstream_token" {
		return errInvalidConfig
	}
	return nil
}

func downstreamForm(intent ProvisionIntent) url.Values {
	return url.Values{"is_api": {"1"}, "downstream_url": {intent.DownstreamURL}, "downstream_id": {intent.DownstreamID}}
}

func downstreamSecretFields(intent ProvisionIntent) map[string]string {
	return map[string]string{"downstream_token": intent.DownstreamTokenBinding}
}

func nestedArrayFirst(raw json.RawMessage, key string) string {
	var root struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &root) != nil {
		return ""
	}
	var values []json.RawMessage
	if json.Unmarshal(root.Data[key], &values) != nil || len(values) != 1 {
		return ""
	}
	return scalar(values[0])
}

var timeNowUTC = func() time.Time { return time.Now().UTC() }
