// This file evaluates native imported selections using the exact current source
// revision and obtains account-authoritative totals through get_total. Reads
// cannot create carts, settle invoices or substitute raw component prices.
package finance

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func executeNativeConfiguration(executor confidentialoutboundcap.Service, contextID, method string, payload json.RawMessage) (json.RawMessage, error) {
	var input channelcommercecap.ConfigurationInput
	if strictDecode(payload, &input) != nil || input.Validate() != nil || !positiveDecimal(input.PublicSlug) {
		return nil, errInvalidConfig
	}

	raw, err := catalogRead(executor, contextID, "/cart/get_product_config", map[string][]string{"pid": {input.PublicSlug}}, nativeCatalogResponseFields())
	if err != nil {
		return nil, err
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	projected, projectErr := projectCatalogDetail(raw, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: input.PublicSlug})
	if projectErr != nil {
		return nil, projectErr
	}
	detail = *projected
	if detail.ImportSnapshot == nil || detail.ImportSnapshot.ConfigurationRevision != input.ConfigurationRevision {
		return nil, errInvalidConfig
	}
	snapshot := detail.ImportSnapshot
	view, err := evaluateNativeSelections(snapshot, input.Values)
	if err != nil {
		return nil, err
	}
	if method == resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration {
		return json.Marshal(view)
	}
	if !view.Valid || input.CurrencyCode == "" {
		return nil, errInvalidConfig
	}
	return nativeBillingTerms(executor, contextID, snapshot, input.CurrencyCode, view.Values)
}

func evaluateNativeSelections(snapshot *channelcommercecap.ImportSnapshot, values []channelcommercecap.PurchaseValue) (*channelcommercecap.ConfigurationEvaluation, error) {
	if snapshot == nil || snapshot.Schema == nil {
		return nil, errInvalidConfig
	}
	var (
		supplied = map[string]channelcommercecap.PurchaseValue{}
		result   = &channelcommercecap.ConfigurationEvaluation{Configuration: channelcommercecap.Configuration{ConfigurationRevision: snapshot.ConfigurationRevision, Fields: []channelcommercecap.ConfigurationField{}, Values: []channelcommercecap.PurchaseValue{}}, Valid: true, Issues: []channelcommercecap.ConfigurationIssue{}}
		fields   = map[string]*capmodel.ConfigurationFieldDeclaration{}
	)
	for _, field := range snapshot.Schema.Fields {
		fields[field.Key] = field
	}
	for _, value := range values {
		if value.FieldToken == "" || supplied[value.FieldToken].FieldToken != "" {
			return nil, errInvalidConfig
		}
		supplied[value.FieldToken] = value
	}
	for _, mapping := range snapshot.Mappings {
		field := fields[mapping.FieldKey]
		if field == nil {
			return nil, errInvalidResponse
		}
		view := channelcommercecap.ConfigurationField{FieldToken: mapping.FieldToken, Type: field.Type, Label: channelcommercecap.LocalizedText{ZhCN: field.Label.ZhCN, EnUS: field.Label.EnUS}, Visible: true, Required: field.Required, Options: []channelcommercecap.ConfigurationOption{}, Constraints: channelcommercecap.ConfigurationConstraints{Minimum: field.Minimum, Maximum: field.Maximum, MaximumLength: field.MaximumLength}}
		view.Options = nativePublicOptions(field)
		value, exists := supplied[mapping.FieldToken]
		delete(supplied, mapping.FieldToken)
		if !exists {
			value = nativeDefaultSelection(field, mapping.FieldToken)
		}
		valid := validateNativeSelection(field, value)
		if !valid {
			result.Valid = false
			result.Issues = append(result.Issues, channelcommercecap.ConfigurationIssue{FieldToken: mapping.FieldToken, IssueCode: "invalid_selection"})
		}
		result.Fields = append(result.Fields, view)
		result.Values = append(result.Values, value)
	}
	if len(supplied) != 0 {
		return nil, errInvalidConfig
	}
	if err := applyNativeSelectionRules(snapshot, result); err != nil {
		return nil, err
	}
	if result.Configuration.Validate() != nil {
		return nil, errInvalidResponse
	}
	return result, nil
}

