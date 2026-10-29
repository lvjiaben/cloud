// This file assembles the installation-local database, installation identity,
// process-authentication keyring, and supplier-identity service used by the
// gateway process boundary.

package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/datalifecycle"
	"lina-platform/idc-compliance-gateway/internal/localusage"
	"lina-platform/idc-compliance-gateway/internal/migration"
	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// Local runtime file and connection bounds prevent unbounded startup inputs.
const (
	maximumDSNFileBytes  = 4096
	maximumDatabaseConns = 16
	maximumIdleConns     = 4
)

// localRuntime owns the one database pool and keyring shared by all requests.
type localRuntime struct {
	database                *sql.DB
	keyring                 *processauth.Ring
	supplierIdentityKeyring *supplieridentity.Ring
	callbackEvidenceKeyring *callbacksecurity.Ring
	supplierIdentity        *supplieridentity.Service
	dataLifecycle           *datalifecycle.Service
	supplierWriteGuard      *supplierguard.Service
	supplierCallback        *suppliercallback.Boundary
	supplierReconciliation  *suppliercallback.QueryService
	supplierUsage           *localusage.Service
	supplierUsageCollection *localusage.CollectionService
	installationID          string
	processAuthWindow       time.Duration
}

// localSupplierRuntime groups the supplier services assembled from one shared
// database, identity keyring, evidence keyring, and installation identifier.
type localSupplierRuntime struct {
	identity        *supplieridentity.Service
	dataLifecycle   *datalifecycle.Service
	writeGuard      *supplierguard.Service
	callback        *suppliercallback.Boundary
	reconciliation  *suppliercallback.QueryService
	usage           *localusage.Service
	usageCollection *localusage.CollectionService
}

// buildLocalRuntime validates private files and constructs the startup-owned runtime.
func buildLocalRuntime(
	ctx context.Context,
	config RuntimeConfig,
	startupTimeout time.Duration,
) (result *localRuntime, returnError error) {
	if ctx == nil || ctx.Err() != nil || startupTimeout <= 0 ||
		config.ProcessAuthWindow <= 0 ||
		config.ProcessAuthWindow > processauth.MaximumAuthenticationWindow {
		return nil, fmt.Errorf("local runtime configuration is invalid")
	}
	installationIDBytes, err := readBoundedFile(
		config.InstallationIDFile, processauth.MaximumInstallationIDFileSize, true,
	)
	if err != nil {
		return nil, fmt.Errorf("read local installation id: %w", err)
	}
	defer clearRuntimeBytes(installationIDBytes)
	installationID, err := processauth.ParseInstallationID(installationIDBytes)
	if err != nil {
		return nil, fmt.Errorf("parse local installation id: %w", err)
	}
	keyringBytes, err := readBoundedFile(
		config.ProcessAuthKeyringFile, processauth.MaximumKeyringSize, true,
	)
	if err != nil {
		return nil, fmt.Errorf("read local process-auth keyring: %w", err)
	}
	defer clearRuntimeBytes(keyringBytes)
	keyringFile, err := processauth.Parse(keyringBytes)
	if err != nil {
		return nil, fmt.Errorf("parse local process-auth keyring: %w", err)
	}
	keyring, err := processauth.New(keyringFile)
	if err != nil {
		return nil, fmt.Errorf("construct local process-auth keyring: %w", err)
	}
	closeKeyring := true
	defer func() {
		if closeKeyring {
			keyring.Clear()
		}
	}()
	supplierKeyringBytes, err := readBoundedFile(
		config.SupplierIdentityKeyringFile, supplieridentity.MaximumKeyringSize, true,
	)
	if err != nil {
		return nil, fmt.Errorf("read local supplier identity keyring: %w", err)
	}
	defer clearRuntimeBytes(supplierKeyringBytes)
	supplierKeyringFile, err := supplieridentity.Parse(supplierKeyringBytes)
	if err != nil {
		return nil, fmt.Errorf("parse local supplier identity keyring: %w", err)
	}
	supplierKeyring, err := supplieridentity.New(supplierKeyringFile)
	if err != nil {
		return nil, fmt.Errorf("construct local supplier identity keyring: %w", err)
	}
	closeSupplierKeyring := true
	defer func() {
		if closeSupplierKeyring {
			supplierKeyring.Clear()
		}
	}()
	callbackKeyringBytes, err := readBoundedFile(
		config.CallbackEvidenceKeyringFile, callbacksecurity.MaximumKeyringSize, true,
	)
	if err != nil {
		return nil, fmt.Errorf("read local callback evidence keyring: %w", err)
	}
	defer clearRuntimeBytes(callbackKeyringBytes)
	callbackKeyringFile, err := callbacksecurity.Parse(callbackKeyringBytes)
	if err != nil {
		return nil, fmt.Errorf("parse local callback evidence keyring: %w", err)
	}
	callbackKeyring, err := callbacksecurity.New(callbackKeyringFile)
	if err != nil {
		return nil, fmt.Errorf("construct local callback evidence keyring: %w", err)
	}
	closeCallbackKeyring := true
	defer func() {
		if closeCallbackKeyring {
			callbackKeyring.Clear()
		}
	}()

	dsnBytes, err := readBoundedFile(config.DatabaseDSNFile, maximumDSNFileBytes, true)
	if err != nil {
		return nil, fmt.Errorf("read local database DSN file: %w", err)
	}
	defer clearRuntimeBytes(dsnBytes)
	dsnBytes = bytes.TrimSpace(dsnBytes)
	if len(dsnBytes) == 0 || bytes.IndexByte(dsnBytes, 0) >= 0 {
		return nil, fmt.Errorf("local database DSN file is empty or invalid")
	}
	database, err := sql.Open("pgx", string(dsnBytes))
	if err != nil {
		return nil, fmt.Errorf("open local gateway database: %w", err)
	}
	closeDatabase := true
	defer func() {
		if closeDatabase {
			if closeError := database.Close(); closeError != nil {
				returnError = errors.Join(
					returnError, fmt.Errorf("close failed local gateway database: %w", closeError),
				)
			}
		}
	}()
	database.SetMaxOpenConns(maximumDatabaseConns)
	database.SetMaxIdleConns(maximumIdleConns)
	database.SetConnMaxLifetime(30 * time.Minute)
	startupContext, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	if err = database.PingContext(startupContext); err != nil {
		return nil, fmt.Errorf("ping local gateway database: %w", err)
	}
	if err = migration.Apply(startupContext, database); err != nil {
		return nil, fmt.Errorf("apply local gateway migrations: %w", err)
	}
	supplierRuntime, err := buildLocalSupplierRuntime(
		startupContext,
		database,
		installationID,
		supplierKeyring,
		callbackKeyring,
	)
	if err != nil {
		return nil, err
	}
	closeKeyring = false
	closeSupplierKeyring = false
	closeCallbackKeyring = false
	closeDatabase = false
	return &localRuntime{
		database: database, keyring: keyring,
		supplierIdentityKeyring: supplierKeyring, supplierIdentity: supplierRuntime.identity,
		dataLifecycle:           supplierRuntime.dataLifecycle,
		callbackEvidenceKeyring: callbackKeyring, supplierWriteGuard: supplierRuntime.writeGuard,
		supplierCallback: supplierRuntime.callback, supplierReconciliation: supplierRuntime.reconciliation,
		supplierUsage: supplierRuntime.usage, supplierUsageCollection: supplierRuntime.usageCollection,
		installationID: installationID, processAuthWindow: config.ProcessAuthWindow,
	}, nil
}

