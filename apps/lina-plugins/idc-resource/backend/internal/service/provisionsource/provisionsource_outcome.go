package provisionsource

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/internal/composition"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

type provisionCredentialRow struct {
	CredentialNo string     `orm:"credential_no"`
	ServiceNo    string     `orm:"service_no"`
	TaskNo       string     `orm:"task_no"`
	ChannelNo    string     `orm:"channel_no"`
	SecretRef    string     `orm:"secret_ref"`
	StatusCode   string     `orm:"status_code"`
	RevokedAt    *time.Time `orm:"revoked_at"`
	Version      int64      `orm:"version"`
}

type provisionOutcomeReceiptRow struct {
	ReceiptNo               string `orm:"receipt_no"`
	TaskNo                  string `orm:"task_no"`
	ServiceNo               string `orm:"service_no"`
	OperationNo             string `orm:"operation_no"`
	CustomerNo              string `orm:"customer_no"`
	ConfigurationSnapshotNo string `orm:"configuration_snapshot_no"`
	StatusCode              string `orm:"status_code"`
	RequestDigest           string `orm:"request_digest"`
	TaskEvidenceDigest      string `orm:"task_evidence_digest"`
	CoreEvidenceDigest      string `orm:"core_evidence_digest"`
}

// ReportOutcome applies credential lifecycle only after core proves its own
// terminal state committed. Replays compare the complete immutable receipt.
func (s *Service) ReportOutcome(
	ctx context.Context,
	report provisionresourcecap.OutcomeReport,
) error {
	if s == nil || s.database == nil || s.secrets == nil || ctx == nil || ctx.Err() != nil ||
		provisionresourcecap.ValidateOutcomeReport(report) != nil {
		return provisionresourcecap.ErrSourceUnavailable
	}
	row, err := s.loadTaskByNumber(ctx, report.ProviderTaskNo)
	if err != nil {
		return err
	}
	if row == nil {
		return provisionresourcecap.ErrTaskUnavailable
	}
	if !sameTaskRequest(row, report.Request) || (report.Request.NativePurchase == nil && row.SecretRef == "") {
		return provisionresourcecap.ErrTaskConflict
	}
	projection, err := s.ReadTask(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: row.TaskNo, Request: report.Request})
	if err != nil {
		return err
	}
	if string(projection.Status) != string(report.Status) || subtle.ConstantTimeCompare([]byte(projection.EvidenceDigest), []byte(report.TaskEvidenceDigest)) != 1 {
		return provisionresourcecap.ErrTaskConflict
	}
	if report.Request.NativePurchase != nil {
		return s.persistOutcomeReceipt(ctx, row, report)
	}
	if err = s.healProvisionCredential(ctx, row); err != nil {
		return err
	}
	if report.Status == provisionresourcecap.OutcomeStatusFailed {
		if err = s.revokeFailedProvisionCredential(ctx, row); err != nil {
			return err
		}
	}
	return s.persistOutcomeReceipt(ctx, row, report)
}

func (s *Service) revokeFailedProvisionCredential(ctx context.Context, task *taskRow) error {
	credential, err := loadProvisionCredential(ctx, s.database, task.TaskNo, false)
	if err != nil {
		return err
	}
	if credential == nil || credential.ServiceNo != task.LocalResourceNo ||
		credential.ChannelNo != task.ChannelNo || credential.SecretRef != task.SecretRef {
		return provisionresourcecap.ErrTaskConflict
	}
	if credential.StatusCode == "revoked" {
		return nil
	}
	if credential.StatusCode != "pending" && credential.StatusCode != "revealable" {
		return provisionresourcecap.ErrTaskConflict
	}
	handle, err := secretcap.NewHandle(credential.SecretRef)
	if err != nil {
		return provisionresourcecap.ErrTaskConflict
	}
	manager, err := composition.BindProviderSecretManager(s.secrets, credential.ChannelNo)
	if err != nil {
		return errors.Join(provisionresourcecap.ErrSourceUnavailable, err)
	}
	metadata, err := manager.Metadata(ctx, handle)
	if err != nil {
		return errors.Join(provisionresourcecap.ErrSourceUnavailable, err)
	}
	if metadata.Configured {
		if metadata.UpdatedAt.IsZero() {
			return provisionresourcecap.ErrSourceUnavailable
		}
		if _, err = manager.Revoke(ctx, handle, metadata.UpdatedAt); err != nil {
			return errors.Join(provisionresourcecap.ErrSourceUnavailable, err)
		}
	}
	return nil
}

