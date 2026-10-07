// This file adapts immutable contract-template content and field services to management APIs.

package admin

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontracttemplate"
)

func (c *ControllerV1) ElectronicContractTemplateEditorGet(
	ctx context.Context,
	req *v1.ElectronicContractTemplateEditorGetReq,
) (*v1.ElectronicContractTemplateEditorGetRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.GetEditor(ctx, req.TemplateNo, req.VersionNo)
	if err != nil {
		return nil, err
	}
	mapped := mapElectronicContractTemplateEditor(item)
	return &v1.ElectronicContractTemplateEditorGetRes{Item: mapped}, nil
}

func (c *ControllerV1) ElectronicContractTemplateSourceFileUpdate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateSourceFileUpdateReq,
) (*v1.ElectronicContractTemplateSourceFileUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if c == nil || c.electronicContractTemplates == nil || req == nil || request == nil ||
		request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeInvalid)
	}
	item, err := c.electronicContractTemplates.SaveUploadedSource(ctx, electroniccontracttemplate.UploadedSourceInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
		DeclaredMediaType: request.GetHeader("Content-Type"), Body: request.Request.Body,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplateSourceFileUpdateRes{Item: mapElectronicContractTemplateEditor(item)}, nil
}

func (c *ControllerV1) ElectronicContractTemplateOnlineSourceUpdate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateOnlineSourceUpdateReq,
) (*v1.ElectronicContractTemplateOnlineSourceUpdateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.SaveOnlineSource(ctx, electroniccontracttemplate.OnlineSourceInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, HTML: req.HTML,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplateOnlineSourceUpdateRes{Item: mapElectronicContractTemplateEditor(item)}, nil
}

func (c *ControllerV1) ElectronicContractTemplatePDFPreviewCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplatePDFPreviewCreateReq,
) (*v1.ElectronicContractTemplatePDFPreviewCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.GeneratePreview(ctx, electroniccontracttemplate.GeneratePreviewInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion, ContentRevisionNo: req.ContentRevisionNo,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplatePDFPreviewCreateRes{Item: mapElectronicContractTemplateEditor(item)}, nil
}

func (c *ControllerV1) ElectronicContractTemplatePreviewConfirmationCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplatePreviewConfirmationCreateReq,
) (*v1.ElectronicContractTemplatePreviewConfirmationCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	item, err := c.electronicContractTemplates.ConfirmPreview(ctx, electroniccontracttemplate.ConfirmPreviewInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion, ContentRevisionNo: req.ContentRevisionNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplatePreviewConfirmationCreateRes{Item: mapElectronicContractTemplateEditor(item)}, nil
}

func (c *ControllerV1) ElectronicContractTemplateFieldSetCreate(
	ctx context.Context,
	req *v1.ElectronicContractTemplateFieldSetCreateReq,
) (*v1.ElectronicContractTemplateFieldSetCreateRes, error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	fields := make([]electroniccontracttemplate.FieldPlacement, 0, len(req.Fields))
	for index, field := range req.Fields {
		fields = append(fields, electroniccontracttemplate.FieldPlacement{
			SequenceNo: index + 1, FieldTypeCode: field.FieldTypeCode,
			PartyCode: field.PartyCode, VariableCode: field.VariableCode,
			PageNo: field.PageNo, PositionXPPM: field.PositionXPPM,
			PositionYPPM: field.PositionYPPM, WidthPPM: field.WidthPPM,
			HeightPPM: field.HeightPPM,
		})
	}
	item, err := c.electronicContractTemplates.SaveFieldSet(ctx, electroniccontracttemplate.SaveFieldSetInput{
		TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
		ExpectedVersion: req.ExpectedVersion, ContentRevisionNo: req.ContentRevisionNo,
		IdempotencyKey: req.IdempotencyKey, Fields: fields,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplateFieldSetCreateRes{Item: mapElectronicContractTemplateEditor(item)}, nil
}

func (c *ControllerV1) ElectronicContractTemplatePreviewContentGet(
	ctx context.Context,
	req *v1.ElectronicContractTemplatePreviewContentGetReq,
) (*v1.ElectronicContractTemplatePreviewContentGetRes, error) {
	request := g.RequestFromCtx(ctx)
	if c == nil || c.electronicContractTemplates == nil || req == nil || request == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	content, err := c.electronicContractTemplates.OpenPreview(
		ctx, req.TemplateNo, req.VersionNo, req.ContentRevisionNo,
	)
	if err != nil {
		return nil, err
	}
	defer content.Clear()
	request.Response.Header().Set("Content-Type", "application/pdf")
	request.Response.Header().Set("Content-Length", strconv.Itoa(len(content.Content)))
	request.Response.Header().Set("Content-Disposition", "inline; filename=contract-template-preview.pdf")
	request.Response.Header().Set("Cache-Control", "no-store, max-age=0")
	request.Response.Header().Set("Pragma", "no-cache")
	request.Response.Header().Set("X-Content-Type-Options", "nosniff")
	request.Response.Header().Set("Content-Security-Policy", "sandbox")
	request.Response.Header().Set("X-Contract-Content-SHA256", content.SHA256)
	request.Response.WriteOver(content.Content)
	request.ExitAll()
	return nil, nil
}

func (c *ControllerV1) ElectronicContractTemplateProviderPreviewGet(
	ctx context.Context,
	req *v1.ElectronicContractTemplateProviderPreviewGetReq,
) (*v1.ElectronicContractTemplateProviderPreviewGetRes, error) {
	request := g.RequestFromCtx(ctx)
	if c == nil || c.electronicContractTemplates == nil || req == nil || request == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	previewURL, err := c.electronicContractTemplates.OpenProviderPreview(
		ctx, req.TemplateNo, req.VersionNo,
	)
	if err != nil {
		return nil, err
	}
	request.Response.Header().Set("Cache-Control", "no-store, max-age=0")
	return &v1.ElectronicContractTemplateProviderPreviewGetRes{PreviewURL: previewURL}, nil
}

func mapElectronicContractTemplateEditor(
	item *electroniccontracttemplate.EditorDetail,
) *v1.ElectronicContractTemplateEditorItem {
	if item == nil {
		return nil
	}
	template := mapElectronicContractTemplate(item.Item)
	result := &v1.ElectronicContractTemplateEditorItem{
		Template: &template, Fields: make([]v1.ElectronicContractTemplateFieldItem, 0, len(item.Fields)),
		ProviderComponents: make([]v1.ElectronicContractTemplateProviderComponentItem, 0, len(item.ProviderComponents)),
		ProviderRecipients: make([]v1.ElectronicContractTemplateProviderRecipientItem, 0, len(item.ProviderRecipients)),
		OnlineHTML:         item.OnlineHTML, PreviewConfirmed: item.PreviewConfirmed,
		ProviderMappingSaved: item.ProviderMappingSaved,
		EnableReady:          item.EnableReady, EnableReasonCode: item.EnableReasonCode,
		SubmissionAvailable:  item.SubmissionAvailable,
		SubmissionReasonCode: item.SubmissionReasonCode,
	}
	if item.Content != nil {
		result.Content = &v1.ElectronicContractTemplateContentItem{
			ContentRevisionNo: item.Content.ContentRevisionNo, RevisionNo: item.Content.RevisionNo,
			SourceModeCode: item.Content.SourceModeCode, SourceMediaType: item.Content.SourceMediaType,
			SourceByteSize: item.Content.SourceByteSize, SourceSHA256: item.Content.SourceSHA256,
			ConversionStatusCode: item.Content.ConversionStatusCode,
			PDFSHA256:            item.Content.PDFSHA256, PDFByteSize: item.Content.PDFByteSize,
			PDFPageCount: item.Content.PDFPageCount, CreatedAt: item.Content.CreatedAt.UnixMilli(),
		}
	}
	for _, field := range item.Fields {
		result.Fields = append(result.Fields, v1.ElectronicContractTemplateFieldItem{
			FieldNo: field.FieldNo, SequenceNo: field.SequenceNo,
			FieldTypeCode: field.FieldTypeCode, PartyCode: field.PartyCode,
			VariableCode: field.VariableCode, PageNo: field.PageNo,
			PositionXPPM: field.PositionXPPM, PositionYPPM: field.PositionYPPM,
			WidthPPM: field.WidthPPM, HeightPPM: field.HeightPPM,
		})
	}
	for _, component := range item.ProviderComponents {
		result.ProviderComponents = append(result.ProviderComponents,
			v1.ElectronicContractTemplateProviderComponentItem{
				ComponentRef: component.ComponentRef, Name: component.Name,
				TypeCode: component.TypeCode, Required: component.Required,
				RecipientRef: component.RecipientRef, ValueCode: component.ValueCode,
			})
	}
	for _, recipient := range item.ProviderRecipients {
		result.ProviderRecipients = append(result.ProviderRecipients,
			v1.ElectronicContractTemplateProviderRecipientItem{
				RecipientRef: recipient.RecipientRef, SequenceNo: recipient.SequenceNo,
				TypeCode: recipient.TypeCode,
				RoleName: recipient.RoleName, RequireSign: recipient.RequireSign,
				RoutingOrder: recipient.RoutingOrder, IsPromoter: recipient.IsPromoter,
				SignComponentCount: recipient.SignComponentCount, PartyCode: recipient.PartyCode,
			})
	}
	return result
}
