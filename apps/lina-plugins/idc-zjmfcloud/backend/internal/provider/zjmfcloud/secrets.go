// This file confines dynamic operation secrets to synchronous accessor and owner-encryption callbacks.

package zjmfcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	maximumFormSecretBytes  = 16 << 10
	maximumSecretFieldBytes = 64 << 10
	consoleTicketLifetime   = 2 * time.Minute

	resetPasswordBinding  = "password"
	rescuePasswordBinding = "temporary_password"
	reinstallOutputName   = "generatedCredential"
	consoleOutputName     = "consoleTicket"
)

type confidentialFormPair struct {
	name  string
	value []byte
}

type confidentialJSONField struct {
	name       string
	upstream   string
	required   bool
	numeric    bool
	connection bool
}

func encodeConfidentialForm(pairs ...confidentialFormPair) ([]byte, error) {
	if len(pairs) == 0 || len(pairs) > 8 {
		return nil, errInvalidConfig
	}
	body := make([]byte, 0, 256)
	for index, pair := range pairs {
		if !validFormName(pair.name) || !validConfidentialInput(pair.value) {
			clear(body)
			return nil, errInvalidConfig
		}
		if index > 0 {
			body = append(body, '&')
		}
		body = append(body, pair.name...)
		body = append(body, '=')
		body = appendFormEscaped(body, pair.value)
	}
	return body, nil
}

func appendConfidentialForm(
	base []byte,
	pairs ...confidentialFormPair,
) ([]byte, error) {
	confidential, err := encodeConfidentialForm(pairs...)
	if err != nil {
		return nil, err
	}
	defer clear(confidential)
	result := make([]byte, 0, len(base)+len(confidential)+1)
	result = append(result, base...)
	if len(result) > 0 {
		result = append(result, '&')
	}
	result = append(result, confidential...)
	return result, nil
}

func validFormName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := range value {
		character := value[index]
		if (character < 'a' || character > 'z') && character != '_' {
			return false
		}
	}
	return true
}

func validConfidentialInput(value []byte) bool {
	if len(value) == 0 || len(value) > maximumFormSecretBytes || !utf8.Valid(value) {
		return false
	}
	for len(value) > 0 {
		character, size := utf8.DecodeRune(value)
		if unicode.IsControl(character) {
			return false
		}
		value = value[size:]
	}
	return true
}

func appendFormEscaped(destination []byte, value []byte) []byte {
	const hexadecimal = "0123456789ABCDEF"
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '-', character == '_', character == '.', character == '~':
			destination = append(destination, character)
		case character == ' ':
			destination = append(destination, '+')
		default:
			destination = append(destination, '%', hexadecimal[character>>4], hexadecimal[character&15])
		}
	}
	return destination
}

func exactOperationSecret(
	bindings []resourcecap.SecretBinding,
	name string,
) (secretcap.Handle, error) {
	if len(bindings) != 1 || bindings[0].Name != name || bindings[0].Handle.IsZero() {
		return secretcap.Handle{}, errInvalidConfig
	}
	return bindings[0].Handle, nil
}

func (p *Provider) accessOperationSecret(
	ctx context.Context,
	handle secretcap.Handle,
	consume secretcap.Consumer,
) error {
	purpose, err := secretcap.NewPurpose("provider.operation-secret")
	if err != nil {
		return errInvalidConfig
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return errInvalidConfig
	}
	return p.env.Secrets.Access(ctx, request, consume)
}

func (p *Provider) accessPrincipalSecret(
	ctx context.Context,
	handle secretcap.Handle,
	consume secretcap.Consumer,
) error {
	purpose, err := secretcap.NewPurpose("provider.principal-profile")
	if err != nil {
		return errInvalidConfig
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return errInvalidConfig
	}
	return p.env.Secrets.Access(ctx, request, consume)
}

func (p *Provider) accessPrincipalBindings(
	ctx context.Context,
	bindings map[string]secretcap.Handle,
	consume func(map[string][]byte) error,
) error {
	values := make(map[string][]byte, len(bindings))
	var access func(int) error
	access = func(index int) error {
		if index == len(principalBindingOrder) {
			return consume(values)
		}
		name := principalBindingOrder[index]
		handle, exists := bindings[name]
		if !exists {
			return access(index + 1)
		}
		return p.accessPrincipalSecret(ctx, handle, func(value []byte) error {
			values[name] = value
			defer delete(values, name)
			return access(index + 1)
		})
	}
	return access(0)
}

