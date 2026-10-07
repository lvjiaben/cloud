// This file owns PostgreSQL-authoritative collection scheduling and keeps all
// provider readers frozen at process startup.

package localusage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

const collectionDatabaseTimeout = 5 * time.Second

var installationIDPattern = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)

type collectionLease struct {
	Provider string
	Token    []byte
}

// CollectionService owns the local collection worker and atomic projection writer.
type CollectionService struct {
	database       *sql.DB
	installationID string
	keyring        *callbacksecurity.Ring
	collectors     map[string]Collector
	random         io.Reader
}

// NewCollectionService freezes only externally approved read-only collectors.
func NewCollectionService(
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
	collectors map[string]Collector,
) (*CollectionService, error) {
	if database == nil || !installationIDPattern.MatchString(installationID) || keyring == nil ||
		keyring.ActiveEvidenceVersion() == 0 || keyring.ActiveEventVersion() == 0 ||
		keyring.ActiveReferenceLookupVersion() == 0 {
		return nil, ErrCollectionRejected
	}
	frozen := make(map[string]Collector, len(collectors))
	for provider, collector := range collectors {
		if !validProvider(provider) || nilCollectionDependency(collector) {
			return nil, ErrCollectionRejected
		}
		frozen[provider] = collector
	}
	return &CollectionService{
		database: database, installationID: installationID, keyring: keyring,
		collectors: frozen, random: rand.Reader,
	}, nil
}

// HasCollectors reports whether the production process may start this worker.
func (s *CollectionService) HasCollectors() bool {
	return s != nil && len(s.collectors) > 0
}

// EnsureReady persists explicit enabled or disabled state for both fixed providers.
func (s *CollectionService) EnsureReady(ctx context.Context) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return ErrCollectionRejected
	}
	databaseContext, cancel := context.WithTimeout(ctx, collectionDatabaseTimeout)
	defer cancel()
	for _, provider := range []string{ProviderTencentESSBasic, ProviderNuonuoInvoice} {
		enabled := !nilCollectionDependency(s.collectors[provider])
		result, err := s.database.ExecContext(databaseContext, `
INSERT INTO idc_gateway_supplier_collection_state
    (provider_code, status_code, next_collect_at)
VALUES ($1::varchar(32), CASE WHEN $2 AND EXISTS (
			SELECT 1 FROM idc_gateway_supplier_processing_state
			WHERE provider_code = $1::varchar(32) AND status_code = 'allowed'
		) THEN 'ready' ELSE 'disabled' END,
		CASE WHEN $2 AND EXISTS (
			SELECT 1 FROM idc_gateway_supplier_processing_state
			WHERE provider_code = $1::varchar(32) AND status_code = 'allowed'
        ) THEN CURRENT_TIMESTAMP ELSE NULL END)
ON CONFLICT (provider_code) DO UPDATE
SET status_code = CASE
		WHEN NOT $2 OR NOT EXISTS (
			SELECT 1 FROM idc_gateway_supplier_processing_state
			WHERE provider_code = $1::varchar(32) AND status_code = 'allowed'
        ) THEN 'disabled'
        WHEN idc_gateway_supplier_collection_state.status_code = 'disabled' THEN 'ready'
        ELSE idc_gateway_supplier_collection_state.status_code
    END,
    next_collect_at = CASE
		WHEN NOT $2 OR NOT EXISTS (
			SELECT 1 FROM idc_gateway_supplier_processing_state
			WHERE provider_code = $1::varchar(32) AND status_code = 'allowed'
        ) THEN NULL
        WHEN idc_gateway_supplier_collection_state.status_code = 'disabled'
            THEN CURRENT_TIMESTAMP
        ELSE idc_gateway_supplier_collection_state.next_collect_at
    END,
	lease_sha256 = CASE WHEN NOT $2 OR NOT EXISTS (
			SELECT 1 FROM idc_gateway_supplier_processing_state
			WHERE provider_code = $1::varchar(32) AND status_code = 'allowed'
        ) THEN NULL
        ELSE idc_gateway_supplier_collection_state.lease_sha256 END,
	lease_expires_at = CASE WHEN NOT $2 OR NOT EXISTS (
			SELECT 1 FROM idc_gateway_supplier_processing_state
			WHERE provider_code = $1::varchar(32) AND status_code = 'allowed'
        ) THEN NULL
        ELSE idc_gateway_supplier_collection_state.lease_expires_at END,
    updated_at = CURRENT_TIMESTAMP,
    version = idc_gateway_supplier_collection_state.version + 1`, provider, enabled)
		if err != nil || collectionRowsAffected(result) != 1 {
			return errors.Join(
				ErrCollectionRejected, fmt.Errorf("prepare %s state: %w", provider, err),
			)
		}
	}
	return nil
}

