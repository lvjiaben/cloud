package backend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

type serviceActionRequest struct {
	EntitlementNo        string
	IdempotencyKey       string
	ExternalReference    string
	ProductConfiguration productConfiguration
	ValidationMethod     string
	RequestedActionKey   string
}

type certificateServiceDetail struct {
	Version           string                      `json:"version"`
	ExternalReference string                      `json:"externalReference"`
	UpstreamStatus    string                      `json:"upstreamStatus"`
	IssuedAt          string                      `json:"issuedAt,omitempty"`
	ExpiresAt         string                      `json:"expiresAt,omitempty"`
	Validation        []certificateValidationItem `json:"validation,omitempty"`
}

type certificateValidationItem struct {
	Domain     string `json:"domain,omitempty"`
	Method     string `json:"method,omitempty"`
	RecordType string `json:"recordType,omitempty"`
	Host       string `json:"host,omitempty"`
	Value      string `json:"value,omitempty"`
	FileName   string `json:"fileName,omitempty"`
	Path       string `json:"path,omitempty"`
}

type supplierOrderEnvelope struct {
	Success bool              `json:"success"`
	Data    supplierOrderData `json:"data"`
}

type supplierOrderData struct {
	Status         string          `json:"status"`
	IssuedAt       string          `json:"issued_at"`
	ExpiresAt      string          `json:"expires_at"`
	ValidationInfo json.RawMessage `json:"validation_info"`
	DCVInfo        json.RawMessage `json:"dcv_info"`
	DCV            json.RawMessage `json:"dcv"`
}

