package customeroperations

import (
	"context"
	"encoding/json"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

func restoreAction(action serveroperationresourcecap.Action) bool {
	return action == serveroperationresourcecap.ActionSnapshotRestore || action == serveroperationresourcecap.ActionBackupRestore
}

func accessoryObjectRequest(owned *route, action serveroperationresourcecap.Action, parameters json.RawMessage) (servercommercialcap.ObjectRequest, error) {
	var input struct {
		ObjectNo           string `json:"objectNo"`
		ConfirmedServiceNo string `json:"confirmedServiceNo"`
	}
	if json.Unmarshal(parameters, &input) != nil || input.ObjectNo == "" || (restoreAction(action) && input.ConfirmedServiceNo != owned.ServerNo) {
		return servercommercialcap.ObjectRequest{}, ErrInvalid
	}
	kind := serveroperationresourcecap.AccessoryKindSnapshot
	if action == serveroperationresourcecap.ActionBackupDelete || action == serveroperationresourcecap.ActionBackupRestore {
		kind = serveroperationresourcecap.AccessoryKindBackup
	}
	return servercommercialcap.ObjectRequest{EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo, ObjectNo: input.ObjectNo, Kind: kind}, nil
}

func (s *Service) recordAccessoryDeletion(ctx context.Context, owned *route, action serveroperationresourcecap.Action, parameters json.RawMessage, operationNo, taskNo string) error {
	if action != serveroperationresourcecap.ActionSnapshotDelete && action != serveroperationresourcecap.ActionBackupDelete {
		return nil
	}
	object, err := accessoryObjectRequest(owned, action, parameters)
	if err != nil {
		return err
	}
	_, err = s.database.Model(dao.NativeAccessoryReceipt.Table()).Safe().Ctx(ctx).Data(do.NativeAccessoryReceipt{TaskNo: taskNo, TenantId: 0, OperationNo: operationNo, EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo, ObjectNo: object.ObjectNo, AccessoryKind: string(object.Kind)}).Insert()
	return err
}

type queuedRestoreAccess interface {
	CheckQueuedRestore(context.Context, servercommercialcap.ObjectRequest) error
}
