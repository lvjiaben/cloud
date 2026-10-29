// This file reuses the V10 console procurement workflow with generic common
// options. DELETE performs the source correlation check before any cart write;
// an ambiguous or already-paid response never falls through to a new order.
package v10

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func executeCommonProvision(executor confidentialoutboundcap.Service, contextID string, request resourcecap.DynamicOperationRequest) (json.RawMessage, error) {
	var parameters commonPurchaseParameters
	if strictDecode(request.Parameters, &parameters) != nil || parameters.Module != "idcsmart_common" || parameters.Workflow != "console_cart_v1" || !positiveDecimal(parameters.PID) || parameters.Form == nil || parameters.BillingCycle != "free" && parameters.BillingCycle != "onetime" && !positiveDecimal(parameters.BillingCycle) || parameters.Form["config_options[cycle]"] != parameters.BillingCycle {
		return nil, errInvalidRequest
	}
	if err := validateCommonFrozenForm(parameters.Form); err != nil {
		return nil, err
	}
	cleanup := map[string]string{"downstream_url": request.ProviderCorrelation.DownstreamURL, "downstream_host_id": strconv.FormatInt(request.ProviderCorrelation.DownstreamID, 10)}
	response, err := businessCall(executor, contextID, 2, http.MethodDelete, "/console/v1/cart", nil, cleanup, map[string]string{"downstream_token": "downstream_token"})
	if err != nil || !successful(response) {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	if orderID := nestedString(response.Data, "data", "order_id"); positiveDecimal(orderID) {
		return payOrder(executor, contextID, 3, request.RequestID, orderID, "")
	}
	cart := map[string]string{"product_id": parameters.PID, "qty": "1", "is_downstream": "1"}
	for key, value := range parameters.Form {
		cart[key] = value
	}
	response, err = businessCall(executor, contextID, 3, http.MethodPost, "/console/v1/cart", nil, cart, nil)
	if err != nil || !successful(response) {
		return json.Marshal(failedResult(request.RequestID, ""))
	}
	settle := map[string]string{"positions[0]": "0", "downstream_url": request.ProviderCorrelation.DownstreamURL, "downstream_host_id": strconv.FormatInt(request.ProviderCorrelation.DownstreamID, 10), "downstream_system_type": "linapro", "is_downstream": "1"}
	response, err = businessCall(executor, contextID, 4, http.MethodPost, "/console/v1/cart/settle", nil, settle, map[string]string{"downstream_token": "downstream_token"})
	if err != nil || response.StatusCode >= 500 {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	if !successful(response) {
		return json.Marshal(failedResult(request.RequestID, ""))
	}
	hostID := firstArrayScalar(response.Data, "data", "host_ids")
	orderID := nestedString(response.Data, "data", "order_id")
	amount := nestedString(response.Data, "data", "amount")
	zero, zeroErr := commonAmount(json.RawMessage(strconv.Quote(amount)), nil)
	if zeroErr == nil && zero == "0" && positiveDecimal(hostID) {
		return json.Marshal(succeededResult(request.RequestID, hostID, "V10 supplier provisioned the product"))
	}
	if !positiveDecimal(orderID) {
		return json.Marshal(unknownResult(request.RequestID, hostID))
	}
	return payOrder(executor, contextID, 5, request.RequestID, orderID, hostID)
}
func validateCommonFrozenForm(form map[string]string) error {
	if len(form) > 2048 {
		return errInvalidRequest
	}
	for key, value := range form {
		if key == "config_options[cycle]" {
			continue
		}
		valid := commonConfigFormKey.MatchString(key) || commonCustomFormKey.MatchString(key)
		if !valid || len(value) > 8192 || strings.ContainsRune(value, 0) {
			return errInvalidRequest
		}
	}
	return nil
}
