package productautomation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	secretOperationPrepared            = "prepared"
	secretOperationReady               = "secrets_ready"
	secretOperationCompensationPending = "compensation_pending"
	secretOperationCommitted           = "commit_succeeded"
	secretOperationCompensated         = "compensated"
	secretOperationManualRequired      = "manual_required"
	secretOperationMaximumAttempts     = 5
	secretOperationRecoveryDelay       = 5 * time.Minute
	secretOperationRetryDelay          = time.Minute
	secretOperationLeaseDuration       = time.Minute
)

type interfaceSecretOperation struct {
	ID                  int64  `orm:"id"`
	OperationNo         string `orm:"operation_no"`
	InterfaceInstanceNo string `orm:"interface_instance_no"`
	ExpectedRevision    int64  `orm:"expected_revision"`
	TargetRevision      int64  `orm:"target_revision"`
	StatusCode          string `orm:"status_code"`
	AttemptCount        int    `orm:"attempt_count"`
}

type interfaceSecretOperationField struct {
	FieldKey        string     `orm:"field_key"`
	SecretRef       string     `orm:"secret_ref"`
	SecretUpdatedAt *time.Time `orm:"secret_updated_at"`
	StatusCode      string     `orm:"status_code"`
}

// SecretOperationRecoveryWorker converges only owner-private, non-committed
// secret operations. It is deliberately narrower than the management Service.
type SecretOperationRecoveryWorker interface {
	Run(ctx context.Context, limit int) (int, error)
}

type secretOperationRecoveryWorker struct {
	service    *serviceImpl
	leaseOwner string
}

// NewSecretOperationRecoveryWorker constructs the restart-safe compensation
// worker without creating any HTTP or management surface.
func NewSecretOperationRecoveryWorker(database gdb.DB, secrets secretcap.Manager, random io.Reader) (SecretOperationRecoveryWorker, error) {
	if nilDependency(database) || nilDependency(secrets) || nilDependency(random) {
		return nil, gerror.New("interface secret operation recovery dependencies are unavailable")
	}
	leaseOwner, err := interfaceSecretOperationLeaseOwner(random)
	if err != nil {
		return nil, gerror.Wrap(err, "create interface secret operation recovery identity")
	}
	return &secretOperationRecoveryWorker{service: &serviceImpl{database: database, secrets: secrets}, leaseOwner: leaseOwner}, nil
}

func (w *secretOperationRecoveryWorker) Run(ctx context.Context, limit int) (int, error) {
	if w == nil || w.service == nil {
		return 0, gerror.New("interface secret operation recovery worker is unavailable")
	}
	return w.service.RecoverInterfaceSecretOperations(ctx, limit, w.leaseOwner)
}

func newSecretOperationNo(random io.Reader, interfaceNo string, targetRevision int64) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", gerror.Wrap(err, "generate interface secret operation number")
	}
	payload := append(value[:], []byte(interfaceNo+":"+strconv.FormatInt(targetRevision, 10)+":"+strconv.FormatInt(time.Now().UnixNano(), 10))...)
	digest := sha256.Sum256(payload)
	clear(payload)
	return "pso_" + hex.EncodeToString(digest[:16]), nil
}

func (s *serviceImpl) prepareInterfaceSecretOperation(ctx context.Context, interfaceNo string, expectedRevision int64, fieldKeys []string) (*interfaceSecretOperation, error) {
	if len(fieldKeys) == 0 {
		return nil, nil
	}
	operationNo, err := newSecretOperationNo(s.random, interfaceNo, expectedRevision+1)
	if err != nil {
		return nil, err
	}
	keys := append([]string(nil), fieldKeys...)
	sort.Strings(keys)
	operation := &interfaceSecretOperation{OperationNo: operationNo, InterfaceInstanceNo: interfaceNo, ExpectedRevision: expectedRevision, TargetRevision: expectedRevision + 1, StatusCode: secretOperationPrepared}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		id, insertErr := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation").Ctx(txCtx).Data(gdb.Map{
			"tenant_id": idccap.DefaultTenantID, "operation_no": operationNo, "interface_instance_no": interfaceNo,
			"expected_revision": expectedRevision, "target_revision": expectedRevision + 1, "status_code": secretOperationPrepared,
			"next_attempt_at": time.Now().Add(secretOperationRecoveryDelay),
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "prepare interface secret operation")
		}
		operation.ID = id
		for _, key := range keys {
			if _, insertErr = s.database.Model("plugin_idc_core_product_automation_interface_secret_operation_field").Ctx(txCtx).Data(gdb.Map{
				"tenant_id": idccap.DefaultTenantID, "operation_id": id, "field_key": key, "status_code": secretOperationPrepared,
			}).Insert(); insertErr != nil {
				return gerror.Wrap(insertErr, "prepare interface secret operation field")
			}
		}
		return nil
	})
	return operation, err
}

