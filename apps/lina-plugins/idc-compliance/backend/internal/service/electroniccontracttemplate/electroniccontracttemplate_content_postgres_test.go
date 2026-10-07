// This file proves the immutable local authoring workflow against isolated PostgreSQL.

package electroniccontracttemplate

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/pdfcpu/pdfcpu/pkg/api"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type contractTemplateStorageFake struct {
	storagecap.Service
	mu      sync.Mutex
	objects map[string][]byte
}

func newContractTemplateStorageFake() *contractTemplateStorageFake {
	return &contractTemplateStorageFake{objects: make(map[string][]byte)}
}

func (f *contractTemplateStorageFake) Put(
	_ context.Context,
	input storagecap.PutInput,
) (*storagecap.PutOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if input.Overwrite || input.Path == "" || input.ContentType != "application/octet-stream" {
		return nil, errors.New("invalid private contract-template object write")
	}
	if _, exists := f.objects[input.Path]; exists {
		return nil, errors.New("contract-template object already exists")
	}
	content, err := io.ReadAll(input.Body)
	if err != nil || int64(len(content)) != input.Size {
		return nil, errors.New("invalid private contract-template object body")
	}
	f.objects[input.Path] = append([]byte(nil), content...)
	return &storagecap.PutOutput{Object: contractTemplateStorageObject(input.Path, content)}, nil
}

func (f *contractTemplateStorageFake) Get(
	_ context.Context,
	input storagecap.GetInput,
) (*storagecap.GetOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, found := f.objects[input.Path]
	if !found {
		return &storagecap.GetOutput{Found: false}, nil
	}
	copyValue := append([]byte(nil), content...)
	return &storagecap.GetOutput{
		Found: true, Object: contractTemplateStorageObject(input.Path, copyValue),
		Body: io.NopCloser(bytes.NewReader(copyValue)),
	}, nil
}

func contractTemplateStorageObject(path string, content []byte) *storagecap.Object {
	return &storagecap.Object{
		Path: path, Size: int64(len(content)), ContentType: "application/octet-stream",
		Visibility: storagecap.VisibilityPrivate,
	}
}

type contractTemplateSecretsFake struct {
	secretcap.Manager
	mu       sync.Mutex
	sequence int
	handles  map[string]secretcap.Handle
	values   map[string][]byte
}

func newContractTemplateSecretsFake() *contractTemplateSecretsFake {
	return &contractTemplateSecretsFake{
		handles: make(map[string]secretcap.Handle),
		values:  make(map[string][]byte),
	}
}

func (f *contractTemplateSecretsFake) CreateGenerated(
	_ context.Context,
	idempotencyKey string,
	policy string,
) (secretcap.CreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if policy != secretcap.GeneratedSecretPolicyURLSafe256 || idempotencyKey == "" {
		return secretcap.CreateResult{}, errors.New("invalid generated contract-template key request")
	}
	if handle, exists := f.handles[idempotencyKey]; exists {
		return secretcap.CreateResult{Handle: handle, Reused: true}, nil
	}
	f.sequence++
	handle, err := secretcap.NewHandle(fmt.Sprintf("SEC_CONTRACT_TEMPLATE_%032d", f.sequence))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	raw := bytes.Repeat([]byte{byte(0x30 + f.sequence)}, 32)
	encoded := make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	base64.RawURLEncoding.Encode(encoded, raw)
	clearContentBytes(raw)
	f.handles[idempotencyKey] = handle
	f.values[handle.OpaqueRef()] = encoded
	return secretcap.CreateResult{Handle: handle}, nil
}

func (f *contractTemplateSecretsFake) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	f.mu.Lock()
	value, exists := f.values[request.Handle.OpaqueRef()]
	copyValue := append([]byte(nil), value...)
	f.mu.Unlock()
	if !exists || consume == nil || request.Purpose != "compliance.contract-template-content" {
		clearContentBytes(copyValue)
		return errors.New("contract-template key access rejected")
	}
	defer clearContentBytes(copyValue)
	return consume(copyValue)
}

type contractTemplateConverterFake struct {
	mu    sync.Mutex
	calls int
	pdf   []byte
}

func (f *contractTemplateConverterFake) Convert(
	_ context.Context,
	mediaType string,
	source []byte,
) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mediaType != "text/html" || len(source) == 0 {
		return nil, errors.New("unexpected contract-template conversion input")
	}
	f.calls++
	return append([]byte(nil), f.pdf...), nil
}

