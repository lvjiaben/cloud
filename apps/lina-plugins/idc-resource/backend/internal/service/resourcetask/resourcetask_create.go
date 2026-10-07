// This file creates idempotent Provider tasks without persisting caller keys.

package resourcetask

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

var providerCorrelationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)

const (
	maximumTaskAttempts = 32
	maximumTaskPriority = 1000
)

// CreateInput is one validated, non-secret local intent for a Provider action.
type CreateInput struct {
	RequestID           string
	Capability          resourcecap.CapabilityCode
	LocalResourceNo     string
	ChannelNo           string
	PoolNo              string
	Parameters          json.RawMessage
	OwnerMetadata       json.RawMessage
	Priority            int
	MaxAttempts         int
	AvailableAt         *time.Time
	SecretBindings      []resourcecap.SecretBinding
	ProviderCorrelation *ProviderCorrelationInput
}

type ProviderCorrelationInput struct {
	PluginID, ConfigurationRevision, DownstreamURL string
	CreateToken                                    func(context.Context, int64, string) (secretcap.CreateResult, error)
}

// CreatedTask identifies the durable task selected by the idempotency key.
type CreatedTask struct {
	ID                  int64
	TaskNo              string
	Status              Status
	AvailableAt         time.Time
	Created             bool
	ProviderCorrelation *resourcecap.ProviderCorrelation
}

// Create validates an operation, proves its channel/pool are currently
// enabled, and inserts it once by channel-scoped request digest. The raw
// caller idempotency key never enters PostgreSQL or the returned projection.
func (s *Store) Create(ctx context.Context, input CreateInput) (CreatedTask, error) {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return CreatedTask{}, gerror.New("resource task store is unavailable")
	}
	canonicalParameters, canonicalOwnerMetadata, err := validateCreateInput(input)
	if err != nil {
		return CreatedTask{}, err
	}
	digest := sha256.Sum256([]byte(input.RequestID))
	requestDigest := hex.EncodeToString(digest[:])
	taskNo, err := newTaskNumber(s.random)
	if err != nil {
		return CreatedTask{}, err
	}
	created := CreatedTask{}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, clockErr := resourceDatabaseTime(tx)
		if clockErr != nil {
			return clockErr
		}
		availableAt := now
		if input.AvailableAt != nil {
			availableAt = input.AvailableAt.UTC().Truncate(time.Microsecond)
			if availableAt.Before(now) || availableAt.After(now.Add(maximumTransitionScheduleDelay)) {
				return gerror.New("resource task availability schedule is invalid")
			}
		}
		if err := requireEnabledTaskDestination(ctx, tx, input.ChannelNo, input.PoolNo); err != nil {
			return err
		}
		pool := any(nil)
		if input.PoolNo != "" {
			pool = input.PoolNo
		}
		result, insertErr := tx.Exec(`
INSERT INTO plugin_idc_resource_task
    (tenant_id, task_no, request_id_digest, capability_code, local_resource_no,
     channel_no, pool_no, status_code, priority, sanitized_parameters, owner_metadata,
     max_attempts, available_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', ?, CAST(? AS JSONB), CAST(? AS JSONB), ?, ?)
ON CONFLICT (tenant_id, channel_no, request_id_digest) DO NOTHING`,
			idccap.DefaultTenantID,
			taskNo,
			requestDigest,
			input.Capability,
			input.LocalResourceNo,
			input.ChannelNo,
			pool,
			input.Priority,
			string(canonicalParameters),
			string(canonicalOwnerMetadata),
			input.MaxAttempts,
			availableAt,
		)
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert resource task")
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return gerror.Wrap(affectedErr, "read resource task insert result")
		}
		row, loadErr := loadTaskByDigest(ctx, tx, input.ChannelNo, requestDigest)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return gerror.New("resource task idempotency result is unavailable")
		}
		if !sameCreateIntent(row, input, canonicalParameters, canonicalOwnerMetadata) {
			return gerror.New("resource task idempotency key conflicts with another request")
		}
		if affected == 1 {
			if err := insertTaskSecretBindings(ctx, tx, row.TaskNo, input.SecretBindings, s.random); err != nil {
				return err
			}
			if input.ProviderCorrelation != nil {
				if err := insertProviderCorrelation(ctx, tx, row, input.ChannelNo, input.ProviderCorrelation, s.random); err != nil {
					return err
				}
			}
		} else {
			bindings, bindingErr := loadInputTaskSecretBindings(ctx, tx, []string{row.TaskNo})
			if bindingErr != nil {
				return bindingErr
			}
			if !containsSecretBindings(bindings[row.TaskNo], input.SecretBindings) {
				return gerror.New("resource task idempotency key conflicts with another request")
			}
		}
		if row.AvailableAt == nil {
			return gerror.New("resource task availability is unavailable")
		}
		created = CreatedTask{
			ID: row.Id, TaskNo: row.TaskNo, Status: Status(row.StatusCode),
			AvailableAt: row.AvailableAt.UTC(), Created: affected == 1,
		}
		if input.ProviderCorrelation != nil {
			correlation, correlationErr := loadProviderCorrelation(tx, row.TaskNo, input.ProviderCorrelation)
			if correlationErr != nil {
				return correlationErr
			}
			created.ProviderCorrelation = correlation
		}
		return nil
	})
	if err != nil {
		return CreatedTask{}, gerror.Wrap(err, "create resource task")
	}
	return created, nil
}