func nativeDefaultSelection(field *capmodel.ConfigurationFieldDeclaration, token string) channelcommercecap.PurchaseValue {
	value := channelcommercecap.PurchaseValue{FieldToken: token, Type: field.Type}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		number := field.Minimum
		value.IntegerValue = &number
	case capmodel.ConfigurationFieldTreeSelect:
		parents := map[string]bool{}
		for _, candidate := range field.StaticCandidates {
			parents[candidate.ParentValue] = true
		}
		for _, candidate := range field.StaticCandidates {
			if !parents[candidate.Value] {
				value.TreeOptionTokens = []string{candidate.Value}
				break
			}
		}
	case capmodel.ConfigurationFieldSingleSelect:
		if len(field.StaticCandidates) > 0 {
			value.OptionTokens = []string{field.StaticCandidates[0].Value}
		}
	default:
		text := ""
		value.TextValue = &text
	}
	return value
}

func validateNativeSelection(field *capmodel.ConfigurationFieldDeclaration, value channelcommercecap.PurchaseValue) bool {
	if field.Type != value.Type {
		return false
	}
	selected, err := nativePurchaseValue(value)
	if err != nil {
		return false
	}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		number, err := nativeNonnegative(json.RawMessage(`"` + selected + `"`))
		minimum, minErr := strconv.ParseInt(field.Minimum, 10, 64)
		maximum, maxErr := strconv.ParseInt(field.Maximum, 10, 64)
		return err == nil && minErr == nil && maxErr == nil && number >= minimum && number <= maximum
	case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldTreeSelect:
		for _, candidate := range field.StaticCandidates {
			if candidate.Value == selected {
				return true
			}
		}
		return false
	default:
		return len(selected) <= field.MaximumLength && (!field.Required || strings.TrimSpace(selected) != "")
	}
}

