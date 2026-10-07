// Package channelconfig loads enabled Provider channels without exposing secret references.
package channelconfig

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

const maximumChannelSecretBindings = 64

var configurationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)

// Configuration is the detached in-process construction input for one exact
// enabled channel. Secret references exist only as non-serializable handles.
type Configuration struct {
	PluginID          string
	ProviderID        string
	RuntimeType       string
	ContractVersion   string
	ReleaseVersion    string
	ReleaseDigest     string
	DeclarationDigest string
	Config            resourcecap.ProviderConfig
}

// Reader loads one channel and all active bindings in two bounded queries.
type Reader struct {
	database gdb.DB
}

// NewReader constructs a channel reader against one explicitly injected database.
func NewReader(database gdb.DB) (*Reader, error) {
	if database == nil {
		return nil, gerror.New("resource channel database is required")
	}
	return &Reader{database: database}, nil
}

// ReadEnabled loads only an enabled, non-deleted channel. Disabled channels
// cannot construct a Provider or accept a new resource operation.
func (r *Reader) ReadEnabled(ctx context.Context, channelRef string) (Configuration, error) {
	return r.readCurrent(ctx, channelRef, true)
}

// ReadConfigured loads a complete non-deleted channel for an explicit
// administrator connection check. It does not make the channel eligible for
// business traffic and therefore intentionally accepts the disabled state.
func (r *Reader) ReadConfigured(ctx context.Context, channelRef string) (Configuration, error) {
	return r.readCurrent(ctx, channelRef, false)
}

func (r *Reader) readCurrent(ctx context.Context, channelRef string, enabledOnly bool) (Configuration, error) {
	if r == nil || r.database == nil || ctx == nil || ctx.Err() != nil ||
		resourcecap.ValidateChannelRef(channelRef) != nil {
		return Configuration{}, gerror.New("resource channel configuration is unavailable")
	}
	columns := dao.Channel.Columns()
	var channel entity.Channel
	model := r.database.Model(dao.Channel.Table()).Safe().Ctx(ctx).
		Fields(
			columns.ChannelNo,
			columns.ProviderId,
			columns.ProviderPluginId,
			columns.ProviderRuntimeType,
			columns.ProviderContractVersion,
			columns.ProviderReleaseVersion,
			columns.ProviderReleaseDigest,
			columns.ProviderDeclarationDigest,
			columns.Endpoint,
			columns.NonSecretSettings,
		).
		Where(columns.TenantId, idccap.DefaultTenantID).
		Where(columns.ChannelNo, channelRef).
		WhereNull(columns.DeletedAt)
	if enabledOnly {
		model = model.Where(columns.StatusCode, "enabled")
	} else {
		model = model.WhereIn(columns.StatusCode, []string{"disabled", "enabled", "maintenance"})
	}
	err := model.Scan(&channel)
	if err != nil || channel.ChannelNo == "" {
		return Configuration{}, gerror.New("enabled resource channel is unavailable")
	}

	bindingColumns := dao.ChannelSecretBinding.Columns()
	var bindings []entity.ChannelSecretBinding
	err = r.database.Model(dao.ChannelSecretBinding.Table()).Safe().Ctx(ctx).
		Fields(bindingColumns.BindingName, bindingColumns.SecretRef).
		Where(bindingColumns.TenantId, idccap.DefaultTenantID).
		Where(bindingColumns.ChannelNo, channelRef).
		Where(bindingColumns.StatusCode, "active").
		OrderAsc(bindingColumns.BindingName).
		Limit(maximumChannelSecretBindings + 1).
		Scan(&bindings)
	if err != nil || len(bindings) > maximumChannelSecretBindings {
		return Configuration{}, gerror.New("resource channel bindings are unavailable")
	}
	secretBindings := make([]resourcecap.SecretBinding, 0, len(bindings))
	for _, binding := range bindings {
		handle, handleErr := secretcap.NewHandle(binding.SecretRef)
		if handleErr != nil {
			return Configuration{}, gerror.New("resource channel binding reference is invalid")
		}
		secretBindings = append(secretBindings, resourcecap.SecretBinding{
			Name: binding.BindingName, Handle: handle,
		})
	}
	config := resourcecap.ProviderConfig{
		ChannelRef:     channel.ChannelNo,
		Endpoint:       channel.Endpoint,
		Settings:       json.RawMessage(channel.NonSecretSettings),
		SecretBindings: secretBindings,
	}
	if err = config.Validate(); err != nil {
		return Configuration{}, gerror.New("resource Provider configuration is invalid")
	}
	if err = resourcecap.ValidatePluginID(channel.ProviderPluginId); err != nil {
		return Configuration{}, gerror.New("resource Provider owner identity is invalid")
	}
	if err = resourcecap.ValidateProviderID(channel.ProviderId); err != nil {
		return Configuration{}, gerror.New("resource Provider identity is invalid")
	}
	if !validProviderEvidence(channel) {
		return Configuration{}, gerror.New("resource Provider release evidence is invalid")
	}
	return Configuration{
		PluginID: channel.ProviderPluginId, ProviderID: channel.ProviderId,
		RuntimeType: channel.ProviderRuntimeType, ContractVersion: channel.ProviderContractVersion,
		ReleaseVersion: channel.ProviderReleaseVersion, ReleaseDigest: strings.TrimSpace(channel.ProviderReleaseDigest),
		DeclarationDigest: strings.TrimSpace(channel.ProviderDeclarationDigest), Config: config,
	}, nil
}

