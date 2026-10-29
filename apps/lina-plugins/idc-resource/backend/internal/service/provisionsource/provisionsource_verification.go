// This file preserves independent staff non-acceptance facts for unknown tasks.
// It requires the shared caller transaction and performs no upstream operation.

package provisionsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// VerifyNonAcceptance fences an authorized manual fact against known acceptance.
// The ambient transaction makes owner evidence atomic with core business audit.
func (s *Service) VerifyNonAcceptance(ctx context.Context, input provisionresourcecap.NonAcceptanceVerification) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil ||
		provisionresourcecap.ValidateTaskReadRequest(input.Task) != nil || input.ActorUserID <= 0 ||
		utf8.RuneCountInString(strings.TrimSpace(input.Note)) < 10 || utf8.RuneCountInString(input.Note) > 512 ||
		len(input.Digest) != 64 {
		return provisionresourcecap.ErrTaskConflict
	}
	if _, err := hex.DecodeString(input.Digest); err != nil {
		return provisionresourcecap.ErrTaskConflict
	}
	if _, err := s.database.GetValue(ctx, `SELECT task_no FROM plugin_idc_resource_task WHERE tenant_id=0 AND task_no=? FOR UPDATE`, input.Task.ProviderTaskNo); err != nil {
		return err
	}
	row, err := s.loadTaskByNumber(ctx, input.Task.ProviderTaskNo)
	if err != nil {
		return err
	}
	if row == nil || !sameTaskRequest(row, input.Task.Request) || resourcetask.Status(row.StatusCode) != resourcetask.StatusResultUnknown || !row.ResultUnknown {
		return provisionresourcecap.ErrTaskConflict
	}
	unaccepted, err := s.database.GetValue(ctx, `SELECT NOT EXISTS(SELECT 1 FROM plugin_idc_resource_upstream_task_binding WHERE task_no=?)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_mapping WHERE tenant_id=0 AND local_resource_no=? AND status_code='active')
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_provision_outcome_receipt WHERE tenant_id=0 AND task_no=?)`, row.TaskNo, row.LocalResourceNo, row.TaskNo)
	if err != nil {
		return err
	}
	if !unaccepted.Bool() {
		return provisionresourcecap.ErrTaskConflict
	}
	_, err = s.database.Model(do.ProvisionNonAcceptanceVerification{}).Ctx(ctx).Data(do.ProvisionNonAcceptanceVerification{TenantId: 0, TaskNo: row.TaskNo, RequestDigest: input.Task.Request.RequestDigest, VerificationDigest: input.Digest, ActorUserId: input.ActorUserID, SafeNote: input.Note}).OnConflict("task_no").InsertIgnore()
	if err != nil {
		return err
	}
	digest, err := s.database.GetValue(ctx, `SELECT verification_digest FROM plugin_idc_resource_provision_non_acceptance_verification WHERE tenant_id=0 AND task_no=? AND request_digest=?`, row.TaskNo, input.Task.Request.RequestDigest)
	if err != nil {
		return err
	}
	if digest.String() != input.Digest {
		return provisionresourcecap.ErrTaskConflict
	}
	return nil
}

// applyNonAcceptanceVerification changes only the owner projection; the original
// resource task and all original attempts remain unknown and immutable.
func (s *Service) applyNonAcceptanceVerification(ctx context.Context, row *taskRow, task *provisionresourcecap.Task) error {
	if resourcetask.Status(row.StatusCode) != resourcetask.StatusResultUnknown {
		return nil
	}
	facts, err := decodeTaskOwnerFacts(row.OwnerMetadata)
	if err != nil {
		return err
	}
	var fact *struct {
		VerificationDigest string
		VerifiedAt         time.Time
	}
	if err := s.database.GetScan(ctx, &fact, `SELECT verification_digest,verified_at FROM plugin_idc_resource_provision_non_acceptance_verification WHERE tenant_id=0 AND task_no=? AND request_digest=?`, row.TaskNo, hex.EncodeToString(facts.RequestDigest[:])); err != nil {
		return err
	}
	if fact == nil {
		return nil
	}
	unaccepted, err := s.database.GetValue(ctx, `SELECT NOT EXISTS(SELECT 1 FROM plugin_idc_resource_upstream_task_binding WHERE task_no=?)`, row.TaskNo)
	if err != nil {
		return err
	}
	if !unaccepted.Bool() {
		return provisionresourcecap.ErrTaskConflict
	}
	digest := sha256.Sum256([]byte(task.EvidenceDigest + ":" + fact.VerificationDigest))
	task.Status = provisionresourcecap.TaskStatusFailed
	task.FailureCode = provisionresourcecap.FailureNonAcceptanceVerified
	task.EvidenceDigest = hex.EncodeToString(digest[:])
	task.ObservedAt = fact.VerifiedAt.UTC()
	task.CompletedAt = task.ObservedAt
	return nil
}
