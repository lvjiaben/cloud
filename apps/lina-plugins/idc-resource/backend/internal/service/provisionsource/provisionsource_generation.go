// This file validates frozen server naming and credential policies. Generated
// passwords only exist inside the governed synchronous secret producer.

package provisionsource

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"math/big"
	"strings"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// parseServerGenerationPolicy keeps capability errors inside the fulfillment
// owner's existing safe business error contract.
func parseServerGenerationPolicy(payload json.RawMessage) (resourcecap.ServerGenerationPolicy, error) {
	policy, err := resourcecap.ParseServerGenerationPolicy(payload)
	if err != nil {
		return policy, provisionresourcecap.ErrParametersUnavailable
	}
	return policy, nil
}

func generatedServerHostname(request provisionresourcecap.TaskRequest, policy resourcecap.ServerGenerationPolicy) (string, error) {
	if request.ServerLogin != nil && request.ServerLogin.Hostname != "" {
		if !policy.HostnameCustomerInput || request.Fulfillment.Quantity != 1 {
			return "", provisionresourcecap.ErrParametersUnavailable
		}
		return request.ServerLogin.Hostname, nil
	}
	_, previous := frozenServerIdentity(request)
	if policy.HostnameLength == nil {
		return previous, nil
	}
	// Naming is deterministic for retries and independent from secret entropy.
	// Both alphabet selection and prefix have already been validated.
	alphabets, err := policy.HostnameAlphabets()
	if err != nil {
		return "", err
	}
	alphabet := strings.Join(alphabets, "")
	name := []byte(strings.ToLower(policy.HostnamePrefix))
	digest := sha256.Sum256([]byte(previous))
	for len(name) < *policy.HostnameLength {
		for _, value := range digest {
			if len(name) == *policy.HostnameLength {
				break
			}
			name = append(name, alphabet[int(value)%len(alphabet)])
		}
		digest = sha256.Sum256(digest[:])
	}
	return string(name), nil
}

func (s *Service) generatedPasswordProducer(policy resourcecap.ServerGenerationPolicy) secretcap.PlaintextProducer {
	if policy.PasswordLength == nil {
		return s.passwordProducer()
	}
	return func(consume secretcap.Consumer) error {
		alphabets, err := policy.PasswordAlphabets()
		if err != nil {
			return err
		}
		var (
			alphabet = strings.Join(alphabets, "")
			value    = make([]byte, *policy.PasswordLength)
		)
		defer clear(value)
		for index := range value {
			choices := alphabet
			if index < len(alphabets) {
				choices = alphabets[index]
			}
			selected, readErr := randomGenerationIndex(s.random, len(choices))
			if readErr != nil {
				return readErr
			}
			value[index] = choices[selected]
		}
		// Shuffle mandatory character classes so their positions are not fixed.
		for index := len(value) - 1; index > 0; index-- {
			other, readErr := randomGenerationIndex(s.random, index+1)
			if readErr != nil {
				return readErr
			}
			value[index], value[other] = value[other], value[index]
		}
		return consume(value)
	}
}

func randomGenerationIndex(source io.Reader, count int) (int, error) {
	value, err := rand.Int(source, big.NewInt(int64(count)))
	if err != nil {
		return 0, provisionresourcecap.ErrParametersUnavailable
	}
	return int(value.Int64()), nil
}

// createProvisionCredential recovers the same committed secret before generating
// randomness. A preparation retry must not conflict with an earlier credential.
func (s *Service) createProvisionCredential(ctx context.Context, manager secretcap.Manager, request provisionresourcecap.TaskRequest, policy resourcecap.ServerGenerationPolicy, recoveryKeys ...string) (secretcap.CreateResult, error) {
	if choice := request.ServerLogin; choice != nil && choice.PasswordSecretRef != "" && (len(recoveryKeys) == 0 || recoveryKeys[0] == "") {
		if !policy.PasswordCustomerInput || request.Fulfillment.Quantity != 1 {
			return secretcap.CreateResult{}, provisionresourcecap.ErrParametersUnavailable
		}
		handle, err := secretcap.NewHandle(choice.PasswordSecretRef)
		if err != nil {
			return secretcap.CreateResult{}, provisionresourcecap.ErrParametersUnavailable
		}
		metadata, err := manager.Metadata(ctx, handle)
		if err != nil || !metadata.Configured {
			return secretcap.CreateResult{}, provisionresourcecap.ErrSourceUnavailable
		}
		return secretcap.CreateResult{Handle: handle, Metadata: metadata, Reused: true}, nil
	}
	key := provisionSecretIdempotencyKey(request)
	if len(recoveryKeys) == 1 && recoveryKeys[0] != "" {
		key = recoveryKeys[0]
	}
	if recovered, err := manager.RecoverCreate(ctx, key); err == nil && !recovered.Handle.IsZero() {
		if !recovered.Metadata.Configured {
			return secretcap.CreateResult{}, provisionresourcecap.ErrSourceUnavailable
		}
		return recovered, nil
	}
	created, err := manager.Create(ctx, key, s.generatedPasswordProducer(policy))
	if err != nil {
		if code, ok := bizerr.As(err); ok {
			logger.Warningf(ctx, "provision credential create rejected code=%s", code.RuntimeCode())
		}
		created, err = manager.RecoverCreate(ctx, key)
	}
	if err != nil || created.Handle.IsZero() || !created.Metadata.Configured {
		if code, ok := bizerr.As(err); ok {
			logger.Warningf(ctx, "provision credential recovery rejected code=%s", code.RuntimeCode())
		}
		return secretcap.CreateResult{}, provisionresourcecap.ErrSourceUnavailable
	}
	return created, nil
}
