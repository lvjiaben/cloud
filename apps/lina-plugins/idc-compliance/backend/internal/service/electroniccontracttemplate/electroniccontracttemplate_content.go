// This file implements immutable encrypted template sources, PDF previews, confirmation, and fields.

package electroniccontracttemplate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type editorStateRow struct {
	CurrentContentRevisionNo       *string `json:"current_content_revision_no"`
	CurrentFieldSetNo              *string `json:"current_field_set_no"`
	PreviewConfirmed               bool    `json:"preview_confirmed"`
	ProviderCustomerSignatureCount int     `json:"provider_customer_signature_count"`
	ProviderCustomerSealCount      int     `json:"provider_customer_seal_count"`
	ProviderMerchantSignatureCount int     `json:"provider_merchant_signature_count"`
	ProviderMerchantSealCount      int     `json:"provider_merchant_seal_count"`
	CurrentProviderMappingSetNo    *string `json:"current_provider_mapping_set_no"`
}

type contentRow struct {
	ContentRevisionNo    string     `json:"content_revision_no"`
	RevisionNo           int64      `json:"revision_no"`
	CommandFingerprint   string     `json:"command_fingerprint"`
	SourceModeCode       string     `json:"source_mode_code"`
	EncryptionKeyNo      string     `json:"encryption_key_no"`
	SourceObjectRef      string     `json:"source_object_ref"`
	SourceSHA256         string     `json:"source_sha256"`
	SourceMediaType      string     `json:"source_media_type"`
	SourceByteSize       int64      `json:"source_byte_size"`
	PDFObjectRef         *string    `json:"pdf_object_ref"`
	PDFSHA256            *string    `json:"pdf_sha256"`
	PDFByteSize          *int64     `json:"pdf_byte_size"`
	PDFPageCount         *int       `json:"pdf_page_count"`
	ConversionStatusCode string     `json:"conversion_status_code"`
	CreatedAt            *time.Time `json:"created_at"`
}

type fieldInsertRow struct {
	TenantID      int     `orm:"tenant_id"`
	FieldNo       string  `orm:"field_no"`
	FieldSetNo    string  `orm:"field_set_no"`
	TemplateNo    string  `orm:"template_no"`
	VersionNo     int64   `orm:"version_no"`
	SequenceNo    int     `orm:"sequence_no"`
	FieldTypeCode string  `orm:"field_type_code"`
	PartyCode     string  `orm:"party_code"`
	VariableCode  *string `orm:"variable_code"`
	PageNo        int     `orm:"page_no"`
	PositionXPPM  int     `orm:"position_x_ppm"`
	PositionYPPM  int     `orm:"position_y_ppm"`
	WidthPPM      int     `orm:"width_ppm"`
	HeightPPM     int     `orm:"height_ppm"`
}

const contentProjectionColumns = `
content_revision_no, revision_no, command_fingerprint, source_mode_code,
encryption_key_no, source_object_ref, source_sha256, source_media_type,
source_byte_size, pdf_object_ref, pdf_sha256, pdf_byte_size, pdf_page_count,
conversion_status_code, created_at`

