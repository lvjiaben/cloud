package resourcecap

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const hostInterfaceSettingsKey = "__host"

var authenticationPathPattern = regexp.MustCompile(`^/[A-Za-z0-9._~!$&'()*+,;=:@%/-]{1,511}$`)

// ProviderAuthenticationProfile is a frozen, host-compiled authentication
// policy. Administrators never submit these fields and WASM guests never see
// them.
type ProviderAuthenticationProfile struct {
	Strategy             string            `json:"strategy"`
	Fields               map[string]string `json:"fields,omitempty"`
	LoginPath            string            `json:"loginPath,omitempty"`
	TokenJSONPath        []string          `json:"tokenJSONPath,omitempty"`
	TokenHeader          string            `json:"tokenHeader,omitempty"`
	TokenScheme          string            `json:"tokenScheme,omitempty"`
	RefreshJSONPath      []string          `json:"refreshJSONPath,omitempty"`
	RefreshJSONValues    []string          `json:"refreshJSONValues,omitempty"`
	SessionTTLSeconds    int               `json:"sessionTTLSeconds,omitempty"`
	SerializeInvocations bool              `json:"serializeInvocations,omitempty"`
	CallbackRequired     bool              `json:"callbackRequired,omitempty"`
}

type providerAuthenticationProfiles struct {
	SelectorField string                                   `json:"selectorField,omitempty"`
	Profiles      map[string]ProviderAuthenticationProfile `json:"profiles"`
}

// hostAuthenticationFieldMapping stores field roles as values rather than
// JSON keys. This keeps the frozen host policy compatible with the generic
// secret-free settings scanner without weakening that scanner for arbitrary
// provider configuration.
type hostAuthenticationFieldMapping struct {
	Role string `json:"role"`
	Slot string `json:"slot"`
}

type hostAuthenticationPolicy struct {
	Mode         string                           `json:"mode"`
	Mappings     []hostAuthenticationFieldMapping `json:"mappings,omitempty"`
	Path         string                           `json:"path,omitempty"`
	ReadPath     []string                         `json:"readPath,omitempty"`
	Header       string                           `json:"header,omitempty"`
	Scheme       string                           `json:"scheme,omitempty"`
	RefreshPath  []string                         `json:"refreshPath,omitempty"`
	RefreshValue []string                         `json:"refreshValue,omitempty"`
	TTLSeconds   int                              `json:"ttlSeconds,omitempty"`
	Serialized   bool                             `json:"serialized,omitempty"`
	Callback     bool                             `json:"callback,omitempty"`
}

type hostInterfaceSettings struct {
	Policy                      hostAuthenticationPolicy `json:"policy"`
	APIPathPrefix               string                   `json:"apiPathPrefix,omitempty"`
	InterfaceSettingsProjection []string                 `json:"interfaceSettingsProjection,omitempty"`
}

