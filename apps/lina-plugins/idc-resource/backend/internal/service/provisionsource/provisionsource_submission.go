// This file validates durable non-submission proof before projecting a local
// preparation failure as recoverable. Missing proof or any binding fails closed.
package provisionsource

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func (s *Service) validateNonSubmissionProof(ctx context.Context, row *taskRow) error {
	var proof *struct {
		Reference  string
		Summary    string
		ObservedAt time.Time
	}
	err := s.database.GetScan(ctx, &proof, `SELECT reference,summary,observed_at FROM plugin_idc_resource_task_evidence evidence
 WHERE tenant_id=0 AND task_no=? AND evidence_kind=?
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_upstream_task_binding WHERE task_no=evidence.task_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_mapping WHERE tenant_id=0 AND local_resource_no=? AND status_code='active')
 ORDER BY observed_at DESC,id DESC LIMIT 1`, row.TaskNo, resourcecap.EvidenceKindRequestNotSent, row.LocalResourceNo)
	if err != nil {
		return provisionresourcecap.ErrSourceUnavailable
	}
	if proof == nil || row.ResultUnknown || resourcecap.ValidateProvisionNonSubmissionEvidence(resourcecap.Evidence{Kind: resourcecap.EvidenceKindRequestNotSent, Reference: proof.Reference, Summary: proof.Summary, ObservedAt: proof.ObservedAt}, row.TaskNo) != nil {
		return provisionresourcecap.ErrTaskConflict
	}
	return nil
}
