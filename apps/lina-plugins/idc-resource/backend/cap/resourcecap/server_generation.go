// This file defines the resource owner's secret-free server generation rules.
// Connectors validate the same contract before publication that fulfillment uses.

package resourcecap

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
)

var errServerGenerationPolicy = gerror.New("server generation policy is invalid")

// ServerGenerationPolicy is a secret-free snapshot of plugin-declared rules.
type ServerGenerationPolicy struct {
	HostnamePrefix     string   `json:"hostname_prefix"`
	HostnameLength     *int     `json:"hostname_length"`
	HostnameCharacters []string `json:"hostname_characters"`
	PasswordLength     *int     `json:"password_length"`
	PasswordCharacters []string `json:"password_characters"`
}

const (
	generationLowercase = "lowercase"
	generationUppercase = "uppercase"
	generationDigits    = "digits"
	generationSymbols   = "symbols"
)

// ParseServerGenerationPolicy validates optional frozen plugin rules and applies
// generation defaults; it never reads or returns a secret.
func ParseServerGenerationPolicy(payload json.RawMessage) (ServerGenerationPolicy, error) {
	var policy ServerGenerationPolicy
	if decodeGenerationPolicy(payload, &policy) != nil {
		return policy, errServerGenerationPolicy
	}
	if policy.HostnameLength == nil && (policy.HostnamePrefix != "" || len(policy.HostnameCharacters) != 0) {
		length := 12
		policy.HostnameLength = &length
	}
	if policy.PasswordLength == nil && len(policy.PasswordCharacters) != 0 {
		length := 12
		policy.PasswordLength = &length
	}
	if policy.HostnameLength != nil {
		if *policy.HostnameLength < 4 || *policy.HostnameLength > 63 || len(policy.HostnamePrefix) > 10 || len(policy.HostnamePrefix) >= *policy.HostnameLength {
			return policy, errServerGenerationPolicy
		}
		for _, character := range policy.HostnamePrefix {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') {
				return policy, errServerGenerationPolicy
			}
		}
		if len(policy.HostnameCharacters) == 0 {
			policy.HostnameCharacters = []string{generationLowercase, generationDigits}
		}
		if _, err := generationAlphabets(policy.HostnameCharacters, false); err != nil {
			return policy, err
		}
	}
	if policy.PasswordLength != nil {
		if *policy.PasswordLength < 6 || *policy.PasswordLength > 20 {
			return policy, errServerGenerationPolicy
		}
		if len(policy.PasswordCharacters) == 0 {
			policy.PasswordCharacters = []string{generationLowercase, generationUppercase, generationDigits}
		}
		if _, err := generationAlphabets(policy.PasswordCharacters, true); err != nil {
			return policy, err
		}
	}
	return policy, nil
}

func generationAlphabets(classes []string, password bool) ([]string, error) {
	if len(classes) == 0 || len(classes) > 4 {
		return nil, errServerGenerationPolicy
	}
	seen := make(map[string]bool, len(classes))
	result := make([]string, 0, len(classes))
	for _, class := range classes {
		if seen[class] {
			return nil, errServerGenerationPolicy
		}
		seen[class] = true
		switch class {
		case generationLowercase:
			result = append(result, "abcdefghijklmnopqrstuvwxyz")
		case generationUppercase:
			if !password {
				return nil, errServerGenerationPolicy
			}
			result = append(result, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		case generationDigits:
			result = append(result, "0123456789")
		case generationSymbols:
			if !password {
				return nil, errServerGenerationPolicy
			}
			result = append(result, "!@#$%*-_=+?")
		default:
			return nil, errServerGenerationPolicy
		}
	}
	return result, nil
}

// HostnameAlphabets returns the validated nonsecret naming alphabets.
func (p ServerGenerationPolicy) HostnameAlphabets() ([]string, error) {
	return generationAlphabets(p.HostnameCharacters, false)
}

// PasswordAlphabets returns allowed character classes, never a password.
func (p ServerGenerationPolicy) PasswordAlphabets() ([]string, error) {
	return generationAlphabets(p.PasswordCharacters, true)
}

func decodeGenerationPolicy(payload json.RawMessage, target *ServerGenerationPolicy) error {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || payload[0] != '{' {
		return errServerGenerationPolicy
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(target); err != nil {
		return errServerGenerationPolicy
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errServerGenerationPolicy
	}
	return nil
}
