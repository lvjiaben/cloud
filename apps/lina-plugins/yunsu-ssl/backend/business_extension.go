// This file implements the governed Yunsu SSL product-fulfillment guest handler.

package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

const (
	pluginID                      = "yunsu-ssl"
	ownerPluginID                 = "idc-core"
	candidateMethod               = "configuration.candidates"
	validationMethod              = "configuration.validate"
	provisionMethod               = "provision"
	queryMethod                   = "query"
	serviceDetailMethod           = "service.detail"
	serviceSyncMethod             = "service.sync"
	serviceUpdateValidationMethod = "service.validation.update"
	serviceRevalidateMethod       = "service.revalidate"
	serviceCancelMethod           = "service.cancel"
	serviceDownloadMethod         = "service.download"
	maximumGuestInput             = 1 << 20
)

var safeReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$`)

var productResponseFields = []string{
	"success", "data", "error", "product_id", "name", "brand", "type",
	"file_validation_only", "dns_validation_only",
}

var orderResponseFields = []string{
	"success", "data", "error", "order_id", "status", "expires_at", "issued_at",
	"validation_info", "dcv_info", "dcv", "method", "record_type", "type", "rr",
	"host", "name", "value", "content", "body", "filename", "file_name", "path",
}

// NewGuestHandler validates the exact governed product-fulfillment invocation.
// Supplier endpoint and API credentials are resolved only by idc-core's host proxy.
func NewGuestHandler(interfaces supplierinterfacecap.Service) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if interfaces == nil || request == nil || request.Route == nil || request.Request == nil || request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("SSL connector invocation was not found"), nil
		}
		invocation, err := decodeInvocation(request.Request.Body)
		if err != nil || !validInvocation(invocation) {
			return protocol.NewBadRequestResponse("SSL connector invocation is invalid"), nil
		}
		result, err := executeInvocation(interfaces, invocation)
		if err != nil {
			return protocol.NewInternalErrorResponse("SSL supplier operation could not be completed"), nil
		}
		body, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{
			Version: capmodel.BusinessExtensionInvocationProtocolV1,
			Payload: result,
		})
		if err != nil {
			return protocol.NewInternalErrorResponse("SSL connector result could not be encoded"), nil
		}
		return protocol.NewJSONResponse(http.StatusOK, body), nil
	}
}

func decodeInvocation(content []byte) (*capmodel.BusinessExtensionBridgeRequest, error) {
	if len(content) == 0 || len(content) > maximumGuestInput {
		return nil, io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var invocation capmodel.BusinessExtensionBridgeRequest
	if err := decoder.Decode(&invocation); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, err
	}
	return &invocation, nil
}

func validInvocation(invocation *capmodel.BusinessExtensionBridgeRequest) bool {
	if invocation == nil || invocation.Version != capmodel.BusinessExtensionInvocationProtocolV1 || invocation.ProviderPluginID != pluginID || invocation.OwnerPluginID != ownerPluginID ||
		invocation.Point != capmodel.BusinessExtensionProductFulfillment || invocation.ContractVersion != productfulfillmentcap.ContractVersion || invocation.RuntimeContext == nil || invocation.RuntimeContext.ContextID == "" ||
		invocation.ReleaseVersion == "" || len(invocation.ReleaseDigest) != 64 || len(invocation.DeclarationDigest) != 64 {
		return false
	}
	switch invocation.Method {
	case candidateMethod:
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness || invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	case validationMethod, productfulfillmentcap.MethodValidateProductConfiguration:
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness || invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	case provisionMethod, queryMethod, serviceDetailMethod, serviceSyncMethod,
		serviceUpdateValidationMethod, serviceRevalidateMethod, serviceCancelMethod:
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	default:
		return false
	}
}

func executeInvocation(interfaces supplierinterfacecap.Service, invocation *capmodel.BusinessExtensionBridgeRequest) (json.RawMessage, error) {
	switch invocation.Method {
	case candidateMethod:
		return executeCandidates(interfaces, invocation)
	case productfulfillmentcap.MethodValidateProductConfiguration:
		return executeProductConfigurationValidation(invocation.Payload)
	case validationMethod, provisionMethod, queryMethod:
		var request productfulfillmentcap.Request
		if strictJSON(invocation.Payload, &request) != nil || productfulfillmentcap.ValidateRequest(&request) != nil {
			return nil, io.ErrUnexpectedEOF
		}
		if invocation.Method == validationMethod {
			return executeConfigurationValidation(request)
		}
		if invocation.Method == provisionMethod {
			return executeProvision(interfaces, invocation.RuntimeContext.ContextID, request)
		}
		return executeQuery(interfaces, invocation.RuntimeContext.ContextID, request)
	case serviceDetailMethod:
		return ExecuteServiceDetailAdapter(interfaces, invocation.RuntimeContext.ContextID, invocation.Payload)
	case serviceSyncMethod:
		return ExecuteServiceSyncAdapter(interfaces, invocation.RuntimeContext.ContextID, invocation.Payload)
	case serviceUpdateValidationMethod:
		return ExecuteUpdateValidationAdapter(interfaces, invocation.RuntimeContext.ContextID, invocation.Payload)
	case serviceRevalidateMethod:
		return ExecuteRevalidateAdapter(interfaces, invocation.RuntimeContext.ContextID, invocation.Payload)
	case serviceCancelMethod:
		return ExecuteCancelApplicationAdapter(interfaces, invocation.RuntimeContext.ContextID, invocation.Payload)
	default:
		return nil, io.ErrUnexpectedEOF
	}
}

type candidateRequest struct {
	SchemaVersion string `json:"schemaVersion"`
	Requests      []struct {
		FieldKey string `json:"fieldKey"`
	} `json:"requests"`
}

type candidateResponse struct {
	SchemaVersion string          `json:"schemaVersion"`
	Items         []candidateItem `json:"items"`
}

type candidateItem struct {
	FieldKey   string               `json:"fieldKey"`
	Candidates []certificateProduct `json:"candidates"`
}

type certificateProduct struct {
	Value string                 `json:"value"`
	Label capmodel.LocalizedText `json:"label"`
}

type frozenCertificatePlan struct {
	Version                    string `json:"version"`
	SupplierPlanID             string `json:"supplierPlanId"`
	CertificateValidationLevel string `json:"certificateValidationLevel"`
	DNSValidationSupported     bool   `json:"dnsValidationSupported"`
	FileValidationSupported    bool   `json:"fileValidationSupported"`
}

func executeCandidates(interfaces supplierinterfacecap.Service, invocation *capmodel.BusinessExtensionBridgeRequest) (json.RawMessage, error) {
	var request candidateRequest
	if json.Unmarshal(invocation.Payload, &request) != nil || request.SchemaVersion != "v1" || len(request.Requests) != 1 || request.Requests[0].FieldKey != "plan_id" {
		return nil, io.ErrUnexpectedEOF
	}
	body := json.RawMessage(`{}`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := interfaces.ExecuteOutboundHTTP(ctx, invocation.RuntimeContext.ContextID, supplierinterfacecap.OutboundHTTPRequest{
		Method: http.MethodPost, Path: "/product/list", EndpointField: "endpoint",
		Body: body, SecretHeaders: yunsuSecretHeaders(), ResponseFields: productResponseFields,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, io.ErrUnexpectedEOF
	}
	products, err := decodeProducts(response.Data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(candidateResponse{SchemaVersion: "v1", Items: []candidateItem{{FieldKey: "plan_id", Candidates: products}}})
}

func decodeProducts(payload json.RawMessage) ([]certificateProduct, error) {
	var envelope struct {
		Success bool `json:"success"`
		Data    []struct {
			ProductID json.RawMessage `json:"product_id"`
			Name      string          `json:"name"`
			Brand     string          `json:"brand"`
			Type      string          `json:"type"`
			FileOnly  json.RawMessage `json:"file_validation_only"`
			DNSOnly   json.RawMessage `json:"dns_validation_only"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &envelope) != nil || !envelope.Success || len(envelope.Data) == 0 || len(envelope.Data) > 512 {
		return nil, io.ErrUnexpectedEOF
	}
	result := make([]certificateProduct, 0, len(envelope.Data))
	seen := make(map[string]struct{}, len(envelope.Data))
	for _, item := range envelope.Data {
		productID := scalarString(item.ProductID)
		name := strings.TrimSpace(item.Name)
		brand := strings.TrimSpace(item.Brand)
		certificateType := strings.ToLower(strings.TrimSpace(item.Type))
		fileOnly, fileErr := scalarBool(item.FileOnly)
		dnsOnly, dnsErr := scalarBool(item.DNSOnly)
		if !safeReference.MatchString(productID) || name == "" || len(name) > 255 || strings.ContainsAny(name, "\x00\r\n") || len(brand) > 128 || strings.ContainsAny(brand, "\x00\r\n") ||
			(certificateType != "dv" && certificateType != "ov" && certificateType != "ev") || fileErr != nil || dnsErr != nil || fileOnly && dnsOnly {
			return nil, io.ErrUnexpectedEOF
		}
		plan := frozenCertificatePlan{
			Version: "v1", SupplierPlanID: productID, CertificateValidationLevel: certificateType,
			DNSValidationSupported: !fileOnly, FileValidationSupported: !dnsOnly,
		}
		value, err := encodeFrozenPlan(plan)
		if err != nil {
			return nil, err
		}
		if _, duplicated := seen[value]; duplicated {
			return nil, io.ErrUnexpectedEOF
		}
		seen[value] = struct{}{}
		label := name
		if brand != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(brand)) {
			label = brand + " " + name
		}
		label += " (" + strings.ToUpper(certificateType) + ")"
		result = append(result, certificateProduct{Value: value, Label: capmodel.LocalizedText{ZhCN: label, EnUS: label}})
	}
	return result, nil
}