func secretOperationIdempotencyKey(operationNo, fieldKey string) string {
	return operationNo + ":" + fieldKey
}

func (s *serviceImpl) recordInterfaceSecretOperationField(ctx context.Context, operationID int64, fieldKey string, result secretcap.CreateResult) error {
	updatedAt := result.Metadata.UpdatedAt
	resultSet, err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation_field").Ctx(ctx).
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "operation_id": operationID, "field_key": fieldKey, "status_code": secretOperationPrepared}).
		Data(gdb.Map{"secret_ref": result.Handle.OpaqueRef(), "secret_updated_at": updatedAt, "status_code": "created", "changed_at": time.Now()}).Update()
	if err != nil {
		return gerror.Wrap(err, "record interface secret operation field")
	}
	affected, err := resultSet.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.New("interface secret operation field was not recorded")
	}
	return nil
}

func (s *serviceImpl) markInterfaceSecretOperationReady(ctx context.Context, operationID int64) error {
	result, err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation").Ctx(ctx).
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "id": operationID, "status_code": secretOperationPrepared}).
		Data(gdb.Map{"status_code": secretOperationReady, "changed_at": time.Now()}).Update()
	if err != nil {
		return gerror.Wrap(err, "mark interface secret operation ready")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.New("interface secret operation did not become ready")
	}
	return nil
}

func (s *serviceImpl) commitInterfaceSecretOperation(ctx context.Context, operation *interfaceSecretOperation) error {
	if operation == nil {
		return nil
	}
	result, err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation").Ctx(ctx).
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "id": operation.ID, "operation_no": operation.OperationNo, "status_code": secretOperationReady}).
		Data(gdb.Map{"status_code": secretOperationCommitted, "lease_owner": "", "lease_expires_at": nil, "changed_at": time.Now()}).Update()
	if err != nil {
		return gerror.Wrap(err, "commit interface secret operation")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.New("interface secret operation commit fence failed")
	}
	return nil
}

func (s *serviceImpl) markInterfaceSecretOperationPending(ctx context.Context, operation *interfaceSecretOperation, cause error) error {
	if operation == nil {
		return nil
	}
	digest := sha256.Sum256([]byte(cause.Error()))
	_, err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation").Ctx(ctx).
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "id": operation.ID}).
		WhereIn("status_code", []string{secretOperationPrepared, secretOperationReady, secretOperationCompensationPending}).
		Data(gdb.Map{"status_code": secretOperationCompensationPending, "next_attempt_at": time.Now().Add(secretOperationRetryDelay), "last_error_digest": hex.EncodeToString(digest[:]), "lease_owner": "", "lease_expires_at": nil, "changed_at": time.Now()}).Update()
	return gerror.Wrap(err, "mark interface secret operation pending")
}

