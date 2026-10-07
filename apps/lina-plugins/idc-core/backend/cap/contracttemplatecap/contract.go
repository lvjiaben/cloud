// Package contracttemplatecap defines the IDC core contract-policy template
// directory port implemented by the compliance plugin.
package contracttemplatecap

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

const MaximumTemplates = 200

var (
	ErrSourceUnavailable   = errors.New("contract template source is unavailable")
	ErrTemplateUnavailable = errors.New("contract template is unavailable")
	templateNoPattern      = regexp.MustCompile(`^ctpl_[0-9a-f]{32}$`)
	orderNoPattern         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)
)

type CustomerType string

const (
	CustomerTypeIndividual CustomerType = "individual"
	CustomerTypeEnterprise CustomerType = "enterprise"
)

type SelectionLevel string

const (
	SelectionLevelProduct    SelectionLevel = "product"
	SelectionLevelEnterprise SelectionLevel = "enterprise"
	SelectionLevelOrder      SelectionLevel = "order"
)

// Selection identifies the business setting being edited. Product selection
// deliberately has no customer type because one product can be bought by both
// individuals and enterprises.
type Selection struct {
	Level        SelectionLevel
	CustomerType CustomerType
}

type Template struct {
	TemplateNo       string
	VersionNo        int64
	Name             string
	ScopeCode        string
	SigningOrderCode string
	LanguageCode     string
}

// Source exposes only enabled, installation-local template metadata. It never
// exposes document bodies, credentials, or supplier calls to the core plugin.
type Source interface {
	ListAvailable(context.Context, Selection) ([]Template, error)
	ResolveAvailable(context.Context, Selection, []string) ([]Template, error)
	OrderContractExists(context.Context, string) (bool, error)
}

// OrderContractLockKey is shared by the order-rule owner and contract creator
// so a rule change cannot race a first contract creation.
func OrderContractLockKey(orderNo string) (string, error) {
	if !orderNoPattern.MatchString(orderNo) {
		return "", gerror.New("contract order number is invalid")
	}
	return "contract-create-order:" + orderNo, nil
}

func ValidateCustomerType(value CustomerType) error {
	if value != CustomerTypeIndividual && value != CustomerTypeEnterprise {
		return gerror.New("contract template customer type is invalid")
	}
	return nil
}

func ValidateSelection(value Selection) error {
	switch value.Level {
	case SelectionLevelProduct:
		if value.CustomerType != "" {
			return gerror.New("product contract template selection cannot carry a customer type")
		}
	case SelectionLevelEnterprise:
		if value.CustomerType != CustomerTypeEnterprise {
			return gerror.New("enterprise contract template selection requires an enterprise customer")
		}
	case SelectionLevelOrder:
		if ValidateCustomerType(value.CustomerType) != nil {
			return gerror.New("order contract template selection requires a valid customer type")
		}
	default:
		return gerror.New("contract template selection level is invalid")
	}
	return nil
}

func ValidateTemplate(value Template, selection Selection) error {
	if ValidateSelection(selection) != nil || !templateNoPattern.MatchString(value.TemplateNo) ||
		value.VersionNo <= 0 || !validText(value.Name, 200) ||
		(value.LanguageCode != "zh-CN" && value.LanguageCode != "en-US") ||
		(value.SigningOrderCode != "customer_first" && value.SigningOrderCode != "merchant_first" &&
			value.SigningOrderCode != "parallel") || !scopeAllows(value.ScopeCode, selection) {
		return gerror.New("contract template projection is invalid")
	}
	return nil
}

func ValidateTemplates(values []Template, selection Selection) error {
	if len(values) > MaximumTemplates {
		return gerror.New("contract template collection exceeds limit")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := ValidateTemplate(value, selection); err != nil {
			return err
		}
		if _, duplicate := seen[value.TemplateNo]; duplicate {
			return gerror.New("contract template collection contains a duplicate")
		}
		seen[value.TemplateNo] = struct{}{}
	}
	return nil
}

func ValidateResolveRequest(selection Selection, templateNos []string) error {
	if ValidateSelection(selection) != nil || len(templateNos) == 0 || len(templateNos) > MaximumTemplates {
		return gerror.New("contract template resolve request is invalid")
	}
	seen := make(map[string]struct{}, len(templateNos))
	for _, templateNo := range templateNos {
		if !templateNoPattern.MatchString(templateNo) {
			return gerror.New("contract template resolve request is invalid")
		}
		if _, duplicate := seen[templateNo]; duplicate {
			return gerror.New("contract template resolve request contains a duplicate")
		}
		seen[templateNo] = struct{}{}
	}
	return nil
}

func scopeAllows(scope string, selection Selection) bool {
	switch selection.Level {
	case SelectionLevelProduct:
		return scope == "all_customers"
	case SelectionLevelEnterprise:
		return scope == "all_customers" || scope == "enterprise"
	case SelectionLevelOrder:
		return scope == "all_customers" ||
			(scope == "personal" && selection.CustomerType == CustomerTypeIndividual) ||
			(scope == "enterprise" && selection.CustomerType == CustomerTypeEnterprise)
	default:
		return false
	}
}

func validText(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && strings.IndexFunc(value, unicode.IsControl) < 0
}