// GetEditor returns the exact current safe authoring projection without supplier calls.
func (s *serviceImpl) GetEditor(ctx context.Context, templateNo string, versionNo int64) (*EditorDetail, error) {
	item, err := s.Get(ctx, templateNo, versionNo)
	if err != nil {
		return nil, err
	}
	state, err := s.loadEditorState(ctx, templateNo, versionNo)
	if err != nil {
		return nil, err
	}
	detail := &EditorDetail{
		Item: *item, Fields: []FieldPlacement{}, PreviewConfirmed: state.PreviewConfirmed,
		SubmissionAvailable: false,
	}
	if item.OriginCode == OriginTencentSynced {
		detail.SubmissionReasonCode = "tencent_synced_template_read_only"
		detail.ProviderComponents, detail.ProviderRecipients, err = s.loadProviderMappingProjection(
			ctx, templateNo, versionNo, state.CurrentProviderMappingSetNo,
		)
		if err != nil {
			return nil, err
		}
		detail.ProviderMappingSaved = state.CurrentProviderMappingSetNo != nil
		detail.EnableReasonCode = providerMappingReason(
			*item, *state, detail.ProviderComponents, detail.ProviderRecipients,
		)
		detail.EnableReady = detail.EnableReasonCode == ""
		if !detail.EnableReady && detail.EnableReasonCode == "" {
			detail.EnableReasonCode = "provider_mapping_incomplete"
		}
		return detail, nil
	}
	detail.SubmissionReasonCode = "tencent_template_submission_unavailable"
	if state.CurrentContentRevisionNo == nil {
		detail.EnableReasonCode = "source_missing"
		return detail, nil
	}
	content, err := s.loadContent(ctx, templateNo, versionNo, *state.CurrentContentRevisionNo)
	if err != nil {
		return nil, err
	}
	projected, ok := projectContent(content)
	if !ok {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	detail.Content = projected
	if content.SourceModeCode == SourceModeOnlineEditor {
		source, readErr := s.openContentObject(ctx, content, false)
		if readErr != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		detail.OnlineHTML = string(source)
		clearContentBytes(source)
	}
	if state.CurrentFieldSetNo != nil {
		detail.Fields, err = s.loadFields(ctx, templateNo, versionNo, *state.CurrentFieldSetNo)
		if err != nil {
			return nil, err
		}
	}
	switch {
	case content.ConversionStatusCode != ConversionReady:
		detail.EnableReasonCode = "preview_missing"
	case !state.PreviewConfirmed:
		detail.EnableReasonCode = "preview_not_confirmed"
	case !signingPositionsReady(detail.Fields, item.ScopeCode):
		detail.EnableReasonCode = "signing_positions_incomplete"
	default:
		detail.EnableReady = true
	}
	return detail, nil
}

// OpenProviderPreview resolves the server-owned Tencent ref only after local authorization.
func (s *serviceImpl) OpenProviderPreview(ctx context.Context, templateNo string, versionNo int64) (string, error) {
	item, err := s.Get(ctx, templateNo, versionNo)
	if err != nil {
		return "", err
	}
	if item.OriginCode != OriginTencentSynced || item.ProviderState == ProviderStateOffline {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	previewGateway, ok := s.gateway.(tencentPreviewGateway)
	if !ok || nilDependency(previewGateway) {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	var source *struct {
		ProviderTemplateRef string `json:"provider_template_ref"`
	}
	if err = s.database.GetScan(ctx, &source, `
SELECT provider_template_ref
FROM plugin_idc_compliance_contract_template
WHERE tenant_id = ? AND template_no = ? AND origin_code = 'tencent_synced'
  AND provider_code = 'tencent_essbasic'`, idccap.DefaultTenantID, templateNo); err != nil ||
		source == nil || strings.TrimSpace(source.ProviderTemplateRef) == "" {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	previewURL, err := previewGateway.TencentTemplatePreview(
		ctx, strings.TrimSpace(source.ProviderTemplateRef),
	)
	if err != nil || strings.TrimSpace(previewURL) == "" {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	return strings.TrimSpace(previewURL), nil
}

// SaveUploadedSource appends one encrypted source revision and clears stale preview/field pointers.
func (s *serviceImpl) SaveUploadedSource(ctx context.Context, input UploadedSourceInput) (*EditorDetail, error) {
	input, err := normalizeUploadedSourceInput(input)
	if err != nil {
		return nil, err
	}
	source, err := readBounded(input.Body, maximumUploadedSourceBytes)
	if err != nil {
		return nil, bizerr.NewCode(CodeContentInvalid)
	}
	defer clearContentBytes(source)
	mediaType, err := detectUploadedSourceMediaType(source, input.DeclaredMediaType)
	if err != nil {
		return nil, err
	}
	if mediaType == "text/html" {
		cleaned, cleanErr := sanitizeOnlineHTML(string(source))
		if cleanErr != nil {
			return nil, cleanErr
		}
		clearContentBytes(source)
		source = []byte(cleaned)
	}
	return s.saveSource(ctx, input.TemplateNo, input.VersionNo, input.ExpectedVersion,
		input.IdempotencyKey, SourceModeUploadedFile, mediaType, source)
}

// SaveOnlineSource appends one sanitized HTML source revision without auto-generating a PDF.
func (s *serviceImpl) SaveOnlineSource(ctx context.Context, input OnlineSourceInput) (*EditorDetail, error) {
	input, err := normalizeOnlineSourceInput(input)
	if err != nil {
		return nil, err
	}
	cleaned, err := sanitizeOnlineHTML(input.HTML)
	if err != nil {
		return nil, err
	}
	source := []byte(cleaned)
	defer clearContentBytes(source)
	return s.saveSource(ctx, input.TemplateNo, input.VersionNo, input.ExpectedVersion,
		input.IdempotencyKey, SourceModeOnlineEditor, "text/html", source)
}

func (s *serviceImpl) saveSource(
	ctx context.Context,
	templateNo string,
	versionNo int64,
	expectedVersion int64,
	idempotencyKey string,
	sourceMode string,
	mediaType string,
	source []byte,
) (*EditorDetail, error) {
	actor, err := s.authorize(ctx, PermissionManage, templateNo)
	if err != nil {
		return nil, err
	}
	if !s.contentReady() {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	sourceDigest := sha256Hex(source)
	commandDigest := sha256Hex([]byte("contract-template-source\x00" + idempotencyKey))
	fingerprint := sha256Hex([]byte(strings.Join([]string{
		"contract-template-source-v1", templateNo, strconv.FormatInt(versionNo, 10),
		sourceMode, mediaType, sourceDigest,
	}, "\x00")))
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadMutableRow(ctx, tx, templateNo, versionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		existing, loadErr := loadContentByCommandTX(ctx, tx, commandDigest)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.CommandFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.OriginCode != OriginLocal || row.StatusCode != StatusDraft ||
			row.LockVersion != expectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		_, appendErr := s.appendContentRevision(ctx, tx, appendContentInput{
			Actor: actor, TemplateNo: templateNo, VersionNo: versionNo,
			CommandDigest: commandDigest, CommandFingerprint: fingerprint,
			SourceMode: sourceMode, SourceMediaType: mediaType, Source: source,
			ConversionStatus: ConversionSourceSaved,
		})
		if appendErr != nil {
			return appendErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.source",
			IdempotencyKey:   commandDigest, OperationCode: "compliance.contract-template.source",
			TargetType: auditTargetType, TargetNo: templateNo, ResultCode: "succeeded",
			ReasonCode: "immutable_source_saved", RiskCode: auditRiskHigh,
			RequiredPermission: PermissionManage, BeforeSnapshot: templateAuditSnapshot(row),
			AfterSnapshot: map[string]any{
				"version_no": versionNo, "source_mode": sourceMode,
				"source_media_type": mediaType, "source_byte_size": len(source),
				"lock_version": expectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.GetEditor(ctx, templateNo, versionNo)
}

// GeneratePreview converts the exact current source outside the database transaction,
// then appends a new source+PDF revision only if the optimistic pointers are unchanged.
func (s *serviceImpl) GeneratePreview(ctx context.Context, input GeneratePreviewInput) (*EditorDetail, error) {
	input, err := normalizeGeneratePreviewInput(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionManage, input.TemplateNo)
	if err != nil {
		return nil, err
	}
	if !s.contentReady() {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	state, err := s.loadEditorState(ctx, input.TemplateNo, input.VersionNo)
	if err != nil || state.CurrentContentRevisionNo == nil ||
		*state.CurrentContentRevisionNo != input.ContentRevisionNo {
		return nil, bizerr.NewCode(CodeConflict)
	}
	item, err := s.get(ctx, input.TemplateNo, input.VersionNo, false)
	if err != nil || item.OriginCode != OriginLocal || item.StatusCode != StatusDraft ||
		item.LockVersion != input.ExpectedVersion {
		return nil, firstError(err, bizerr.NewCode(CodeConflict))
	}
	current, err := s.loadContent(ctx, input.TemplateNo, input.VersionNo, input.ContentRevisionNo)
	if err != nil {
		return nil, err
	}
	source, err := s.openContentObject(ctx, current, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clearContentBytes(source)
	var pdf []byte
	if current.SourceMediaType == "application/pdf" {
		pdf = append([]byte(nil), source...)
	} else {
		conversionContext, cancel := context.WithTimeout(ctx, 90*time.Second)
		pdf, err = s.converter.Convert(conversionContext, current.SourceMediaType, source)
		cancel()
		if err != nil {
			clearContentBytes(pdf)
			return nil, bizerr.NewCode(CodeConversionUnavailable)
		}
	}
	defer clearContentBytes(pdf)
	pageCount, err := validatePDF(pdf)
	if err != nil {
		return nil, bizerr.NewCode(CodeConversionUnavailable)
	}
	pdfDigest := sha256Hex(pdf)
	commandDigest := sha256Hex([]byte("contract-template-preview\x00" + input.IdempotencyKey))
	fingerprint := sha256Hex([]byte(strings.Join([]string{
		"contract-template-preview-v1", input.TemplateNo, strconv.FormatInt(input.VersionNo, 10),
		input.ContentRevisionNo, current.SourceSHA256, pdfDigest, strconv.Itoa(pageCount),
	}, "\x00")))
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadMutableRow(ctx, tx, input.TemplateNo, input.VersionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		existing, loadErr := loadContentByCommandTX(ctx, tx, commandDigest)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.CommandFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.OriginCode != OriginLocal || row.StatusCode != StatusDraft ||
			row.LockVersion != input.ExpectedVersion || row.CurrentContentRevisionNo == nil ||
			*row.CurrentContentRevisionNo != input.ContentRevisionNo {
			return bizerr.NewCode(CodeConflict)
		}
		_, appendErr := s.appendContentRevision(ctx, tx, appendContentInput{
			Actor: actor, TemplateNo: input.TemplateNo, VersionNo: input.VersionNo,
			CommandDigest: commandDigest, CommandFingerprint: fingerprint,
			SourceMode: current.SourceModeCode, SourceMediaType: current.SourceMediaType,
			Source: source, PDF: pdf, PDFPageCount: pageCount, ConversionStatus: ConversionReady,
		})
		if appendErr != nil {
			return appendErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.preview",
			IdempotencyKey:   commandDigest, OperationCode: "compliance.contract-template.preview",
			TargetType: auditTargetType, TargetNo: input.TemplateNo, ResultCode: "succeeded",
			ReasonCode: "immutable_pdf_generated", RiskCode: auditRiskHigh,
			RequiredPermission: PermissionManage, BeforeSnapshot: templateAuditSnapshot(row),
			AfterSnapshot: map[string]any{
				"version_no": input.VersionNo, "pdf_byte_size": len(pdf),
				"pdf_page_count": pageCount, "lock_version": input.ExpectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.GetEditor(ctx, input.TemplateNo, input.VersionNo)
}

// ConfirmPreview records an explicit confirmation for the exact current immutable PDF.
func (s *serviceImpl) ConfirmPreview(ctx context.Context, input ConfirmPreviewInput) (*EditorDetail, error) {
	input, err := normalizeConfirmPreviewInput(input)
	if err != nil {
		return nil, err
	}
	if _, err = s.authorize(ctx, PermissionManage, input.TemplateNo); err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadMutableRow(ctx, tx, input.TemplateNo, input.VersionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		if row.OriginCode != OriginLocal || row.StatusCode != StatusDraft ||
			row.LockVersion != input.ExpectedVersion || row.CurrentContentRevisionNo == nil ||
			*row.CurrentContentRevisionNo != input.ContentRevisionNo || row.PreviewConfirmed {
			return bizerr.NewCode(CodeConflict)
		}
		content, loadErr := loadContentTX(ctx, tx, input.TemplateNo, input.VersionNo, input.ContentRevisionNo)
		if loadErr != nil || content == nil || content.ConversionStatusCode != ConversionReady ||
			content.PDFObjectRef == nil {
			return firstError(loadErr, bizerr.NewCode(CodeConflict))
		}
		result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET preview_confirmed = TRUE, lock_version = lock_version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND lock_version = ?
  AND status_code = 'draft' AND current_content_revision_no = ? AND preview_confirmed = FALSE`,
			idccap.DefaultTenantID, input.TemplateNo, input.VersionNo,
			input.ExpectedVersion, input.ContentRevisionNo)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOne(result); updateErr != nil {
			return updateErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.preview-confirm",
			IdempotencyKey:   input.TemplateNo + ":" + stringInt(input.VersionNo) + ":" + stringInt(input.ExpectedVersion),
			OperationCode:    "compliance.contract-template.preview-confirm",
			TargetType:       auditTargetType, TargetNo: input.TemplateNo, ResultCode: "succeeded",
			ReasonCode: "pdf_preview_confirmed", RiskCode: auditRiskHigh,
			RequiredPermission: PermissionManage, BeforeSnapshot: templateAuditSnapshot(row),
			AfterSnapshot: map[string]any{
				"version_no": input.VersionNo, "content_revision_no": input.ContentRevisionNo,
				"preview_confirmed": true, "lock_version": input.ExpectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.GetEditor(ctx, input.TemplateNo, input.VersionNo)
}

// OpenPreview decrypts only the exact current immutable local PDF.
func (s *serviceImpl) OpenPreview(ctx context.Context, templateNo string, versionNo int64, contentRevisionNo string) (*PreviewContent, error) {
	templateNo = strings.TrimSpace(templateNo)
	contentRevisionNo = strings.TrimSpace(contentRevisionNo)
	if !templateNoPattern.MatchString(templateNo) || versionNo <= 0 ||
		!contentRevisionNoPattern.MatchString(contentRevisionNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorize(ctx, PermissionQuery, templateNo); err != nil {
		return nil, err
	}
	state, err := s.loadEditorState(ctx, templateNo, versionNo)
	if err != nil || state.CurrentContentRevisionNo == nil ||
		*state.CurrentContentRevisionNo != contentRevisionNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	content, err := s.loadContent(ctx, templateNo, versionNo, contentRevisionNo)
	if err != nil || content.ConversionStatusCode != ConversionReady || content.PDFObjectRef == nil ||
		content.PDFSHA256 == nil {
		return nil, firstError(err, bizerr.NewCode(CodeNotAccessible))
	}
	pdf, err := s.openContentObject(ctx, content, true)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &PreviewContent{Content: pdf, SHA256: *content.PDFSHA256}, nil
}

// SaveFieldSet appends one complete immutable placement set for the confirmed current PDF.
func (s *serviceImpl) SaveFieldSet(ctx context.Context, input SaveFieldSetInput) (*EditorDetail, error) {
	input, err := normalizeSaveFieldSetInput(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionManage, input.TemplateNo)
	if err != nil {
		return nil, err
	}
	commandDigest := sha256Hex([]byte("contract-template-fields\x00" + input.IdempotencyKey))
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadMutableRow(ctx, tx, input.TemplateNo, input.VersionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		content, loadErr := loadContentTX(ctx, tx, input.TemplateNo, input.VersionNo, input.ContentRevisionNo)
		if loadErr != nil || content == nil || content.PDFPageCount == nil {
			return firstError(loadErr, bizerr.NewCode(CodeConflict))
		}
		fields, normalizeErr := normalizeFields(input.Fields, safeString(row.ScopeCode), *content.PDFPageCount)
		if normalizeErr != nil {
			return normalizeErr
		}
		fingerprint, fingerprintErr := fieldSetFingerprint(input.ContentRevisionNo, fields)
		if fingerprintErr != nil {
			return fingerprintErr
		}
		var existing *struct {
			FieldSetNo         string `json:"field_set_no"`
			CommandFingerprint string `json:"command_fingerprint"`
		}
		if loadErr = tx.Ctx(ctx).GetScan(&existing, `
SELECT field_set_no, command_fingerprint
FROM plugin_idc_compliance_contract_template_field_set
WHERE tenant_id = ? AND command_idempotency_digest = ?
FOR UPDATE`, idccap.DefaultTenantID, commandDigest); loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.CommandFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.OriginCode != OriginLocal || row.StatusCode != StatusDraft ||
			row.LockVersion != input.ExpectedVersion || !row.PreviewConfirmed ||
			row.CurrentContentRevisionNo == nil || *row.CurrentContentRevisionNo != input.ContentRevisionNo ||
			content.ConversionStatusCode != ConversionReady {
			return bizerr.NewCode(CodeConflict)
		}
		fieldSetNo, createErr := newContentNo("ctfs_")
		if createErr != nil {
			return createErr
		}
		if _, createErr = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_contract_template_field_set
    (tenant_id, field_set_no, template_no, version_no, content_revision_no,
     command_idempotency_digest, command_fingerprint, created_by_user_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, fieldSetNo, input.TemplateNo,
			input.VersionNo, input.ContentRevisionNo, commandDigest, fingerprint, actor); createErr != nil {
			return createErr
		}
		fieldRows := make([]fieldInsertRow, 0, len(fields))
		for index := range fields {
			fieldNo, numberErr := newContentNo("ctpf_")
			if numberErr != nil {
				return numberErr
			}
			fields[index].FieldNo = fieldNo
			var variable *string
			if fields[index].VariableCode != "" {
				variableCode := fields[index].VariableCode
				variable = &variableCode
			}
			fieldRows = append(fieldRows, fieldInsertRow{
				TenantID: idccap.DefaultTenantID, FieldNo: fieldNo,
				FieldSetNo: fieldSetNo, TemplateNo: input.TemplateNo,
				VersionNo: input.VersionNo, SequenceNo: fields[index].SequenceNo,
				FieldTypeCode: fields[index].FieldTypeCode, PartyCode: fields[index].PartyCode,
				VariableCode: variable, PageNo: fields[index].PageNo,
				PositionXPPM: fields[index].PositionXPPM, PositionYPPM: fields[index].PositionYPPM,
				WidthPPM: fields[index].WidthPPM, HeightPPM: fields[index].HeightPPM,
			})
		}
		if _, createErr = tx.Ctx(ctx).Model("plugin_idc_compliance_contract_template_field").Data(fieldRows).Insert(); createErr != nil {
			return createErr
		}
		result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET current_field_set_no = ?, lock_version = lock_version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND lock_version = ?
  AND status_code = 'draft' AND current_content_revision_no = ? AND preview_confirmed = TRUE`,
			fieldSetNo, idccap.DefaultTenantID, input.TemplateNo, input.VersionNo,
			input.ExpectedVersion, input.ContentRevisionNo)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOne(result); updateErr != nil {
			return updateErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.fields", IdempotencyKey: commandDigest,
			OperationCode: "compliance.contract-template.fields", TargetType: auditTargetType,
			TargetNo: input.TemplateNo, ResultCode: "succeeded", ReasonCode: "immutable_fields_saved",
			RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
			BeforeSnapshot: templateAuditSnapshot(row), AfterSnapshot: map[string]any{
				"version_no": input.VersionNo, "field_count": len(fields),
				"signing_positions_complete": signingPositionsReady(fields, safeString(row.ScopeCode)),
				"lock_version":               input.ExpectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.GetEditor(ctx, input.TemplateNo, input.VersionNo)
}

type appendContentInput struct {
	Actor              int
	TemplateNo         string
	VersionNo          int64
	CommandDigest      string
	CommandFingerprint string
	SourceMode         string
	SourceMediaType    string
	Source             []byte
	PDF                []byte
	PDFPageCount       int
	ConversionStatus   string
}

func (s *serviceImpl) appendContentRevision(ctx context.Context, tx gdb.TX, input appendContentInput) (string, error) {
	contentNo, err := newContentNo("ctpc_")
	if err != nil {
		return "", err
	}
	nextRevisionValue, err := tx.Ctx(ctx).GetValue(`
SELECT COALESCE(MAX(revision_no), 0) + 1
FROM plugin_idc_compliance_contract_template_content
WHERE tenant_id = ? AND template_no = ? AND version_no = ?`,
		idccap.DefaultTenantID, input.TemplateNo, input.VersionNo)
	nextRevision := nextRevisionValue.Int64()
	if err != nil || nextRevision <= 0 {
		return "", firstError(err, bizerr.NewCode(CodeUnavailable))
	}
	created, err := s.keyManager.CreateGenerated(
		ctx, "ctpl-content-"+input.CommandDigest, secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || created.Handle.IsZero() {
		return "", firstError(err, bizerr.NewCode(CodeUnavailable))
	}
	keyReference := created.Handle.OpaqueRef()
	sourcePath := contentObjectPath(input.TemplateNo, input.VersionNo, contentNo, "source")
	pdfPath := ""
	err = s.withContentKey(ctx, keyReference, func(key []byte) error {
		sourceEnvelope, encryptErr := encryptContentEnvelope(key, input.Source, contentObjectLabel(contentNo, "source"))
		if encryptErr != nil {
			return encryptErr
		}
		defer clearContentBytes(sourceEnvelope)
		if encryptErr = s.putOrConfirmContentEnvelope(ctx, sourcePath, sourceEnvelope); encryptErr != nil {
			return encryptErr
		}
		if len(input.PDF) > 0 {
			pdfPath = contentObjectPath(input.TemplateNo, input.VersionNo, contentNo, "preview")
			pdfEnvelope, pdfErr := encryptContentEnvelope(key, input.PDF, contentObjectLabel(contentNo, "preview"))
			if pdfErr != nil {
				return pdfErr
			}
			defer clearContentBytes(pdfEnvelope)
			return s.putOrConfirmContentEnvelope(ctx, pdfPath, pdfEnvelope)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	pdfDigest := any(nil)
	pdfSize := any(nil)
	pdfPages := any(nil)
	pdfObject := any(nil)
	if len(input.PDF) > 0 {
		pdfDigest = sha256Hex(input.PDF)
		pdfSize = len(input.PDF)
		pdfPages = input.PDFPageCount
		pdfObject = pdfPath
	}
	if _, err = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_contract_template_content
    (tenant_id, content_revision_no, template_no, version_no, revision_no,
     command_idempotency_digest, command_fingerprint, source_mode_code,
     encryption_key_no, source_object_ref, source_sha256, source_media_type,
     source_byte_size, pdf_object_ref, pdf_sha256, pdf_byte_size, pdf_page_count,
     conversion_status_code, created_by_user_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idccap.DefaultTenantID, contentNo, input.TemplateNo, input.VersionNo, nextRevision,
		input.CommandDigest, input.CommandFingerprint, input.SourceMode, keyReference,
		sourcePath, sha256Hex(input.Source), input.SourceMediaType, len(input.Source),
		pdfObject, pdfDigest, pdfSize, pdfPages, input.ConversionStatus, input.Actor); err != nil {
		return "", err
	}
	result, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET current_content_revision_no = ?, current_field_set_no = NULL,
    preview_confirmed = FALSE, lock_version = lock_version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND status_code = 'draft'`,
		contentNo, idccap.DefaultTenantID, input.TemplateNo, input.VersionNo)
	if err != nil {
		return "", err
	}
	if err = requireOne(result); err != nil {
		return "", err
	}
	return contentNo, nil
}

func (s *serviceImpl) loadEditorState(ctx context.Context, templateNo string, versionNo int64) (*editorStateRow, error) {
	var row *editorStateRow
	if err := s.database.GetScan(ctx, &row, `
SELECT current_content_revision_no, current_field_set_no, preview_confirmed,
       provider_customer_signature_count, provider_customer_seal_count,
       provider_merchant_signature_count, provider_merchant_seal_count,
       current_provider_mapping_set_no
FROM plugin_idc_compliance_contract_template_version
WHERE tenant_id = ? AND template_no = ? AND version_no = ?`,
		idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return row, nil
}

func (s *serviceImpl) loadContent(ctx context.Context, templateNo string, versionNo int64, contentNo string) (*contentRow, error) {
	var row *contentRow
	if err := s.database.GetScan(ctx, &row, `SELECT `+contentProjectionColumns+`
FROM plugin_idc_compliance_contract_template_content
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND content_revision_no = ?`,
		idccap.DefaultTenantID, templateNo, versionNo, contentNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return row, nil
}

func loadContentTX(ctx context.Context, tx gdb.TX, templateNo string, versionNo int64, contentNo string) (*contentRow, error) {
	var row *contentRow
	err := tx.Ctx(ctx).GetScan(&row, `SELECT `+contentProjectionColumns+`
FROM plugin_idc_compliance_contract_template_content
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND content_revision_no = ?
FOR UPDATE`, idccap.DefaultTenantID, templateNo, versionNo, contentNo)
	return row, err
}

func loadContentByCommandTX(ctx context.Context, tx gdb.TX, digest string) (*contentRow, error) {
	var row *contentRow
	err := tx.Ctx(ctx).GetScan(&row, `SELECT `+contentProjectionColumns+`
FROM plugin_idc_compliance_contract_template_content
WHERE tenant_id = ? AND command_idempotency_digest = ?
FOR UPDATE`, idccap.DefaultTenantID, digest)
	return row, err
}

func (s *serviceImpl) loadFields(ctx context.Context, templateNo string, versionNo int64, fieldSetNo string) ([]FieldPlacement, error) {
	if !fieldSetNoPattern.MatchString(fieldSetNo) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	var fields []FieldPlacement
	if err := s.database.GetScan(ctx, &fields, `
SELECT field_no, sequence_no, field_type_code, party_code,
       COALESCE(variable_code, '') AS variable_code, page_no,
       position_x_ppm, position_y_ppm, width_ppm, height_ppm
FROM plugin_idc_compliance_contract_template_field
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND field_set_no = ?
ORDER BY sequence_no`, idccap.DefaultTenantID, templateNo, versionNo, fieldSetNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(fields) < 1 || len(fields) > maximumTemplateFields {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	for index := range fields {
		if !fieldNoPattern.MatchString(fields[index].FieldNo) || fields[index].SequenceNo != index+1 {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	return fields, nil
}

func (s *serviceImpl) openContentObject(ctx context.Context, row *contentRow, pdf bool) ([]byte, error) {
	if row == nil || !validDigest(row.SourceSHA256) || row.EncryptionKeyNo == "" {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	path := row.SourceObjectRef
	digest := row.SourceSHA256
	maximum := maximumUploadedSourceBytes
	label := contentObjectLabel(row.ContentRevisionNo, "source")
	if pdf {
		if row.PDFObjectRef == nil || row.PDFSHA256 == nil || !validDigest(*row.PDFSHA256) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		path, digest, maximum = *row.PDFObjectRef, *row.PDFSHA256, maximumPDFBytes
		label = contentObjectLabel(row.ContentRevisionNo, "preview")
	}
	envelope, err := s.readContentEnvelope(ctx, path, int64(maximum))
	if err != nil {
		return nil, err
	}
	defer clearContentBytes(envelope)
	var plaintext []byte
	err = s.withContentKey(ctx, row.EncryptionKeyNo, func(key []byte) error {
		var decryptErr error
		plaintext, decryptErr = decryptContentEnvelope(key, envelope, label, maximum)
		return decryptErr
	})
	if err != nil {
		clearContentBytes(plaintext)
		return nil, err
	}
	actual := sha256.Sum256(plaintext)
	actualHex := hex.EncodeToString(actual[:])
	if actualHex != digest {
		clearContentBytes(plaintext)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return plaintext, nil
}

func projectContent(row *contentRow) (*ContentRevision, bool) {
	if row == nil || !contentRevisionNoPattern.MatchString(row.ContentRevisionNo) || row.RevisionNo <= 0 ||
		!validDigest(row.SourceSHA256) || row.SourceByteSize <= 0 || row.CreatedAt == nil ||
		(row.SourceModeCode != SourceModeUploadedFile && row.SourceModeCode != SourceModeOnlineEditor) ||
		(row.ConversionStatusCode != ConversionSourceSaved && row.ConversionStatusCode != ConversionReady) {
		return nil, false
	}
	result := &ContentRevision{
		ContentRevisionNo: row.ContentRevisionNo, RevisionNo: row.RevisionNo,
		SourceModeCode: row.SourceModeCode, SourceMediaType: row.SourceMediaType,
		SourceByteSize: row.SourceByteSize, SourceSHA256: row.SourceSHA256,
		ConversionStatusCode: row.ConversionStatusCode, CreatedAt: row.CreatedAt.UTC(),
	}
	if row.ConversionStatusCode == ConversionReady {
		if row.PDFSHA256 == nil || row.PDFByteSize == nil || row.PDFPageCount == nil ||
			!validDigest(*row.PDFSHA256) || *row.PDFByteSize <= 0 || *row.PDFPageCount <= 0 {
			return nil, false
		}
		result.PDFSHA256 = *row.PDFSHA256
		result.PDFByteSize = *row.PDFByteSize
		result.PDFPageCount = *row.PDFPageCount
	}
	return result, true
}

func (s *serviceImpl) contentReady() bool {
	return s != nil && !nilDependency(s.storage) && !nilDependency(s.keyAccessor) &&
		!nilDependency(s.keyManager) && !nilDependency(s.converter)
}

func contentObjectPath(templateNo string, versionNo int64, contentNo, kind string) string {
	return "contract-templates/" + templateNo + "/v" + strconv.FormatInt(versionNo, 10) + "/" + contentNo + "/" + kind + ".lct"
}

func contentObjectLabel(contentNo, kind string) string {
	return contentNo + "/" + kind
}
