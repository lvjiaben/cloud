package serveroperationsource

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"io"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
)

func (s *Service) translateAccessoryParameters(ctx context.Context, serviceNo string,
	action serveroperationresourcecap.Action, parameters json.RawMessage) (json.RawMessage, error) {
	if !accessoryAction(action) {
		return append(json.RawMessage(nil), parameters...), nil
	}
	directories, err := accessorydirectory.New(s.database)
	if err != nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	directory, err := directories.Read(ctx, serviceNo)
	if err != nil || directory == nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionBackupCreate,
		serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore,
		serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore:
		return translateSnapshotOrBackup(ctx, s.database, directories, directory, serviceNo, action, parameters)
	case serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach:
		return translateSecurityGroup(ctx, directories, directory, serviceNo, action, parameters)
	case serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete:
		return translateNAT(ctx, directories, directory, serviceNo, action, parameters)
	case serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount:
		return translateVirtualMedia(ctx, directories, directory, serviceNo, action, parameters)
	case serveroperationresourcecap.ActionTrafficAdd:
		return translateTraffic(directory, parameters)
	default:
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
}

func translateSnapshotOrBackup(ctx context.Context, database gdb.DB, directories accessorydirectory.Service,
	directory *accessorydirectory.Directory, serviceNo string, action serveroperationresourcecap.Action,
	parameters json.RawMessage) (json.RawMessage, error) {
	if action == serveroperationresourcecap.ActionSnapshotCreate || action == serveroperationresourcecap.ActionBackupCreate {
		var input struct {
			DiskObjectNo string `json:"diskObjectNo"`
			Name         string `json:"name"`
		}
		if strictDecode(parameters, &input) != nil {
			return nil, serveroperationresourcecap.ErrTaskConflict
		}
		resolved, err := resolveDirectoryObject(ctx, directories, directory, serviceNo,
			input.DiskObjectNo, resourcecap.AccessoryKindDisk, resourcecap.AccessoryStateAttached)
		if err != nil {
			return nil, err
		}
		return marshalProviderParameters(map[string]any{"action": "create", "disk_ref": resolved.ExternalRef, "name": input.Name})
	}
	var input struct {
		ObjectNo           string `json:"objectNo"`
		ConfirmedServiceNo string `json:"confirmedServiceNo"`
	}
	if strictDecode(parameters, &input) != nil {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	isRestore := action == serveroperationresourcecap.ActionSnapshotRestore ||
		action == serveroperationresourcecap.ActionBackupRestore
	if (isRestore && !confirmedRestoreTarget(ctx, database, serviceNo, input.ConfirmedServiceNo)) || (!isRestore && input.ConfirmedServiceNo != "") {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	kind := resourcecap.AccessoryKindSnapshot
	if action == serveroperationresourcecap.ActionBackupDelete || action == serveroperationresourcecap.ActionBackupRestore {
		kind = resourcecap.AccessoryKindBackup
	}
	// Deletion needs an exact fresh identity and explicit confirmation, not a
	// claim that the object can be restored. Restore still requires readiness.
	requiredState := resourcecap.AccessoryState("")
	if isRestore {
		requiredState = resourcecap.AccessoryStateAvailable
	}
	resolved, err := resolveDirectoryObject(ctx, directories, directory, serviceNo,
		input.ObjectNo, kind, requiredState)
	if err != nil {
		return nil, err
	}
	operation := "delete"
	if isRestore {
		operation = "restore"
	}
	return marshalProviderParameters(map[string]any{"action": operation, "snapshot_ref": resolved.ExternalRef})
}

// Native customers type their public server number; legacy source callers keep
// the local number. Either must resolve to the exact resource-owned server.
func confirmedRestoreTarget(ctx context.Context, database gdb.DB, serviceNo, confirmed string) bool {
	if confirmed == serviceNo {
		return true
	}
	value, err := database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_server_entitlement_projection WHERE tenant_id=0 AND local_resource_no=? AND server_no=?)`, serviceNo, confirmed)
	return err == nil && value.Bool()
}

func translateSecurityGroup(ctx context.Context, directories accessorydirectory.Service,
	directory *accessorydirectory.Directory, serviceNo string, action serveroperationresourcecap.Action,
	parameters json.RawMessage) (json.RawMessage, error) {
	if action == serveroperationresourcecap.ActionSecurityGroupDetach {
		if !emptyParameters(parameters) || countState(directory, resourcecap.AccessoryKindSecurityGroup, resourcecap.AccessoryStateAttached) != 1 {
			return nil, serveroperationresourcecap.ErrTaskConflict
		}
		return json.RawMessage(`{"action":"detach"}`), nil
	}
	var input struct {
		ObjectNo string `json:"objectNo"`
	}
	if strictDecode(parameters, &input) != nil || countState(directory, resourcecap.AccessoryKindSecurityGroup, resourcecap.AccessoryStateAttached) != 0 {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	resolved, err := resolveDirectoryObject(ctx, directories, directory, serviceNo,
		input.ObjectNo, resourcecap.AccessoryKindSecurityGroup, resourcecap.AccessoryStateAvailable)
	if err != nil {
		return nil, err
	}
	return marshalProviderParameters(map[string]any{"action": "attach", "security_group_ref": resolved.ExternalRef})
}

func translateNAT(ctx context.Context, directories accessorydirectory.Service,
	directory *accessorydirectory.Directory, serviceNo string, action serveroperationresourcecap.Action,
	parameters json.RawMessage) (json.RawMessage, error) {
	if action == serveroperationresourcecap.ActionNATCreate {
		var input struct {
			Name         string `json:"name"`
			ExternalPort int64  `json:"externalPort"`
			InternalPort int64  `json:"internalPort"`
			Protocol     string `json:"protocol"`
		}
		if strictDecode(parameters, &input) != nil {
			return nil, serveroperationresourcecap.ErrTaskConflict
		}
		return marshalProviderParameters(map[string]any{"action": "create", "name": input.Name,
			"external_port": input.ExternalPort, "internal_port": input.InternalPort, "protocol": input.Protocol})
	}
	var input struct {
		ObjectNo string `json:"objectNo"`
	}
	if strictDecode(parameters, &input) != nil {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	resolved, err := resolveDirectoryObject(ctx, directories, directory, serviceNo,
		input.ObjectNo, resourcecap.AccessoryKindNAT, resourcecap.AccessoryStateActive)
	if err != nil {
		return nil, err
	}
	return marshalProviderParameters(map[string]any{"action": "delete", "nat_ref": resolved.ExternalRef})
}

func translateVirtualMedia(ctx context.Context, directories accessorydirectory.Service,
	directory *accessorydirectory.Directory, serviceNo string, action serveroperationresourcecap.Action,
	parameters json.RawMessage) (json.RawMessage, error) {
	if action == serveroperationresourcecap.ActionVirtualMediaUnmount {
		if !emptyParameters(parameters) || countState(directory, resourcecap.AccessoryKindVirtualMedia, resourcecap.AccessoryStateMounted) != 1 {
			return nil, serveroperationresourcecap.ErrTaskConflict
		}
		return json.RawMessage(`{"action":"unmount"}`), nil
	}
	var input struct {
		ObjectNo string `json:"objectNo"`
	}
	if strictDecode(parameters, &input) != nil || countState(directory, resourcecap.AccessoryKindVirtualMedia, resourcecap.AccessoryStateMounted) != 0 {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	resolved, err := resolveDirectoryObject(ctx, directories, directory, serviceNo,
		input.ObjectNo, resourcecap.AccessoryKindVirtualMedia, resourcecap.AccessoryStateAvailable)
	if err != nil {
		return nil, err
	}
	return marshalProviderParameters(map[string]any{"action": "mount", "image_ref": resolved.ExternalRef})
}

func translateTraffic(directory *accessorydirectory.Directory, parameters json.RawMessage) (json.RawMessage, error) {
	var input struct {
		TrafficGB int64 `json:"trafficGb"`
	}
	if strictDecode(parameters, &input) != nil || countState(directory, resourcecap.AccessoryKindTraffic, resourcecap.AccessoryStateActive) != 1 {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	return marshalProviderParameters(map[string]any{"action": "add", "traffic_gb": input.TrafficGB})
}

func resolveDirectoryObject(ctx context.Context, directories accessorydirectory.Service,
	directory *accessorydirectory.Directory, serviceNo, objectNo string, kind resourcecap.AccessoryKind,
	state resourcecap.AccessoryState) (*accessorydirectory.ResolvedObject, error) {
	if !validDirectoryReference(objectNo, 64) {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	matches := 0
	for _, item := range directory.Items {
		if item.ObjectNo == objectNo && item.Kind == kind && (state == "" || item.State == state) {
			matches++
		}
	}
	if matches != 1 {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	resolved, err := directories.Resolve(ctx, serviceNo, objectNo, kind)
	if err != nil || resolved == nil || resolved.SnapshotNo != directory.SnapshotNo {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	return resolved, nil
}

func countState(directory *accessorydirectory.Directory, kind resourcecap.AccessoryKind,
	state resourcecap.AccessoryState) int {
	count := 0
	for _, item := range directory.Items {
		if item.Kind == kind && item.State == state {
			count++
		}
	}
	return count
}

func accessoryAction(action serveroperationresourcecap.Action) bool {
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupCreate,
		serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore,
		serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach,
		serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete,
		serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount,
		serveroperationresourcecap.ActionTrafficAdd:
		return true
	default:
		return false
	}
}

func strictDecode(parameters json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(parameters))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return serveroperationresourcecap.ErrTaskConflict
	}
	return nil
}

func emptyParameters(parameters json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(parameters, &value) == nil && len(value) == 0
}

func marshalProviderParameters(value map[string]any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	return encoded, nil
}
