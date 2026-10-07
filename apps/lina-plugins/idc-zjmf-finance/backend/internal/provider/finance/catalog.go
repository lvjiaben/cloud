// This file reads the native finance product catalog with bounded snapshots.
// It never writes upstream data, infers hardware roles, or treats option charges
// as a complete product price. Only source-proven billing intervals are exposed.

package finance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// Native cart/all is unpaged. Reject excessive snapshots instead of truncating
// their visible product set and then reporting a fabricated total.
const catalogMaxResponseBytes = 1 << 20

type catalogPaymentType string

const (
	catalogPaymentOnce      catalogPaymentType = "onetime"
	catalogPaymentRecurring catalogPaymentType = "recurring"
)

// catalogCycle records the actual finance pricing-column/installation-fee pair.
type catalogCycle struct {
	amount string
	setup  string
	period resourceinterfaceprojectioncap.CatalogPeriod
	count  int
}

var catalogCycles = []catalogCycle{
	{"onetime", "osetupfee", resourceinterfaceprojectioncap.CatalogPeriodOnce, 1},
	{"hour", "hsetupfee", resourceinterfaceprojectioncap.CatalogPeriodHour, 1},
	{"day", "dsetupfee", resourceinterfaceprojectioncap.CatalogPeriodDay, 1},
	{"monthly", "msetupfee", resourceinterfaceprojectioncap.CatalogPeriodMonth, 1},
	{"quarterly", "qsetupfee", resourceinterfaceprojectioncap.CatalogPeriodMonth, 3},
	{"semiannually", "ssetupfee", resourceinterfaceprojectioncap.CatalogPeriodMonth, 6},
	{"annually", "asetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 1},
	{"biennially", "bsetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 2},
	{"triennially", "tsetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 3},
	{"fourly", "foursetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 4},
	{"fively", "fivesetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 5},
	{"sixly", "sixsetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 6},
	{"sevenly", "sevensetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 7},
	{"eightly", "eightsetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 8},
	{"ninely", "ninesetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 9},
	{"tenly", "tensetupfee", resourceinterfaceprojectioncap.CatalogPeriodYear, 10},
}

var (
	catalogMissingPrice = regexp.MustCompile(`^-1(?:\.0+)?$`)
	catalogDecimal      = regexp.MustCompile(`^[0-9]{1,24}(\.[0-9]{1,12})?$`)
)

// catalogWireProduct contains only non-secret source fields needed by the view.
type catalogWireProduct struct {
	ID          json.RawMessage `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	PayType     string          `json:"pay_type"`
}

// executeCatalogList performs exactly one authenticated GET after login.
func executeCatalogList(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourceinterfaceprojectioncap.CatalogQuery
	if strictDecode(payload, &query) != nil || query.Validate() != nil {
		return nil, errInvalidConfig
	}
	response, err := catalogRead(executor, contextID, "/cart/all", nil, []string{"status", "data", "products", "id", "name", "description"})
	if err != nil {
		return nil, err
	}
	page, err := projectCatalogList(response, query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(page)
}

// executeCatalogDetail reads one selected product and all its option prices in
// one native request. Authentication and secret fields stay with the host.
func executeCatalogDetail(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourceinterfaceprojectioncap.CatalogProductQuery
	if strictDecode(payload, &query) != nil || query.Validate() != nil || !positiveDecimal(query.Reference) {
		return nil, errInvalidConfig
	}
	fields := []string{"status", "data", "products", "id", "name", "description", "pay_type", "product_pricings", "code", "config_groups", "options", "sub", "option_name", "pricings"}
	for _, cycle := range catalogCycles {
		fields = append(fields, cycle.amount, cycle.setup)
	}
	response, err := catalogRead(executor, contextID, "/cart/get_product_config", map[string][]string{"pid": {query.Reference}}, fields)
	if err != nil {
		return nil, err
	}
	detail, err := projectCatalogDetail(response, query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(detail)
}

// catalogRead uses an endpoint-specific response allowlist. It deliberately
// omits supplier messages and private product configuration from the guest.
func catalogRead(executor confidentialoutboundcap.Service, contextID, path string, query map[string][]string, fields []string) (json.RawMessage, error) {
	response, err := confidentialCall(executor, contextID, 2, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: path,
		Query: query, ResponseFields: fields, UseAuthentication: true,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 || len(response.Data) > catalogMaxResponseBytes {
		return nil, errInvalidResponse
	}
	return response.Data, nil
}

// projectCatalogList counts the actual visible set, not cart/all's count, which
// can include products in hidden groups. Validation precedes local filtering.
func projectCatalogList(raw json.RawMessage, query resourceinterfaceprojectioncap.CatalogQuery) (*resourceinterfaceprojectioncap.CatalogPage, error) {
	var envelope struct {
		Status json.RawMessage `json:"status"`
		Data   *struct {
			Groups []struct {
				Products []catalogWireProduct `json:"products"`
			} `json:"products"`
		} `json:"data"`
	}
	if query.Validate() != nil || len(raw) > catalogMaxResponseBytes || json.Unmarshal(raw, &envelope) != nil || scalar(envelope.Status) != "200" ||
		envelope.Data == nil || envelope.Data.Groups == nil || len(envelope.Data.Groups) > maxCandidateItems {
		return nil, errInvalidResponse
	}
	var (
		items   = make([]resourceinterfaceprojectioncap.CatalogProduct, 0)
		seen    = make(map[string]struct{})
		keyword = strings.ToLower(query.Keyword)
	)
	for _, group := range envelope.Data.Groups {
		if group.Products == nil || len(group.Products) > maxCandidateItems {
			return nil, errInvalidResponse
		}
		for _, product := range group.Products {
			item, err := projectCatalogProduct(product)
			if err != nil || len(seen) >= maxCandidateItems {
				return nil, errInvalidResponse
			}
			if _, duplicate := seen[item.Reference]; duplicate {
				return nil, errInvalidResponse
			}
			seen[item.Reference] = struct{}{}
			if keyword == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Description), keyword) {
				items = append(items, item)
			}
		}
	}
	var (
		start = min((query.Page-1)*query.PageSize, len(items))
		end   = min(start+query.PageSize, len(items))
		page  = &resourceinterfaceprojectioncap.CatalogPage{Items: items[start:end], Total: len(items)}
	)
	if page.Validate(query) != nil {
		return nil, errInvalidResponse
	}
	return page, nil
}

// projectCatalogDetail preserves only known component prices. Trial, free, or
// unsupported billing definitions never become guessed recurring schedules.
func projectCatalogDetail(raw json.RawMessage, query resourceinterfaceprojectioncap.CatalogProductQuery) (*resourceinterfaceprojectioncap.CatalogDetail, error) {
	var envelope struct {
		Status json.RawMessage `json:"status"`
		Data   *struct {
			Product catalogWireProduct           `json:"products"`
			Prices  []map[string]json.RawMessage `json:"product_pricings"`
			Groups  []struct {
				Options []struct {
					ID      json.RawMessage `json:"id"`
					Name    string          `json:"option_name"`
					Choices []struct {
						ID     json.RawMessage              `json:"id"`
						Name   string                       `json:"option_name"`
						Prices []map[string]json.RawMessage `json:"pricings"`
					} `json:"sub"`
				} `json:"options"`
			} `json:"config_groups"`
		} `json:"data"`
	}
	if query.Validate() != nil || !positiveDecimal(query.Reference) || len(raw) > catalogMaxResponseBytes || json.Unmarshal(raw, &envelope) != nil ||
		scalar(envelope.Status) != "200" || envelope.Data == nil || len(envelope.Data.Groups) > 128 {
		return nil, errInvalidResponse
	}
	product, err := projectCatalogProduct(envelope.Data.Product)
	if err != nil || product.Reference != query.Reference {
		return nil, errInvalidResponse
	}
	var payment struct {
		Type  catalogPaymentType `json:"pay_type"`
		Trial json.RawMessage    `json:"pay_ontrial_status"`
	}
	// Unknown billing metadata is readable, but has no importable base schedule.
	if envelope.Data.Product.PayType != "" && json.Unmarshal([]byte(envelope.Data.Product.PayType), &payment) != nil {
		return nil, errInvalidResponse
	}
	prices, complete, err := projectCatalogPrices(envelope.Data.Prices, payment.Type)
	if err != nil {
		return nil, err
	}
	trial := strings.TrimSpace(string(payment.Trial))
	if trial != "" && trial != "null" && trial != "false" && trial != "0" && trial != `"0"` && trial != `""` {
		complete = false
	}
	detail := &resourceinterfaceprojectioncap.CatalogDetail{
		Product: product, BasePrices: prices, Options: make([]resourceinterfaceprojectioncap.CatalogOption, 0),
		PriceScheduleComplete: complete && envelope.Data.Groups != nil,
		ProductConfiguration: []resourceinterfaceprojectioncap.CatalogConfigurationValue{
			{FieldKey: "upstream_product_id", OptionValue: &product.Reference},
		},
	}
	choiceCount := 0
	for _, group := range envelope.Data.Groups {
		if group.Options == nil {
			detail.PriceScheduleComplete = false
		}
		for _, option := range group.Options {
			if len(detail.Options) >= 128 || len(option.Choices) > maxCandidateItems || !positiveDecimal(scalar(option.ID)) {
				return nil, errInvalidResponse
			}
			name, err := catalogPlainText(option.Name, 256, false)
			if err != nil {
				return nil, err
			}
			projected := resourceinterfaceprojectioncap.CatalogOption{Reference: scalar(option.ID), Name: name, Choices: make([]resourceinterfaceprojectioncap.CatalogChoice, 0, len(option.Choices))}
			if len(option.Choices) == 0 {
				detail.PriceScheduleComplete = false
			}
			for _, choice := range option.Choices {
				if choiceCount >= 2048 || !positiveDecimal(scalar(choice.ID)) {
					return nil, errInvalidResponse
				}
				label, err := catalogPlainText(choice.Name, 256, false)
				if err != nil {
					return nil, err
				}
				choicePrices, choiceComplete, err := projectCatalogPrices(choice.Prices, payment.Type)
				if err != nil {
					return nil, err
				}
				projected.Choices = append(projected.Choices, resourceinterfaceprojectioncap.CatalogChoice{Reference: scalar(choice.ID), Name: label, Prices: choicePrices})
				detail.PriceScheduleComplete = detail.PriceScheduleComplete && choiceComplete
				choiceCount++
			}
			detail.Options = append(detail.Options, projected)
		}
	}
	if detail.Validate(query) != nil {
		return nil, errInvalidResponse
	}
	return detail, nil
}

// projectCatalogPrices keeps major-unit decimal strings unchanged. Absent fees
// make that price incomplete; negative-one values are the native disabled flag.
func projectCatalogPrices(rows []map[string]json.RawMessage, payment catalogPaymentType) ([]resourceinterfaceprojectioncap.CatalogPrice, bool, error) {
	prices := make([]resourceinterfaceprojectioncap.CatalogPrice, 0)
	complete := payment == catalogPaymentOnce || payment == catalogPaymentRecurring
	if len(rows) > 120 {
		return nil, false, errInvalidResponse
	}
	for _, row := range rows {
		for _, cycle := range catalogCycles {
			if payment != catalogPaymentOnce && payment != catalogPaymentRecurring ||
				(payment == catalogPaymentOnce) != (cycle.period == resourceinterfaceprojectioncap.CatalogPeriodOnce) {
				continue
			}
			amount, exists := row[cycle.amount]
			if !exists || bytes.Equal(bytes.TrimSpace(amount), []byte("null")) {
				complete = false
				continue
			}
			value := scalar(amount)
			if catalogMissingPrice.MatchString(value) {
				continue
			}
			if !catalogDecimal.MatchString(value) {
				return nil, false, errInvalidResponse
			}
			fee, exists := row[cycle.setup]
			if !exists || bytes.Equal(bytes.TrimSpace(fee), []byte("null")) {
				complete = false
				continue
			}
			setup := scalar(fee)
			if !catalogDecimal.MatchString(setup) {
				return nil, false, errInvalidResponse
			}
			prices = append(prices, resourceinterfaceprojectioncap.CatalogPrice{CurrencyCode: scalar(row["code"]), Period: cycle.period, PeriodCount: cycle.count, Amount: value, SetupFee: setup})
			if len(prices) > 120 {
				return nil, false, errInvalidResponse
			}
		}
	}
	return prices, complete && len(prices) > 0, nil
}

// projectCatalogProduct verifies identity and strips presentation markup.
func projectCatalogProduct(product catalogWireProduct) (resourceinterfaceprojectioncap.CatalogProduct, error) {
	var (
		result resourceinterfaceprojectioncap.CatalogProduct
		err    error
	)
	result.Reference = scalar(product.ID)
	if !positiveDecimal(result.Reference) {
		return result, errInvalidResponse
	}
	result.Name, err = catalogPlainText(product.Name, 256, false)
	if err != nil {
		return result, err
	}
	result.Description, err = catalogPlainText(product.Description, 1024, true)
	return result, err
}

// catalogPlainText discards attributes, scripts, style, comments and embedded
// documents. Only normalized visible text can enter the cross-plugin DTO.
func catalogPlainText(value string, limit int, optional bool) (string, error) {
	if !utf8.ValidString(value) || len(value) > 64<<10 {
		return "", errInvalidResponse
	}
	document, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return "", errInvalidResponse
	}
	var text strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script", "style", "iframe", "object", "embed", "template", "noscript":
				return
			}
		}
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
			text.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(document)
	cleaned := strings.Join(strings.FieldsFunc(text.String(), func(char rune) bool { return unicode.IsSpace(char) || unicode.IsControl(char) }), " ")
	// Entity-escaped tags are text, not trusted markup. Avoid returning literal
	// tag delimiters that a later unsafe renderer could interpret a second time.
	cleaned = strings.NewReplacer("<", "＜", ">", "＞").Replace(cleaned)
	if !optional && cleaned == "" || utf8.RuneCountInString(cleaned) > limit {
		return "", errInvalidResponse
	}
	return cleaned, nil
}
