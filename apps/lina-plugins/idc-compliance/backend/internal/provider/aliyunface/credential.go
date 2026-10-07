// This file owns the atomic AccessKey credential-envelope encoding shared by
// the configuration service and the provider client.

package aliyunface

import (
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	minimumAccessKeyIDBytes     = 8
	maximumAccessKeyIDBytes     = 256
	minimumAccessKeySecretBytes = 16
	maximumAccessKeySecretBytes = 512
)

// ValidateCredential checks the fixed bounded printable-ASCII RAM credential
// shape without copying, serializing, hashing, logging, or retaining it.
func ValidateCredential(accessKeyID []byte, accessKeySecret []byte) error {
	if !validCredentialPart(accessKeyID, minimumAccessKeyIDBytes, maximumAccessKeyIDBytes) ||
		!validCredentialPart(accessKeySecret, minimumAccessKeySecretBytes, maximumAccessKeySecretBytes) {
		return gerror.New("Aliyun AccessKey credential is invalid")
	}
	return nil
}

// NewCredentialProducer validates and detaches one complete AccessKey pair.
// The returned producer is consumed synchronously by secretcap and clears its
// encoded envelope after the callback returns.
func NewCredentialProducer(
	accessKeyID []byte,
	accessKeySecret []byte,
) (secretcap.PlaintextProducer, error) {
	if err := ValidateCredential(accessKeyID, accessKeySecret); err != nil {
		return nil, err
	}
	idCopy := append([]byte(nil), accessKeyID...)
	secretCopy := append([]byte(nil), accessKeySecret...)
	return func(consume secretcap.Consumer) error {
		defer clearBytes(idCopy)
		defer clearBytes(secretCopy)
		if consume == nil {
			return gerror.New("Aliyun AccessKey credential consumer is required")
		}
		envelope, err := json.Marshal(credentialEnvelope{
			AccessKeyID: idCopy, AccessKeySecret: secretCopy,
		})
		if err != nil {
			return gerror.Wrap(err, "encode Aliyun AccessKey credential envelope")
		}
		defer clearBytes(envelope)
		return consume(envelope)
	}, nil
}

func validCredentialPart(value []byte, minimum int, maximum int) bool {
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character <= ' ' || character > '~' {
			return false
		}
	}
	return true
}