func nativeBillingTerms(executor confidentialoutboundcap.Service, contextID string, snapshot *channelcommercecap.ImportSnapshot, currency string, values []channelcommercecap.PurchaseValue) (json.RawMessage, error) {
	// The finance product-definition endpoint exposes its actual default-currency
	// row. Obtain that row again only for its native numeric currency identifier;
	// never guess that CNY is 1 or use the customer's display language.
	raw, err := catalogReadSequence(executor, contextID, 3, "/cart/get_product_config", map[string][]string{"pid": {snapshot.Product.PublicSlug}}, []string{"status", "data", "product_pricings", "currency", "code"})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Status json.RawMessage `json:"status"`
		Data   struct {
			Prices []map[string]json.RawMessage `json:"product_pricings"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || scalar(envelope.Status) != "200" {
		return nil, errInvalidResponse
	}
	currencyID := ""
	for _, row := range envelope.Data.Prices {
		if scalar(row["code"]) == currency {
			if currencyID != "" {
				return nil, errInvalidResponse
			}
			currencyID = scalar(row["currency"])
		}
	}
	if !positiveDecimal(currencyID) {
		return nil, errInvalidConfig
	}
	result := channelcommercecap.BillingTerms{ConfigurationRevision: snapshot.ConfigurationRevision, CurrencyCode: currency, Terms: []channelcommercecap.BillingTerm{}}
	sequence := uint8(4)
	for _, schedule := range snapshot.Prices {
		if schedule.CurrencyCode != currency {
			continue
		}
		purchase := channelcommercecap.Purchase{PublicSlug: snapshot.Product.PublicSlug, ConfigurationRevision: snapshot.ConfigurationRevision, BillingTermToken: schedule.TermReference, CurrencyCode: currency, PurchaseCycles: 1, Quantity: 1, Values: values}
		parameters, err := BuildNativePurchaseParameters(context.Background(), purchase)
		if err != nil {
			return nil, err
		}
		selected, err := decodePurchaseParameters(parameters)
		if err != nil {
			return nil, err
		}
		form := map[string]string{"pid": selected.ProductID, "billingcycle": selected.BillingCycle, "currencyid": currencyID, "qty": "1"}
		for id, value := range selected.ConfigOptions {
			form["configoption["+id+"]"] = value
		}
		for id, value := range selected.CustomFields {
			form["customfield["+id+"]"] = value
		}
		response, err := confidentialCall(executor, contextID, sequence, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: "/cart/get_total", Form: form, UseAuthentication: true, ResponseFields: []string{"status", "currency", "id", "code", "products", "type", "bates", "signal_price", "signal_setupfee", "sale_signal_price", "sale_setupfee_total"}})
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidResponse
		}
		term, err := nativeAccountTerm(response.Data, currency, currencyID, schedule)
		if err != nil {
			return nil, err
		}
		result.Terms = append(result.Terms, *term)
		sequence++
	}
	if len(result.Terms) == 0 || result.Validate() != nil {
		return nil, errInvalidResponse
	}
	return json.Marshal(result)
}

func nativeAccountTerm(raw json.RawMessage, currency, currencyID string, schedule channelcommercecap.ImportPriceSchedule) (*channelcommercecap.BillingTerm, error) {
	var envelope struct {
		Status   json.RawMessage `json:"status"`
		Currency struct {
			ID   json.RawMessage `json:"id"`
			Code string          `json:"code"`
		} `json:"currency"`
		Product struct {
			Account   json.RawMessage `json:"type"`
			Price     json.RawMessage `json:"signal_price"`
			Setup     json.RawMessage `json:"signal_setupfee"`
			SalePrice json.RawMessage `json:"sale_signal_price"`
			SaleSetup json.RawMessage `json:"sale_setupfee_total"`
		} `json:"products"`
	}
	if len(raw) > catalogMaxResponseBytes || json.Unmarshal(raw, &envelope) != nil || scalar(envelope.Status) != "200" || scalar(envelope.Currency.ID) != currencyID || envelope.Currency.Code != currency {
		return nil, errInvalidResponse
	}
	priceRaw, setupRaw := envelope.Product.Price, envelope.Product.Setup
	if string(envelope.Product.Account) != "false" {
		if len(envelope.Product.Account) == 0 || string(envelope.Product.Account) == "null" {
			return nil, errInvalidResponse
		}
		priceRaw, setupRaw = envelope.Product.SalePrice, envelope.Product.SaleSetup
	}
	if !catalogDecimal.MatchString(scalar(priceRaw)) || !catalogDecimal.MatchString(scalar(setupRaw)) {
		return nil, errInvalidResponse
	}
	price, err := nativeMinor(scalar(priceRaw), true)
	if err != nil {
		return nil, err
	}
	setup, err := nativeMinor(scalar(setupRaw), true)
	if err != nil || price > int64(^uint64(0)>>1)-setup {
		return nil, errInvalidResponse
	}
	return &channelcommercecap.BillingTerm{BillingTermToken: schedule.TermReference, Label: channelcommercecap.LocalizedText{ZhCN: schedule.TermReference, EnUS: schedule.TermReference}, ChargeType: schedule.ChargeType, PeriodUnit: schedule.PeriodUnit, PeriodCount: schedule.PeriodCount, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: strconv.FormatInt(setup, 10), SubtotalAmount: strconv.FormatInt(price, 10), TaxAmount: "0", TotalAmount: strconv.FormatInt(price+setup, 10)}, nil
}

// attachNativePricePreview runs only for the staff owner's explicit preview
// read. Frozen purchase reads omit this flag and do not quote a second default
// selection or reuse a confidential request sequence for different values.
func attachNativePricePreview(executor confidentialoutboundcap.Service, contextID string, detail *resourceinterfaceprojectioncap.CatalogDetail) error {
	if detail.ImportSnapshot == nil {
		return nil
	}
	snapshot := detail.ImportSnapshot
	view, err := evaluateNativeSelections(snapshot, nil)
	if err != nil {
		return err
	}
	preview := &channelcommercecap.ProductSnapshot{Product: snapshot.Product, Configuration: view.Configuration, ConfigurationMode: "remote_evaluation", Currencies: snapshot.Currencies, BillingTerms: []channelcommercecap.CurrencyBillingTerms{}, PricingReady: false}
	if view.Valid {
		for _, currency := range snapshot.Currencies {
			raw, quoteErr := nativeBillingTerms(executor, contextID, snapshot, currency.CurrencyCode, view.Values)
			if quoteErr != nil {
				return quoteErr
			}
			var terms channelcommercecap.BillingTerms
			if json.Unmarshal(raw, &terms) != nil {
				return errInvalidResponse
			}
			preview.BillingTerms = append(preview.BillingTerms, channelcommercecap.CurrencyBillingTerms{CurrencyCode: terms.CurrencyCode, Precision: currency.Precision, Terms: terms.Terms})
		}
		preview.PricingReady = true
	}
	if preview.Validate() != nil {
		return errInvalidResponse
	}
	detail.PeerPurchase = preview
	return nil
}