func (s *Service) persistOutcomeReceipt(
	ctx context.Context,
	task *taskRow,
	report provisionresourcecap.OutcomeReport,
) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var credential *provisionCredentialRow
		var err error
		if report.Request.NativePurchase == nil {
			credential, err = loadProvisionCredential(ctx, s.database, task.TaskNo, true)
			if err != nil {
				return err
			}
			if credential == nil || credential.ServiceNo != task.LocalResourceNo || credential.ChannelNo != task.ChannelNo || credential.SecretRef != task.SecretRef {
				return provisionresourcecap.ErrTaskConflict
			}
		} else {
			if _, err = s.database.Model(do.Task{}).Ctx(ctx).Where(do.Task{TenantId: 0, TaskNo: task.TaskNo}).Fields("task_no").LockUpdate().One(); err != nil {
				return err
			}
		}
		var existing *provisionOutcomeReceiptRow
		if err = s.database.GetScan(ctx, &existing, `
SELECT receipt_no, task_no, service_no, operation_no, customer_no,
       configuration_snapshot_no, status_code, request_digest,
       task_evidence_digest, core_evidence_digest
FROM plugin_idc_resource_provision_outcome_receipt
WHERE tenant_id = ? AND task_no = ?`, idccap.DefaultTenantID, task.TaskNo); err != nil {
			return gerror.Wrap(err, "load resource provisioning outcome receipt")
		}
		if existing != nil {
			if sameOutcomeReceipt(existing, task, report) {
				return nil
			}
			return provisionresourcecap.ErrTaskConflict
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		if credential != nil {
			switch report.Status {
			case provisionresourcecap.OutcomeStatusSucceeded:
				if credential.StatusCode == "pending" {
					if err = requireOutcomeUpdate(ctx, s.database, `
UPDATE plugin_idc_resource_provision_credential
SET status_code = 'revealable', version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND status_code = 'pending' AND version = ?`,
						now, idccap.DefaultTenantID, task.TaskNo, credential.Version); err != nil {
						return err
					}
				} else if credential.StatusCode != "revealable" && credential.StatusCode != "revealed" {
					return provisionresourcecap.ErrTaskConflict
				}
			case provisionresourcecap.OutcomeStatusFailed:
				if credential.StatusCode != "revoked" {
					if credential.StatusCode != "pending" && credential.StatusCode != "revealable" {
						return provisionresourcecap.ErrTaskConflict
					}
					if err = requireOutcomeUpdate(ctx, s.database, `
UPDATE plugin_idc_resource_provision_credential
SET status_code = 'revoked', revoked_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND status_code IN ('pending', 'revealable')
  AND version = ?`, now, now, idccap.DefaultTenantID, task.TaskNo, credential.Version); err != nil {
						return err
					}
				}
			case provisionresourcecap.OutcomeStatusManualReview:
				if credential.StatusCode != "pending" {
					return provisionresourcecap.ErrTaskConflict
				}
			default:
				return provisionresourcecap.ErrTaskConflict
			}
		}
		digest := sha256.Sum256([]byte("provision-outcome/v1\x00" + task.TaskNo))
		_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provision_outcome_receipt (
    tenant_id, receipt_no, task_no, service_no, operation_no, customer_no,
    configuration_snapshot_no, status_code, request_digest,
    task_evidence_digest, core_evidence_digest, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID,
			"poc_"+hex.EncodeToString(digest[:16]), task.TaskNo, task.LocalResourceNo,
			report.Request.OperationNo, report.Request.CustomerNo,
			report.Request.ConfigurationSnapshotNo, string(report.Status),
			report.Request.RequestDigest, report.TaskEvidenceDigest,
			report.CoreEvidenceDigest, now)
		return gerror.Wrap(err, "insert resource provisioning outcome receipt")
	})
}

func loadProvisionCredential(
	ctx context.Context,
	database gdb.DB,
	taskNo string,
	lock bool,
) (*provisionCredentialRow, error) {
	query := `
SELECT credential_no, service_no, task_no, channel_no, secret_ref,
       status_code, revoked_at, version
FROM plugin_idc_resource_provision_credential
WHERE tenant_id = ? AND task_no = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var row *provisionCredentialRow
	if err := database.GetScan(ctx, &row, query, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, gerror.Wrap(err, "load resource provisioning credential")
	}
	return row, nil
}

func sameOutcomeReceipt(
	receipt *provisionOutcomeReceiptRow,
	task *taskRow,
	report provisionresourcecap.OutcomeReport,
) bool {
	return receipt != nil && receipt.TaskNo == task.TaskNo &&
		receipt.ServiceNo == task.LocalResourceNo &&
		receipt.OperationNo == report.Request.OperationNo &&
		receipt.CustomerNo == report.Request.CustomerNo &&
		receipt.ConfigurationSnapshotNo == report.Request.ConfigurationSnapshotNo &&
		receipt.StatusCode == string(report.Status) &&
		subtle.ConstantTimeCompare([]byte(receipt.RequestDigest), []byte(report.Request.RequestDigest)) == 1 &&
		subtle.ConstantTimeCompare([]byte(receipt.TaskEvidenceDigest), []byte(report.TaskEvidenceDigest)) == 1 &&
		subtle.ConstantTimeCompare([]byte(receipt.CoreEvidenceDigest), []byte(report.CoreEvidenceDigest)) == 1
}

func requireOutcomeUpdate(
	ctx context.Context,
	database gdb.DB,
	query string,
	arguments ...any,
) error {
	result, err := database.Exec(ctx, query, arguments...)
	if err != nil {
		return gerror.Wrap(err, "persist resource provisioning outcome")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return provisionresourcecap.ErrTaskConflict
	}
	return nil
}
