// Package resourceinterfaceprojection persists secret-free resource channel
// references as product-automation interfaces.
package resourceinterfaceprojection

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"slices"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/lib/pq"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

type serviceImpl struct{ database gdb.DB }

// usageReader owns the local database and the startup-injected staff authority.
type usageReader struct {
	database   gdb.DB
	authorizer managementcap.Authorizer
}

var _ resourceinterfaceprojectioncap.UsageReader = (*usageReader)(nil)

// NewUsageReader constructs a read-only projection with explicit dependencies.
func NewUsageReader(database gdb.DB, authorizer managementcap.Authorizer) (resourceinterfaceprojectioncap.UsageReader, error) {
	if database == nil || authorizer == nil {
		return nil, gerror.New("resource interface usage dependencies are unavailable")
	}
	return &usageReader{database: database, authorizer: authorizer}, nil
}

var _ resourceinterfaceprojectioncap.Sink = (*serviceImpl)(nil)

// New constructs the core-owned secret-free projection sink.
func New(database gdb.DB) (resourceinterfaceprojectioncap.Sink, error) {
	if database == nil {
		return nil, gerror.New("resource interface projection database is unavailable")
	}
	return &serviceImpl{database: database}, nil
}

func (service *serviceImpl) Upsert(ctx context.Context, projection resourceinterfaceprojectioncap.Projection) error {
	if service == nil || service.database == nil || ctx == nil || ctx.Err() != nil ||
		resourceinterfaceprojectioncap.ValidateProjection(projection) != nil {
		return gerror.New("resource interface projection is unavailable")
	}
	methods := append([]string(nil), projection.AuthorizedMethods...)
	slices.Sort(methods)
	methodsValue, err := postgresTextArray(methods)
	if err != nil {
		return gerror.Wrap(err, "encode resource interface projection methods")
	}
	return service.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var current *struct {
			DisplayName, OwnerID, ChannelRef, PluginID, ReleaseVersion, ContractVersion string
			ReleaseDigest, DeclarationDigest, State                                     string
			AuthorizedMethods                                                           []string
			Version                                                                     int64
		}
		if err := service.database.GetScan(txCtx, &current, `SELECT display_name,resource_owner_id AS owner_id,resource_channel_ref AS channel_ref,plugin_id,plugin_release_version AS release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state AS state,version FROM plugin_idc_core_product_automation_interface_instance WHERE tenant_id=0 AND interface_instance_no=? LIMIT 1 FOR UPDATE`, projection.InterfaceInstanceNo); err != nil {
			return err
		}
		if current == nil {
			if _, err := service.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_automation_interface_instance(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state,version,created_by,changed_by,interface_origin_code,resource_owner_id,resource_channel_ref) VALUES(0,?,? ,?,'product-fulfillment',?,?,CAST(? AS TEXT[]),?,?,?,1,?,?,'resource',?,?)`, projection.InterfaceInstanceNo, projection.DisplayName, projection.PluginID, projection.PluginReleaseVersion, projection.ContractVersion, methodsValue, projection.ReleaseDigest, projection.DeclarationDigest, projection.NewBusinessState, projection.ActorUserID, projection.ActorUserID, projection.OwnerID, projection.ResourceChannelRef); err != nil {
				return err
			}
			declaration := []byte(`{"version":"resource-owner-v1"}`)
			values := []byte(`{}`)
			if _, err := service.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by,resource_owner_configuration_revision) VALUES(0,?,1,'resource-owner-v1',CAST(? AS JSONB),CAST(? AS JSONB),?,?,0,?,?)`, projection.InterfaceInstanceNo, string(declaration), string(values), digest(declaration), digest(values), projection.ActorUserID, projection.InterfaceConfigurationRevision); err != nil {
				return err
			}
			return nil
		}
		if current.OwnerID != projection.OwnerID || current.ChannelRef != projection.ResourceChannelRef ||
			current.PluginID != projection.PluginID || current.ReleaseVersion != projection.PluginReleaseVersion ||
			current.ContractVersion != projection.ContractVersion || current.ReleaseDigest != projection.ReleaseDigest ||
			current.DeclarationDigest != projection.DeclarationDigest || !slices.Equal(current.AuthorizedMethods, methods) {
			return gerror.New("resource interface projection conflicts with immutable evidence")
		}
		var configuration *struct {
			Revision                           int64
			ResourceOwnerConfigurationRevision string
		}
		if err := service.database.GetScan(txCtx, &configuration, `SELECT revision,resource_owner_configuration_revision FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE tenant_id=0 AND interface_instance_no=? ORDER BY revision DESC LIMIT 1 FOR UPDATE`, projection.InterfaceInstanceNo); err != nil || configuration == nil {
			if err != nil {
				return err
			}
			return gerror.New("resource interface configuration projection is unavailable")
		}
		if configuration.ResourceOwnerConfigurationRevision != projection.InterfaceConfigurationRevision {
			declaration := []byte(`{"version":"resource-owner-v1"}`)
			values := []byte(`{}`)
			if _, err := service.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by,resource_owner_configuration_revision) VALUES(0,?,?,'resource-owner-v1',CAST(? AS JSONB),CAST(? AS JSONB),?,?,0,?,?)`, projection.InterfaceInstanceNo, configuration.Revision+1, string(declaration), string(values), digest(declaration), digest(values), projection.ActorUserID, projection.InterfaceConfigurationRevision); err != nil {
				return err
			}
		}
		version := current.Version
		if current.DisplayName != projection.DisplayName {
			result, err := service.database.Exec(txCtx, `UPDATE plugin_idc_core_product_automation_interface_instance SET display_name=?,version=version+1,changed_by=? WHERE tenant_id=0 AND interface_instance_no=? AND version=?`, projection.DisplayName, projection.ActorUserID, projection.InterfaceInstanceNo, version)
			if err != nil || affectedRows(result) != 1 {
				return gerror.New("resource interface projection display name conflicted")
			}
			version++
		}
		if current.State != projection.NewBusinessState {
			result, err := service.database.Exec(txCtx, `UPDATE plugin_idc_core_product_automation_interface_instance SET new_business_state=?,version=version+1,changed_by=? WHERE tenant_id=0 AND interface_instance_no=? AND version=?`, projection.NewBusinessState, projection.ActorUserID, projection.InterfaceInstanceNo, version)
			if err != nil || affectedRows(result) != 1 {
				return gerror.New("resource interface projection state conflicted")
			}
		}
		return nil
	})
}

func postgresTextArray(values []string) (string, error) {
	value, err := pq.Array(values).Value()
	if err != nil {
		return "", err
	}
	encoded, ok := value.(string)
	if !ok {
		return "", gerror.New("postgres text array encoding is unavailable")
	}
	return encoded, nil
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func affectedRows(result sql.Result) int64 {
	if result == nil {
		return 0
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0
	}
	return rows
}