func compileHostAuthenticationPolicy(profile ProviderAuthenticationProfile) hostAuthenticationPolicy {
	roles := make([]string, 0, len(profile.Fields))
	for role := range profile.Fields {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	mappings := make([]hostAuthenticationFieldMapping, 0, len(roles))
	for _, role := range roles {
		mappings = append(mappings, hostAuthenticationFieldMapping{Role: role, Slot: profile.Fields[role]})
	}
	return hostAuthenticationPolicy{
		Mode: profile.Strategy, Mappings: mappings, Path: profile.LoginPath,
		ReadPath: append([]string(nil), profile.TokenJSONPath...), Header: profile.TokenHeader, Scheme: profile.TokenScheme,
		RefreshPath: append([]string(nil), profile.RefreshJSONPath...), RefreshValue: append([]string(nil), profile.RefreshJSONValues...),
		TTLSeconds: profile.SessionTTLSeconds, Serialized: profile.SerializeInvocations, Callback: profile.CallbackRequired,
	}
}

func (policy hostAuthenticationPolicy) authenticationProfile() (ProviderAuthenticationProfile, error) {
	fields := make(map[string]string, len(policy.Mappings))
	for _, mapping := range policy.Mappings {
		if mapping.Role == "" || mapping.Slot == "" {
			return ProviderAuthenticationProfile{}, gerror.New("resource Provider host authentication policy is invalid")
		}
		if _, exists := fields[mapping.Role]; exists {
			return ProviderAuthenticationProfile{}, gerror.New("resource Provider host authentication policy is invalid")
		}
		fields[mapping.Role] = mapping.Slot
	}
	return ProviderAuthenticationProfile{
		Strategy: policy.Mode, Fields: fields, LoginPath: policy.Path,
		TokenJSONPath: append([]string(nil), policy.ReadPath...), TokenHeader: policy.Header, TokenScheme: policy.Scheme,
		RefreshJSONPath: append([]string(nil), policy.RefreshPath...), RefreshJSONValues: append([]string(nil), policy.RefreshValue...),
		SessionTTLSeconds: policy.TTLSeconds, SerializeInvocations: policy.Serialized, CallbackRequired: policy.Callback,
	}, nil
}

// ValidateProviderMetadata validates the owner-defined metadata against the
// same frozen interface schema used by the administration form.
func ValidateProviderMetadata(metadata *capmodel.BusinessExtensionProviderMetadata, schema *capmodel.ConfigurationSchemaDeclaration) error {
	if metadata == nil || metadata.Type != DynamicProviderMetadataType || metadata.Version != DynamicProviderContractVersion || metadata.Values == nil {
		return gerror.New("resource dynamic Provider metadata is invalid")
	}
	allowedKeys := map[string]struct{}{"providerId": {}, "authenticationProfiles": {}, "interfaceSettingsProjection": {}, "apiPathPrefix": {}}
	for key := range metadata.Values {
		if _, ok := allowedKeys[key]; !ok {
			return gerror.New("resource dynamic Provider metadata is invalid")
		}
	}
	if _, err := declaredAPIPathPrefix(metadata); err != nil {
		return err
	}
	profiles, projection, err := decodeProviderProfiles(metadata)
	if err != nil {
		return err
	}
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	if schema != nil {
		for _, field := range schema.Fields {
			if field != nil {
				fields[field.Key] = field
			}
		}
	}
	for _, key := range projection {
		field := fields[key]
		if field == nil || field.Type == capmodel.ConfigurationFieldSecret {
			return gerror.New("resource Provider interface settings projection is invalid")
		}
	}
	if profiles.SelectorField != "" {
		selector := fields[profiles.SelectorField]
		if selector == nil || selector.Type != capmodel.ConfigurationFieldSingleSelect || len(selector.StaticCandidates) == 0 {
			return gerror.New("resource Provider authentication selector is invalid")
		}
		values := make(map[string]struct{}, len(selector.StaticCandidates))
		for _, candidate := range selector.StaticCandidates {
			if candidate != nil {
				values[candidate.Value] = struct{}{}
			}
		}
		for value := range profiles.Profiles {
			if _, ok := values[value]; !ok {
				return gerror.New("resource Provider authentication selector value is invalid")
			}
		}
		if len(values) != len(profiles.Profiles) {
			return gerror.New("resource Provider authentication profiles do not cover every selector value")
		}
	} else if len(profiles.Profiles) != 1 || profiles.Profiles["default"].Strategy == "" {
		return gerror.New("resource Provider default authentication profile is invalid")
	}
	for _, profile := range profiles.Profiles {
		if err = validateAuthenticationProfile(profile, fields); err != nil {
			return err
		}
	}
	return nil
}

func decodeProviderProfiles(metadata *capmodel.BusinessExtensionProviderMetadata) (providerAuthenticationProfiles, []string, error) {
	var profiles providerAuthenticationProfiles
	var projection []string
	if metadata == nil || metadata.Values == nil {
		return profiles, nil, gerror.New("resource Provider authentication profiles are invalid")
	}
	encoded, err := json.Marshal(metadata.Values["authenticationProfiles"])
	if err != nil || string(encoded) == "null" || json.Unmarshal(encoded, &profiles) != nil || len(profiles.Profiles) == 0 || len(profiles.Profiles) > 16 {
		return profiles, nil, gerror.New("resource Provider authentication profiles are invalid")
	}
	if raw, exists := metadata.Values["interfaceSettingsProjection"]; exists {
		encoded, err = json.Marshal(raw)
		if err != nil || json.Unmarshal(encoded, &projection) != nil || len(projection) > 32 {
			return profiles, nil, gerror.New("resource Provider interface settings projection is invalid")
		}
	}
	sort.Strings(projection)
	for index, key := range projection {
		if validateSafeName("interface setting projection", key, maximumNameBytes) != nil || (index > 0 && projection[index-1] == key) {
			return profiles, nil, gerror.New("resource Provider interface settings projection is invalid")
		}
	}
	return profiles, projection, nil
}

func validateAuthenticationProfile(profile ProviderAuthenticationProfile, fields map[string]*capmodel.ConfigurationFieldDeclaration) error {
	if len(profile.Fields) == 0 || len(profile.Fields) > 8 {
		return gerror.New("resource Provider authentication fields are invalid")
	}
	if err := validateAuthenticationFields(profile, fields); err != nil {
		return err
	}
	if profile.Strategy != "login_session" {
		return validateRequestAuthenticationProfile(profile)
	}
	return validateLoginAuthenticationProfile(profile)
}

func validateAuthenticationFields(profile ProviderAuthenticationProfile, fields map[string]*capmodel.ConfigurationFieldDeclaration) error {
	allowedTargets := map[string]map[string]struct{}{
		"login_session":       {"username": {}, "password": {}},
		"per_request_form":    {"username": {}, "password": {}},
		"per_request_basic":   {"username": {}, "password": {}},
		"per_request_headers": {"access-user": {}, "access-token": {}},
		"bt_panel_double_md5": {"material": {}},
	}
	targets, ok := allowedTargets[profile.Strategy]
	if !ok {
		return gerror.New("resource Provider authentication strategy is invalid")
	}
	for target, slot := range profile.Fields {
		field := fields[slot]
		if _, ok = targets[target]; !ok || field == nil || field.Type != capmodel.ConfigurationFieldSecret {
			return gerror.New("resource Provider authentication field mapping is invalid")
		}
	}
	if profile.Strategy == "per_request_basic" && (len(profile.Fields) != 2 || profile.Fields["username"] == "" || profile.Fields["password"] == "") {
		return gerror.New("resource Provider Basic authentication requires both credential bindings")
	}
	if profile.Strategy == "bt_panel_double_md5" && (len(profile.Fields) != 1 || profile.Fields["material"] == "") {
		return gerror.New("resource Provider BT Panel authentication field mapping is invalid")
	}
	return nil
}

func validateRequestAuthenticationProfile(profile ProviderAuthenticationProfile) error {
	if profile.LoginPath != "" || len(profile.TokenJSONPath) != 0 || profile.TokenHeader != "" || profile.TokenScheme != "" ||
		len(profile.RefreshJSONPath) != 0 || len(profile.RefreshJSONValues) != 0 || profile.SessionTTLSeconds != 0 {
		return gerror.New("resource Provider request authentication profile is invalid")
	}
	return nil
}

func validateLoginAuthenticationProfile(profile ProviderAuthenticationProfile) error {
	if !authenticationPathPattern.MatchString(profile.LoginPath) || strings.Contains(profile.LoginPath, "..") ||
		len(profile.TokenJSONPath) == 0 || len(profile.TokenJSONPath) > 4 ||
		(profile.TokenHeader != "Authorization" && profile.TokenHeader != "Access-Token" && profile.TokenHeader != "X-Auth-Token") ||
		(profile.TokenScheme != "" && profile.TokenScheme != "Bearer") || profile.SessionTTLSeconds < 60 || profile.SessionTTLSeconds > 3600 ||
		(len(profile.RefreshJSONPath) == 0) != (len(profile.RefreshJSONValues) == 0) || len(profile.RefreshJSONPath) > 4 || len(profile.RefreshJSONValues) > 8 {
		return gerror.New("resource Provider login authentication profile is invalid")
	}
	for _, path := range append(append([]string(nil), profile.TokenJSONPath...), profile.RefreshJSONPath...) {
		if path != "$" && validateSafeName("authentication JSON path", path, 64) != nil {
			return gerror.New("resource Provider authentication JSON path is invalid")
		}
	}
	for _, value := range profile.RefreshJSONValues {
		if value == "" || len(value) > 64 || strings.ContainsAny(value, "\r\n\x00") {
			return gerror.New("resource Provider authentication refresh value is invalid")
		}
	}
	return nil
}

// CompileInterfaceSettings combines administrator-visible values with the
// exact profile frozen in the selected provider release.
func CompileInterfaceSettings(payload json.RawMessage, metadata *capmodel.BusinessExtensionProviderMetadata) (json.RawMessage, error) {
	var values map[string]json.RawMessage
	if json.Unmarshal(payload, &values) != nil || values == nil {
		return nil, gerror.New("resource Provider interface settings are invalid")
	}
	if _, exists := values[hostInterfaceSettingsKey]; exists {
		return nil, gerror.New("resource Provider host settings cannot be supplied by an administrator")
	}
	for key := range values {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		if strings.HasPrefix(normalized, "auth") || normalized == "serializeinvocations" || normalized == "callbackrequired" {
			return nil, gerror.New("resource Provider host settings cannot be supplied by an administrator")
		}
	}
	profiles, projection, err := decodeProviderProfiles(metadata)
	if err != nil {
		return nil, err
	}
	profileKey := "default"
	if profiles.SelectorField != "" {
		if json.Unmarshal(values[profiles.SelectorField], &profileKey) != nil || profileKey == "" {
			return nil, gerror.New("resource Provider authentication selector is missing")
		}
	}
	profile, ok := profiles.Profiles[profileKey]
	if !ok {
		return nil, gerror.New("resource Provider authentication profile is unavailable")
	}
	prefix, err := declaredAPIPathPrefix(metadata)
	if err != nil {
		return nil, err
	}
	compiled, err := json.Marshal(hostInterfaceSettings{Policy: compileHostAuthenticationPolicy(profile), InterfaceSettingsProjection: projection, APIPathPrefix: prefix})
	if err != nil {
		return nil, err
	}
	values[hostInterfaceSettingsKey] = compiled
	return json.Marshal(values)
}

// ProjectPublicInterfaceSettings removes all host-only policy before returning
// settings to an administrator or an isolated provider guest.
func ProjectPublicInterfaceSettings(payload json.RawMessage, guest bool) (json.RawMessage, error) {
	var values map[string]json.RawMessage
	if json.Unmarshal(payload, &values) != nil || values == nil {
		return nil, gerror.New("resource Provider interface settings are invalid")
	}
	var host hostInterfaceSettings
	if raw := values[hostInterfaceSettingsKey]; len(raw) != 0 && json.Unmarshal(raw, &host) != nil {
		return nil, gerror.New("resource Provider host settings are invalid")
	}
	delete(values, hostInterfaceSettingsKey)
	if guest {
		projected := make(map[string]json.RawMessage, len(host.InterfaceSettingsProjection))
		for _, key := range host.InterfaceSettingsProjection {
			if value, ok := values[key]; ok {
				projected[key] = value
			}
		}
		values = projected
	}
	return json.Marshal(values)
}

// DecodeHostAuthenticationSettings returns only owner-compiled policy.
func DecodeHostAuthenticationSettings(payload json.RawMessage) (ProviderAuthenticationProfile, error) {
	var values map[string]json.RawMessage
	var host hostInterfaceSettings
	if json.Unmarshal(payload, &values) != nil || json.Unmarshal(values[hostInterfaceSettingsKey], &host) != nil {
		return ProviderAuthenticationProfile{}, gerror.New("resource Provider host authentication settings are invalid")
	}
	return host.Policy.authenticationProfile()
}

// declaredAPIPathPrefix accepts one connector-owned API version segment only.
func declaredAPIPathPrefix(metadata *capmodel.BusinessExtensionProviderMetadata) (string, error) {
	raw, exists := metadata.Values["apiPathPrefix"]
	if !exists {
		return "", nil
	}
	prefix, ok := raw.(string)
	if !ok || len(prefix) > 16 || !providerAPIPrefixPattern.MatchString(prefix) {
		return "", gerror.New("resource Provider API path prefix is invalid")
	}
	return prefix, nil
}

var providerAPIPrefixPattern = regexp.MustCompile(`^/v[0-9]+$`)

// ResolveInterfaceAPIEndpoint adds the connector-owned API prefix to the
// administrator's root address. The prefix remains in host-only compiled policy,
// never in guest configuration or user-entered settings; an existing matching
// version is preserved and an incompatible explicit version is rejected.
func ResolveInterfaceAPIEndpoint(endpoint string, payload json.RawMessage) (string, error) {
	if len(payload) == 0 {
		return endpoint, nil
	}
	var values map[string]json.RawMessage
	var host hostInterfaceSettings
	if json.Unmarshal(payload, &values) != nil || (len(values[hostInterfaceSettingsKey]) > 0 && json.Unmarshal(values[hostInterfaceSettingsKey], &host) != nil) {
		return "", gerror.New("resource Provider host settings are invalid")
	}
	if host.APIPathPrefix == "" {
		return endpoint, nil
	}
	if len(host.APIPathPrefix) > 16 || !providerAPIPrefixPattern.MatchString(host.APIPathPrefix) {
		return "", gerror.New("resource Provider API path prefix is invalid")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", gerror.Wrap(err, "parse resource Provider root address")
	}
	base := strings.TrimRight(parsed.Path, "/")
	last := base[strings.LastIndex(base, "/")+1:]
	if providerAPIPrefixPattern.MatchString("/"+last) && "/"+last != host.APIPathPrefix {
		return "", gerror.New("resource Provider API version is incompatible")
	}
	if !strings.HasSuffix(base, host.APIPathPrefix) {
		base += host.APIPathPrefix
	}
	parsed.Path = base
	return parsed.String(), nil
}