// RunWorker performs an immediate bounded pass and then polls on the frozen cadence.
func (s *CollectionService) RunWorker(ctx context.Context) error {
	if s == nil || ctx == nil {
		return ErrCollectionRejected
	}
	if !s.HasCollectors() {
		return ErrCollectorDisabled
	}
	for {
		if err := s.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		timer := time.NewTimer(CollectionPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}

// RunOnce claims and processes at most the two fixed provider readers.
func (s *CollectionService) RunOnce(ctx context.Context) error {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return ErrCollectionRejected
	}
	if !s.HasCollectors() {
		return ErrCollectorDisabled
	}
	for processed := 0; processed < 2; processed++ {
		leases, err := s.claim(ctx)
		if err != nil {
			return err
		}
		if len(leases) == 0 {
			return nil
		}
		if err = s.processLease(ctx, leases[0]); err != nil {
			return err
		}
	}
	return nil
}

func (s *CollectionService) processLease(ctx context.Context, lease collectionLease) error {
	defer clear(lease.Token)
	collector := s.collectors[lease.Provider]
	if nilCollectionDependency(collector) {
		return ErrCollectionRejected
	}
	collectionContext, cancel := context.WithTimeout(ctx, CollectionExecutionTimeout)
	observation, collectError := collector.Collect(collectionContext)
	cancel()
	defer clearCollection(&observation)
	if collectError != nil {
		if ctx.Err() != nil {
			return nil
		}
		return s.recordFailure(ctx, lease, collectionFailureCode(collectError))
	}
	if observation.ProviderCode != lease.Provider || !validCollection(observation) {
		return s.recordFailure(ctx, lease, CollectionFailureMalformed)
	}
	if _, err := s.persist(ctx, lease, observation); err != nil {
		if errors.Is(err, ErrCollectionLeaseLost) {
			return nil
		}
		failure := CollectionFailurePersistence
		if errors.Is(err, errCollectionConflict) {
			failure = CollectionFailureRejected
		}
		if failureError := s.recordFailure(ctx, lease, failure); failureError != nil {
			return errors.Join(err, failureError)
		}
	}
	return nil
}

func (s *CollectionService) claim(ctx context.Context) (leases []collectionLease, returnError error) {
	providers := make([]string, 0, len(s.collectors))
	for provider := range s.collectors {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	if len(providers) == 0 {
		return []collectionLease{}, nil
	}
	databaseContext, cancel := context.WithTimeout(ctx, collectionDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return nil, ErrCollectionRejected
	}
	defer finishCollectionTransaction(transaction, &returnError)
	arguments := make([]any, 0, len(providers))
	parameters := make([]string, 0, len(providers))
	for _, provider := range providers {
		arguments = append(arguments, provider)
		parameters = append(parameters, fmt.Sprintf("$%d", len(arguments)))
	}
	rows, err := transaction.QueryContext(databaseContext, `
SELECT collection.provider_code
FROM idc_gateway_supplier_collection_state collection
WHERE collection.provider_code IN (`+strings.Join(parameters, ", ")+`)
  AND EXISTS (
      SELECT 1 FROM idc_gateway_supplier_processing_state processing
      WHERE processing.provider_code = collection.provider_code
        AND processing.status_code = 'allowed'
  )
  AND next_collect_at <= CURRENT_TIMESTAMP
  AND (
      status_code IN ('ready', 'retry_wait')
      OR (status_code = 'running' AND lease_expires_at <= CURRENT_TIMESTAMP)
  )
ORDER BY next_collect_at, provider_code
LIMIT 1
FOR UPDATE SKIP LOCKED`, arguments...)
	if err != nil {
		return nil, ErrCollectionRejected
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			returnError = ErrCollectionRejected
		}
	}()
	claimedProviders := make([]string, 0, 1)
	for rows.Next() {
		var provider string
		if err = rows.Scan(&provider); err != nil {
			return nil, ErrCollectionRejected
		}
		claimedProviders = append(claimedProviders, provider)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrCollectionRejected
	}
	if err = rows.Close(); err != nil {
		return nil, ErrCollectionRejected
	}
	leases = make([]collectionLease, 0, len(claimedProviders))
	for _, provider := range claimedProviders {
		token := make([]byte, sha256.Size)
		if _, err = io.ReadFull(s.random, token); err != nil {
			clear(token)
			return nil, ErrCollectionRejected
		}
		digest := sha256.Sum256(token)
		result, updateError := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_supplier_collection_state
SET status_code = 'running', last_started_at = CURRENT_TIMESTAMP,
    lease_sha256 = $2,
    lease_expires_at = CURRENT_TIMESTAMP + $3::bigint * INTERVAL '1 millisecond',
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE provider_code = $1`, provider, digest[:], CollectionLeaseDuration.Milliseconds())
		if updateError != nil || collectionRowsAffected(result) != 1 {
			clear(token)
			return nil, ErrCollectionRejected
		}
		leases = append(leases, collectionLease{Provider: provider, Token: token})
	}
	if err = transaction.Commit(); err != nil {
		for index := range leases {
			clear(leases[index].Token)
		}
		return nil, ErrCollectionRejected
	}
	return leases, nil
}

func (s *CollectionService) recordFailure(
	ctx context.Context,
	lease collectionLease,
	code CollectionFailureCode,
) error {
	if !validProvider(lease.Provider) || len(lease.Token) != sha256.Size ||
		!validCollectionFailureCode(code) {
		return ErrCollectionRejected
	}
	databaseContext, cancel := context.WithTimeout(ctx, collectionDatabaseTimeout)
	defer cancel()
	digest := sha256.Sum256(lease.Token)
	result, err := s.database.ExecContext(databaseContext, `
UPDATE idc_gateway_supplier_collection_state
SET status_code = 'retry_wait',
    failure_count = LEAST(32, failure_count + 1),
    last_failure_code = $3,
    next_collect_at = CURRENT_TIMESTAMP + CASE LEAST(6, failure_count + 1)
        WHEN 1 THEN INTERVAL '1 minute'
        WHEN 2 THEN INTERVAL '2 minutes'
        WHEN 3 THEN INTERVAL '4 minutes'
        WHEN 4 THEN INTERVAL '8 minutes'
        WHEN 5 THEN INTERVAL '16 minutes'
        ELSE INTERVAL '30 minutes'
    END,
    lease_sha256 = NULL, lease_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE provider_code = $1 AND status_code = 'running'
  AND lease_sha256 = $2 AND lease_expires_at >= CURRENT_TIMESTAMP`,
		lease.Provider, digest[:], string(code))
	if err != nil {
		return ErrCollectionRejected
	}
	if collectionRowsAffected(result) != 1 {
		return ErrCollectionLeaseLost
	}
	return nil
}

func finishCollectionTransaction(transaction *sql.Tx, returnError *error) {
	rollbackError := transaction.Rollback()
	if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) && *returnError == nil {
		*returnError = ErrCollectionRejected
	}
}

func collectionRowsAffected(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return count
}

func nilCollectionDependency(value any) bool {
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
