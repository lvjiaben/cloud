// Package localops enforces installation-local ownership for supplier
// callbacks, active reconciliation, manual takeover, and encrypted evidence.
// It deliberately contains no software-vendor routing or operator authority.
package localops

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"
)

const (
	ProviderTencentESSBasic = "tencent_essbasic"
	ProviderNuonuoInvoice   = "nuonuo_invoice"
	InstallationScope       = "local_installation"
	maximumPayloadBytes     = 256 << 10
	maximumSealedBytes      = maximumPayloadBytes + 64
	maximumSignatureBytes   = 16 << 10
	recentFullAuthWindow    = 5 * time.Minute
)

var (
	ErrInvalid         = errors.New("local supplier operation input is invalid")
	ErrAdapterDisabled = errors.New("local supplier adapter is disabled")
	ErrRejected        = errors.New("local supplier operation was rejected")
	ErrRecentAuth      = errors.New("recent current-merchant employee authentication is required")
	operationNoPattern = regexp.MustCompile(`^lop_[A-Za-z0-9]{8,36}$`)
	reasonCodePattern  = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,47}$`)
	employeePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
)

type Status string

const (
	StatusPending        Status = "pending"
	StatusAccepted       Status = "accepted"
	StatusSucceeded      Status = "succeeded"
	StatusFailed         Status = "failed"
	StatusResultUnknown  Status = "result_unknown"
	StatusManualTakeover Status = "manual_takeover"
)

// Operation is the bounded local projection used for active reconciliation.
type Operation struct {
	OperationNo string
	Provider    string
	Status      Status
}

// Observation is a provider-neutral authoritative result.
type Observation struct {
	Status       Status
	StableCode   string
	ExternalRef  []byte
	EvidenceKind string
	Evidence     []byte
	ObservedAt   time.Time
}

// VerifiedCallback is produced only after provider-specific signature checks.
type VerifiedCallback struct {
	OperationNo string
	EventKey    []byte
	Observation Observation
}

// CallbackInput never accepts an installation identifier or remote route.
type CallbackInput struct {
	Provider  string
	Body      []byte
	Signature []byte
}

// EmployeeAuthorization is resolved from the current installation's trusted session.
type EmployeeAuthorization struct {
	PrincipalNo             string
	CurrentMerchantEmployee bool
	FullAuthAt              time.Time
}

type TakeoverInput struct {
	OperationNo       string
	ReasonCode        string
	EvidenceSetSHA256 [sha256.Size]byte
}

// SealedEvidence contains only encrypted local persistence material.
type SealedEvidence struct {
	EventHMAC         [sha256.Size]byte
	PayloadSHA256     [sha256.Size]byte
	PayloadCiphertext []byte
	KeyVersion        string
}

type CallbackVerifier interface {
	Verify(context.Context, []byte, []byte) (VerifiedCallback, error)
}

type ActiveQuerier interface {
	Query(context.Context, Operation) (Observation, error)
}

type CircuitGate interface {
	Allow(context.Context, string) error
	Observe(context.Context, string, error)
}

type EvidenceSealer interface {
	Seal(kind string, eventKey, payload []byte) (SealedEvidence, error)
}

type EmployeeAuthority interface {
	AuthorizeTakeover(context.Context, string) (EmployeeAuthorization, error)
}

// Store is implemented against only the current installation's database.
type Store interface {
	AppendCallback(context.Context, string, VerifiedCallback, SealedEvidence) error
	AppendQuery(context.Context, Operation, Observation, SealedEvidence) error
	AppendTakeover(context.Context, TakeoverInput, EmployeeAuthorization) error
}

type Service struct {
	verifiers map[string]CallbackVerifier
	queriers  map[string]ActiveQuerier
	circuit   CircuitGate
	sealer    EvidenceSealer
	authority EmployeeAuthority
	store     Store
	now       func() time.Time
}

// New creates an explicit local-only policy graph. Missing provider clients
// remain disabled; no fallback to a vendor-operated client is possible.
func New(
	verifiers map[string]CallbackVerifier,
	queriers map[string]ActiveQuerier,
	circuit CircuitGate,
	sealer EvidenceSealer,
	authority EmployeeAuthority,
	store Store,
) (*Service, error) {
	if nilDependency(circuit) || nilDependency(sealer) ||
		nilDependency(authority) || nilDependency(store) {
		return nil, fmt.Errorf("local supplier operation dependencies are invalid")
	}
	if err := validateClientMap(verifiers); err != nil {
		return nil, err
	}
	if err := validateClientMap(queriers); err != nil {
		return nil, err
	}
	return &Service{
		verifiers: cloneMap(verifiers), queriers: cloneMap(queriers),
		circuit: circuit, sealer: sealer, authority: authority, store: store,
		now: time.Now,
	}, nil
}

// AcceptCallback verifies provider authenticity before sealing and appending
// evidence to the local database. Raw callback bytes never reach Store.
func (s *Service) AcceptCallback(ctx context.Context, input CallbackInput) error {
	if s == nil || ctx == nil || ctx.Err() != nil || !validProvider(input.Provider) ||
		len(input.Body) == 0 || len(input.Body) > maximumPayloadBytes ||
		len(input.Signature) == 0 || len(input.Signature) > maximumSignatureBytes {
		return ErrInvalid
	}
	verifier := s.verifiers[input.Provider]
	if nilDependency(verifier) {
		return ErrAdapterDisabled
	}
	verified, err := verifier.Verify(ctx, input.Body, input.Signature)
	if err != nil || !validVerifiedCallback(verified) {
		return ErrRejected
	}
	sealed, err := s.sealer.Seal(
		verified.Observation.EvidenceKind, verified.EventKey, verified.Observation.Evidence,
	)
	if err != nil || !validSealedEvidence(sealed) {
		return ErrRejected
	}
	clearBytes(verified.EventKey)
	verified.EventKey = nil
	clearObservation(&verified.Observation)
	return s.store.AppendCallback(ctx, input.Provider, verified, sealed)
}

// Reconcile performs one circuit-governed active query through the exact local
// provider client and persists only encrypted evidence and stable codes.
func (s *Service) Reconcile(ctx context.Context, operation Operation) error {
	if s == nil || ctx == nil || ctx.Err() != nil || !validOperation(operation) {
		return ErrInvalid
	}
	querier := s.queriers[operation.Provider]
	if nilDependency(querier) {
		return ErrAdapterDisabled
	}
	if err := s.circuit.Allow(ctx, operation.Provider); err != nil {
		return ErrRejected
	}
	observation, queryErr := querier.Query(ctx, operation)
	s.circuit.Observe(ctx, operation.Provider, queryErr)
	if queryErr != nil || !validObservation(observation) {
		clearObservation(&observation)
		return ErrRejected
	}
	eventKey := []byte(operation.OperationNo + "\x00query\x00" + observation.ObservedAt.UTC().Format(time.RFC3339Nano))
	sealed, err := s.sealer.Seal(observation.EvidenceKind, eventKey, observation.Evidence)
	clearBytes(eventKey)
	if err != nil || !validSealedEvidence(sealed) {
		clearObservation(&observation)
		return ErrRejected
	}
	clearObservation(&observation)
	return s.store.AppendQuery(ctx, operation, observation, sealed)
}

// Takeover derives the actor from current-installation employee authority. No
// platform-operator principal can be supplied by the request.
func (s *Service) Takeover(ctx context.Context, input TakeoverInput) error {
	if s == nil || ctx == nil || ctx.Err() != nil ||
		!operationNoPattern.MatchString(input.OperationNo) ||
		!reasonCodePattern.MatchString(input.ReasonCode) ||
		input.EvidenceSetSHA256 == [sha256.Size]byte{} {
		return ErrInvalid
	}
	authorization, err := s.authority.AuthorizeTakeover(ctx, input.OperationNo)
	if err != nil || !authorization.CurrentMerchantEmployee ||
		!employeePattern.MatchString(authorization.PrincipalNo) || authorization.FullAuthAt.IsZero() {
		return ErrRecentAuth
	}
	now := s.now().UTC()
	authenticatedAt := authorization.FullAuthAt.UTC()
	if authenticatedAt.After(now.Add(15*time.Second)) ||
		now.Sub(authenticatedAt) > recentFullAuthWindow {
		return ErrRecentAuth
	}
	return s.store.AppendTakeover(ctx, input, authorization)
}

func validProvider(provider string) bool {
	return provider == ProviderTencentESSBasic || provider == ProviderNuonuoInvoice
}

func validOperation(operation Operation) bool {
	if !operationNoPattern.MatchString(operation.OperationNo) || !validProvider(operation.Provider) {
		return false
	}
	switch operation.Status {
	case StatusPending, StatusAccepted, StatusResultUnknown:
		return true
	default:
		return false
	}
}

func validObservation(observation Observation) bool {
	if observation.ObservedAt.IsZero() || len(observation.StableCode) == 0 ||
		len(observation.StableCode) > 64 || len(observation.EvidenceKind) < 2 ||
		len(observation.EvidenceKind) > 48 || len(observation.Evidence) == 0 ||
		len(observation.Evidence) > maximumPayloadBytes {
		return false
	}
	switch observation.Status {
	case StatusAccepted, StatusSucceeded, StatusFailed, StatusResultUnknown:
		return true
	default:
		return false
	}
}

func validVerifiedCallback(callback VerifiedCallback) bool {
	return operationNoPattern.MatchString(callback.OperationNo) &&
		len(callback.EventKey) >= 16 && len(callback.EventKey) <= 256 &&
		validObservation(callback.Observation)
}

func validSealedEvidence(evidence SealedEvidence) bool {
	return evidence.EventHMAC != [sha256.Size]byte{} &&
		evidence.PayloadSHA256 != [sha256.Size]byte{} &&
		len(evidence.PayloadCiphertext) >= 17 &&
		len(evidence.PayloadCiphertext) <= maximumSealedBytes &&
		len(evidence.KeyVersion) >= 1 && len(evidence.KeyVersion) <= 32
}

func validateClientMap[T any](clients map[string]T) error {
	for provider, client := range clients {
		if !validProvider(provider) || nilDependency(client) {
			return fmt.Errorf("local supplier client registration is invalid")
		}
	}
	return nil
}

func cloneMap[T any](source map[string]T) map[string]T {
	result := make(map[string]T, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func clearObservation(observation *Observation) {
	if observation == nil {
		return
	}
	clearBytes(observation.ExternalRef)
	clearBytes(observation.Evidence)
	observation.ExternalRef = nil
	observation.Evidence = nil
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
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
