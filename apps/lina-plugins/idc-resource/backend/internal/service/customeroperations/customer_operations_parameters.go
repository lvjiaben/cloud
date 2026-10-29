// This file normalizes exact native management inputs. Commercial changes are
// excluded until their entitlement-aware quote and settlement owner admits them.
package customeroperations

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strconv"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

const modeGraceful = "graceful"
const modeHard = "hard"

var nativeActions = []serveroperationresourcecap.Action{
	serveroperationresourcecap.ActionStart, serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot,
	serveroperationresourcecap.ActionResetPassword, serveroperationresourcecap.ActionConsole, serveroperationresourcecap.ActionReinstall,
	serveroperationresourcecap.ActionEnterRescue, serveroperationresourcecap.ActionExitRescue,
	serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach,
	serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete,
	serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount,
	serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore,
	serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore,
}

func normalizeParameters(action serveroperationresourcecap.Action, raw json.RawMessage) (json.RawMessage, error) {
	if !slices.Contains(nativeActions, action) {
		return nil, ErrInvalid
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	values, err := scalarObject(raw)
	if err != nil {
		return nil, err
	}
	allowed := []string{}
	switch action {
	case serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot:
		allowed = []string{"mode"}
		mode, exists := values["mode"]
		if !exists {
			mode = modeGraceful
			values["mode"] = mode
		}
		if mode != modeGraceful && mode != modeHard {
			return nil, ErrInvalid
		}
	case serveroperationresourcecap.ActionEnterRescue:
		allowed = []string{"system"}
		system, ok := values["system"].(string)
		if !ok || (system != "linux" && system != "windows") {
			return nil, ErrInvalid
		}
	case serveroperationresourcecap.ActionReinstall:
		allowed = []string{"os", "port", "format_data_disk", "system_disk_size"}
		if err = normalizeReinstall(values); err != nil {
			return nil, err
		}
	case serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupRestore:
		allowed = []string{"objectNo", "confirmedServiceNo"}
		object, ok := values["objectNo"].(string)
		confirmed, confirmedOK := values["confirmedServiceNo"].(string)
		if !ok || !keyPattern.MatchString(object) || !confirmedOK || !serverPattern.MatchString(confirmed) {
			return nil, ErrInvalid
		}
	case serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionNATDelete, serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionBackupDelete:
		allowed = []string{"objectNo"}
		no, ok := values["objectNo"].(string)
		if !ok || !keyPattern.MatchString(no) {
			return nil, ErrInvalid
		}
	case serveroperationresourcecap.ActionNATCreate:
		allowed = []string{"name", "externalPort", "internalPort", "protocol"}
		if err = normalizeNAT(values); err != nil {
			return nil, err
		}
	}
	for key := range values {
		if !slices.Contains(allowed, key) {
			return nil, ErrInvalid
		}
	}
	normalized, err := json.Marshal(values)
	if err != nil {
		return nil, ErrInvalid
	}
	return normalized, nil
}

// scalarObject rejects duplicate keys, nested input and trailing JSON instead of
// silently normalizing ambiguous user data into a different confirmed command.
func scalarObject(raw []byte) (map[string]any, error) {
	if len(raw) > 16*1024 {
		return nil, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, ErrInvalid
	}
	result := map[string]any{}
	for decoder.More() {
		token, readErr := decoder.Token()
		if readErr != nil {
			return nil, ErrInvalid
		}
		key, ok := token.(string)
		if !ok {
			return nil, ErrInvalid
		}
		if _, exists := result[key]; exists {
			return nil, ErrInvalid
		}
		value, readErr := decoder.Token()
		if readErr != nil {
			return nil, ErrInvalid
		}
		switch value.(type) {
		case string, json.Number, bool:
		default:
			return nil, ErrInvalid
		}
		result[key] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, ErrInvalid
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return result, nil
}

func positiveScalar(value any, maximum int64) (string, bool) {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	default:
		return "", false
	}
	number, err := strconv.ParseInt(text, 10, 64)
	if err != nil || number <= 0 || number > maximum || strconv.FormatInt(number, 10) != text {
		return "", false
	}
	return text, true
}

func highRisk(action serveroperationresourcecap.Action, parameters json.RawMessage) bool {
	if action == serveroperationresourcecap.ActionResetPassword || action == serveroperationresourcecap.ActionEnterRescue || action == serveroperationresourcecap.ActionExitRescue || action == serveroperationresourcecap.ActionReinstall {
		return true
	}
	if action == serveroperationresourcecap.ActionStop || action == serveroperationresourcecap.ActionReboot {
		var value struct {
			Mode string `json:"mode"`
		}
		return json.Unmarshal(parameters, &value) != nil || value.Mode == modeHard
	}
	return false
}

func confirmationDigest(command Command, parameters json.RawMessage) string {
	// Every component is length-delimited by JSON. No password or secret handle
	// enters confirmation storage; secret values use the separate typed input.
	value, err := json.Marshal(struct {
		Version, ServerNo, Action, Key string
		Parameters                     json.RawMessage
	}{"native-server-confirmation/v1", command.ServerNo, command.Action, command.IdempotencyKey, parameters})
	if err != nil {
		return ""
	}
	return parametersDigest(value)
}

// normalizeReinstall preserves omitted advanced options instead of inventing defaults.
func normalizeReinstall(values map[string]any) error {
	os, ok := positiveScalar(values["os"], 1<<53-1)
	if !ok {
		return ErrInvalid
	}
	values["os"] = os
	for _, key := range []string{"port", "system_disk_size"} {
		if value, exists := values[key]; exists {
			maximum := int64(1048576)
			if key == "port" {
				maximum = 65535
			}
			number, valid := positiveScalar(value, maximum)
			if !valid {
				return ErrInvalid
			}
			values[key] = number
		}
	}
	if format, exists := values["format_data_disk"]; exists {
		switch format {
		case json.Number("0"), "0":
			values["format_data_disk"] = 0
		case json.Number("1"), "1":
			values["format_data_disk"] = 1
		default:
			return ErrInvalid
		}
	}

	return nil
}

// normalizeNAT accepts only exact bounded ports and known transport protocols.
func normalizeNAT(values map[string]any) error {
	name, ok := values["name"].(string)
	if !ok || len(name) == 0 || len(name) > 128 {
		return ErrInvalid
	}
	for _, key := range []string{"externalPort", "internalPort"} {
		number, valid := positiveScalar(values[key], 65535)
		if !valid {
			return ErrInvalid
		}
		parsed, parseErr := strconv.ParseInt(number, 10, 64)
		if parseErr != nil {
			return ErrInvalid
		}
		values[key] = parsed
	}
	protocol, ok := values["protocol"].(string)
	if !ok || (protocol != "tcp" && protocol != "udp" && protocol != "tcp_udp") {
		return ErrInvalid
	}
	return nil
}