func (s *serviceImpl) compensateInterfaceSecretOperation(ctx context.Context, operation *interfaceSecretOperation) error {
	if operation == nil {
		return nil
	}
	var fields []interfaceSecretOperationField
	if err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation_field").Ctx(ctx).
		Fields("field_key", "secret_ref", "secret_updated_at", "status_code").
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "operation_id": operation.ID}).OrderAsc("field_key").Scan(&fields); err != nil {
		return gerror.Wrap(err, "load interface secret operation fields")
	}
	var failures []error
	for _, field := range fields {
		if field.StatusCode == "revoked" {
			continue
		}
		var result secretcap.CreateResult
		var err error
		if field.SecretRef != "" && field.SecretUpdatedAt != nil {
			result.Handle, err = secretcap.NewHandle(field.SecretRef)
			result.Metadata.UpdatedAt = *field.SecretUpdatedAt
		} else {
			result, err = s.secrets.RecoverCreate(ctx, secretOperationIdempotencyKey(operation.OperationNo, field.FieldKey))
			if err == nil {
				if recordErr := s.recordInterfaceSecretOperationField(ctx, operation.ID, field.FieldKey, result); recordErr != nil {
					failures = append(failures, recordErr)
					continue
				}
			}
		}
		if err != nil {
			failures = append(failures, gerror.Wrap(err, "recover interface secret operation field"))
			continue
		}
		if _, err = s.secrets.Revoke(ctx, result.Handle, result.Metadata.UpdatedAt); err != nil {
			failures = append(failures, gerror.Wrap(err, "revoke interface secret operation field"))
			continue
		}
		if _, err = s.database.Model("plugin_idc_core_product_automation_interface_secret_operation_field").Ctx(ctx).
			Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "operation_id": operation.ID, "field_key": field.FieldKey, "status_code": "created"}).
			Data(gdb.Map{"status_code": "revoked", "changed_at": time.Now()}).Update(); err != nil {
			failures = append(failures, gerror.Wrap(err, "record revoked interface secret operation field"))
		}
	}
	if cleanupErr := errors.Join(failures...); cleanupErr != nil {
		return errors.Join(cleanupErr, s.recordInterfaceSecretOperationFailure(ctx, operation, cleanupErr))
	}
	_, err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation").Ctx(ctx).
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "id": operation.ID}).
		WhereIn("status_code", []string{secretOperationPrepared, secretOperationReady, secretOperationCompensationPending}).
		Data(gdb.Map{"status_code": secretOperationCompensated, "lease_owner": "", "lease_expires_at": nil, "changed_at": time.Now()}).Update()
	return gerror.Wrap(err, "finish interface secret operation compensation")
}

func (s *serviceImpl) recordInterfaceSecretOperationFailure(ctx context.Context, operation *interfaceSecretOperation, cause error) error {
	nextAttempt := operation.AttemptCount + 1
	status := secretOperationCompensationPending
	if nextAttempt >= secretOperationMaximumAttempts {
		status = secretOperationManualRequired
	}
	digest := sha256.Sum256([]byte(cause.Error()))
	_, err := s.database.Model("plugin_idc_core_product_automation_interface_secret_operation").Ctx(ctx).
		Where(gdb.Map{"tenant_id": idccap.DefaultTenantID, "id": operation.ID}).
		Data(gdb.Map{"status_code": status, "attempt_count": nextAttempt, "next_attempt_at": time.Now().Add(secretOperationRetryDelay), "last_error_digest": hex.EncodeToString(digest[:]), "lease_owner": "", "lease_expires_at": nil, "changed_at": time.Now()}).Update()
	operation.AttemptCount = nextAttempt
	return gerror.Wrap(err, "record interface secret operation recovery failure")
}

// RecoverInterfaceSecretOperations leases and converges a bounded number of
// non-committed operations. It never reads configuration plaintext and never
// revokes an operation that committed with its configuration revision.
func (s *serviceImpl) RecoverInterfaceSecretOperations(ctx context.Context, limit int, leaseOwner string) (int, error) {
	if limit <= 0 || limit > 32 || leaseOwner == "" || len(leaseOwner) > 64 {
		return 0, gerror.New("interface secret operation recovery input is invalid")
	}
	processed := 0
	for processed < limit {
		var operation *interfaceSecretOperation
		err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			query := `SELECT id,operation_no,interface_instance_no,expected_revision,target_revision,status_code,attempt_count
				FROM plugin_idc_core_product_automation_interface_secret_operation
				WHERE tenant_id=$1 AND status_code IN ('prepared','secrets_ready','compensation_pending')
				  AND next_attempt_at<=CURRENT_TIMESTAMP AND (lease_expires_at IS NULL OR lease_expires_at<CURRENT_TIMESTAMP)
				ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`
			if scanErr := s.database.GetScan(txCtx, &operation, query, idccap.DefaultTenantID); scanErr != nil || operation == nil {
				return scanErr
			}
			_, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_automation_interface_secret_operation
				SET lease_owner=$1,lease_expires_at=$2,changed_at=CURRENT_TIMESTAMP WHERE id=$3`, leaseOwner, time.Now().Add(secretOperationLeaseDuration), operation.ID)
			return updateErr
		})
		if err != nil {
			return processed, gerror.Wrap(err, "lease interface secret operation")
		}
		if operation == nil {
			return processed, nil
		}
		if err = s.compensateInterfaceSecretOperation(ctx, operation); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func interfaceSecretOperationLeaseOwner(random io.Reader) (string, error) {
	value := make([]byte, 12)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return "secret-recovery-" + strconv.FormatInt(time.Now().Unix(), 36) + "-" + hex.EncodeToString(value), nil
}
