package v10

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const maxCandidateItems = 512

var configOptionName = regexp.MustCompile(`^config_options\[configoption\]\[([1-9][0-9]*)\]$`)

var candidateFields = map[string]struct{}{
	"upstream_product_id": {}, "upstream_region_option_id": {}, "upstream_line_option_id": {},
	"upstream_os_option_id": {}, "region": {}, "line": {}, "operating_system": {},
}

type candidateRequest struct {
	SchemaVersion     string               `json:"schemaVersion"`
	CurrentSelections []candidateSelection `json:"currentSelections"`
	Requests          []candidateQuery     `json:"requests"`
}

type candidateSelection struct {
	FieldKey string   `json:"fieldKey"`
	Values   []string `json:"values"`
}

type candidateQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues"`
	SelectedValues []string `json:"selectedValues"`
}

func executeCandidates(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request candidateRequest
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || request.SchemaVersion != "v1" ||
		len(request.Requests) == 0 || len(request.Requests) > 8 || len(request.CurrentSelections) > 16 {
		return nil, errInvalidRequest
	}
	for _, query := range request.Requests {
		if _, allowed := candidateFields[query.FieldKey]; !allowed || len(query.ParentValues) > 8 || len(query.SelectedValues) > maxCandidateItems {
			return nil, errInvalidRequest
		}
	}
	needProducts := false
	needOptions := false
	for _, query := range request.Requests {
		needProducts = needProducts || query.FieldKey == "upstream_product_id"
		needOptions = needOptions || query.FieldKey != "upstream_product_id"
	}
	var products []resourceinterfaceprojectioncap.Candidate
	var options []htmlOptionGroup
	sequence := uint8(2)
	if needProducts {
		response, err := businessCall(executor, contextID, sequence, http.MethodGet, "/api/v1/product", nil, nil, nil)
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidResponse
		}
		products, err = projectProducts(response.Data)
		if err != nil {
			return nil, err
		}
		sequence++
	}
	if needOptions {
		productID := selectedValue(request.CurrentSelections, "upstream_product_id")
		if !positiveDecimal(productID) {
			return nil, errInvalidRequest
		}
		response, err := businessCall(executor, contextID, sequence, http.MethodGet, "/console/v1/product/"+productID+"/config_option", nil, nil, nil)
		if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidResponse
		}
		content := nestedString(response.Data, "data", "content")
		options, err = projectConfigurationHTML(content)
		if err != nil {
			return nil, err
		}
	}
	fields := make([]resourceinterfaceprojectioncap.CandidateField, 0, len(request.Requests))
	for _, query := range request.Requests {
		items := products
		if query.FieldKey != "upstream_product_id" {
			items = optionCandidates(query.FieldKey, request.CurrentSelections, options)
			if len(items) == 0 {
				return nil, errInvalidResponse
			}
		}
		fields = append(fields, resourceinterfaceprojectioncap.CandidateField{FieldKey: query.FieldKey, Items: items})
	}
	return json.Marshal(resourceinterfaceprojectioncap.CandidateResult{Fields: fields})
}

type htmlOptionGroup struct {
	id      string
	label   string
	options []resourceinterfaceprojectioncap.Candidate
}

