// This file validates bounded customer search inputs and output projections.

package customermanagement

import (
	"encoding/json"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const (
	defaultMaximumPageSize       = 25
	absoluteMaximumPageSize      = 100
	defaultMaximumResponseBytes  = 512 * 1024
	absoluteMaximumResponseBytes = 4 * 1024 * 1024
	maximumNamePrefixRunes       = 256
)

type normalizedSearch struct {
	customerCode string
	namePrefix   string
	phone        string
	typeCode     customer.Type
	status       customer.Status
	groupNo      string
	tagNo        string
	cursor       *cursorBoundary
	limit        int
	page         int
}

func normalizeConfig(config Config) (Config, error) {
	if config.MaximumPageSize < 0 || config.MaximumResponseBytes < 0 {
		return Config{}, gerror.New("customer management configuration cannot be negative")
	}
	if config.MaximumPageSize == 0 {
		config.MaximumPageSize = absoluteMaximumPageSize
	}
	if config.MaximumResponseBytes == 0 {
		config.MaximumResponseBytes = defaultMaximumResponseBytes
	}
	if config.MaximumPageSize > absoluteMaximumPageSize ||
		config.MaximumResponseBytes > absoluteMaximumResponseBytes {
		return Config{}, gerror.New("customer management configuration exceeds safety bounds")
	}
	return config, nil
}

func normalizeSearch(input SearchInput, maximumPageSize int) (normalizedSearch, error) {
	customerCode := strings.TrimSpace(input.CustomerCode)
	namePrefix := strings.ToLower(strings.Join(strings.Fields(input.NamePrefix), " "))
	phone := strings.TrimSpace(input.Phone)
	if !utf8.ValidString(namePrefix) || utf8.RuneCountInString(namePrefix) > maximumNamePrefixRunes ||
		!validDigits(customerCode, 1, 18) || !validDigits(phone, 6, 20) ||
		!validType(input.Type) || !validStatus(input.Status) ||
		!validOptionalSegmentNo(input.GroupNo, "GRP_") ||
		!validOptionalSegmentNo(input.TagNo, "TAG_") {
		return normalizedSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	if input.Limit == 0 {
		input.Limit = min(defaultMaximumPageSize, maximumPageSize)
	}
	if input.Limit < 1 || input.Limit > maximumPageSize || input.Page < 0 ||
		(input.Page > 0 && (input.Cursor != "" || input.Page-1 > math.MaxInt/input.Limit)) {
		return normalizedSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	normalized := normalizedSearch{
		customerCode: customerCode, namePrefix: namePrefix, phone: phone, typeCode: input.Type, status: input.Status,
		groupNo: input.GroupNo, tagNo: input.TagNo, limit: input.Limit, page: input.Page,
	}
	if input.Cursor != "" {
		cursor, err := decodeCustomerCursor(input.Cursor, normalized)
		if err != nil {
			return normalizedSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
		}
		normalized.cursor = cursor
	}
	return normalized, nil
}

func validDigits(value string, minimum, maximum int) bool {
	if value == "" {
		return true
	}
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validOptionalSegmentNo(value string, prefix string) bool {
	if value == "" {
		return true
	}
	if len(value) != len(prefix)+32 || !strings.HasPrefix(value, prefix) {
		return false
	}
	for _, character := range value[len(prefix):] {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validType(value customer.Type) bool {
	return value == "" || value == customer.TypeIndividual || value == customer.TypeEnterprise
}

func validStatus(value customer.Status) bool {
	switch value {
	case "", customer.StatusPendingActivation, customer.StatusActive,
		customer.StatusRestricted, customer.StatusDisabled, customer.StatusAnonymized:
		return true
	default:
		return false
	}
}

func validatePage(page *Page, maximumPageSize int, maximumResponseBytes int) error {
	if page == nil || page.Customers == nil || len(page.Customers) > maximumPageSize ||
		(page.NextCursor != "" && len(page.NextCursor) > maximumEncodedCursorLength) {
		return gerror.New("customer management page projection is invalid")
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		return gerror.Wrap(err, "measure customer management page")
	}
	if len(encoded) > maximumResponseBytes {
		return gerror.New("customer management page exceeds response safety bound")
	}
	return nil
}

func validateOverview(overview *Overview, maximumResponseBytes int) error {
	if overview == nil || !validPersistedCustomerNo(overview.Customer.CustomerNo) ||
		overview.Groups.Items == nil || overview.Tags.Items == nil ||
		len(overview.Groups.Items) > maximumSegmentPreviewSize ||
		len(overview.Tags.Items) > maximumSegmentPreviewSize ||
		overview.Balances == nil || len(overview.Balances) > 64 {
		return gerror.New("customer management overview projection is invalid")
	}
	for _, balance := range overview.Balances {
		if balance.AccountTypeCode == "" || len(balance.AccountTypeCode) > 32 ||
			len(balance.CurrencyCode) != 3 || balance.StatusCode == "" || len(balance.StatusCode) > 24 ||
			balance.BalanceMinor < 0 || balance.HeldBalanceMinor < 0 ||
			balance.AvailableBalanceMinor < 0 ||
			balance.HeldBalanceMinor+balance.AvailableBalanceMinor != balance.BalanceMinor ||
			balance.UpdatedAt.IsZero() {
			return gerror.New("customer management balance projection is invalid")
		}
	}
	encoded, err := json.Marshal(overview)
	if err != nil {
		return gerror.Wrap(err, "measure customer management overview")
	}
	if len(encoded) > maximumResponseBytes {
		return gerror.New("customer management overview exceeds response safety bound")
	}
	return nil
}