type rawValidationRecord struct {
	Domain     string `json:"domain"`
	Method     string `json:"method"`
	RecordType string `json:"record_type"`
	Type       string `json:"type"`
	RR         string `json:"rr"`
	Host       string `json:"host"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	Content    string `json:"content"`
	Body       string `json:"body"`
	Filename   string `json:"filename"`
	FileName   string `json:"file_name"`
	Path       string `json:"path"`
}

func decodeServiceActionRequest(payload json.RawMessage, actionKey string, mutation, requireValidationMethod bool) (serviceActionRequest, error) {
	var envelope productfulfillmentcap.ManagedServiceActionRequest
	if strictJSON(payload, &envelope) != nil || productfulfillmentcap.ValidateManagedServiceActionRequest(&envelope, actionKey, mutation) != nil {
		return serviceActionRequest{}, io.ErrUnexpectedEOF
	}
	var request serviceActionRequest
	request.EntitlementNo, request.IdempotencyKey, request.ExternalReference = envelope.EntitlementNo, envelope.IdempotencyKey, envelope.ExternalReference
	request.RequestedActionKey = envelope.RequestedActionKey
	if strictJSON(envelope.ProductConfiguration, &request.ProductConfiguration) != nil || hydrateProductConfiguration(&request.ProductConfiguration) != nil || !validProductConfiguration(request.ProductConfiguration) {
		return serviceActionRequest{}, io.ErrUnexpectedEOF
	}
	if len(envelope.Input) > 0 {
		var input struct {
			ValidationMethod string `json:"validationMethod,omitempty"`
		}
		if strictJSON(envelope.Input, &input) != nil {
			return serviceActionRequest{}, io.ErrUnexpectedEOF
		}
		request.ValidationMethod = input.ValidationMethod
	}
	if requireValidationMethod || request.RequestedActionKey == "update-validation" {
		if request.ValidationMethod != "dns" && request.ValidationMethod != "file" {
			return serviceActionRequest{}, io.ErrUnexpectedEOF
		}
		if request.ValidationMethod == "dns" && !request.ProductConfiguration.DNSValidationSupported ||
			request.ValidationMethod == "file" && !request.ProductConfiguration.FileValidationSupported ||
			request.ProductConfiguration.CertificateScope == "ip" && request.ValidationMethod != "file" ||
			request.ProductConfiguration.CertificateScope == "wildcard" && request.ValidationMethod != "dns" {
			return serviceActionRequest{}, io.ErrUnexpectedEOF
		}
	} else if request.ValidationMethod != "" && request.RequestedActionKey != "update-validation" {
		return serviceActionRequest{}, io.ErrUnexpectedEOF
	}
	return request, nil
}

// ExecuteServiceDetailAdapter serves only an owner-governed existing-business invocation.
func ExecuteServiceDetailAdapter(interfaces supplierinterfacecap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	request, err := decodeServiceActionRequest(payload, "detail", false, false)
	if err != nil {
		return nil, err
	}
	detail, err := loadCertificateDetail(interfaces, contextID, request.ExternalReference, "/order/detail")
	if err != nil {
		return nil, err
	}
	return encodeServiceActionResult("detail", productfulfillmentcap.ManagedServiceActionStatusProjected, request.ExternalReference, detail)
}

// ExecuteServiceSyncAdapter refreshes one existing certificate through the owner-governed action bridge.
func ExecuteServiceSyncAdapter(interfaces supplierinterfacecap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	request, err := decodeServiceActionRequest(payload, "sync", false, false)
	if err != nil {
		return nil, err
	}
	detail, err := loadCertificateDetail(interfaces, contextID, request.ExternalReference, "/order/sync")
	if err != nil {
		return nil, err
	}
	decision := ""
	if request.RequestedActionKey != "" {
		decision = serviceActionQueryDecision(request, detail)
	}
	return encodeServiceActionResultWithDecision("sync", productfulfillmentcap.ManagedServiceActionStatusProjected, request.ExternalReference, detail, decision)
}

// ExecuteUpdateValidationAdapter changes DNS/file validation only after the
// future owner bridge has authorized the existing service.
func ExecuteUpdateValidationAdapter(interfaces supplierinterfacecap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	request, err := decodeServiceActionRequest(payload, "update-validation", true, true)
	if err != nil {
		return nil, err
	}
	if err = requirePendingCertificate(interfaces, contextID, request.ExternalReference); err != nil {
		return nil, err
	}
	if err = executeSimpleOrderAction(interfaces, contextID, "/order/update-dcv", map[string]string{"order_id": request.ExternalReference, "method": request.ValidationMethod}); err != nil {
		return nil, err
	}
	detail, err := loadCertificateDetail(interfaces, contextID, request.ExternalReference, "/order/sync")
	if err != nil {
		return nil, err
	}
	return encodeServiceActionResult("update-validation", productfulfillmentcap.ManagedServiceActionStatusSucceeded, request.ExternalReference, detail)
}

// ExecuteRevalidateAdapter requests another validation attempt only after the
// future owner bridge has authorized the existing service.
func ExecuteRevalidateAdapter(interfaces supplierinterfacecap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	request, err := decodeServiceActionRequest(payload, "revalidate", true, false)
	if err != nil {
		return nil, err
	}
	if err = requirePendingCertificate(interfaces, contextID, request.ExternalReference); err != nil {
		return nil, err
	}
	if err = executeSimpleOrderAction(interfaces, contextID, "/order/revalidate", map[string]string{"order_id": request.ExternalReference}); err != nil {
		return nil, err
	}
	detail, err := loadCertificateDetail(interfaces, contextID, request.ExternalReference, "/order/sync")
	if err != nil {
		return nil, err
	}
	return encodeServiceActionResult("revalidate", productfulfillmentcap.ManagedServiceActionStatusSucceeded, request.ExternalReference, detail)
}

// ExecuteCancelApplicationAdapter cancels a pre-issuance application without
// refund semantics after the future owner bridge authorizes the service.
func ExecuteCancelApplicationAdapter(interfaces supplierinterfacecap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	request, err := decodeServiceActionRequest(payload, "cancel", true, false)
	if err != nil {
		return nil, err
	}
	if err = requirePendingCertificate(interfaces, contextID, request.ExternalReference); err != nil {
		return nil, err
	}
	if err = executeSimpleOrderAction(interfaces, contextID, "/order/cancel", map[string]string{"order_id": request.ExternalReference}); err != nil {
		return nil, err
	}
	return encodeServiceActionResult("cancel", productfulfillmentcap.ManagedServiceActionStatusSucceeded, request.ExternalReference, nil)
}

func encodeServiceActionResult(actionKey, status, externalReference string, projection any) (json.RawMessage, error) {
	return encodeServiceActionResultWithDecision(actionKey, status, externalReference, projection, "")
}

func encodeServiceActionResultWithDecision(actionKey, status, externalReference string, projection any, decision string) (json.RawMessage, error) {
	var content json.RawMessage
	if projection != nil {
		encoded, err := json.Marshal(projection)
		if err != nil {
			return nil, err
		}
		content = encoded
	}
	result := &productfulfillmentcap.ManagedServiceActionResult{
		Version: productfulfillmentcap.ManagedServiceActionVersion, ActionKey: actionKey,
		Status: status, ExternalReference: externalReference, Projection: content, Decision: decision,
	}
	if err := productfulfillmentcap.ValidateManagedServiceActionResult(result, actionKey, status == productfulfillmentcap.ManagedServiceActionStatusProjected); err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func serviceActionQueryDecision(request serviceActionRequest, detail *certificateServiceDetail) string {
	status := strings.ToLower(strings.TrimSpace(detail.UpstreamStatus))
	switch request.RequestedActionKey {
	case "cancel":
		if status == "cancelled" || status == "canceled" {
			return productfulfillmentcap.ManagedServiceQueryDecisionAlreadySucceeded
		}
	case "update-validation":
		if pendingCertificateStatus(status) && validationMethodAlreadyApplied(detail.Validation, request.ValidationMethod) {
			return productfulfillmentcap.ManagedServiceQueryDecisionAlreadySucceeded
		}
	case "revalidate":
		// Each new idempotency key represents one deliberate retry. Replays of the
		// same key are stopped by the owner task's uniqueness constraint.
	default:
		return productfulfillmentcap.ManagedServiceQueryDecisionForbidden
	}
	if pendingCertificateStatus(status) {
		return productfulfillmentcap.ManagedServiceQueryDecisionSubmitAllowed
	}
	return productfulfillmentcap.ManagedServiceQueryDecisionForbidden
}

func validationMethodAlreadyApplied(items []certificateValidationItem, expected string) bool {
	if expected == "" || len(items) == 0 {
		return false
	}
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item.Method)) != expected {
			return false
		}
	}
	return true
}

func requirePendingCertificate(interfaces supplierinterfacecap.Service, contextID, externalReference string) error {
	detail, err := loadCertificateDetail(interfaces, contextID, externalReference, "/order/sync")
	if err != nil || !pendingCertificateStatus(detail.UpstreamStatus) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func pendingCertificateStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "processing", "verifying", "validation", "awaiting_validation":
		return true
	default:
		return false
	}
}

func executeSimpleOrderAction(interfaces supplierinterfacecap.Service, contextID, path string, input map[string]string) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := interfaces.ExecuteOutboundHTTP(ctx, contextID, supplierinterfacecap.OutboundHTTPRequest{
		Method: http.MethodPost, Path: path, EndpointField: "endpoint", Body: body,
		SecretHeaders: yunsuSecretHeaders(), ResponseFields: orderResponseFields,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return io.ErrUnexpectedEOF
	}
	var result struct {
		Success bool `json:"success"`
	}
	if json.Unmarshal(response.Data, &result) != nil || !result.Success {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func loadCertificateDetail(interfaces supplierinterfacecap.Service, contextID, externalReference, path string) (*certificateServiceDetail, error) {
	body, err := json.Marshal(map[string]string{"order_id": externalReference})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := interfaces.ExecuteOutboundHTTP(ctx, contextID, supplierinterfacecap.OutboundHTTPRequest{
		Method: http.MethodPost, Path: path, EndpointField: "endpoint", Body: body,
		SecretHeaders: yunsuSecretHeaders(), ResponseFields: orderResponseFields,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, io.ErrUnexpectedEOF
	}
	var envelope supplierOrderEnvelope
	if json.Unmarshal(response.Data, &envelope) != nil || !envelope.Success {
		return nil, io.ErrUnexpectedEOF
	}
	status := normalizedEvidenceString(envelope.Data.Status, 64)
	if status == "" {
		return nil, io.ErrUnexpectedEOF
	}
	detail := &certificateServiceDetail{
		Version: productfulfillmentcap.ManagedServiceActionVersion, ExternalReference: externalReference,
		UpstreamStatus: strings.ToLower(status), IssuedAt: normalizedEvidenceString(envelope.Data.IssuedAt, 64),
		ExpiresAt: normalizedEvidenceString(envelope.Data.ExpiresAt, 64),
	}
	for _, raw := range []json.RawMessage{envelope.Data.ValidationInfo, envelope.Data.DCVInfo, envelope.Data.DCV} {
		items, err := decodeValidationItems(raw, 0)
		if err != nil {
			return nil, err
		}
		detail.Validation = append(detail.Validation, items...)
		if len(detail.Validation) > maximumCertificateDomains {
			return nil, io.ErrUnexpectedEOF
		}
	}
	return detail, nil
}

func decodeValidationItems(raw json.RawMessage, depth int) ([]certificateValidationItem, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if depth > 2 {
		return nil, io.ErrUnexpectedEOF
	}
	var record rawValidationRecord
	if json.Unmarshal(raw, &record) == nil {
		item := certificateValidationItem{
			Domain: normalizedEvidenceString(record.Domain, 255), Method: normalizedEvidenceString(record.Method, 16),
			RecordType: firstSafeString(16, record.RecordType, record.Type, record.RR),
			Host:       firstSafeString(255, record.Host, record.Name), Value: firstSafeString(8192, record.Value, record.Content, record.Body),
			FileName: firstSafeString(255, record.Filename, record.FileName), Path: normalizedEvidenceString(record.Path, 1024),
		}
		if item.Domain != "" || item.Method != "" || item.RecordType != "" || item.Host != "" || item.Value != "" || item.FileName != "" || item.Path != "" {
			if item.Path != "" && (!strings.HasPrefix(item.Path, "/.well-known/pki-validation/") || strings.Contains(item.Path, "..") || strings.ContainsAny(item.Path, "\\?#")) {
				return nil, io.ErrUnexpectedEOF
			}
			return []certificateValidationItem{item}, nil
		}
	}
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) == nil {
		return decodeValidationChildren(array, depth)
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || len(object) > maximumCertificateDomains {
		return nil, io.ErrUnexpectedEOF
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	children := make([]json.RawMessage, 0, len(object))
	for _, key := range keys {
		children = append(children, object[key])
	}
	return decodeValidationChildren(children, depth)
}

func decodeValidationChildren(children []json.RawMessage, depth int) ([]certificateValidationItem, error) {
	result := make([]certificateValidationItem, 0, len(children))
	for _, child := range children {
		items, err := decodeValidationItems(child, depth+1)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(result) > maximumCertificateDomains {
			return nil, io.ErrUnexpectedEOF
		}
	}
	return result, nil
}

func normalizedEvidenceString(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n") {
		return ""
	}
	return value
}

func firstSafeString(maximum int, values ...string) string {
	for _, value := range values {
		if value = normalizedEvidenceString(value, maximum); value != "" {
			return value
		}
	}
	return ""
}
