// This file implements bounded customer-owned persistence and append-only revocation history.

package sshkey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/audit"
)

const (
	createAuditScope = "customer.ssh_public_key.create"
	revokeAuditScope = "customer.ssh_public_key.revoke"
)

type keyRow struct {
	KeyNo, Name, AlgorithmCode, PublicKey, FingerprintSHA256 string
	RequestFingerprint, IdempotencyKeyDigest                 string
	Version                                                  int64
	CreatedAt                                                time.Time
	RevokedAt                                                *time.Time
}

func (s *serviceImpl) List(ctx context.Context) ([]PublicKey, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	var rows []keyRow
	if err = s.database.GetScan(ctx, &rows, `SELECT key_no,name,algorithm_code,fingerprint_sha256,version,created_at,revoked_at
FROM plugin_idc_core_customer_ssh_public_key
WHERE tenant_id=0 AND customer_no=? ORDER BY id DESC LIMIT 100`, customerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	items := make([]PublicKey, 0, len(rows))
	for _, row := range rows {
		items = append(items, project(row))
	}
	return items, nil
}

func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*PublicKey, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	name, idempotencyKey := strings.TrimSpace(input.Name), strings.TrimSpace(input.IdempotencyKey)
	parsed, err := parsePublicKey(input.AuthorizedKey)
	if err != nil {
		return nil, err
	}
	if len(name) < 1 || len(name) > 80 || strings.ContainsAny(name, "\r\n") || len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	requestFingerprint := digestHex("ssh-key-create-v1\x00" + name + "\x00" + parsed.canonical)
	idempotencyDigest := digestHex("ssh-key-create-idempotency-v1\x00" + customerNo + "\x00" + idempotencyKey)
	keyNo, err := newKeyNo()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var result *PublicKey
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		inserted, insertErr := tx.Exec(`INSERT INTO plugin_idc_core_customer_ssh_public_key
(tenant_id,key_no,customer_no,name,algorithm_code,public_key,fingerprint_sha256,request_fingerprint,idempotency_key_digest,version)
VALUES (0,?,?,?,?,?,?,?,?,1) ON CONFLICT DO NOTHING`,
			keyNo, customerNo, name, parsed.algorithm, parsed.canonical, parsed.fingerprint, requestFingerprint, idempotencyDigest)
		if insertErr != nil {
			if isUniqueViolation(insertErr) {
				return bizerr.NewCode(CodeDuplicate)
			}
			return insertErr
		}
		affected, affectedErr := inserted.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		var row *keyRow
		if affected == 1 {
			row, insertErr = loadKey(tx, customerNo, keyNo, false)
		} else {
			row, insertErr = loadKeyByIdempotency(tx, customerNo, idempotencyDigest)
			if insertErr == nil && row == nil {
				return bizerr.NewCode(CodeDuplicate)
			}
		}
		if insertErr != nil || row == nil {
			return gerror.Wrap(insertErr, "load SSH public key create result")
		}
		if row.RequestFingerprint != requestFingerprint {
			return bizerr.NewCode(CodeIdempotencyConflict)
		}
		item := project(*row)
		result = &item
		if affected == 0 {
			return nil
		}
		_, insertErr = s.audit.AppendTransition(txCtx, audit.TransitionInput{Fields: audit.Fields{
			IdempotencyScope: createAuditScope, IdempotencyKey: idempotencyKey,
			OperationCode: createAuditScope, TargetType: "customer_ssh_public_key", TargetNo: item.KeyNo,
			CustomerNo: customerNo, Result: audit.ResultSucceeded, Risk: audit.RiskLow,
		}, BeforeSnapshot: map[string]any{}, AfterSnapshot: safeSnapshot(item)})
		return insertErr
	})
	return mapResult(result, err)
}

func (s *serviceImpl) Revoke(ctx context.Context, input RevokeInput) (*PublicKey, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	keyNo, idempotencyKey := strings.TrimSpace(input.KeyNo), strings.TrimSpace(input.IdempotencyKey)
	if !validKeyNo(keyNo) || input.ExpectedVersion < 1 || len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *PublicKey
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		before, loadErr := loadKey(tx, customerNo, keyNo, true)
		if loadErr != nil {
			return bizerr.WrapCode(loadErr, CodeNotAccessible)
		}
		if before == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if before.RevokedAt != nil {
			if before.Version == input.ExpectedVersion+1 {
				item := project(*before)
				result = &item
				return nil
			}
			return bizerr.NewCode(CodeVersionConflict)
		}
		if before.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeVersionConflict)
		}
		updated, updateErr := tx.Exec(`UPDATE plugin_idc_core_customer_ssh_public_key
SET revoked_at=CURRENT_TIMESTAMP,version=version+1
WHERE tenant_id=0 AND customer_no=? AND key_no=? AND revoked_at IS NULL AND version=?`,
			customerNo, keyNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, affectedErr := updated.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.WrapCode(affectedErr, CodeVersionConflict)
		}
		after, loadErr := loadKey(tx, customerNo, keyNo, false)
		if loadErr != nil || after == nil {
			return gerror.Wrap(loadErr, "load revoked SSH public key")
		}
		item := project(*after)
		result = &item
		_, updateErr = s.audit.AppendTransition(txCtx, audit.TransitionInput{Fields: audit.Fields{
			IdempotencyScope: revokeAuditScope, IdempotencyKey: idempotencyKey,
			OperationCode: revokeAuditScope, TargetType: "customer_ssh_public_key", TargetNo: item.KeyNo,
			CustomerNo: customerNo, Result: audit.ResultSucceeded, Risk: audit.RiskMedium,
		}, BeforeSnapshot: safeSnapshot(project(*before)), AfterSnapshot: safeSnapshot(item)})
		return updateErr
	})
	return mapResult(result, err)
}

