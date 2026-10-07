// This file encodes, verifies, and clears the encrypted legal-identity snapshot payload.

package personalverification

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	snapshotVersion        byte = 2
	snapshotChallengeBytes int  = 32
)

func newSnapshotChallenge(random io.Reader) ([]byte, error) {
	if random == nil {
		return nil, gerror.New("personal identity snapshot random source is required")
	}
	challenge := make([]byte, snapshotChallengeBytes)
	if _, err := io.ReadFull(random, challenge); err != nil {
		clearBytes(challenge)
		return nil, gerror.Wrap(err, "read personal identity snapshot challenge")
	}
	return challenge, nil
}

func newSnapshotProducer(challenge, name, identity []byte) (secretcap.PlaintextProducer, error) {
	if len(challenge) != snapshotChallengeBytes || len(name) == 0 || len(name) > 128 ||
		len(identity) == 0 || len(identity) > 128 {
		return nil, gerror.New("personal identity snapshot is invalid")
	}
	payload := encodeSnapshot(challenge, name, identity)
	if len(payload) == 0 {
		return nil, gerror.New("personal identity snapshot is invalid")
	}
	return func(consume secretcap.Consumer) error {
		defer clearBytes(payload)
		if consume == nil {
			return gerror.New("personal identity snapshot consumer is required")
		}
		return consume(payload)
	}, nil
}

func snapshotDigest(challenge []byte) string {
	if len(challenge) != snapshotChallengeBytes {
		return ""
	}
	digest := sha256.Sum256(challenge)
	return hex.EncodeToString(digest[:])
}

func encodeSnapshot(challenge, name, identity []byte) []byte {
	if len(challenge) != snapshotChallengeBytes || len(name) == 0 || len(name) > 128 ||
		len(identity) == 0 || len(identity) > 128 {
		return nil
	}
	payload := make([]byte, 5+snapshotChallengeBytes+len(name)+len(identity))
	payload[0] = snapshotVersion
	copy(payload[1:1+snapshotChallengeBytes], challenge)
	nameOffset := 1 + snapshotChallengeBytes
	binary.BigEndian.PutUint16(payload[nameOffset:nameOffset+2], uint16(len(name)))
	copy(payload[nameOffset+2:nameOffset+2+len(name)], name)
	offset := nameOffset + 2 + len(name)
	binary.BigEndian.PutUint16(payload[offset:offset+2], uint16(len(identity)))
	copy(payload[offset+2:], identity)
	return payload
}

func decodeSnapshot(payload []byte) ([]byte, []byte, []byte, error) {
	if len(payload) < 5+snapshotChallengeBytes || payload[0] != snapshotVersion {
		return nil, nil, nil, gerror.New("personal identity snapshot is invalid")
	}
	challenge := payload[1 : 1+snapshotChallengeBytes]
	nameOffset := 1 + snapshotChallengeBytes
	nameLength := int(binary.BigEndian.Uint16(payload[nameOffset : nameOffset+2]))
	if nameLength <= 0 || nameLength > 128 || nameOffset+2+nameLength+2 > len(payload) {
		return nil, nil, nil, gerror.New("personal identity snapshot is invalid")
	}
	nameStart := nameOffset + 2
	offset := nameStart + nameLength
	identityLength := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
	if identityLength <= 0 || identityLength > 128 || offset+2+identityLength != len(payload) {
		return nil, nil, nil, gerror.New("personal identity snapshot is invalid")
	}
	return challenge, payload[nameStart:offset], payload[offset+2:], nil
}