func buildLocalIdentityLifecycle(
	ctx context.Context,
	database *sql.DB,
	installationID string,
	supplierKeyring *supplieridentity.Ring,
	callbackKeyring *callbacksecurity.Ring,
	canonicalizers map[string]supplieridentity.Canonicalizer,
) (*supplieridentity.Service, *datalifecycle.Service, error) {
	identity, err := supplieridentity.NewService(
		database, installationID, supplierKeyring, canonicalizers,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("construct local supplier identity service: %w", err)
	}
	if err = identity.EnsureReady(ctx); err != nil {
		return nil, nil, fmt.Errorf("prepare local supplier identity service: %w", err)
	}
	lifecycle, err := datalifecycle.NewService(
		database, installationID, identity, supplierKeyring, callbackKeyring,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("construct local data lifecycle service: %w", err)
	}
	if err = lifecycle.EnsureReady(ctx); err != nil {
		return nil, nil, fmt.Errorf("prepare local data lifecycle service: %w", err)
	}
	return identity, lifecycle, nil
}

// buildLocalSupplierRuntime assembles supplier-neutral boundaries and the
// optional buyer-owned Tencent adapter, including its explicitly gated callback.
// buildLocalSupplierRuntime keeps the invoice supplier boundary closed until its evidence gate passes.
func buildLocalSupplierRuntime(
	ctx context.Context, database *sql.DB, installationID string,
	supplierKeyring *supplieridentity.Ring, callbackKeyring *callbacksecurity.Ring,
) (*localSupplierRuntime, error) {
	identity, lifecycle, err := buildLocalIdentityLifecycle(ctx, database, installationID, supplierKeyring, callbackKeyring, map[string]supplieridentity.Canonicalizer{})
	if err != nil {
		return nil, err
	}
	writeGuard, err := supplierguard.NewService(database, installationID, supplierguard.DefaultPolicy())
	if err != nil {
		return nil, fmt.Errorf("construct local supplier write guard: %w", err)
	}
	reconciliation, err := suppliercallback.NewQueryService(database, installationID, callbackKeyring, map[string]suppliercallback.ReadOnlyQuerier{})
	if err != nil {
		return nil, fmt.Errorf("construct closed local supplier query boundary: %w", err)
	}
	callbackService, err := suppliercallback.NewService(database, installationID, callbackKeyring)
	if err != nil {
		return nil, fmt.Errorf("construct local supplier callback service: %w", err)
	}
	callback, err := suppliercallback.NewBoundary(map[string]suppliercallback.Verifier{}, callbackService)
	if err != nil {
		return nil, fmt.Errorf("construct local supplier callback boundary: %w", err)
	}
	usage, usageCollection, err := buildLocalSupplierUsage(ctx, database, installationID, callbackKeyring)
	if err != nil {
		return nil, err
	}
	return &localSupplierRuntime{identity: identity, dataLifecycle: lifecycle, writeGuard: writeGuard,
		callback: callback, reconciliation: reconciliation, usage: usage, usageCollection: usageCollection}, nil
}

