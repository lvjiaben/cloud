// Package datalifecycle executes installation-local revocation, export, hold,
// retention, and restore verification without a software-vendor dependency.
package datalifecycle

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"regexp"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
)

const (
	// ProviderTencentESSBasic identifies the installation-local Tencent target.
	ProviderTencentESSBasic = "tencent_essbasic"
	// ProviderNuonuoInvoice identifies the installation-local NuoNuo target.
	ProviderNuonuoInvoice = "nuonuo_invoice"
	// MaximumExportEvidence bounds one evidence archive.
	MaximumExportEvidence = 16
	// MaximumRetentionRows bounds one retention transaction.
	MaximumRetentionRows = 1000
)

var (
	ErrInvalid             = errors.New("local data lifecycle input is invalid")
	ErrUnavailable         = errors.New("local data lifecycle service is unavailable")
	ErrConflict            = errors.New("local data lifecycle operation conflicts with current state")
	ErrStatutoryRetention  = errors.New("statutory evidence cannot be deleted")
	ErrLegalHold           = errors.New("active legal hold blocks retention")
	ErrIncompleteOperation = errors.New("incomplete supplier operation blocks retention")
	ErrSupplierActive      = errors.New("active supplier processing blocks retention")
	ErrCryptoEraseRequired = errors.New("cryptographic erasure requires a separate reviewed terminal action")
	employeePattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
	stableCodePattern      = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,47}$`)
	bindingPattern         = regexp.MustCompile(`^lsb_[0-9a-f]{32}$`)
	routePattern           = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)
	evidencePattern        = regexp.MustCompile(`^lev_[0-9a-f]{32}$`)
	installationPattern    = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)
)

// Service executes lifecycle operations only against the current installation.
type Service struct {
	database         *sql.DB
	installationID   string
	supplierIdentity *supplieridentity.Service
	supplierKeyring  *supplieridentity.Ring
	evidenceKeyring  *callbacksecurity.Ring
	random           io.Reader
}

// NewService binds the shared startup database and purpose-separated keyrings.
func NewService(
	database *sql.DB,
	installationID string,
	identity *supplieridentity.Service,
	supplierKeyring *supplieridentity.Ring,
	evidenceKeyring *callbacksecurity.Ring,
) (*Service, error) {
	if database == nil || identity == nil || supplierKeyring == nil || evidenceKeyring == nil ||
		!installationPattern.MatchString(installationID) {
		return nil, ErrUnavailable
	}
	return &Service{
		database: database, installationID: installationID, supplierIdentity: identity,
		supplierKeyring: supplierKeyring, evidenceKeyring: evidenceKeyring, random: rand.Reader,
	}, nil
}

// EnsureReady verifies the migration-created fail-closed provider controls.
func (s *Service) EnsureReady(ctx context.Context) error {
	if !s.validContext(ctx) {
		return ErrUnavailable
	}
	var instanceCount, providerCount int
	err := s.database.QueryRowContext(ctx, `
SELECT
    (SELECT COUNT(*) FROM idc_gateway_local_supplier_instance
      WHERE installation_id = $1),
    (SELECT COUNT(*) FROM idc_gateway_supplier_processing_state
      WHERE provider_code IN ('tencent_essbasic', 'nuonuo_invoice'))`,
		s.installationID,
	).Scan(&instanceCount, &providerCount)
	if err != nil || instanceCount != 1 || providerCount != 2 {
		return ErrUnavailable
	}
	return nil
}

func (s *Service) validContext(ctx context.Context) bool {
	return s != nil && s.database != nil && ctx != nil && ctx.Err() == nil
}

func validProvider(value string) bool {
	return value == ProviderTencentESSBasic || value == ProviderNuonuoInvoice
}

func validEmployee(value string) bool { return employeePattern.MatchString(value) }

func validDataClass(value string) bool {
	switch value {
	case "supplier_callback", "supplier_evidence", "supplier_usage", "supplier_cost",
		"supplier_sla", "process_nonce", "audit":
		return true
	default:
		return false
	}
}

func randomIdentifier(source io.Reader, prefix string) (string, error) {
	value := make([]byte, 16)
	if source == nil {
		source = rand.Reader
	}
	if _, err := io.ReadFull(source, value); err != nil {
		clear(value)
		return "", err
	}
	defer clear(value)
	const alphabet = "0123456789abcdef"
	var builder strings.Builder
	builder.Grow(len(prefix) + 32)
	builder.WriteString(prefix)
	for _, item := range value {
		builder.WriteByte(alphabet[item>>4])
		builder.WriteByte(alphabet[item&15])
	}
	return builder.String(), nil
}

func digestFrame(domain string, fields ...string) [sha256.Size]byte {
	return sha256.Sum256([]byte(domain + "\x00" + strings.Join(fields, "\x00")))
}

func finishTransaction(transaction *sql.Tx, returnError *error) {
	if transaction == nil || returnError == nil {
		return
	}
	rollbackError := transaction.Rollback()
	if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) && *returnError == nil {
		*returnError = errors.Join(ErrUnavailable, rollbackError)
	}
}

func rowsAffected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return count
}

func classifyDatabaseError(err error) error {
	if err == nil {
		return ErrUnavailable
	}
	return errors.Join(ErrUnavailable, err)
}
