// Package productfulfillmentcap defines the bounded, provider-neutral contract
// used by one frozen product-fulfillment extension.
package productfulfillmentcap

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	ContractVersion      = "v1"
	RequestVersion       = "v1"
	MaximumPayloadBytes  = 1 << 20
	MaximumEvidenceBytes = 64 << 10
)

type Action string

const (
	ActionQuery  Action = "query"
	ActionSubmit Action = "submit"
)

type ResultStatus string

const (
	StatusNotFound      ResultStatus = "not_found"
	StatusNotAccepted   ResultStatus = "not_accepted"
	StatusAccepted      ResultStatus = "accepted"
	StatusRunning       ResultStatus = "running"
	StatusSucceeded     ResultStatus = "succeeded"
	StatusFailed        ResultStatus = "failed"
	StatusResultUnknown ResultStatus = "result_unknown"
)

// Request contains only frozen business evidence. Provider credentials are
// accessed through the source-only runtime callback and never serialized here.
type Request struct {
	Version                    string           `json:"version"`
	IntentNo                   string           `json:"intentNo"`
	IdempotencyKey             string           `json:"idempotencyKey"`
	OrderNo                    string           `json:"orderNo"`
	OrderItemNo                string           `json:"orderItemNo"`
	UnitSequence               int              `json:"unitSequence"`
	Quantity                   int              `json:"quantity"`
	ProductNo                  string           `json:"productNo"`
	ProductVersionNo           string           `json:"productVersionNo"`
	ConfigurationRevision      string           `json:"configurationRevision"`
	ProductConfiguration       json.RawMessage  `json:"productConfiguration"`
	ProductConfigurationDigest string           `json:"productConfigurationDigest"`
	CustomerConfiguration      json.RawMessage  `json:"customerConfiguration"`
	CustomerConfigurationID    string           `json:"customerConfigurationId"`
	Billing                    *BillingSchedule `json:"billing,omitempty"`
	PreviousExternalReference  string           `json:"previousExternalReference,omitempty"`
}

// Result distinguishes safe retry from uncertain acceptance. Evidence is a
// bounded provider document kept only in owner-private history.
type Result struct {
	Version           string          `json:"version"`
	Status            ResultStatus    `json:"status"`
	ExternalReference string          `json:"externalReference,omitempty"`
	Evidence          json.RawMessage `json:"evidence,omitempty"`
	Deliveries        []Delivery      `json:"deliveries,omitempty"`
}

// Delivery freezes one upstream delivery/service association. One-time results
// have no UpstreamServiceNo; all timestamps use Unix milliseconds.
type Delivery struct {
	UpstreamDeliveryNo string `json:"upstreamDeliveryNo"`
	UpstreamServiceNo  string `json:"upstreamServiceNo,omitempty"`
	StartedAt          int64  `json:"startedAt"`
	ExpiresAt          int64  `json:"expiresAt,omitempty"`
}

var (
	safeID        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$`)
	digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

func ValidateRequest(value *Request) error {
	if !validRequestIdentity(value) {
		return fmt.Errorf("product fulfillment request is invalid")
	}
	if !validJSONObject(value.ProductConfiguration) || !digestPattern.MatchString(value.ProductConfigurationDigest) {
		return fmt.Errorf("product fulfillment product configuration is invalid")
	}
	if !validJSONObject(value.CustomerConfiguration) {
		return fmt.Errorf("product fulfillment customer configuration is invalid")
	}
	if value.Billing != nil {
		if err := value.Billing.Validate(); err != nil {
			return err
		}
	}
	if value.PreviousExternalReference != "" && !safeID.MatchString(value.PreviousExternalReference) {
		return fmt.Errorf("product fulfillment previous external reference is invalid")
	}
	return nil
}

func validRequestIdentity(value *Request) bool {
	return value != nil && value.Version == RequestVersion && safeID.MatchString(value.IntentNo) &&
		safeID.MatchString(value.IdempotencyKey) && safeID.MatchString(value.OrderNo) &&
		safeID.MatchString(value.OrderItemNo) && safeID.MatchString(value.ProductNo) &&
		safeID.MatchString(value.ProductVersionNo) && value.UnitSequence >= 1 && value.Quantity >= 1 &&
		value.Quantity <= 1000 && value.UnitSequence <= value.Quantity &&
		strings.TrimSpace(value.ConfigurationRevision) != "" && safeID.MatchString(value.CustomerConfigurationID)
}

func validJSONObject(value json.RawMessage) bool {
	if len(value) == 0 || len(value) > MaximumPayloadBytes || !json.Valid(value) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func ValidateResult(action Action, value *Result) error {
	if value == nil || value.Version != RequestVersion {
		return fmt.Errorf("product fulfillment result is invalid")
	}
	if !validResultStatus(value.Status) {
		return fmt.Errorf("product fulfillment result status is invalid")
	}
	if !validResultAction(action) {
		return fmt.Errorf("product fulfillment action is invalid")
	}
	if !validActionStatus(action, value.Status) {
		return fmt.Errorf("product fulfillment result does not match its action")
	}
	if (value.Status == StatusNotFound || value.Status == StatusNotAccepted) && value.ExternalReference != "" {
		return fmt.Errorf("negative product fulfillment result cannot carry an external reference")
	}
	if value.ExternalReference != "" && !safeID.MatchString(value.ExternalReference) {
		return fmt.Errorf("product fulfillment external reference is invalid")
	}
	if len(value.Evidence) > MaximumEvidenceBytes || (len(value.Evidence) > 0 && !json.Valid(value.Evidence)) {
		return fmt.Errorf("product fulfillment evidence is invalid")
	}
	if (value.Status == StatusAccepted || value.Status == StatusRunning) && value.ExternalReference == "" {
		return fmt.Errorf("accepted product fulfillment result requires an external reference")
	}
	if len(value.Deliveries) > 1000 || (len(value.Deliveries) > 0 && value.Status != StatusSucceeded) {
		return fmt.Errorf("product fulfillment deliveries are invalid")
	}
	seen := make(map[string]bool, len(value.Deliveries))
	for _, delivery := range value.Deliveries {
		key := delivery.UpstreamDeliveryNo + ":" + delivery.UpstreamServiceNo
		if len(delivery.UpstreamDeliveryNo) > 40 || len(delivery.UpstreamServiceNo) > 40 || !safeID.MatchString(delivery.UpstreamDeliveryNo) || (delivery.UpstreamServiceNo != "" && !safeID.MatchString(delivery.UpstreamServiceNo)) || delivery.StartedAt < 1 || delivery.ExpiresAt < 0 || (delivery.ExpiresAt > 0 && delivery.ExpiresAt <= delivery.StartedAt) || seen[key] {
			return fmt.Errorf("product fulfillment delivery is invalid")
		}
		seen[key] = true
	}
	return nil
}

func validResultStatus(status ResultStatus) bool {
	switch status {
	case StatusNotFound, StatusNotAccepted, StatusAccepted, StatusRunning, StatusSucceeded, StatusFailed, StatusResultUnknown:
		return true
	default:
		return false
	}
}

func validResultAction(action Action) bool { return action == ActionQuery || action == ActionSubmit }

func validActionStatus(action Action, status ResultStatus) bool {
	return (status != StatusNotFound || action == ActionQuery) &&
		(status != StatusNotAccepted || action == ActionSubmit)
}
