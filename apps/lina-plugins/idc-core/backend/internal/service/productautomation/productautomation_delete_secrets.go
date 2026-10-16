// This file revokes private credentials while deleting a never-used interface.
// The owner-scoped manager shares the database transaction, so an audit or
// deletion failure rolls back credential revocation together with the interface.

package productautomation

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

// revokeUnusedInterfaceSecrets includes previous revisions: deleting an unused
// interface must not leave any of its superseded credentials active.
func (s *serviceImpl) revokeUnusedInterfaceSecrets(ctx context.Context, interfaceNo string) error {
	var refs []struct {
		SecretRef string `orm:"secret_ref"`
	}
	if err := s.database.GetScan(ctx, &refs, `SELECT DISTINCT r.secret_ref
 FROM plugin_idc_core_product_automation_interface_secret_reference r
 JOIN plugin_idc_core_product_automation_interface_configuration_revision c ON c.id=r.configuration_revision_id
 WHERE c.tenant_id=? AND c.interface_instance_no=? ORDER BY r.secret_ref`, idccap.DefaultTenantID, interfaceNo); err != nil {
		return gerror.Wrap(err, "load deleted interface credentials")
	}
	for _, ref := range refs {
		handle, err := secretcap.NewHandle(ref.SecretRef)
		if err != nil {
			return err
		}
		if nilDependency(s.secrets) {
			return gerror.New("interface credential manager is unavailable")
		}
		metadata, err := s.secrets.Metadata(ctx, handle)
		if err != nil {
			return err
		}
		if _, err = s.secrets.Revoke(ctx, handle, metadata.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}