func (s *serviceImpl) ConsumeActive(ctx context.Context, keyNo string, consume InjectionConsumer) error {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil || !validKeyNo(strings.TrimSpace(keyNo)) || consume == nil {
		return bizerr.NewCode(CodeNotAccessible)
	}
	var row *keyRow
	if err = s.database.GetScan(ctx, &row, `SELECT key_no,public_key,fingerprint_sha256,revoked_at
FROM plugin_idc_core_customer_ssh_public_key
WHERE tenant_id=0 AND customer_no=? AND key_no=? AND revoked_at IS NULL LIMIT 1`, customerNo, keyNo); err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil || row.RevokedAt != nil {
		return bizerr.NewCode(CodeNotAccessible)
	}
	material := []byte(row.PublicKey)
	defer clear(material)
	return consume(InjectionKey{KeyNo: row.KeyNo, PublicKey: material, Fingerprint: row.FingerprintSHA256})
}

func (s *serviceImpl) currentCustomer(ctx context.Context) (string, error) {
	if s == nil || s.database == nil || s.customers == nil || s.audit == nil || ctx == nil || ctx.Err() != nil {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || len(subject.CustomerNo) < 1 || len(subject.CustomerNo) > 40 {
		return "", bizerr.WrapCode(err, CodeNotAccessible)
	}
	return subject.CustomerNo, nil
}

func loadKey(tx gdb.TX, customerNo, keyNo string, lock bool) (*keyRow, error) {
	query := `SELECT key_no,name,algorithm_code,public_key,fingerprint_sha256,request_fingerprint,idempotency_key_digest,version,created_at,revoked_at
FROM plugin_idc_core_customer_ssh_public_key WHERE tenant_id=0 AND customer_no=? AND key_no=?`
	if lock {
		query += ` FOR UPDATE`
	}
	var row *keyRow
	err := tx.GetScan(&row, query, customerNo, keyNo)
	return row, err
}

func loadKeyByIdempotency(tx gdb.TX, customerNo, digest string) (*keyRow, error) {
	var row *keyRow
	err := tx.GetScan(&row, `SELECT key_no,name,algorithm_code,public_key,fingerprint_sha256,request_fingerprint,idempotency_key_digest,version,created_at,revoked_at
FROM plugin_idc_core_customer_ssh_public_key WHERE tenant_id=0 AND customer_no=? AND idempotency_key_digest=? FOR UPDATE`, customerNo, digest)
	return row, err
}

func project(row keyRow) PublicKey {
	status := StatusActive
	if row.RevokedAt != nil {
		status = StatusRevoked
	}
	return PublicKey{KeyNo: row.KeyNo, Name: row.Name, Algorithm: row.AlgorithmCode,
		Fingerprint: row.FingerprintSHA256, Status: status, Version: row.Version,
		CreatedAt: row.CreatedAt.UTC(), RevokedAt: row.RevokedAt}
}

func safeSnapshot(item PublicKey) map[string]any {
	return map[string]any{"key_no": item.KeyNo, "name": item.Name, "algorithm": item.Algorithm,
		"fingerprint": item.Fingerprint, "status": item.Status, "version": item.Version}
}

func newKeyNo() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", gerror.Wrap(err, "generate SSH public key number")
	}
	return "SSH_" + hex.EncodeToString(raw[:]), nil
}

func digestHex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func validKeyNo(value string) bool { return len(value) == 36 && strings.HasPrefix(value, "SSH_") }

func isUniqueViolation(err error) bool {
	text := strings.ToLower(gerror.Current(err).Error())
	return strings.Contains(text, "duplicate key") || strings.Contains(text, "unique constraint")
}

func mapResult(result *PublicKey, err error) (*PublicKey, error) {
	if err == nil {
		return result, nil
	}
	for _, code := range []*bizerr.Code{CodeInvalid, CodePrivateKeyRejected, CodeNotAccessible, CodeDuplicate,
		CodeVersionConflict, CodeIdempotencyConflict, CodeUnavailable} {
		if bizerr.Is(err, code) {
			return nil, err
		}
	}
	return nil, bizerr.WrapCode(err, CodeUnavailable)
}
