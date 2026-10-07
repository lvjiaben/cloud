// This file maps bounded contract lifecycle and archive evidence.

package admin

import (
	"context"
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

// ElectronicContractGet returns one safe interactive management detail.
func (c *ControllerV1) ElectronicContractGet(
	ctx context.Context,
	req *v1.ElectronicContractGetReq,
) (*v1.ElectronicContractGetRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	detail, err := c.electronicContractPreparation.GetManagementContract(ctx, req.ContractNo)
	if err != nil {
		return nil, err
	}
	result := &v1.ElectronicContractGetRes{
		Item: electronicContractListItem(detail.Item), HistoryTruncated: detail.HistoryTruncated,
		Flows:        make([]v1.ElectronicContractFlowItem, 0, len(detail.Flows)),
		Events:       make([]v1.ElectronicContractEventItem, 0, len(detail.Events)),
		Files:        make([]v1.ElectronicContractFileItem, 0, len(detail.Files)),
		ArchiveJobs:  make([]v1.ElectronicContractArchiveJobItem, 0, len(detail.ArchiveJobs)),
		Replacements: make([]v1.ElectronicContractReplacementItem, 0, len(detail.Replacements)),
	}
	for _, item := range detail.Flows {
		result.Flows = append(result.Flows, v1.ElectronicContractFlowItem{
			FlowNo: item.FlowNo, SequenceNo: item.SequenceNo, PurposeCode: item.PurposeCode,
			StatusCode: item.StatusCode, ProviderStatus: item.ProviderStatus, FailureCode: item.FailureCode,
			ExpiresAt: contractUnixMilli(item.ExpiresAt), SubmittedAt: contractUnixMilli(item.SubmittedAt),
			CompletedAt: contractUnixMilli(item.CompletedAt), CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	for _, item := range detail.Events {
		result.Events = append(result.Events, v1.ElectronicContractEventItem{
			FlowNo: item.FlowNo, SourceKind: item.SourceKind, ProviderStatus: item.ProviderStatus,
			ProjectedStatus: item.ProjectedStatus, DecisionCode: item.DecisionCode, ReasonCode: item.ReasonCode,
			SupplierObservedAt: item.SupplierObservedAt.UnixMilli(), ObservedAt: item.ObservedAt.UnixMilli(),
			CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	for _, item := range detail.Files {
		result.Files = append(result.Files, v1.ElectronicContractFileItem{
			FileNo: item.FileNo, FlowNo: item.FlowNo, FileTypeCode: item.FileTypeCode,
			ContentSha256: item.ContentSHA256, MediaType: item.MediaType, ByteSize: item.ByteSize,
			StatusCode: item.StatusCode, Immutable: item.Immutable,
			ArchivedAt: contractUnixMilli(item.ArchivedAt), RetentionUntil: contractUnixMilli(item.RetentionUntil),
			LegalHold: item.LegalHold, SupplierObservedAt: contractUnixMilli(item.SupplierObservedAt),
			CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	for _, item := range detail.ArchiveJobs {
		result.ArchiveJobs = append(result.ArchiveJobs, v1.ElectronicContractArchiveJobItem{
			JobNo: item.JobNo, FlowNo: item.FlowNo, ArtifactTypeCode: item.ArtifactTypeCode,
			StatusCode: item.StatusCode, AttemptCount: item.AttemptCount, LastReasonCode: item.LastReasonCode,
			SupplierObservedAt: contractUnixMilli(item.SupplierObservedAt), CompletedAt: contractUnixMilli(item.CompletedAt),
			NextAttemptAt: contractUnixMilli(item.NextAttemptAt), CreatedAt: item.CreatedAt.UnixMilli(),
			UpdatedAt: item.UpdatedAt.UnixMilli(),
		})
	}
	for _, item := range detail.Replacements {
		result.Replacements = append(result.Replacements, v1.ElectronicContractReplacementItem{
			ReplacementNo: item.ReplacementNo, OldContractNo: item.OldContractNo,
			NewContractNo: item.NewContractNo, CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return result, nil
}

func electronicContractListItem(
	item electroniccontractpreparation.ManagementContractListItem,
) v1.ElectronicContractListItem {
	return v1.ElectronicContractListItem{
		ContractNo: item.ContractNo, CustomerNo: item.CustomerNo, OrderNo: item.OrderNo,
		ContractTypeCode: item.ContractTypeCode, RequiredForFulfillment: item.RequiredForFulfillment,
		TemplateNo: item.TemplateNo, TemplateVersion: item.TemplateVersion, TemplateName: item.TemplateName,
		LanguageCode: item.LanguageCode, SubjectTypeCode: item.SubjectTypeCode,
		SigningOrderCode: item.SigningOrderCode, AmountMinor: strconv.FormatInt(item.AmountMinor, 10),
		CurrencyCode: item.CurrencyCode, StatusCode: item.StatusCode,
		LaunchedAt: contractUnixMilli(item.LaunchedAt), CompletedAt: contractUnixMilli(item.CompletedAt),
		TerminatedAt: contractUnixMilli(item.TerminatedAt), SigningExpiresAt: contractUnixMilli(item.SigningExpiresAt),
		CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), Version: item.Version,
	}
}

func contractUnixMilli(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UTC().UnixMilli()
}
