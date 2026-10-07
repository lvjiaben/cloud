// This file produces bounded encrypted local evidence archives and immutable manifests.

package datalifecycle

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

const maximumExportTTL = 7 * 24 * time.Hour

// EvidenceExportInput scopes one merchant-authorized local evidence export.
type EvidenceExportInput struct {
	Provider            string
	RouteNo             string
	EvidenceNos         []string
	EmployeePrincipalNo string
	ArchiveKey          []byte
	ExpiresAt           time.Time
}

// EvidenceExportResult returns the encrypted archive and safe manifest metadata.
type EvidenceExportResult struct {
	ExportNo       string
	Archive        []byte
	ManifestSHA256 [sha256.Size]byte
	EvidenceCount  int
	ExpiresAt      time.Time
}

type exportedEvidence struct {
	EvidenceNo       string    `json:"evidenceNo"`
	RouteNo          string    `json:"routeNo"`
	ProviderCode     string    `json:"providerCode"`
	OperationScope   string    `json:"operationScope"`
	SourceKind       string    `json:"sourceKind"`
	NormalizedStatus string    `json:"normalizedStatus"`
	DecisionCode     string    `json:"decisionCode"`
	ObservedAt       time.Time `json:"observedAt"`
	PayloadSHA256    string    `json:"payloadSha256"`
	Payload          []byte    `json:"payload"`
}

type evidenceArchive struct {
	SchemaVersion string             `json:"schemaVersion"`
	ExportNo      string             `json:"exportNo"`
	CreatedAt     time.Time          `json:"createdAt"`
	ScopeSHA256   string             `json:"scopeSha256"`
	Evidence      []exportedEvidence `json:"evidence"`
}

type encryptedEnvelope struct {
	SchemaVersion string `json:"schemaVersion"`
	Algorithm     string `json:"algorithm"`
	ExportNo      string `json:"exportNo"`
	ScopeSHA256   string `json:"scopeSha256"`
	Nonce         string `json:"nonce"`
	Ciphertext    string `json:"ciphertext"`
}

