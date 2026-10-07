// This file validates finite configuration, keys, persistent watermarks, and
// delegates bounded public JSON inspection to its dedicated internal package.

package versioncache

import (
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/internal/service/versioncache/internal/publicjson"
)

const (
	defaultMaximumStale         = time.Minute
	defaultLockLease            = 10 * time.Second
	defaultAllowedClockSkew     = 30 * time.Second
	defaultMaximumEntries       = 1024
	defaultMaximumLocalBytes    = 32 * 1024 * 1024
	defaultMaximumSnapshotBytes = 512 * 1024
	defaultMaximumJSONDepth     = 24
	defaultMaximumJSONNodes     = 20000

	absoluteMaximumStale         = 15 * time.Minute
	absoluteMaximumEntries       = 10000
	absoluteMaximumLocalBytes    = 128 * 1024 * 1024
	absoluteMaximumSnapshotBytes = 768 * 1024
	absoluteMaximumJSONDepth     = 64
	absoluteMaximumJSONNodes     = 100000
	minimumLockLease             = time.Second
	maximumLockLease             = 5 * time.Minute
	maximumKeyIDBytes            = 160
	maximumSourceVersionBytes    = 128
)

func normalizeConfig(config Config) (Config, error) {
	if config.MaximumStale < 0 || config.LockLease < 0 || config.AllowedClockSkew < 0 ||
		config.MaximumEntries < 0 || config.MaximumLocalBytes < 0 ||
		config.MaximumSnapshotBytes < 0 || config.MaximumJSONDepth < 0 ||
		config.MaximumJSONNodes < 0 {
		return Config{}, bizerr.NewCode(CodeInvalid)
	}
	if config.MaximumStale == 0 {
		config.MaximumStale = defaultMaximumStale
	}
	if config.LockLease == 0 {
		config.LockLease = defaultLockLease
	}
	if config.AllowedClockSkew == 0 {
		config.AllowedClockSkew = defaultAllowedClockSkew
	}
	if config.MaximumEntries == 0 {
		config.MaximumEntries = defaultMaximumEntries
	}
	if config.MaximumLocalBytes == 0 {
		config.MaximumLocalBytes = defaultMaximumLocalBytes
	}
	if config.MaximumSnapshotBytes == 0 {
		config.MaximumSnapshotBytes = defaultMaximumSnapshotBytes
	}
	if config.MaximumJSONDepth == 0 {
		config.MaximumJSONDepth = defaultMaximumJSONDepth
	}
	if config.MaximumJSONNodes == 0 {
		config.MaximumJSONNodes = defaultMaximumJSONNodes
	}
	if config.MaximumStale > absoluteMaximumStale ||
		config.LockLease < minimumLockLease || config.LockLease > maximumLockLease ||
		config.AllowedClockSkew > 2*time.Minute ||
		config.MaximumEntries > absoluteMaximumEntries ||
		config.MaximumLocalBytes > absoluteMaximumLocalBytes ||
		config.MaximumSnapshotBytes > absoluteMaximumSnapshotBytes ||
		config.MaximumSnapshotBytes > config.MaximumLocalBytes ||
		config.MaximumJSONDepth > absoluteMaximumJSONDepth ||
		config.MaximumJSONNodes > absoluteMaximumJSONNodes {
		return Config{}, bizerr.NewCode(CodeInvalid)
	}
	return config, nil
}

func normalizeKey(key Key) (Key, error) {
	if !validKind(key.Kind) || key.ID == "" || len(key.ID) > maximumKeyIDBytes ||
		key.ID != strings.TrimSpace(key.ID) || !utf8.ValidString(key.ID) {
		return Key{}, bizerr.NewCode(CodeInvalid)
	}
	for _, character := range key.ID {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' ||
			character == '_' || character == ':' || character == '.' || character == '/' {
			continue
		}
		return Key{}, bizerr.NewCode(CodeInvalid)
	}
	if publicjson.ContainsCredentialText(key.ID) {
		return Key{}, bizerr.NewCode(CodeInvalid)
	}
	return key, nil
}

func validKind(kind Kind) bool {
	switch kind {
	case KindPublishedProduct, KindPublishedTemplate, KindRebuildableAggregate:
		return true
	default:
		return false
	}
}

func validSourceVersion(version string) bool {
	if version == "" || len(version) > maximumSourceVersionBytes || version != strings.TrimSpace(version) {
		return false
	}
	for _, character := range []byte(version) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' ||
			character == ':' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func (s *serviceImpl) normalizeSnapshot(snapshot Snapshot) (Snapshot, error) {
	if !validSourceVersion(snapshot.Version) {
		return Snapshot{}, bizerr.NewCode(CodeInvalid)
	}
	canonical, err := publicjson.Canonicalize(snapshot.Document, publicjson.Limits{
		MaximumBytes: s.config.MaximumSnapshotBytes,
		MaximumDepth: s.config.MaximumJSONDepth,
		MaximumNodes: s.config.MaximumJSONNodes,
	})
	if err != nil {
		return Snapshot{}, bizerr.NewCode(CodeInvalid)
	}
	return Snapshot{Version: snapshot.Version, Document: canonical}, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
