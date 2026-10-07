// This file strictly parses one OpenSSH authorized-key line and rejects private material.

package sshkey

import (
	"bytes"
	"strings"

	"golang.org/x/crypto/ssh"

	"lina-core/pkg/bizerr"
)

const maximumAuthorizedKeyBytes = 16 * 1024

var supportedAlgorithms = map[string]struct{}{
	ssh.KeyAlgoRSA:        {},
	ssh.KeyAlgoED25519:    {},
	ssh.KeyAlgoECDSA256:   {},
	ssh.KeyAlgoECDSA384:   {},
	ssh.KeyAlgoECDSA521:   {},
	ssh.KeyAlgoSKECDSA256: {},
	ssh.KeyAlgoSKED25519:  {},
}

type parsedPublicKey struct {
	algorithm, canonical, fingerprint string
}

func parsePublicKey(value string) (parsedPublicKey, error) {
	trimmed := strings.TrimSpace(value)
	upper := strings.ToUpper(trimmed)
	if strings.Contains(upper, "PRIVATE KEY") || strings.HasPrefix(upper, "PUTTY-USER-KEY-FILE-") {
		return parsedPublicKey{}, bizerr.NewCode(CodePrivateKeyRejected)
	}
	if len(trimmed) < 32 || len(trimmed) > maximumAuthorizedKeyBytes || strings.ContainsAny(trimmed, "\r\n") {
		return parsedPublicKey{}, bizerr.NewCode(CodeInvalid)
	}
	key, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(trimmed))
	if err != nil || key == nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return parsedPublicKey{}, bizerr.NewCode(CodeInvalid)
	}
	if _, certificate := key.(*ssh.Certificate); certificate {
		return parsedPublicKey{}, bizerr.NewCode(CodeInvalid)
	}
	if _, supported := supportedAlgorithms[key.Type()]; !supported {
		return parsedPublicKey{}, bizerr.NewCode(CodeInvalid)
	}
	canonical := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	return parsedPublicKey{algorithm: key.Type(), canonical: canonical, fingerprint: ssh.FingerprintSHA256(key)}, nil
}
