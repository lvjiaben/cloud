// Package interfacerevision appends immutable channel configuration snapshots.
package interfacerevision

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
)

type bindingRow struct {
	InterfaceInstanceNo, DisplayName, PluginID, PluginReleaseVersion    string
	ContractVersion, ReleaseDigest, DeclarationDigest, NewBusinessState string
	AuthorizedMethods                                                   []string `orm:"authorized_methods"`
	BindingVersion                                                      int64    `orm:"binding_version"`
	Endpoint, NonSecretSettings                                         string
}

// Append records the exact current endpoint, public settings and active secret
// handles, advances the current pointer, and emits the secret-free core projection.
func Append(ctx context.Context, tx gdb.TX, channelNo string, actor int) (string, error) {
	if ctx == nil || ctx.Err() != nil || tx == nil || resourcecap.ValidateChannelRef(channelNo) != nil || actor <= 0 {
		return "", gerror.New("resource interface revision append is invalid")
	}
	var row bindingRow
	if err := tx.GetScan(&row, `
SELECT binding.interface_instance_no,binding.display_name,
       binding.product_plugin_id AS plugin_id,
       binding.product_plugin_release_version AS plugin_release_version,
       binding.product_contract_version AS contract_version,
       binding.product_authorized_methods AS authorized_methods,
       binding.product_release_digest AS release_digest,
       binding.product_declaration_digest AS declaration_digest,
       binding.new_business_state,binding.binding_version,
       channel.endpoint,channel.non_secret_settings::TEXT AS non_secret_settings
FROM plugin_idc_resource_interface_channel_binding binding
JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id=binding.tenant_id AND channel.channel_no=binding.channel_no
WHERE binding.tenant_id=? AND binding.channel_no=? FOR UPDATE`, idccap.DefaultTenantID, channelNo); err != nil || row.InterfaceInstanceNo == "" {
		return "", gerror.Wrap(err, "load resource interface revision source")
	}
	var secretRows []struct{ BindingName, SecretRef string }
	if err := tx.GetScan(&secretRows, `SELECT binding_name,secret_ref
FROM plugin_idc_resource_channel_secret_binding
WHERE tenant_id=? AND channel_no=? AND status_code='active'
ORDER BY binding_name LIMIT ?`, idccap.DefaultTenantID, channelNo, resourcecap.MaximumSecretBindingCount+1); err != nil || len(secretRows) > resourcecap.MaximumSecretBindingCount {
		return "", gerror.Wrap(err, "load resource interface revision secrets")
	}
	encoded, err := encodeSecretBindings(secretRows)
	if err != nil {
		return "", err
	}
	revisionNo, err := configurationRevisionNo()
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(`INSERT INTO plugin_idc_resource_interface_configuration_revision
    (tenant_id,revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by)
VALUES (?,?,?,?,?,CAST(? AS jsonb),CAST(? AS jsonb),?)`, idccap.DefaultTenantID, revisionNo,
		row.InterfaceInstanceNo, channelNo, row.Endpoint, row.NonSecretSettings, string(encoded), actor); err != nil {
		return "", err
	}
	if _, err = tx.Exec(`UPDATE plugin_idc_resource_interface_channel_binding
SET current_configuration_revision=?,binding_version=binding_version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND channel_no=?`, revisionNo, idccap.DefaultTenantID, channelNo); err != nil {
		return "", err
	}
	if row.PluginID == "" {
		return revisionNo, nil
	}
	projection := resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo: row.InterfaceInstanceNo, DisplayName: row.DisplayName, ActorUserID: actor,
		OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: channelNo,
		PluginID: row.PluginID, PluginReleaseVersion: row.PluginReleaseVersion,
		ContractVersion: row.ContractVersion, AuthorizedMethods: row.AuthorizedMethods,
		ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest,
		NewBusinessState: row.NewBusinessState, InterfaceConfigurationRevision: revisionNo,
	}
	sequenceValue, err := tx.GetValue(`SELECT COALESCE(MAX(projection_sequence),0)+1
FROM plugin_idc_resource_interface_projection_outbox WHERE tenant_id=? AND interface_instance_no=?`,
		idccap.DefaultTenantID, row.InterfaceInstanceNo)
	if err != nil {
		return "", err
	}
	if err = interfaceprojection.Enqueue(ctx, tx, projection, sequenceValue.Int64()); err != nil {
		return "", err
	}
	return revisionNo, nil
}

func encodeSecretBindings(rows []struct{ BindingName, SecretRef string }) ([]byte, error) {
	bindings := make(map[string]string, len(rows))
	for _, item := range rows {
		bindings[item.BindingName] = item.SecretRef
	}
	return json.Marshal(bindings)
}

func configurationRevisionNo() (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", err
	}
	return "rci_" + hex.EncodeToString(randomBytes[:]), nil
}