func executeProvision(interfaces supplierinterfacecap.Service, contextID string, request productfulfillmentcap.Request) (json.RawMessage, error) {
	product, customer, err := decodeConfigurations(request)
	if err != nil {
		return nil, err
	}
	domains, err := validateDomains(product, customer)
	if err != nil {
		return nil, err
	}
	orderData := map[string]any{
		"validation": customer.ValidationMethod,
		"email":      customer.Email,
		"phone":      customer.Phone,
		"period":     12,
	}
	copyOptionalOrderFields(orderData, customer)
	body, err := json.Marshal(map[string]any{"product_id": product.SupplierPlanID, "domain": strings.Join(domains, ","), "order_data": orderData})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := interfaces.ExecuteOutboundHTTP(ctx, contextID, supplierinterfacecap.OutboundHTTPRequest{
		Method: http.MethodPost, Path: "/payment/balance", EndpointField: "endpoint",
		Body: body, SecretHeaders: yunsuSecretHeaders(), ResponseFields: orderResponseFields,
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Success bool            `json:"success"`
		OrderID json.RawMessage `json:"order_id"`
	}
	if json.Unmarshal(response.Data, &envelope) != nil {
		return nil, io.ErrUnexpectedEOF
	}
	orderID := scalarString(envelope.OrderID)
	status := productfulfillmentcap.StatusNotAccepted
	if response.StatusCode >= 200 && response.StatusCode < 300 && envelope.Success && safeReference.MatchString(orderID) {
		status = productfulfillmentcap.StatusAccepted
	}
	result := &productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: status, ExternalReference: orderID}
	if status == productfulfillmentcap.StatusNotAccepted {
		result.ExternalReference = ""
	}
	if productfulfillmentcap.ValidateResult(productfulfillmentcap.ActionSubmit, result) != nil {
		return nil, io.ErrUnexpectedEOF
	}
	return json.Marshal(result)
}