// ExportEvidence returns a bounded AES-256-GCM archive. ArchiveKey is cleared
// before return and is never stored in PostgreSQL or the archive metadata.
func (s *Service) ExportEvidence(
	ctx context.Context,
	input EvidenceExportInput,
) (exportResult EvidenceExportResult, returnError error) {
	ownedKey := append([]byte(nil), input.ArchiveKey...)
	clear(input.ArchiveKey)
	defer clear(ownedKey)
	input.Provider = strings.TrimSpace(input.Provider)
	input.RouteNo = strings.TrimSpace(input.RouteNo)
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	input.EvidenceNos = canonicalEvidenceNos(input.EvidenceNos)
	if !s.validContext(ctx) || !validProvider(input.Provider) ||
		(input.RouteNo != "" && !routePattern.MatchString(input.RouteNo)) ||
		(input.RouteNo == "" && len(input.EvidenceNos) == 0) ||
		len(input.EvidenceNos) > MaximumExportEvidence || !validEmployee(input.EmployeePrincipalNo) ||
		len(ownedKey) != 32 || input.ExpiresAt.IsZero() {
		return EvidenceExportResult{}, ErrInvalid
	}
	for _, evidenceNo := range input.EvidenceNos {
		if !evidencePattern.MatchString(evidenceNo) {
			return EvidenceExportResult{}, ErrInvalid
		}
	}
	exportNo, err := randomIdentifier(s.random, "lex_")
	if err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	scopeDigest := digestFrame(
		"idc-evidence-export-scope-v1", input.Provider, input.RouteNo,
		strings.Join(input.EvidenceNos, ","),
	)
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead, ReadOnly: false,
	})
	if err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	var now time.Time
	if err = transaction.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&now); err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	now = now.UTC()
	defer func() {
		finishTransaction(transaction, &returnError)
		if returnError != nil {
			if recordError := s.recordFailedExport(
				ctx, exportNo, input.EmployeePrincipalNo, scopeDigest, now,
			); recordError != nil {
				returnError = classifyDatabaseError(recordError)
			}
		}
	}()
	if !input.ExpiresAt.After(now) || input.ExpiresAt.After(now.Add(maximumExportTTL)) {
		return EvidenceExportResult{}, ErrInvalid
	}
	evidence, err := s.readEvidence(ctx, transaction, input)
	if err != nil {
		return EvidenceExportResult{}, err
	}
	defer clearExportEvidence(evidence)
	archive := evidenceArchive{
		SchemaVersion: "v1", ExportNo: exportNo, CreatedAt: now,
		ScopeSHA256: hex.EncodeToString(scopeDigest[:]), Evidence: evidence,
	}
	plaintext, err := json.Marshal(archive)
	if err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	defer clear(plaintext)
	block, err := aes.NewCipher(ownedKey)
	if err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(s.random, nonce); err != nil {
		clear(nonce)
		return EvidenceExportResult{}, ErrUnavailable
	}
	defer clear(nonce)
	aad := []byte(exportNo + "\x00" + hex.EncodeToString(scopeDigest[:]))
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	clear(aad)
	defer clear(ciphertext)
	envelopeBytes, err := json.Marshal(encryptedEnvelope{
		SchemaVersion: "v1", Algorithm: "AES-256-GCM", ExportNo: exportNo,
		ScopeSHA256: hex.EncodeToString(scopeDigest[:]),
		Nonce:       base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext:  base64.RawURLEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return EvidenceExportResult{}, ErrUnavailable
	}
	manifestDigest := sha256.Sum256(envelopeBytes)
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_evidence_export
    (export_no, requested_by_employee_no, scope_sha256, manifest_sha256,
     status_code, requested_at, completed_at, expires_at)
VALUES ($1, $2, $3, $4, 'completed', $5, CURRENT_TIMESTAMP, $6)`,
		exportNo, input.EmployeePrincipalNo, scopeDigest[:], manifestDigest[:], now, input.ExpiresAt.UTC())
	if err != nil || rowsAffected(result) != 1 {
		clear(envelopeBytes)
		return EvidenceExportResult{}, classifyDatabaseError(err)
	}
	if err = transaction.Commit(); err != nil {
		clear(envelopeBytes)
		return EvidenceExportResult{}, classifyDatabaseError(err)
	}
	return EvidenceExportResult{
		ExportNo: exportNo, Archive: envelopeBytes, ManifestSHA256: manifestDigest,
		EvidenceCount: len(evidence), ExpiresAt: input.ExpiresAt.UTC(),
	}, nil
}

func (s *Service) readEvidence(
	ctx context.Context,
	transaction *sql.Tx,
	input EvidenceExportInput,
) (result []exportedEvidence, returnError error) {
	arguments := []any{input.Provider}
	where := "provider_code = $1"
	if input.RouteNo != "" {
		arguments = append(arguments, input.RouteNo)
		where += " AND route_no = $2"
	}
	if len(input.EvidenceNos) > 0 {
		arguments = append(arguments, input.EvidenceNos)
		where += " AND evidence_no = ANY($" + strconv.Itoa(len(arguments)) + ")"
	}
	rows, err := transaction.QueryContext(ctx, `
SELECT evidence_no, route_no, provider_code, operation_scope, source_kind,
       payload_ciphertext, payload_nonce, payload_sha256,
       cipher_format_version, encryption_key_version,
       normalized_status, decision_code, observed_at
FROM idc_gateway_local_callback_evidence
WHERE `+where+`
ORDER BY evidence_no
LIMIT 17`, arguments...)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			clearExportEvidence(result)
			result = nil
			returnError = ErrUnavailable
		}
	}()
	result = make([]exportedEvidence, 0, MaximumExportEvidence)
	for rows.Next() {
		var item exportedEvidence
		var payloadDigest []byte
		var sealed callbacksecurity.SealedEvidence
		if err = rows.Scan(
			&item.EvidenceNo, &item.RouteNo, &item.ProviderCode, &item.OperationScope,
			&item.SourceKind, &sealed.Ciphertext, &sealed.Nonce, &payloadDigest,
			&sealed.FormatVersion, &sealed.KeyVersion, &item.NormalizedStatus,
			&item.DecisionCode, &item.ObservedAt,
		); err != nil || len(payloadDigest) != sha256.Size {
			clearExportEvidence(result)
			return nil, ErrUnavailable
		}
		copy(sealed.PlainSHA256[:], payloadDigest)
		plaintext, openErr := s.evidenceKeyring.Open(
			item.EvidenceNo, item.RouteNo, item.ProviderCode, item.SourceKind, sealed,
		)
		clear(sealed.Ciphertext)
		clear(sealed.Nonce)
		if openErr != nil {
			clearExportEvidence(result)
			return nil, ErrUnavailable
		}
		item.Payload = plaintext
		item.PayloadSHA256 = hex.EncodeToString(sealed.PlainSHA256[:])
		result = append(result, item)
		if len(result) > MaximumExportEvidence {
			clearExportEvidence(result)
			return nil, ErrConflict
		}
	}
	if err = rows.Err(); err != nil || len(result) == 0 ||
		(len(input.EvidenceNos) > 0 && len(result) != len(input.EvidenceNos)) {
		clearExportEvidence(result)
		return nil, ErrConflict
	}
	return result, nil
}

func (s *Service) recordFailedExport(
	ctx context.Context,
	exportNo string,
	employee string,
	scopeDigest [sha256.Size]byte,
	requestedAt time.Time,
) error {
	result, err := s.database.ExecContext(ctx, `
INSERT INTO idc_gateway_evidence_export
    (export_no, requested_by_employee_no, scope_sha256, manifest_sha256,
     status_code, requested_at, completed_at)
VALUES ($1, $2, $3, NULL, 'failed', $4, CURRENT_TIMESTAMP)
ON CONFLICT (export_no) DO NOTHING`, exportNo, employee, scopeDigest[:], requestedAt)
	if err != nil {
		return err
	}
	if affected, rowsError := result.RowsAffected(); rowsError != nil {
		return rowsError
	} else if affected < 0 || affected > 1 {
		return ErrUnavailable
	}
	return nil
}

func canonicalEvidenceNos(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func clearExportEvidence(items []exportedEvidence) {
	for index := range items {
		clear(items[index].Payload)
		items[index].Payload = nil
	}
}