// ReadRevision loads one immutable endpoint/settings/secret-reference snapshot
// for a paid task while still requiring the owning channel to be operational.
func (r *Reader) ReadRevision(ctx context.Context, channelRef, revisionNo string) (Configuration, error) {
	if r == nil || r.database == nil || ctx == nil || ctx.Err() != nil ||
		resourcecap.ValidateChannelRef(channelRef) != nil || !configurationRevisionPattern.MatchString(revisionNo) {
		return Configuration{}, gerror.New("resource channel revision is unavailable")
	}
	var row struct {
		ChannelNo, ProviderID, ProviderPluginID, ProviderRuntimeType string
		ProviderContractVersion, ProviderReleaseVersion              string
		ProviderReleaseDigest, ProviderDeclarationDigest             string
		Endpoint, NonSecretSettings, SecretBindings                  string
	}
	err := r.database.GetScan(ctx, &row, `
SELECT channel.channel_no,channel.provider_id,channel.provider_plugin_id,
       channel.provider_runtime_type,channel.provider_contract_version,
       channel.provider_release_version,channel.provider_release_digest,
       channel.provider_declaration_digest,revision.endpoint,
       revision.non_secret_settings::TEXT AS non_secret_settings,
       revision.secret_bindings::TEXT AS secret_bindings
FROM plugin_idc_resource_channel channel
JOIN plugin_idc_resource_interface_configuration_revision revision
  ON revision.tenant_id=channel.tenant_id AND revision.channel_no=channel.channel_no
WHERE channel.tenant_id=? AND channel.channel_no=? AND revision.revision_no=?
  AND channel.status_code='enabled' AND channel.deleted_at IS NULL
LIMIT 1`, idccap.DefaultTenantID, channelRef, revisionNo)
	if err != nil || row.ChannelNo == "" {
		return Configuration{}, gerror.New("resource channel revision is unavailable")
	}
	var refs map[string]string
	if json.Unmarshal([]byte(row.SecretBindings), &refs) != nil || len(refs) > maximumChannelSecretBindings {
		return Configuration{}, gerror.New("resource channel revision bindings are invalid")
	}
	bindings := make([]resourcecap.SecretBinding, 0, len(refs))
	for name, ref := range refs {
		handle, handleErr := secretcap.NewHandle(ref)
		if handleErr != nil {
			return Configuration{}, gerror.New("resource channel revision binding is invalid")
		}
		bindings = append(bindings, resourcecap.SecretBinding{Name: name, Handle: handle})
	}
	config := resourcecap.ProviderConfig{ChannelRef: row.ChannelNo, Endpoint: row.Endpoint,
		Settings: json.RawMessage(row.NonSecretSettings), SecretBindings: bindings}
	if config.Validate() != nil {
		return Configuration{}, gerror.New("resource channel revision configuration is invalid")
	}
	return Configuration{PluginID: row.ProviderPluginID, ProviderID: row.ProviderID,
		RuntimeType: row.ProviderRuntimeType, ContractVersion: row.ProviderContractVersion,
		ReleaseVersion: row.ProviderReleaseVersion, ReleaseDigest: strings.TrimSpace(row.ProviderReleaseDigest),
		DeclarationDigest: strings.TrimSpace(row.ProviderDeclarationDigest), Config: config}, nil
}

func validProviderEvidence(channel entity.Channel) bool {
	switch channel.ProviderRuntimeType {
	case "source":
		return channel.ProviderContractVersion == "" && channel.ProviderReleaseVersion == "" &&
			strings.TrimSpace(channel.ProviderReleaseDigest) == "" && strings.TrimSpace(channel.ProviderDeclarationDigest) == ""
	case "dynamic":
		return channel.ProviderContractVersion == resourcecap.DynamicProviderContractVersion &&
			channel.ProviderReleaseVersion != "" && len(strings.TrimSpace(channel.ProviderReleaseDigest)) == 64 &&
			len(strings.TrimSpace(channel.ProviderDeclarationDigest)) == 64
	default:
		return false
	}
}