func executeQuery(interfaces supplierinterfacecap.Service, contextID string, request productfulfillmentcap.Request) (json.RawMessage, error) {
	if request.PreviousExternalReference == "" {
		return json.Marshal(&productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: productfulfillmentcap.StatusNotFound})
	}
	body, err := json.Marshal(map[string]string{"order_id": request.PreviousExternalReference})
	if err != nil {
		return nil, gerror.Wrap(err, "encode Yunsu SSL order query")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := interfaces.ExecuteOutboundHTTP(ctx, contextID, supplierinterfacecap.OutboundHTTPRequest{
		Method: http.MethodPost, Path: "/order/sync", EndpointField: "endpoint",
		Body: body, SecretHeaders: yunsuSecretHeaders(), ResponseFields: orderResponseFields,
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if json.Unmarshal(response.Data, &envelope) != nil {
		return nil, io.ErrUnexpectedEOF
	}
	status := mapOrderStatus(response.StatusCode, envelope.Success, envelope.Data.Status)
	evidence, err := json.Marshal(map[string]string{"upstreamStatus": strings.ToLower(strings.TrimSpace(envelope.Data.Status))})
	if err != nil {
		return nil, gerror.Wrap(err, "encode Yunsu SSL query evidence")
	}
	result := &productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: status, ExternalReference: request.PreviousExternalReference, Evidence: evidence}
	if status == productfulfillmentcap.StatusNotFound {
		result.ExternalReference = ""
	}
	if productfulfillmentcap.ValidateResult(productfulfillmentcap.ActionQuery, result) != nil {
		return nil, io.ErrUnexpectedEOF
	}
	return json.Marshal(result)
}

type productConfiguration struct {
	PlanID                     string `json:"plan_id"`
	CertificateScope           string `json:"certificate_scope"`
	SupplierPlanID             string `json:"-"`
	CertificateValidationLevel string `json:"-"`
	DNSValidationSupported     bool   `json:"-"`
	FileValidationSupported    bool   `json:"-"`
}

type customerConfiguration struct {
	PrimaryDomain                  string `json:"primary_domain"`
	AdditionalDomains              string `json:"additional_domains"`
	ValidationMethod               string `json:"validation_method"`
	Email                          string `json:"email"`
	Phone                          string `json:"phone"`
	CSR                            string `json:"csr"`
	ContactFirstName               string `json:"contact_first_name"`
	ContactLastName                string `json:"contact_last_name"`
	ContactTitle                   string `json:"contact_title"`
	OrganizationName               string `json:"organization_name"`
	OrganizationPhone              string `json:"organization_phone"`
	OrganizationAddress            string `json:"organization_address"`
	OrganizationCity               string `json:"organization_city"`
	OrganizationState              string `json:"organization_state"`
	OrganizationCountry            string `json:"organization_country"`
	OrganizationPostcode           string `json:"organization_postcode"`
	OrganizationRegistrationNumber string `json:"organization_registration_number"`
}

func decodeConfigurations(request productfulfillmentcap.Request) (productConfiguration, customerConfiguration, error) {
	var product productConfiguration
	var customer customerConfiguration
	if strictJSON(request.ProductConfiguration, &product) != nil || hydrateProductConfiguration(&product) != nil || strictJSON(request.CustomerConfiguration, &customer) != nil || validateCertificateConfiguration(product, customer) != nil {
		return productConfiguration{}, customerConfiguration{}, io.ErrUnexpectedEOF
	}
	return product, customer, nil
}

func encodeFrozenPlan(plan frozenCertificatePlan) (string, error) {
	content, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	return "ysp1." + base64.RawURLEncoding.EncodeToString(content), nil
}

func hydrateProductConfiguration(product *productConfiguration) error {
	if product == nil || !strings.HasPrefix(product.PlanID, "ysp1.") {
		return io.ErrUnexpectedEOF
	}
	content, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(product.PlanID, "ysp1."))
	if err != nil || len(content) > 1024 {
		return io.ErrUnexpectedEOF
	}
	var plan frozenCertificatePlan
	if strictJSON(content, &plan) != nil || plan.Version != "v1" || !safeReference.MatchString(plan.SupplierPlanID) ||
		(plan.CertificateValidationLevel != "dv" && plan.CertificateValidationLevel != "ov" && plan.CertificateValidationLevel != "ev") ||
		(!plan.DNSValidationSupported && !plan.FileValidationSupported) {
		return io.ErrUnexpectedEOF
	}
	product.SupplierPlanID = plan.SupplierPlanID
	product.CertificateValidationLevel = plan.CertificateValidationLevel
	product.DNSValidationSupported = plan.DNSValidationSupported
	product.FileValidationSupported = plan.FileValidationSupported
	return nil
}