func insertProviderCorrelation(ctx context.Context, tx gdb.TX, row *entity.Task, channelNo string, input *ProviderCorrelationInput, random io.Reader) error {
	if row == nil || row.Id <= 0 || input == nil || input.CreateToken == nil || resourcecap.ValidatePluginID(input.PluginID) != nil ||
		!providerCorrelationRevisionPattern.MatchString(input.ConfigurationRevision) || input.DownstreamURL == "" {
		return gerror.New("resource task Provider correlation is invalid")
	}
	result, err := input.CreateToken(ctx, row.Id, row.TaskNo)
	if err != nil || result.Handle.IsZero() || !result.Metadata.Configured {
		return gerror.New("resource task Provider correlation secret is unavailable")
	}
	if err = insertTaskSecretBindings(ctx, tx, row.TaskNo, []resourcecap.SecretBinding{{Name: "downstream_token", Handle: result.Handle}}, random); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO plugin_idc_resource_provider_correlation
    (tenant_id,task_id,task_no,plugin_id,channel_no,configuration_revision,downstream_url,token_secret_ref)
VALUES (?,?,?,?,?,?,?,?)`, idccap.DefaultTenantID, row.Id, row.TaskNo, input.PluginID, channelNo,
		input.ConfigurationRevision, input.DownstreamURL, result.Handle.OpaqueRef())
	return err
}

func loadProviderCorrelation(tx gdb.TX, taskNo string, expected *ProviderCorrelationInput) (*resourcecap.ProviderCorrelation, error) {
	var row struct {
		TaskID                                         int64 `orm:"task_id"`
		PluginID, ConfigurationRevision, DownstreamURL string
	}
	if err := tx.GetScan(&row, `SELECT task_id,plugin_id,configuration_revision,downstream_url
FROM plugin_idc_resource_provider_correlation WHERE tenant_id=? AND task_no=?`, idccap.DefaultTenantID, taskNo); err != nil ||
		row.TaskID <= 0 || row.PluginID != expected.PluginID || row.ConfigurationRevision != expected.ConfigurationRevision || row.DownstreamURL != expected.DownstreamURL {
		return nil, gerror.New("resource task Provider correlation conflicts")
	}
	return &resourcecap.ProviderCorrelation{DownstreamID: row.TaskID, DownstreamURL: row.DownstreamURL}, nil
}

func containsSecretBindings(actual, expected []resourcecap.SecretBinding) bool {
	for _, wanted := range expected {
		found := false
		for _, item := range actual {
			if item.Name == wanted.Name && item.Handle.Equal(wanted.Handle) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// validateCreateInput validates the Provider request and returns canonical non-secret parameters.
func validateCreateInput(input CreateInput) (json.RawMessage, json.RawMessage, error) {
	if input.ProviderCorrelation != nil {
		probe := &resourcecap.ProviderCorrelation{DownstreamID: 1, DownstreamURL: input.ProviderCorrelation.DownstreamURL}
		if input.Capability != resourcecap.CapabilityProvision || input.ProviderCorrelation.CreateToken == nil {
			return nil, nil, gerror.New("resource task Provider correlation is invalid")
		}
		if err := resourcecap.ValidatePluginID(input.ProviderCorrelation.PluginID); err != nil {
			return nil, nil, gerror.Wrap(err, "resource task Provider correlation plugin is invalid")
		}
		if !providerCorrelationRevisionPattern.MatchString(input.ProviderCorrelation.ConfigurationRevision) {
			return nil, nil, gerror.New("resource task Provider correlation revision is invalid")
		}
		if err := resourcecap.ValidateProviderCorrelation(probe); err != nil {
			return nil, nil, gerror.Wrap(err, "resource task Provider correlation URL is invalid")
		}
		for _, binding := range input.SecretBindings {
			if binding.Name == "downstream_token" {
				return nil, nil, gerror.New("resource task Provider correlation token is owner-only")
			}
		}
	}
	request := resourcecap.OperationRequest{
		RequestID:      input.RequestID,
		Capability:     input.Capability,
		ResourceRef:    input.LocalResourceNo,
		Parameters:     input.Parameters,
		SecretBindings: input.SecretBindings,
	}
	if err := request.Validate(); err != nil {
		return nil, nil, gerror.New("resource task operation is invalid")
	}
	if resourcecap.ValidateChannelRef(input.ChannelNo) != nil || !workerIDPattern.MatchString(input.ChannelNo) ||
		(input.PoolNo != "" && !workerIDPattern.MatchString(input.PoolNo)) {
		return nil, nil, gerror.New("resource task destination is invalid")
	}
	if input.Priority < 0 || input.Priority > maximumTaskPriority ||
		input.MaxAttempts <= 0 || input.MaxAttempts > maximumTaskAttempts {
		return nil, nil, gerror.New("resource task execution policy is invalid")
	}
	canonical, err := canonicalJSONObject(input.Parameters)
	if err != nil {
		return nil, nil, gerror.New("resource task parameters are invalid")
	}
	ownerMetadata, err := canonicalJSONObject(input.OwnerMetadata)
	if err != nil {
		return nil, nil, gerror.New("resource task owner metadata is invalid")
	}
	if err = resourcecap.ValidateOwnerMetadata(ownerMetadata); err != nil {
		return nil, nil, gerror.Wrap(err, "resource task owner metadata is invalid")
	}
	return canonical, ownerMetadata, nil
}

// insertTaskSecretBindings stores bounded input handles under the new task transaction.
func insertTaskSecretBindings(
	ctx context.Context,
	tx gdb.TX,
	taskNo string,
	bindings []resourcecap.SecretBinding,
	random io.Reader,
) error {
	for _, binding := range bindings {
		bindingNo, err := newBindingNumber(random)
		if err != nil {
			return err
		}
		if _, err = tx.Model(dao.TaskSecretBinding.Table()).Safe().Ctx(ctx).
			Data(do.TaskSecretBinding{
				TenantId:         idccap.DefaultTenantID,
				BindingNo:        bindingNo,
				TaskNo:           taskNo,
				BindingName:      binding.Name,
				BindingDirection: "input",
				SecretRef:        binding.Handle.OpaqueRef(),
			}).Insert(); err != nil {
			return gerror.Wrap(err, "insert resource task secret binding")
		}
	}
	return nil
}

// loadInputTaskSecretBindings restores active handles with one bounded collection query.
func loadInputTaskSecretBindings(
	ctx context.Context,
	tx gdb.TX,
	taskNos []string,
) (map[string][]resourcecap.SecretBinding, error) {
	bindings := make(map[string][]resourcecap.SecretBinding, len(taskNos))
	if len(taskNos) == 0 {
		return bindings, nil
	}
	columns := dao.TaskSecretBinding.Columns()
	var rows []entity.TaskSecretBinding
	limit := len(taskNos)*resourcecap.MaximumSecretBindingCount + 1
	err := tx.Model(dao.TaskSecretBinding.Table()).Safe().Ctx(ctx).
		Fields(columns.TaskNo, columns.BindingName, columns.SecretRef).
		Where(do.TaskSecretBinding{
			TenantId: idccap.DefaultTenantID, BindingDirection: "input", StatusCode: "active",
		}).
		WhereIn(columns.TaskNo, taskNos).
		OrderAsc(columns.TaskNo).
		OrderAsc(columns.BindingName).
		Limit(limit).
		Scan(&rows)
	if err != nil || len(rows) >= limit {
		return nil, gerror.New("resource task secret bindings are unavailable")
	}
	for _, row := range rows {
		if len(bindings[row.TaskNo]) >= resourcecap.MaximumSecretBindingCount {
			return nil, gerror.New("resource task secret bindings are unavailable")
		}
		handle, handleErr := secretcap.NewHandle(row.SecretRef)
		if handleErr != nil {
			return nil, gerror.New("resource task secret binding is invalid")
		}
		bindings[row.TaskNo] = append(bindings[row.TaskNo], resourcecap.SecretBinding{
			Name: row.BindingName, Handle: handle,
		})
	}
	return bindings, nil
}

// revokeTaskInputBindings closes active input handles after submission ownership ends.
func revokeTaskInputBindings(
	ctx context.Context,
	tx gdb.TX,
	taskNos []string,
	now time.Time,
) error {
	if len(taskNos) == 0 {
		return nil
	}
	columns := dao.TaskSecretBinding.Columns()
	_, err := tx.Model(dao.TaskSecretBinding.Table()).Safe().Ctx(ctx).
		Where(do.TaskSecretBinding{
			TenantId: idccap.DefaultTenantID, BindingDirection: "input", StatusCode: "active",
		}).
		WhereIn(columns.TaskNo, taskNos).
		Data(do.TaskSecretBinding{
			StatusCode: "revoked",
			RevokedAt:  &now,
			Version:    gdb.Raw(columns.Version + " + 1"),
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "revoke resource task input bindings")
	}
	return nil
}

// requireEnabledTaskDestination share-locks the exact enabled channel and optional pool.
func requireEnabledTaskDestination(
	ctx context.Context,
	tx gdb.TX,
	channelNo string,
	poolNo string,
) error {
	channelColumns := dao.Channel.Columns()
	var channel entity.Channel
	err := tx.Model(dao.Channel.Table()).Safe().Ctx(ctx).
		Fields(channelColumns.Id).
		Where(do.Channel{
			TenantId: idccap.DefaultTenantID, ChannelNo: channelNo, StatusCode: "enabled",
		}).
		Lock(gdb.LockForShare).
		Scan(&channel)
	if err != nil {
		return gerror.Wrap(err, "lock enabled resource task channel")
	}
	if channel.Id == 0 {
		return gerror.New("enabled resource task channel is unavailable")
	}
	if poolNo == "" {
		return nil
	}
	poolColumns := dao.Pool.Columns()
	var pool entity.Pool
	err = tx.Model(dao.Pool.Table()).Safe().Ctx(ctx).
		Fields(poolColumns.Id).
		Where(do.Pool{
			TenantId: idccap.DefaultTenantID, PoolNo: poolNo,
			ChannelNo: channelNo, StatusCode: "enabled",
		}).
		Lock(gdb.LockForShare).
		Scan(&pool)
	if err != nil {
		return gerror.Wrap(err, "lock enabled resource task pool")
	}
	if pool.Id == 0 {
		return gerror.New("enabled resource task pool is unavailable")
	}
	return nil
}

// loadTaskByDigest loads the non-secret idempotency projection for one channel.
func loadTaskByDigest(
	ctx context.Context,
	tx gdb.TX,
	channelNo string,
	requestDigest string,
) (*entity.Task, error) {
	columns := dao.Task.Columns()
	var row entity.Task
	err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Fields(
			columns.Id,
			columns.TaskNo,
			columns.CapabilityCode,
			columns.LocalResourceNo,
			columns.ChannelNo,
			columns.PoolNo,
			columns.StatusCode,
			columns.Priority,
			columns.SanitizedParameters,
			columns.OwnerMetadata,
			columns.MaxAttempts,
			columns.AvailableAt,
		).
		Where(do.Task{
			TenantId: idccap.DefaultTenantID, ChannelNo: channelNo, RequestIdDigest: requestDigest,
		}).
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "load idempotent resource task")
	}
	if row.TaskNo == "" {
		return nil, nil
	}
	return &row, nil
}

// sameCreateIntent rejects reuse of an idempotency key for changed operation semantics.
func sameCreateIntent(
	row *entity.Task,
	input CreateInput,
	parameters json.RawMessage,
	ownerMetadata json.RawMessage,
) bool {
	if row == nil || row.CapabilityCode != string(input.Capability) ||
		row.LocalResourceNo != input.LocalResourceNo || row.ChannelNo != input.ChannelNo ||
		row.PoolNo != input.PoolNo || row.Priority != input.Priority ||
		row.MaxAttempts != input.MaxAttempts {
		return false
	}
	if input.AvailableAt != nil &&
		(row.AvailableAt == nil ||
			!row.AvailableAt.UTC().Truncate(time.Microsecond).
				Equal(input.AvailableAt.UTC().Truncate(time.Microsecond))) {
		return false
	}
	persisted, err := canonicalJSONObject([]byte(row.SanitizedParameters))
	if err != nil || !bytes.Equal(persisted, parameters) {
		return false
	}
	persistedOwner, err := canonicalJSONObject([]byte(row.OwnerMetadata))
	return err == nil && bytes.Equal(persistedOwner, ownerMetadata)
}

// canonicalJSONObject validates and deterministically serializes one JSON object.
func canonicalJSONObject(payload json.RawMessage) (json.RawMessage, error) {
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, gerror.New("resource task parameters are invalid")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, gerror.New("resource task parameters are invalid")
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return nil, gerror.New("resource task parameters are invalid")
	}
	return canonical, nil
}

// newTaskNumber creates one unpredictable owner task number.
func newTaskNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource task number")
	}
	return "task_" + hex.EncodeToString(content[:]), nil
}

// newBindingNumber creates one unpredictable task-secret binding number.
func newBindingNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource task secret binding number")
	}
	return "binding_" + hex.EncodeToString(content[:]), nil
}