func buildReinstallCredential(object rawObject) ([]byte, error) {
	return buildConfidentialJSONObject(object, []confidentialJSONField{
		{name: "username", upstream: "user", required: true},
		{name: "password", upstream: "password", required: true},
		{name: "port", upstream: "port", numeric: true},
	})
}

func (p *Provider) buildConsoleTicket(object rawObject) ([]byte, error) {
	for _, key := range []string{"vnc_url_https", "vnc_url_http", "vnc_url"} {
		if raw, exists := object[key]; exists && !p.validConsoleConnection(raw) {
			return nil, errInvalidResponse
		}
	}
	return buildConfidentialJSONObject(object, []confidentialJSONField{
		{name: "vnc_url_https", upstream: "vnc_url_https", connection: true},
		{name: "vnc_url_http", upstream: "vnc_url_http", connection: true},
		{name: "vnc_url", upstream: "vnc_url", connection: true},
		{name: "token", upstream: "token"},
		{name: "vnc_password", upstream: "vnc_pass"},
	})
}

func (p *Provider) validConsoleConnection(raw json.RawMessage) bool {
	if len(raw) == 0 || len(raw) > maximumSecretFieldBytes {
		return false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil || value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "wss") ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || parsed.Fragment != "" ||
		!p.consoleHostAllowed(parsed.Hostname()) {
		return false
	}
	if port := parsed.Port(); port != "" {
		parsedPort, err := strconv.ParseUint(port, 10, 16)
		if err != nil || parsedPort == 0 {
			return false
		}
	}
	return true
}

func buildConfidentialJSONObject(
	object rawObject,
	fields []confidentialJSONField,
) ([]byte, error) {
	result := make([]byte, 0, 512)
	result = append(result, '{')
	fieldCount := 0
	connectionCount := 0
	requiresConnection := false
	for _, field := range fields {
		requiresConnection = requiresConnection || field.connection
		raw, exists := object[field.upstream]
		if !exists {
			if field.required {
				clear(result)
				return nil, errInvalidResponse
			}
			continue
		}
		value := bytes.TrimSpace(raw)
		if !validConfidentialJSONScalar(value, field.numeric) {
			clear(result)
			return nil, errInvalidResponse
		}
		if fieldCount > 0 {
			result = append(result, ',')
		}
		result = append(result, '"')
		result = append(result, field.name...)
		result = append(result, '"', ':')
		result = append(result, value...)
		fieldCount++
		if field.connection {
			connectionCount++
		}
	}
	result = append(result, '}')
	if fieldCount == 0 || (requiresConnection && connectionCount == 0) ||
		len(result) > resourcecap.MaximumSecretPlaintextBytes {
		clear(result)
		return nil, errInvalidResponse
	}
	return result, nil
}

func validConfidentialJSONScalar(value []byte, numeric bool) bool {
	if len(value) == 0 || len(value) > maximumSecretFieldBytes || !json.Valid(value) {
		return false
	}
	if numeric {
		return validNumericReference(string(value))
	}
	return value[0] == '"' && value[len(value)-1] == '"' && !bytes.Equal(value, []byte(`""`))
}

func (p *Provider) storeSecretOutput(
	ctx context.Context,
	request resourcecap.OperationRequest,
	name string,
	policy resourcecap.SecretOutputPolicy,
	expiresAt time.Time,
	plaintext []byte,
) (resourcecap.SecretOutput, error) {
	defer clear(plaintext)
	handle, err := resourcecap.StoreSecret(
		ctx,
		p.env.SecretSink,
		resourcecap.SecretStoreRequest{
			RequestID:  request.RequestID,
			Capability: request.Capability,
			Name:       name,
			Policy:     policy,
			ExpiresAt:  expiresAt,
		},
		func(consume resourcecap.SecretConsumer) error {
			return consume(plaintext)
		},
	)
	if err != nil {
		return resourcecap.SecretOutput{}, errInvalidResponse
	}
	return resourcecap.SecretOutput{
		Name:      name,
		Policy:    policy,
		Handle:    handle,
		ExpiresAt: expiresAt,
	}, nil
}
