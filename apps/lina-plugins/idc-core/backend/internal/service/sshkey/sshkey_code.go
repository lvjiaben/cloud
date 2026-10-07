// This file defines stable localized business errors for SSH public-key operations.

package sshkey

import (
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var (
	CodeInvalid             = bizerr.MustDefine("IDC_SSH_PUBLIC_KEY_INVALID", "The SSH public key request is invalid", gcode.CodeInvalidParameter)
	CodePrivateKeyRejected  = bizerr.MustDefine("IDC_SSH_PRIVATE_KEY_REJECTED", "Private keys are not accepted", gcode.CodeInvalidParameter)
	CodeNotAccessible       = bizerr.MustDefine("IDC_SSH_PUBLIC_KEY_NOT_ACCESSIBLE", "The SSH public key is not accessible", gcode.CodeNotAuthorized)
	CodeDuplicate           = bizerr.MustDefine("IDC_SSH_PUBLIC_KEY_DUPLICATE", "An active SSH public key with the same name or fingerprint already exists", gcode.CodeInvalidOperation)
	CodeVersionConflict     = bizerr.MustDefine("IDC_SSH_PUBLIC_KEY_VERSION_CONFLICT", "The SSH public key has changed", gcode.CodeInvalidOperation)
	CodeIdempotencyConflict = bizerr.MustDefine("IDC_SSH_PUBLIC_KEY_IDEMPOTENCY_CONFLICT", "The SSH public key request conflicts with an earlier request", gcode.CodeInvalidOperation)
	CodeUnavailable         = bizerr.MustDefine("IDC_SSH_PUBLIC_KEY_UNAVAILABLE", "The SSH public key service is temporarily unavailable", gcode.CodeInternalError)
)
