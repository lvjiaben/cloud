// This file defines the signed local process contract for exact Tencent status deliveries.
package processauth

const (
	TencentContractDeliveryPagesPath            = "/internal/v1/tencent-essbasic/contract-flow-delivery-pages"
	TencentContractDeliveryPageLimitQuery       = "limit"
	TencentContractDeliveryReadScope            = "tencent-essbasic.contract-flow-delivery.read"
	TencentContractDeliveryAcknowledgementsPath = "/internal/v1/tencent-essbasic/contract-flow-delivery-acknowledgements"
	TencentContractDeliveryAcknowledgeScope     = "tencent-essbasic.contract-flow-delivery.acknowledge"
)

type TencentContractDeliveryItem struct {
	DeliveryNo           string `json:"deliveryNo"`
	RouteNo              string `json:"routeNo"`
	RouteVersion         int64  `json:"routeVersion"`
	Status               string `json:"status"`
	ProviderStatus       string `json:"providerStatus"`
	ReconciliationStatus string `json:"reconciliationStatus"`
	SourceKind           string `json:"sourceKind"`
	DecisionCode         string `json:"decisionCode"`
	SupplierObservedUnix int64  `json:"supplierObservedUnix"`
	ObservedUnix         int64  `json:"observedUnix"`
}

type TencentContractDeliveryPageResponse struct {
	Items []TencentContractDeliveryItem `json:"items"`
}

type TencentContractDeliveryAcknowledgementRequest struct {
	DeliveryNos []string `json:"deliveryNos"`
}

type TencentContractDeliveryAcknowledgementResponse struct {
	Acknowledged int `json:"acknowledged"`
}
