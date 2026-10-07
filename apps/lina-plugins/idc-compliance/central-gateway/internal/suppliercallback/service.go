// Package suppliercallback accepts only provider-verified callback events and
// converges them into installation-local PostgreSQL route, evidence, and
// delivery state.
package suppliercallback

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

const (
	MaximumRawBodyBytes       = 256 << 10
	MaximumSignatureBytes     = 16 << 10
	MaximumSignedHeaderBytes  = 16 << 10
	MaximumSignedHeaderCount  = 32
	MaximumEvidenceFrameBytes = (2 * MaximumRawBodyBytes) + MaximumSignatureBytes +
		MaximumSignedHeaderBytes + 4096
	MaximumEvidenceCiphertextBytes = MaximumEvidenceFrameBytes + 16

	ProviderTencentESSBasic = "tencent_essbasic"
	ProviderNuonuoInvoice   = "nuonuo_invoice"
)

var (
	ErrInvalid          = errors.New("local supplier callback input is invalid")
	ErrVerifierDisabled = errors.New("supplier callback verifier is disabled")
	ErrRejected         = errors.New("local supplier callback was rejected")
	ErrConflict         = errors.New("local supplier callback conflicts with stored evidence")

	routeNoPattern        = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)
	installationIDPattern = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)
	operationScopePattern = regexp.MustCompile(`^[a-z][a-z0-9.:-]{1,63}$`)
)

type Status string

const (
	StatusRunning       Status = "running"
	StatusSucceeded     Status = "succeeded"
	StatusFailed        Status = "failed"
	StatusResultUnknown Status = "result_unknown"
)

// RawCallback contains exactly received bytes; provider-specific verifiers
// decide which signed headers participate in their future official contract.
type RawCallback struct {
	Provider      string
	Body          []byte
	Signature     []byte
	SignedHeaders map[string][]byte
}

// VerifiedEvent is produced only after a provider-specific verifier validates
// application identity, signature, time window, event type, and exact bytes.
type VerifiedEvent struct {
	RouteNo            string
	Provider           string
	OperationScope     string
	ExternalEventID    []byte
	Status             Status
	ProviderStatus     string
	EvidenceFrame      []byte
	SupplierObservedAt time.Time
}

// ResolvedReference is the installation-local owner of one protected provider object.
type ResolvedReference struct {
	RouteNo        string
	OperationScope string
}

type Verifier interface {
	Verify(context.Context, RawCallback) (VerifiedEvent, error)
}

// Result is the safe local convergence projection.
type Result struct {
	RouteNo              string
	ProjectedStatus      Status
	ProviderStatus       string
	ReconciliationStatus string
	RouteVersion         int64
	EvidenceNo           string
	DeliveryNo           string
	DecisionCode         string
	SupplierObservedAt   time.Time
	Replayed             bool
}

// Service owns the neutral, provider-independent convergence transaction.
type Service struct {
	database       *sql.DB
	installationID string
	keyring        *callbacksecurity.Ring
	random         io.Reader
}

// Boundary keeps raw bytes on the provider-verifier side of the neutral core.
type Boundary struct {
	verifiers map[string]Verifier
	service   *Service
}

func NewService(
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
) (*Service, error) {
	if database == nil || !installationIDPattern.MatchString(installationID) || keyring == nil ||
		keyring.ActiveEvidenceVersion() == 0 || keyring.ActiveEventVersion() == 0 {
		return nil, ErrInvalid
	}
	return &Service{
		database: database, installationID: installationID, keyring: keyring, random: rand.Reader,
	}, nil
}

func NewBoundary(verifiers map[string]Verifier, service *Service) (*Boundary, error) {
	if service == nil {
		return nil, ErrInvalid
	}
	result := make(map[string]Verifier, len(verifiers))
	for provider, verifier := range verifiers {
		if !validProvider(provider) || nilDependency(verifier) {
			return nil, ErrInvalid
		}
		result[provider] = verifier
	}
	return &Boundary{verifiers: result, service: service}, nil
}