func TestElectronicContractTemplateContentPostgreSQLWorkflow(t *testing.T) {
	database, cleanup := newTemplateTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	authorizer := &templateAuthorizer{authorized: true}
	storage := newContractTemplateStorageFake()
	secrets := newContractTemplateSecretsFake()
	pdf := contractTemplateTestPDF(t)
	converter := &contractTemplateConverterFake{pdf: pdf}
	serviceValue, err := NewWithContent(
		database,
		managementRuntime(authorizer, &templateAudit{}),
		&templateGateway{},
		storage,
		secrets,
		secrets,
		converter,
	)
	if err != nil {
		t.Fatalf("construct content service: %v", err)
	}
	service := serviceValue.(*serviceImpl)

	created, err := service.Create(ctx, CreateInput{
		Name: "Local authoring", ScopeCode: ScopeAllCustomers,
		SigningOrderCode: SigningOrderCustomerFirst,
		LanguageCode:     LanguageZhCN,
		IdempotencyKey:   "content-workflow-create-0001",
	})
	if err != nil {
		t.Fatalf("create local draft: %v", err)
	}
	html := `<h1 style="text-align:center">Agreement</h1>` +
		`<p>{{customer_name}} and {{merchant_name}}</p>`
	saved, err := service.SaveOnlineSource(ctx, OnlineSourceInput{
		TemplateNo: created.TemplateNo, VersionNo: created.VersionNo,
		ExpectedVersion: created.LockVersion, IdempotencyKey: "content-workflow-source-0001",
		HTML: html,
	})
	if err != nil || saved.Content == nil || saved.Content.RevisionNo != 1 ||
		saved.Content.ConversionStatusCode != ConversionSourceSaved || saved.PreviewConfirmed ||
		len(saved.Fields) != 0 || saved.SubmissionAvailable ||
		saved.SubmissionReasonCode != "tencent_template_submission_unavailable" {
		t.Fatalf("save online source result=%+v err=%v", saved, err)
	}
	if saved.OnlineHTML == "" || !bytes.Contains([]byte(saved.OnlineHTML), []byte("{{customer_name}}")) {
		t.Fatalf("sanitized online source is missing: %q", saved.OnlineHTML)
	}

	replayed, err := service.SaveOnlineSource(ctx, OnlineSourceInput{
		TemplateNo: created.TemplateNo, VersionNo: created.VersionNo,
		ExpectedVersion: created.LockVersion, IdempotencyKey: "content-workflow-source-0001",
		HTML: html,
	})
	if err != nil || replayed.Content == nil || replayed.Content.ContentRevisionNo != saved.Content.ContentRevisionNo {
		t.Fatalf("replay online source result=%+v err=%v", replayed, err)
	}
	requireTemplateRowCount(t, database, "plugin_idc_compliance_contract_template_content", 1)

	preview, err := service.GeneratePreview(ctx, GeneratePreviewInput{
		TemplateNo: saved.TemplateNo, VersionNo: saved.VersionNo,
		ExpectedVersion: saved.LockVersion, ContentRevisionNo: saved.Content.ContentRevisionNo,
		IdempotencyKey: "content-workflow-preview-0001",
	})
	if err != nil || preview.Content == nil || preview.Content.RevisionNo != 2 ||
		preview.Content.ConversionStatusCode != ConversionReady || preview.Content.PDFSHA256 == "" ||
		preview.Content.PDFPageCount != 1 || preview.PreviewConfirmed {
		t.Fatalf("generate preview result=%+v err=%v", preview, err)
	}
	opened, err := service.OpenPreview(
		ctx, preview.TemplateNo, preview.VersionNo, preview.Content.ContentRevisionNo,
	)
	if err != nil || opened.SHA256 != preview.Content.PDFSHA256 || !bytes.Equal(opened.Content, pdf) {
		t.Fatalf("open preview digest=%q err=%v", opened.SHA256, err)
	}
	opened.Clear()
	if opened.Content != nil {
		t.Fatal("opened preview bytes were not released")
	}

	confirmed, err := service.ConfirmPreview(ctx, ConfirmPreviewInput{
		TemplateNo: preview.TemplateNo, VersionNo: preview.VersionNo,
		ExpectedVersion: preview.LockVersion, ContentRevisionNo: preview.Content.ContentRevisionNo,
	})
	if err != nil || !confirmed.PreviewConfirmed {
		t.Fatalf("confirm preview result=%+v err=%v", confirmed, err)
	}
	placed, err := service.SaveFieldSet(ctx, SaveFieldSetInput{
		TemplateNo: confirmed.TemplateNo, VersionNo: confirmed.VersionNo,
		ExpectedVersion: confirmed.LockVersion, ContentRevisionNo: confirmed.Content.ContentRevisionNo,
		IdempotencyKey: "content-workflow-fields-0001",
		Fields: []FieldPlacement{
			{FieldTypeCode: FieldTypeSignature, PartyCode: PartyCustomer, PageNo: 1, PositionXPPM: 80_000, PositionYPPM: 720_000, WidthPPM: 260_000, HeightPPM: 80_000},
			{FieldTypeCode: FieldTypeEnterpriseSeal, PartyCode: PartyMerchant, PageNo: 1, PositionXPPM: 650_000, PositionYPPM: 690_000, WidthPPM: 150_000, HeightPPM: 150_000},
		},
	})
	if err != nil || !placed.EnableReady || len(placed.Fields) != 2 || placed.SubmissionAvailable {
		t.Fatalf("save field set result=%+v err=%v", placed, err)
	}
	if _, err = service.Enable(ctx, StateInput{
		TemplateNo: placed.TemplateNo, VersionNo: placed.VersionNo,
		ExpectedVersion: placed.LockVersion,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("unsubmitted local template enable error=%v", err)
	}

	replaced, err := service.SaveUploadedSource(ctx, UploadedSourceInput{
		TemplateNo: placed.TemplateNo, VersionNo: placed.VersionNo,
		ExpectedVersion: placed.LockVersion, DeclaredMediaType: "application/pdf",
		IdempotencyKey: "content-workflow-source-0002", Body: bytes.NewReader(pdf),
	})
	if err != nil || replaced.Content == nil || replaced.Content.RevisionNo != 3 ||
		replaced.Content.SourceModeCode != SourceModeUploadedFile ||
		replaced.Content.ConversionStatusCode != ConversionSourceSaved ||
		replaced.PreviewConfirmed || len(replaced.Fields) != 0 || replaced.EnableReady {
		t.Fatalf("replace source result=%+v err=%v", replaced, err)
	}
	requireTemplateRowCount(t, database, "plugin_idc_compliance_contract_template_content", 3)
	requireTemplateRowCount(t, database, "plugin_idc_compliance_contract_template_field_set", 1)
	requireTemplateRowCount(t, database, "plugin_idc_compliance_contract_template_field", 2)

	converter.mu.Lock()
	conversionCalls := converter.calls
	converter.mu.Unlock()
	if conversionCalls != 1 {
		t.Fatalf("unexpected conversion calls: %d", conversionCalls)
	}
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if len(storage.objects) != 4 {
		t.Fatalf("unexpected immutable private object count: %d", len(storage.objects))
	}
	for path, content := range storage.objects {
		if bytes.Contains(content, []byte("Agreement")) || bytes.Contains(content, []byte("%PDF-")) {
			t.Fatalf("private object %q contains plaintext", path)
		}
	}
}

func managementRuntime(
	authorizer *templateAuthorizer,
	audit *templateAudit,
) managementcap.Runtime {
	return managementcap.Runtime{Authorizer: authorizer, Audit: audit}
}

func requireTemplateRowCount(t *testing.T, database gdb.DB, table string, expected int) {
	t.Helper()
	value, err := database.GetValue(context.Background(), "SELECT COUNT(*) FROM "+table)
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	actual := value.Int()
	if actual != expected {
		t.Fatalf("count %s=%d, want %d", table, actual, expected)
	}
}

func contractTemplateTestPDF(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			canvas.Set(x, y, color.White)
		}
	}
	var imageBuffer bytes.Buffer
	if err := png.Encode(&imageBuffer, canvas); err != nil {
		t.Fatalf("encode contract-template test image: %v", err)
	}
	var pdfBuffer bytes.Buffer
	if err := api.ImportImages(
		nil, &pdfBuffer, []io.Reader{bytes.NewReader(imageBuffer.Bytes())}, nil, nil,
	); err != nil {
		t.Fatalf("create contract-template test PDF: %v", err)
	}
	pdf := pdfBuffer.Bytes()
	if pageCount, err := validatePDF(pdf); err != nil || pageCount != 1 {
		t.Fatalf("validate contract-template test PDF pages=%d err=%v", pageCount, err)
	}
	return append([]byte(nil), pdf...)
}
