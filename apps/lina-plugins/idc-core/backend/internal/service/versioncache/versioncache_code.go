// This file defines stable errors for the version-cache boundary.

package versioncache

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	// CodeInvalid reports an unsupported kind, key, configuration, or snapshot.
	CodeInvalid = bizerr.MustDefine(
		"IDC_VERSION_CACHE_INVALID",
		"The cached snapshot request is invalid",
		gcode.CodeInvalidParameter,
	)
	// CodeUnavailable reports failed cache coordination or invalidation.
	CodeUnavailable = bizerr.MustDefine(
		"IDC_VERSION_CACHE_UNAVAILABLE",
		"Snapshot cache coordination is temporarily unavailable",
		gcode.CodeInternalError,
	)
	// CodeAuthorityUnavailable reports failure to obtain a safe bounded snapshot.
	CodeAuthorityUnavailable = bizerr.MustDefine(
		"IDC_VERSION_CACHE_AUTHORITY_UNAVAILABLE",
		"The authoritative snapshot is temporarily unavailable",
		gcode.CodeInternalError,
	)
)
