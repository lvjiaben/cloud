// Package productdeliverycap defines the customer-safe delivery directory
// published by the commerce owner to exact product plugins.
package productdeliverycap

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	OwnerPluginID          = "idc-core"
	ServiceName            = "product-delivery-read"
	ContractVersion        = "v1"
	SourceContractPackage  = "lina-plugin-idc-core/backend/cap/productdeliverycap"
	DynamicContractPackage = SourceContractPackage + "/bridge"
	MaximumPageSize        = 50
	MaximumPayloadBytes    = 64 * 1024
)

const (
	MethodCurrentCustomerDeliveriesList = "current-customer-deliveries.list"
	MethodCurrentCustomerDeliveryGet    = "current-customer-delivery.get"
)

var (
	publicReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$`)
	cursorPattern          = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// CurrentCustomerDeliveryListRequest selects one bounded page. Customer and
// plugin identities are always taken from the trusted invocation context.
type CurrentCustomerDeliveryListRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

// CurrentCustomerDeliveryGetRequest selects one delivery by its opaque public
// routing key. The key is not intended for display.
type CurrentCustomerDeliveryGetRequest struct {
	DeliveryNo string `json:"deliveryNo"`
}

// CurrentCustomerDeliveryItem omits customer, order, interface, entitlement,
// supplier evidence and configuration values.
type CurrentCustomerDeliveryItem struct {
	DeliveryNo        string `json:"deliveryNo"`
	ProductNameZhCN   string `json:"productNameZhCn"`
	ProductNameEnUS   string `json:"productNameEnUs"`
	Status            string `json:"status"`
	Quantity          int    `json:"quantity"`
	Ongoing           bool   `json:"ongoing"`
	ProviderReference string `json:"providerReference,omitempty"`
	StartedAt         int64  `json:"startedAt,omitempty"`
	CompletedAt       int64  `json:"completedAt,omitempty"`
	UpdatedAt         int64  `json:"updatedAt"`
}

type CurrentCustomerDeliveryPage struct {
	Items      []CurrentCustomerDeliveryItem `json:"items"`
	NextCursor string                        `json:"nextCursor,omitempty"`
}

type CurrentCustomerDeliveryResult struct {
	Item CurrentCustomerDeliveryItem `json:"item"`
}

// CurrentCustomerDeliveryService is implemented by idc-core. CallerPluginID
// comes from the host capability envelope and must never come from a payload.
type CurrentCustomerDeliveryService interface {
	ListCurrentCustomerDeliveries(context.Context, string, CurrentCustomerDeliveryListRequest) (*CurrentCustomerDeliveryPage, error)
	GetCurrentCustomerDelivery(context.Context, string, CurrentCustomerDeliveryGetRequest) (*CurrentCustomerDeliveryResult, error)
}

func ValidateListRequest(request CurrentCustomerDeliveryListRequest) error {
	if request.Limit < 1 || request.Limit > MaximumPageSize || len(request.Cursor) > 128 ||
		(request.Cursor != "" && !cursorPattern.MatchString(request.Cursor)) {
		return gerror.New("productdeliverycap: list request is invalid")
	}
	return nil
}

func ValidateGetRequest(request CurrentCustomerDeliveryGetRequest) error {
	if !validReference(request.DeliveryNo, 40) {
		return gerror.New("productdeliverycap: detail request is invalid")
	}
	return nil
}

func ValidatePage(page *CurrentCustomerDeliveryPage, request CurrentCustomerDeliveryListRequest) error {
	if ValidateListRequest(request) != nil || page == nil || len(page.Items) > request.Limit ||
		(page.NextCursor != "" && !cursorPattern.MatchString(page.NextCursor)) {
		return gerror.New("productdeliverycap: page is invalid")
	}
	seen := make(map[string]struct{}, len(page.Items))
	for _, item := range page.Items {
		if err := validateItem(item); err != nil {
			return err
		}
		if _, exists := seen[item.DeliveryNo]; exists {
			return gerror.New("productdeliverycap: page contains a duplicate")
		}
		seen[item.DeliveryNo] = struct{}{}
	}
	return nil
}

func ValidateResult(result *CurrentCustomerDeliveryResult, request CurrentCustomerDeliveryGetRequest) error {
	if ValidateGetRequest(request) != nil || result == nil || result.Item.DeliveryNo != request.DeliveryNo {
		return gerror.New("productdeliverycap: result is invalid")
	}
	return validateItem(result.Item)
}

func validateItem(item CurrentCustomerDeliveryItem) error {
	if !validReference(item.DeliveryNo, 40) || !validLabel(item.ProductNameZhCN) ||
		!validLabel(item.ProductNameEnUS) || !validStatus(item.Status) || item.Quantity < 1 ||
		item.Quantity > 1000 || item.UpdatedAt < 1 || item.StartedAt < 0 || item.CompletedAt < 0 ||
		(item.ProviderReference != "" && !validReference(item.ProviderReference, 256)) {
		return gerror.New("productdeliverycap: delivery projection is invalid")
	}
	return nil
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && publicReferencePattern.MatchString(value)
}

func validLabel(value string) bool {
	count := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && value == strings.TrimSpace(value) && count >= 1 && count <= 200
}

func validStatus(value string) bool {
	switch value {
	case "awaiting_payment", "awaiting_review", "processing", "manual_processing", "completed", "not_completed", "cancelled":
		return true
	default:
		return false
	}
}