// buildLocalSupplierUsage assembles the read model and closed production collector boundary.
func buildLocalSupplierUsage(
	ctx context.Context,
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
) (*localusage.Service, *localusage.CollectionService, error) {
	usageStore, err := localusage.NewPostgreSQLStore(database)
	if err != nil {
		return nil, nil, fmt.Errorf("construct local supplier usage store: %w", err)
	}
	usage, err := localusage.New(usageStore)
	if err != nil {
		return nil, nil, fmt.Errorf("construct local supplier usage service: %w", err)
	}
	// Production collectors remain deliberately empty until Tencent and Nuonuo
	// usage-query contracts pass their external-evidence gates. This still
	// persists an explicit disabled state and never falls back to a central API.
	collection, err := localusage.NewCollectionService(
		database, installationID, keyring, map[string]localusage.Collector{},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("construct closed local supplier usage collection: %w", err)
	}
	if err = collection.EnsureReady(ctx); err != nil {
		return nil, nil, fmt.Errorf("prepare local supplier usage collection: %w", err)
	}
	return usage, collection, nil
}

// Ready reports whether identity, keys, and the local database are usable.
func (r *localRuntime) Ready(ctx context.Context) bool {
	if r == nil || r.database == nil || r.keyring == nil || r.keyring.ActiveKeyID() == "" ||
		r.supplierIdentityKeyring == nil || r.supplierIdentity == nil ||
		r.dataLifecycle == nil ||
		r.callbackEvidenceKeyring == nil || r.supplierWriteGuard == nil ||
		r.supplierCallback == nil || r.supplierReconciliation == nil ||
		r.supplierUsage == nil || r.supplierUsageCollection == nil ||
		r.supplierIdentityKeyring.ActiveVersion(supplieridentity.PurposeSubjectEncryption) == 0 ||
		r.supplierIdentityKeyring.ActiveVersion(supplieridentity.PurposeSubjectLookup) == 0 ||
		r.supplierIdentityKeyring.ActiveVersion(supplieridentity.PurposeIdentityOperation) == 0 ||
		r.callbackEvidenceKeyring.ActiveEvidenceVersion() == 0 ||
		r.callbackEvidenceKeyring.ActiveEventVersion() == 0 ||
		r.callbackEvidenceKeyring.ActiveReferenceVersion() == 0 ||
		r.callbackEvidenceKeyring.ActiveReferenceLookupVersion() == 0 ||
		r.callbackEvidenceKeyring.ActiveTakeoverOperationVersion() == 0 ||
		r.installationID == "" || r.processAuthWindow <= 0 || ctx == nil {
		return false
	}
	checkContext, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return r.database.PingContext(checkContext) == nil
}

// Close clears copied keys and closes the shared database pool.
func (r *localRuntime) Close() error {
	if r == nil {
		return nil
	}
	if r.keyring != nil {
		r.keyring.Clear()
	}
	if r.supplierIdentityKeyring != nil {
		r.supplierIdentityKeyring.Clear()
	}
	if r.callbackEvidenceKeyring != nil {
		r.callbackEvidenceKeyring.Clear()
	}
	if r.database == nil {
		return nil
	}
	return r.database.Close()
}

// readBoundedFile reads one opened regular file with optional private permissions.
func readBoundedFile(
	path string,
	maximum int64,
	secret bool,
) (content []byte, returnError error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeError := file.Close(); closeError != nil && returnError == nil {
			clearRuntimeBytes(content)
			content = nil
			returnError = fmt.Errorf("close bounded file: %w", closeError)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path is not a regular file")
	}
	if secret && runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("secret file permissions must not grant group or other access")
	}
	content, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		clearRuntimeBytes(content)
		return nil, err
	}
	if int64(len(content)) > maximum {
		clearRuntimeBytes(content)
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return content, nil
}

// clearRuntimeBytes removes sensitive startup data from the supplied slice.
func clearRuntimeBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
