// This file validates bounded source documents, online HTML, PDFs, and normalized field placements.

package electroniccontracttemplate

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"golang.org/x/net/html"

	"lina-core/pkg/bizerr"
)

const (
	maximumUploadedSourceBytes = 20 * 1024 * 1024
	maximumOnlineHTMLBytes     = 5 * 1024 * 1024
	maximumTemplateFields      = 100
	maximumTemplatePages       = 500
)

var (
	contentRevisionNoPattern = regexp.MustCompile(`^ctpc_[0-9a-f]{32}$`)
	fieldSetNoPattern        = regexp.MustCompile(`^ctfs_[0-9a-f]{32}$`)
	fieldNoPattern           = regexp.MustCompile(`^ctpf_[0-9a-f]{32}$`)
	contractVariablePattern  = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)
)

var contractVariables = map[string]string{
	VariableContractNo:   PartyMerchant,
	VariableCustomerName: PartyCustomer,
	VariableMerchantName: PartyMerchant,
	VariableOrderNo:      PartyMerchant,
	VariableServiceName:  PartyMerchant,
	VariableServiceTerm:  PartyMerchant,
	VariableAmount:       PartyMerchant,
	VariableCurrency:     PartyMerchant,
	VariableSigningDate:  PartyMerchant,
}

func normalizeUploadedSourceInput(input UploadedSourceInput) (UploadedSourceInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.DeclaredMediaType = strings.TrimSpace(input.DeclaredMediaType)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || input.Body == nil ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return UploadedSourceInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeOnlineSourceInput(input OnlineSourceInput) (OnlineSourceInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || !idempotencyPattern.MatchString(input.IdempotencyKey) ||
		len(input.HTML) == 0 || len(input.HTML) > maximumOnlineHTMLBytes {
		return OnlineSourceInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeGeneratePreviewInput(input GeneratePreviewInput) (GeneratePreviewInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.ContentRevisionNo = strings.TrimSpace(input.ContentRevisionNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || !contentRevisionNoPattern.MatchString(input.ContentRevisionNo) ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return GeneratePreviewInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeConfirmPreviewInput(input ConfirmPreviewInput) (ConfirmPreviewInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.ContentRevisionNo = strings.TrimSpace(input.ContentRevisionNo)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || !contentRevisionNoPattern.MatchString(input.ContentRevisionNo) {
		return ConfirmPreviewInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeSaveFieldSetInput(input SaveFieldSetInput) (SaveFieldSetInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.ContentRevisionNo = strings.TrimSpace(input.ContentRevisionNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || !contentRevisionNoPattern.MatchString(input.ContentRevisionNo) ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) || len(input.Fields) < 1 ||
		len(input.Fields) > maximumTemplateFields {
		return SaveFieldSetInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func detectUploadedSourceMediaType(source []byte, declared string) (string, error) {
	mediaType, _, err := mime.ParseMediaType(declared)
	if err != nil {
		return "", bizerr.NewCode(CodeContentInvalid)
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch mediaType {
	case "application/pdf":
		if !bytes.HasPrefix(source, []byte("%PDF-")) {
			return "", bizerr.NewCode(CodeContentInvalid)
		}
	case "application/msword":
		if len(source) < 8 || !bytes.Equal(source[:8], []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
			return "", bizerr.NewCode(CodeContentInvalid)
		}
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		if err = validateDOCX(source); err != nil {
			return "", bizerr.NewCode(CodeContentInvalid)
		}
	case "text/html":
		if _, err = sanitizeOnlineHTML(string(source)); err != nil {
			return "", bizerr.NewCode(CodeContentInvalid)
		}
	default:
		return "", bizerr.NewCode(CodeContentInvalid)
	}
	return mediaType, nil
}

func validateDOCX(source []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil || len(reader.File) == 0 || len(reader.File) > 5000 {
		return gerror.New("DOCX archive is invalid")
	}
	foundContentTypes := false
	foundDocument := false
	var expanded uint64
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, `\`, "/")
		if strings.HasPrefix(name, "/") || strings.Contains(name, "../") || strings.ContainsRune(name, 0) {
			return gerror.New("DOCX archive path is invalid")
		}
		expanded += file.UncompressedSize64
		if expanded > 100*1024*1024 {
			return gerror.New("DOCX archive is too large")
		}
		foundContentTypes = foundContentTypes || name == "[Content_Types].xml"
		foundDocument = foundDocument || name == "word/document.xml"
	}
	if !foundContentTypes || !foundDocument {
		return gerror.New("DOCX archive is incomplete")
	}
	return nil
}

func sanitizeOnlineHTML(source string) (string, error) {
	if len(source) == 0 || len(source) > maximumOnlineHTMLBytes || strings.ContainsRune(source, 0) {
		return "", bizerr.NewCode(CodeContentInvalid)
	}
	document, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", bizerr.NewCode(CodeContentInvalid)
	}
	body := findHTMLBody(document)
	if body == nil {
		return "", bizerr.NewCode(CodeContentInvalid)
	}
	root := &html.Node{Type: html.ElementNode, Data: "div"}
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		clean, cleanErr := sanitizeHTMLNode(child)
		if cleanErr != nil {
			return "", cleanErr
		}
		if clean != nil {
			root.AppendChild(clean)
		}
	}
	var output bytes.Buffer
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err = html.Render(&output, child); err != nil {
			return "", bizerr.NewCode(CodeContentInvalid)
		}
	}
	cleaned := strings.TrimSpace(output.String())
	if cleaned == "" || len(cleaned) > maximumOnlineHTMLBytes {
		return "", bizerr.NewCode(CodeContentInvalid)
	}
	for _, match := range contractVariablePattern.FindAllStringSubmatch(cleaned, -1) {
		if len(match) != 2 || contractVariables[match[1]] == "" {
			return "", bizerr.NewCode(CodeContentInvalid)
		}
	}
	return "<!doctype html><html><head><meta charset=\"utf-8\"><style>body{font-family:sans-serif;font-size:12pt;line-height:1.65}table{border-collapse:collapse;width:100%}td,th{border:1px solid #333;padding:6px}.page-break{break-before:page}</style></head><body>" + cleaned + "</body></html>", nil
}

func findHTMLBody(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && node.Data == "body" {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if result := findHTMLBody(child); result != nil {
			return result
		}
	}
	return nil
}

func sanitizeHTMLNode(node *html.Node) (*html.Node, error) {
	if node == nil {
		return nil, nil
	}
	switch node.Type {
	case html.TextNode:
		return &html.Node{Type: html.TextNode, Data: node.Data}, nil
	case html.CommentNode:
		return nil, nil
	case html.ElementNode:
	default:
		return nil, nil
	}
	allowed := map[string]bool{
		"p": true, "div": true, "br": true, "h1": true, "h2": true, "h3": true,
		"strong": true, "b": true, "em": true, "i": true, "u": true, "s": true,
		"ol": true, "ul": true, "li": true, "table": true, "thead": true, "tbody": true,
		"tr": true, "th": true, "td": true, "a": true, "hr": true, "span": true,
	}
	tag := strings.ToLower(node.Data)
	if !allowed[tag] {
		return nil, bizerr.NewCode(CodeContentInvalid)
	}
	clean := &html.Node{Type: html.ElementNode, Data: tag}
	for _, attribute := range node.Attr {
		name := strings.ToLower(attribute.Key)
		value := strings.TrimSpace(attribute.Val)
		switch {
		case tag == "a" && name == "href":
			if !safeContractLink(value) {
				return nil, bizerr.NewCode(CodeContentInvalid)
			}
			clean.Attr = append(clean.Attr, html.Attribute{Key: "href", Val: value})
		case name == "style":
			style, valid := safeContractStyle(value)
			if !valid {
				return nil, bizerr.NewCode(CodeContentInvalid)
			}
			clean.Attr = append(clean.Attr, html.Attribute{Key: "style", Val: style})
		case tag == "span" && name == "data-contract-variable" && contractVariables[value] != "":
			clean.Attr = append(clean.Attr, html.Attribute{Key: name, Val: value})
		case name == "class" && value == "page-break":
			clean.Attr = append(clean.Attr, html.Attribute{Key: "class", Val: value})
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		sanitized, err := sanitizeHTMLNode(child)
		if err != nil {
			return nil, err
		}
		if sanitized != nil {
			clean.AppendChild(sanitized)
		}
	}
	return clean, nil
}

func safeContractLink(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func safeContractStyle(value string) (string, bool) {
	allowed := make([]string, 0, 2)
	for _, declaration := range strings.Split(value, ";") {
		declaration = strings.TrimSpace(declaration)
		if declaration == "" {
			continue
		}
		parts := strings.SplitN(strings.ToLower(declaration), ":", 2)
		if len(parts) != 2 {
			return "", false
		}
		name, setting := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if name == "text-align" && (setting == "left" || setting == "center" || setting == "right" || setting == "justify") {
			allowed = append(allowed, name+":"+setting)
		} else if name == "break-before" && setting == "page" {
			allowed = append(allowed, name+":"+setting)
		} else {
			return "", false
		}
	}
	if len(allowed) == 0 {
		return "", false
	}
	sort.Strings(allowed)
	return strings.Join(allowed, ";"), true
}

func validatePDF(pdf []byte) (int, error) {
	if len(pdf) == 0 || len(pdf) > maximumPDFBytes || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return 0, bizerr.NewCode(CodeContentInvalid)
	}
	configuration := model.NewDefaultConfiguration()
	configuration.ValidationMode = model.ValidationStrict
	if err := api.Validate(bytes.NewReader(pdf), configuration); err != nil {
		return 0, bizerr.NewCode(CodeContentInvalid)
	}
	count, err := api.PageCount(bytes.NewReader(pdf), configuration)
	if err != nil || count < 1 || count > maximumTemplatePages {
		return 0, bizerr.NewCode(CodeContentInvalid)
	}
	return count, nil
}

func normalizeFields(fields []FieldPlacement, scope string, pageCount int) ([]FieldPlacement, error) {
	if !validScope(scope) || pageCount < 1 || pageCount > maximumTemplatePages ||
		len(fields) < 1 || len(fields) > maximumTemplateFields {
		return nil, bizerr.NewCode(CodeContentInvalid)
	}
	result := make([]FieldPlacement, len(fields))
	for index, source := range fields {
		field := source
		field.FieldNo = ""
		field.SequenceNo = index + 1
		field.FieldTypeCode = strings.TrimSpace(field.FieldTypeCode)
		field.PartyCode = strings.TrimSpace(field.PartyCode)
		field.VariableCode = strings.TrimSpace(field.VariableCode)
		if field.PageNo < 1 || field.PageNo > pageCount ||
			field.PositionXPPM < 0 || field.PositionYPPM < 0 || field.WidthPPM < 1 || field.HeightPPM < 1 ||
			field.PositionXPPM+field.WidthPPM > 1_000_000 || field.PositionYPPM+field.HeightPPM > 1_000_000 ||
			(field.PartyCode != PartyCustomer && field.PartyCode != PartyMerchant) {
			return nil, bizerr.NewCode(CodeContentInvalid)
		}
		switch field.FieldTypeCode {
		case FieldTypeTextVariable:
			if contractVariables[field.VariableCode] == "" || contractVariables[field.VariableCode] != field.PartyCode {
				return nil, bizerr.NewCode(CodeContentInvalid)
			}
		case FieldTypeSignature:
			if field.VariableCode != "" {
				return nil, bizerr.NewCode(CodeContentInvalid)
			}
		case FieldTypeEnterpriseSeal:
			if field.VariableCode != "" || field.PartyCode == PartyCustomer && scope != ScopeEnterprise {
				return nil, bizerr.NewCode(CodeContentInvalid)
			}
		case FieldTypeSigningDate:
			if field.VariableCode != "" {
				return nil, bizerr.NewCode(CodeContentInvalid)
			}
		default:
			return nil, bizerr.NewCode(CodeContentInvalid)
		}
		result[index] = field
	}
	return result, nil
}

func signingPositionsReady(fields []FieldPlacement, scope string) bool {
	customer, merchant := false, false
	for _, field := range fields {
		if field.FieldTypeCode != FieldTypeSignature && field.FieldTypeCode != FieldTypeEnterpriseSeal {
			continue
		}
		if field.PartyCode == PartyCustomer &&
			(field.FieldTypeCode == FieldTypeSignature || scope == ScopeEnterprise) {
			customer = true
		}
		if field.PartyCode == PartyMerchant {
			merchant = true
		}
	}
	return customer && merchant
}

func fieldSetFingerprint(contentRevisionNo string, fields []FieldPlacement) (string, error) {
	encoded, err := json.Marshal(struct {
		Version           string           `json:"version"`
		ContentRevisionNo string           `json:"contentRevisionNo"`
		Fields            []FieldPlacement `json:"fields"`
	}{Version: "contract-template-fields-v1", ContentRevisionNo: contentRevisionNo, Fields: fields})
	if err != nil {
		return "", err
	}
	return sha256Hex(encoded), nil
}

func newContentNo(prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}