func projectConfigurationHTML(content string) ([]htmlOptionGroup, error) {
	if strings.TrimSpace(content) == "" || len(content) > 1<<20 {
		return nil, errInvalidResponse
	}
	document, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, errInvalidResponse
	}
	groups := make([]htmlOptionGroup, 0)
	seen := make(map[string]struct{})
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "select" {
			name := attribute(node, "name")
			matches := configOptionName.FindStringSubmatch(name)
			if len(matches) == 2 && positiveDecimal(matches[1]) {
				label := selectLabel(node)
				items := selectOptions(node)
				if label != "" && len(label) <= 256 && len(items) > 0 {
					if _, duplicate := seen[matches[1]]; !duplicate && len(groups) < 128 {
						seen[matches[1]] = struct{}{}
						groups = append(groups, htmlOptionGroup{id: matches[1], label: label, options: items})
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if len(groups) == 0 {
		return nil, errInvalidResponse
	}
	return groups, nil
}

func optionCandidates(fieldKey string, selections []candidateSelection, groups []htmlOptionGroup) []resourceinterfaceprojectioncap.Candidate {
	switch fieldKey {
	case "upstream_region_option_id", "upstream_line_option_id", "upstream_os_option_id":
		items := make([]resourceinterfaceprojectioncap.Candidate, 0, len(groups))
		for _, group := range groups {
			items = append(items, resourceinterfaceprojectioncap.Candidate{Value: group.id, LabelZhCN: group.label, LabelEnUS: group.label})
		}
		return items
	case "region", "line", "operating_system":
		selector := map[string]string{"region": "upstream_region_option_id", "line": "upstream_line_option_id", "operating_system": "upstream_os_option_id"}[fieldKey]
		selected := selectedValue(selections, selector)
		for _, group := range groups {
			if group.id == selected {
				return append([]resourceinterfaceprojectioncap.Candidate(nil), group.options...)
			}
		}
	}
	return nil
}

func selectedValue(selections []candidateSelection, fieldKey string) string {
	for _, selection := range selections {
		if selection.FieldKey == fieldKey && len(selection.Values) == 1 {
			return selection.Values[0]
		}
	}
	return ""
}

func selectOptions(node *html.Node) []resourceinterfaceprojectioncap.Candidate {
	items := make([]resourceinterfaceprojectioncap.Candidate, 0)
	seen := make(map[string]struct{})
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.ElementNode && current.Data == "option" {
			value := strings.TrimSpace(attribute(current, "value"))
			label := strings.TrimSpace(nodeText(current))
			if value != "" && len(value) <= 64 && label != "" && len(label) <= 256 {
				if _, duplicate := seen[value]; !duplicate && len(items) < maxCandidateItems {
					seen[value] = struct{}{}
					items = append(items, resourceinterfaceprojectioncap.Candidate{Value: value, LabelZhCN: label, LabelEnUS: label})
				}
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return items
}

func selectLabel(node *html.Node) string {
	for _, key := range []string{"data-name", "aria-label", "title"} {
		if value := strings.TrimSpace(attribute(node, key)); value != "" {
			return value
		}
	}
	id := attribute(node, "id")
	if node.Parent != nil {
		for child := node.Parent.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "label" && (id == "" || attribute(child, "for") == id) {
				if value := strings.TrimSpace(nodeText(child)); value != "" {
					return value
				}
			}
		}
	}
	return ""
}

func attribute(node *html.Node, key string) string {
	for _, item := range node.Attr {
		if item.Key == key {
			return item.Val
		}
	}
	return ""
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
			builder.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func projectProducts(raw json.RawMessage) ([]resourceinterfaceprojectioncap.Candidate, error) {
	var envelope struct {
		Data struct {
			List []struct {
				ID   json.RawMessage `json:"id"`
				Name string          `json:"name"`
			} `json:"list"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Data.List) == 0 || len(envelope.Data.List) > maxCandidateItems {
		return nil, errInvalidResponse
	}
	items := make([]resourceinterfaceprojectioncap.Candidate, 0, len(envelope.Data.List))
	seen := make(map[string]struct{}, len(envelope.Data.List))
	for _, product := range envelope.Data.List {
		value := scalar(product.ID)
		label := strings.TrimSpace(product.Name)
		if !positiveDecimal(value) || label == "" || len(label) > 256 {
			return nil, errInvalidResponse
		}
		if _, exists := seen[value]; exists {
			return nil, errInvalidResponse
		}
		seen[value] = struct{}{}
		items = append(items, resourceinterfaceprojectioncap.Candidate{Value: value, LabelZhCN: label, LabelEnUS: label})
	}
	return items, nil
}

func scalar(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		return number.String()
	}
	return ""
}