func scalarBool(value json.RawMessage) (bool, error) {
	var boolean bool
	if json.Unmarshal(value, &boolean) == nil {
		return boolean, nil
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		switch number.String() {
		case "0":
			return false, nil
		case "1":
			return true, nil
		}
	}
	return false, io.ErrUnexpectedEOF
}

func strictJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return err
	}
	return nil
}

func validateDomains(product productConfiguration, customer customerConfiguration) ([]string, error) {
	return normalizedDomainList(product, customer)
}

func copyOptionalOrderFields(target map[string]any, customer customerConfiguration) {
	fields := map[string]string{
		"csr": strings.TrimSpace(customer.CSR), "contact_first_name": customer.ContactFirstName,
		"contact_last_name": customer.ContactLastName, "contact_title": customer.ContactTitle,
		"organization_name": customer.OrganizationName, "organization_phone": customer.OrganizationPhone,
		"organization_address": customer.OrganizationAddress, "organization_city": customer.OrganizationCity,
		"organization_state": customer.OrganizationState, "organization_country": customer.OrganizationCountry,
		"organization_postcode":            customer.OrganizationPostcode,
		"organization_registration_number": customer.OrganizationRegistrationNumber,
	}
	for key, value := range fields {
		if value = strings.TrimSpace(value); value != "" {
			target[key] = value
		}
	}
}

func mapOrderStatus(statusCode int, success bool, status string) productfulfillmentcap.ResultStatus {
	if statusCode == http.StatusNotFound {
		return productfulfillmentcap.StatusNotFound
	}
	if statusCode < 200 || statusCode >= 300 || !success {
		return productfulfillmentcap.StatusResultUnknown
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "issued":
		return productfulfillmentcap.StatusSucceeded
	case "failed", "cancelled", "canceled", "rejected", "revoked":
		return productfulfillmentcap.StatusFailed
	case "pending", "processing", "verifying", "validation", "awaiting_validation":
		return productfulfillmentcap.StatusRunning
	default:
		return productfulfillmentcap.StatusResultUnknown
	}
}

func scalarString(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}

func yunsuSecretHeaders() map[string]string {
	return map[string]string{"X-API-Key": "api_key", "X-API-Secret": "api_secret"}
}
