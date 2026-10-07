// This file authorizes and routes one-time confidential server output without
// retaining plaintext in the account service.

package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type accountOutputRow struct {
	ActionCode     string `orm:"action_code"`
	ChannelNo      string `orm:"channel_no"`
	ProviderTaskNo string `orm:"provider_task_no"`
	OutputBindings string `orm:"output_bindings"`
}

func (s *accountServiceImpl) RedeemCurrentCustomerOutput(ctx context.Context,
	serviceNo, operationNo string, consume serveroperationresourcecap.ConfidentialOutputConsumer) error {
	if s == nil || s.database == nil || s.customers == nil || s.outputs == nil || ctx == nil ||
		ctx.Err() != nil || !validReference(serviceNo, 40) || !validReference(operationNo, 40) || consume == nil {
		return ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return ErrServiceUnavailable
	}
	var row *accountOutputRow
	if err = s.database.GetScan(ctx, &row, `SELECT execution.action_code,execution.channel_no,
COALESCE(execution.provider_task_no,'') AS provider_task_no,execution.output_bindings::TEXT AS output_bindings
FROM plugin_idc_core_business_operation operation
JOIN plugin_idc_core_server_operation_execution execution
 ON execution.tenant_id=operation.tenant_id AND execution.operation_no=operation.operation_no
WHERE operation.tenant_id=0 AND operation.customer_no=? AND operation.service_no=?
 AND operation.operation_no=? AND operation.status_code='succeeded' AND execution.status_code='succeeded'`,
		subject.CustomerNo, serviceNo, operationNo); err != nil {
		return gerror.Wrap(err, "read customer server confidential output")
	}
	if row == nil || row.ProviderTaskNo == "" || (row.ActionCode != "reinstall" && row.ActionCode != "console") {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	var outputs []persistedSecretOutput
	if json.Unmarshal([]byte(row.OutputBindings), &outputs) != nil || len(outputs) != 1 {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	output := outputs[0]
	expectedName, expectedPolicy := "generatedCredential", serveroperationresourcecap.SecretOutputPolicyOneTimeCredential
	if row.ActionCode == "console" {
		expectedName, expectedPolicy = "consoleTicket", serveroperationresourcecap.SecretOutputPolicyShortLivedTicket
	}
	if output.Name != expectedName || output.Policy != string(expectedPolicy) {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	handle, err := secretcap.NewHandle(output.Ref)
	if err != nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	expiresAt := output.ExpiresAt
	if expectedPolicy == serveroperationresourcecap.SecretOutputPolicyOneTimeCredential {
		expiresAt = time.Time{}
	}
	err = s.outputs.RedeemOutput(ctx, serveroperationresourcecap.ConfidentialOutputRequest{
		ProviderTaskNo: row.ProviderTaskNo, ChannelNo: row.ChannelNo, Name: expectedName,
		Policy: expectedPolicy, Handle: handle, ExpiresAt: expiresAt,
	}, func(payload []byte) error {
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_server_operation_execution
SET output_bindings='[]'::jsonb,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND service_no=? AND output_bindings<>'[]'::jsonb`,
			operationNo, serviceNo)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "clear redeemed server operation output marker")
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "read redeemed server operation marker result")
		}
		if affected != 1 {
			return serveroperationresourcecap.ErrOutputConsumed
		}
		return consume(payload)
	})
	return err
}