// Accept verifies exact raw bytes and clears all caller-owned callback buffers
// before returning. Missing production verifiers fail closed without fallback.
func (b *Boundary) Accept(ctx context.Context, input RawCallback) (Result, error) {
	defer clearRawCallback(&input)
	if b == nil || b.service == nil || ctx == nil || ctx.Err() != nil || !validRawCallback(input) {
		return Result{}, ErrInvalid
	}
	verifier := b.verifiers[input.Provider]
	if nilDependency(verifier) {
		return Result{}, ErrVerifierDisabled
	}
	event, err := verifier.Verify(ctx, input)
	if err != nil || !validVerifiedEvent(event) || event.Provider != input.Provider {
		clearVerifiedEvent(&event)
		return Result{}, ErrRejected
	}
	defer clearVerifiedEvent(&event)
	return b.service.converge(ctx, event)
}

func validRawCallback(input RawCallback) bool {
	if !validProvider(input.Provider) || len(input.Body) == 0 ||
		len(input.Body) > MaximumRawBodyBytes || len(input.Signature) == 0 ||
		len(input.Signature) > MaximumSignatureBytes ||
		len(input.SignedHeaders) > MaximumSignedHeaderCount {
		return false
	}
	total := 0
	for name, value := range input.SignedHeaders {
		if len(name) == 0 || len(name) > 128 || strings.ContainsAny(name, "\r\n\x00") ||
			len(value) == 0 || len(value) > 4096 || bytes.ContainsAny(value, "\r\n\x00") {
			return false
		}
		total += len(name) + len(value)
		if total > MaximumSignedHeaderBytes {
			return false
		}
	}
	return true
}

func validVerifiedEvent(event VerifiedEvent) bool {
	return routeNoPattern.MatchString(event.RouteNo) && validProvider(event.Provider) &&
		operationScopePattern.MatchString(event.OperationScope) &&
		len(event.ExternalEventID) > 0 && len(event.ExternalEventID) <= 256 &&
		len(event.EvidenceFrame) > 0 && len(event.EvidenceFrame) <= MaximumEvidenceFrameBytes &&
		validStatus(event.Status) &&
		(event.ProviderStatus == "" || validProviderStatus(event.Provider, event.ProviderStatus))
}

func validStatus(status Status) bool {
	switch status {
	case StatusRunning, StatusSucceeded, StatusFailed, StatusResultUnknown:
		return true
	default:
		return false
	}
}

func validProvider(provider string) bool {
	return provider == ProviderTencentESSBasic || provider == ProviderNuonuoInvoice
}

func validProviderStatus(provider string, status string) bool {
	if provider != ProviderTencentESSBasic {
		return false
	}
	switch status {
	case "INIT", "PART", "WILLEXPIRE", "ALL", "REJECT", "DEADLINE", "CANCEL",
		"INVALID", "EXCEPTION", "RELIEVED":
		return true
	default:
		return false
	}
}

func clearRawCallback(input *RawCallback) {
	if input == nil {
		return
	}
	clear(input.Body)
	clear(input.Signature)
	for name, value := range input.SignedHeaders {
		clear(value)
		delete(input.SignedHeaders, name)
	}
}

func clearVerifiedEvent(event *VerifiedEvent) {
	if event == nil {
		return
	}
	clear(event.ExternalEventID)
	clear(event.EvidenceFrame)
	event.ExternalEventID = nil
	event.EvidenceFrame = nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func randomIdentifier(random io.Reader, prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		clear(value)
		return "", err
	}
	defer clear(value)
	return prefix + hex.EncodeToString(value), nil
}

func finishTransaction(transaction *sql.Tx, returnError *error) {
	rollbackError := transaction.Rollback()
	if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) && *returnError == nil {
		*returnError = fmt.Errorf("rollback local supplier callback: %w", rollbackError)
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

func digestBytes(value []byte) [sha256.Size]byte {
	return sha256.Sum256(value)
}

func equalDigest(left []byte, right [sha256.Size]byte) bool {
	return bytes.Equal(left, right[:])
}
