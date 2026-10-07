// This file maps permissioned staff red-letter endpoints to the service contract.

package admin

import (
	"context"
	"time"

	adminv1 "lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoiceredletter"
)

func (c *ControllerV1) RedLetterTaskGet(
	ctx context.Context,
	req *adminv1.RedLetterTaskGetReq,
) (*adminv1.RedLetterTaskGetRes, error) {
	detail, err := c.taxInvoiceRedLetters.GetAdminDetail(ctx, req.TaskNo)
	if err != nil {
		return nil, err
	}
	res := &adminv1.RedLetterTaskGetRes{Item: mapAdminRedLetter(&detail.Task), HistoryTruncated: detail.HistoryTruncated}
	for _, value := range detail.Candidates {
		res.Candidates = append(res.Candidates, adminv1.RedLetterCandidateItem{RefundAllocationNo: value.RefundAllocationNo, TaxInvoiceNo: value.TaxInvoiceNo, TaxInvoiceAllocationNo: value.TaxInvoiceAllocationNo, TaxInvoiceItemNo: value.TaxInvoiceItemNo, RequestedGrossMinor: value.RequestedGrossMinor, AvailableGrossMinor: value.AvailableGrossMinor, AvailableNetMinor: value.AvailableNetMinor, AvailableTaxMinor: value.AvailableTaxMinor})
	}
	for _, value := range detail.Allocations {
		res.Allocations = append(res.Allocations, adminv1.RedLetterAllocationItem{RefundAllocationNo: value.RefundAllocationNo, TaxInvoiceNo: value.TaxInvoiceNo, TaxInvoiceAllocationNo: value.TaxInvoiceAllocationNo, TaxInvoiceItemNo: value.TaxInvoiceItemNo, GrossAmountMinor: value.GrossAmountMinor, NetAmountMinor: value.NetAmountMinor, TaxAmountMinor: value.TaxAmountMinor})
	}
	if detail.Evidence != nil {
		res.Evidence = &adminv1.RedLetterEvidenceItem{EvidenceNo: detail.Evidence.EvidenceNo, EvidenceReference: detail.Evidence.EvidenceReference, EvidenceSHA256: detail.Evidence.EvidenceSHA256, MediaType: detail.Evidence.MediaType, ByteSize: detail.Evidence.ByteSize, OccurredAt: detail.Evidence.OccurredAt.UnixMilli()}
	}
	return res, nil
}

func (c *ControllerV1) RedLetterTaskList(ctx context.Context, req *adminv1.RedLetterTaskListReq) (*adminv1.RedLetterTaskListRes, error) {
	page, err := c.taxInvoiceRedLetters.ListAdmin(ctx, taxinvoiceredletter.AdminListQuery{Cursor: req.Cursor, CustomerNo: req.CustomerNo, StatusCode: req.StatusCode, Page: req.Page, Limit: req.PageSize})
	if err != nil {
		return nil, err
	}
	res := &adminv1.RedLetterTaskListRes{Items: make([]adminv1.RedLetterTaskItem, 0, len(page.Items)), Total: page.Total, NextCursor: page.NextCursor}
	for index := range page.Items {
		res.Items = append(res.Items, *mapAdminRedLetter(&page.Items[index]))
	}
	return res, nil
}

func (c *ControllerV1) RedLetterAllocationResolutionCreate(
	ctx context.Context,
	req *adminv1.RedLetterAllocationResolutionCreateReq,
) (*adminv1.RedLetterAllocationResolutionCreateRes, error) {
	selections := make([]taxinvoiceredletter.AllocationSelection, len(req.Selections))
	for index, selection := range req.Selections {
		selections[index] = taxinvoiceredletter.AllocationSelection{
			RefundAllocationNo:     selection.RefundAllocationNo,
			TaxInvoiceAllocationNo: selection.TaxInvoiceAllocationNo,
			GrossAmountMinor:       selection.GrossAmountMinor,
		}
	}
	item, err := c.taxInvoiceRedLetters.ResolveAllocation(ctx, taxinvoiceredletter.ResolveInput{
		TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey, Selections: selections,
	})
	if err != nil {
		return nil, err
	}
	return &adminv1.RedLetterAllocationResolutionCreateRes{Item: mapAdminRedLetter(item)}, nil
}

func (c *ControllerV1) RedLetterManualCompletionCreate(
	ctx context.Context,
	req *adminv1.RedLetterManualCompletionCreateReq,
) (*adminv1.RedLetterManualCompletionCreateRes, error) {
	item, err := c.taxInvoiceRedLetters.CompleteManual(ctx, taxinvoiceredletter.ManualCompletionInput{
		TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey, EvidenceReference: req.EvidenceReference,
		EvidenceSHA256: req.EvidenceSHA256, UploadedObjectRef: req.UploadedObjectRef,
		MediaType: req.MediaType, ByteSize: req.ByteSize,
		OccurredAt: time.UnixMilli(req.OccurredAt).UTC(),
	})
	if err != nil {
		return nil, err
	}
	return &adminv1.RedLetterManualCompletionCreateRes{Item: mapAdminRedLetter(item)}, nil
}

func mapAdminRedLetter(item *taxinvoiceredletter.Task) *adminv1.RedLetterTaskItem {
	if item == nil {
		return nil
	}
	result := &adminv1.RedLetterTaskItem{
		TaskNo: item.TaskNo, RefundOperationNo: item.RefundOperationNo,
		TaxInvoiceNo: item.TaxInvoiceNo, CustomerNo: item.CustomerNo,
		CurrencyCode: item.CurrencyCode, StatusCode: item.StatusCode,
		ManualReasonCode: item.ManualReasonCode, AllocationResolved: item.AllocationResolved,
		CustomerConfirmationRequired: item.CustomerConfirmationRequired,
		GrossAmountMinor:             item.GrossAmountMinor, NetAmountMinor: item.NetAmountMinor,
		TaxAmountMinor: item.TaxAmountMinor, Version: item.Version,
		CreatedAt: item.CreatedAt.UnixMilli(), CandidateCount: item.CandidateCount,
		AllocationCount:      item.AllocationCount,
		CompletionEvidenceNo: item.CompletionEvidenceNo,
	}
	if item.CompletedAt != nil {
		result.CompletedAt = item.CompletedAt.UnixMilli()
	}
	return result
}
