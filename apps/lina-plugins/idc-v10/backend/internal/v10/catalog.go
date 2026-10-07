// This file projects the V10 10.4.6 API product list and selected detail.
// These endpoints expose starting prices, not complete billing/configuration
// schedules. No preview price or inferred hardware mapping crosses this boundary.

package v10

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// V10's public product API has no pagination. Bound the entire visible snapshot
// before filtering, and fail instead of presenting a truncated set as complete.
const (
	catalogMaxProducts      = 512
	catalogMaxResponseBytes = 1 << 20
)

// catalogProductWire admits only public product identity and presentation text.
// pay_type, price, cycle and self_defined_field deliberately have no projection.
type catalogProductWire struct {
	ID          json.RawMessage `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
}

// executeCatalogList issues one catalog GET after the existing authentication.
func executeCatalogList(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourceinterfaceprojectioncap.CatalogQuery
	if strictDecode(payload, &query) != nil || query.Validate() != nil {
		return nil, bizerr.NewCode(codeCatalogInvalid)
	}
	raw, err := catalogRead(executor, contextID, "/api/v1/product", []string{"status", "data", "list", "id", "name", "description"})
	if err != nil {
		return nil, err
	}
	page, err := projectCatalogList(raw, query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(page)
}

// executeCatalogDetail reads only the product chosen by the operator. Module
// config HTML is not fetched because it does not prove a full price schedule.
func executeCatalogDetail(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourceinterfaceprojectioncap.CatalogProductQuery
	if strictDecode(payload, &query) != nil || query.Validate() != nil || !positiveDecimal(query.Reference) {
		return nil, bizerr.NewCode(codeCatalogInvalid)
	}
	raw, err := catalogRead(executor, contextID, "/api/v1/product/"+query.Reference, []string{"status", "data", "product", "id", "name", "description"})
	if err != nil {
		return nil, err
	}
	detail, err := projectCatalogDetail(raw, query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(detail)
}

// catalogRead confines the authenticated request and its response allowlist to
// one proven endpoint. Supplier messages, credentials and module JSON stay out.
func catalogRead(executor confidentialoutboundcap.Service, contextID, path string, fields []string) (json.RawMessage, error) {
	response, err := confidentialCall(executor, contextID, 2, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodGet, Path: path,
		ResponseFields: fields, UseAuthentication: true,
	})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 || len(response.Data) > catalogMaxResponseBytes {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return response.Data, nil
}

// projectCatalogList uses the API's entire hidden=0, agentable=1 list as its
// authoritative visible set; no count field or per-product detail is required.
func projectCatalogList(raw json.RawMessage, query resourceinterfaceprojectioncap.CatalogQuery) (*resourceinterfaceprojectioncap.CatalogPage, error) {
	var envelope struct {
		Status json.RawMessage `json:"status"`
		Data   *struct {
			List []catalogProductWire `json:"list"`
		} `json:"data"`
	}
	if query.Validate() != nil || len(raw) > catalogMaxResponseBytes || json.Unmarshal(raw, &envelope) != nil ||
		scalar(envelope.Status) != "200" || envelope.Data == nil || envelope.Data.List == nil || len(envelope.Data.List) > catalogMaxProducts {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	var (
		items   = make([]resourceinterfaceprojectioncap.CatalogProduct, 0, len(envelope.Data.List))
		seen    = make(map[string]struct{}, len(envelope.Data.List))
		keyword = strings.ToLower(query.Keyword)
	)
	for _, product := range envelope.Data.List {
		item, err := projectCatalogProduct(product)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[item.Reference]; duplicate {
			return nil, bizerr.NewCode(codeCatalogUnavailable)
		}
		seen[item.Reference] = struct{}{}
		if keyword == "" || strings.Contains(strings.ToLower(item.Name+" "+item.Description), keyword) {
			items = append(items, item)
		}
	}
	var (
		start = min((query.Page-1)*query.PageSize, len(items))
		end   = min(start+query.PageSize, len(items))
		page  = &resourceinterfaceprojectioncap.CatalogPage{Items: items[start:end], Total: len(items)}
	)
	if page.Validate(query) != nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return page, nil
}

// projectCatalogDetail leaves both schedules empty rather than inventing a
// currency, interval, installation fee, option charge or hardware role.
func projectCatalogDetail(raw json.RawMessage, query resourceinterfaceprojectioncap.CatalogProductQuery) (*resourceinterfaceprojectioncap.CatalogDetail, error) {
	var envelope struct {
		Status json.RawMessage `json:"status"`
		Data   *struct {
			Product *catalogProductWire `json:"product"`
		} `json:"data"`
	}
	if query.Validate() != nil || !positiveDecimal(query.Reference) || len(raw) > catalogMaxResponseBytes ||
		json.Unmarshal(raw, &envelope) != nil || scalar(envelope.Status) != "200" || envelope.Data == nil || envelope.Data.Product == nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	product, err := projectCatalogProduct(*envelope.Data.Product)
	if err != nil {
		return nil, err
	}
	detail := &resourceinterfaceprojectioncap.CatalogDetail{
		Product: product, BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{},
		Options: []resourceinterfaceprojectioncap.CatalogOption{}, PriceScheduleComplete: false,
		ProductConfiguration: []resourceinterfaceprojectioncap.CatalogConfigurationValue{
			{FieldKey: "upstream_product_id", OptionValue: &product.Reference},
		},
	}
	if detail.Validate(query) != nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return detail, nil
}

// projectCatalogProduct verifies exact numeric identity and plain visible text.
func projectCatalogProduct(product catalogProductWire) (resourceinterfaceprojectioncap.CatalogProduct, error) {
	var (
		result = resourceinterfaceprojectioncap.CatalogProduct{Reference: scalar(product.ID)}
		err    error
	)
	if !positiveDecimal(result.Reference) {
		return result, bizerr.NewCode(codeCatalogUnavailable)
	}
	result.Name, err = catalogPlainText(product.Name, 256, false)
	if err != nil {
		return result, err
	}
	result.Description, err = catalogPlainText(product.Description, 1024, true)
	return result, err
}

// catalogPlainText strips executable/embedded content and every attribute;
// unlike module candidate HTML, no source structure is retained or interpreted.
func catalogPlainText(value string, limit int, optional bool) (string, error) {
	if !utf8.ValidString(value) || len(value) > 64<<10 {
		return "", bizerr.NewCode(codeCatalogUnavailable)
	}
	document, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return "", bizerr.NewCode(codeCatalogUnavailable)
	}
	var (
		text  strings.Builder
		visit func(*html.Node)
	)
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
	cleaned = strings.NewReplacer("<", "＜", ">", "＞").Replace(cleaned)
	if (!optional && cleaned == "") || utf8.RuneCountInString(cleaned) > limit {
		return "", bizerr.NewCode(codeCatalogUnavailable)
	}
	return cleaned, nil
}
