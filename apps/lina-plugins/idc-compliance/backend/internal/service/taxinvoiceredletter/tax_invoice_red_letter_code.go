// This file defines stable business errors for the red-letter workflow.

package taxinvoiceredletter

import (
	"errors"
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
)

var errInconsistent = errors.New("red-letter persistence is inconsistent")

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_COMPLIANCE_RED_LETTER_INVALID", "The red-letter request is invalid", gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_COMPLIANCE_RED_LETTER_NOT_ACCESSIBLE", "The red-letter task is not accessible", gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_COMPLIANCE_RED_LETTER_CONFLICT", "The red-letter task conflicts with its current state", gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_COMPLIANCE_RED_LETTER_UNAVAILABLE", "The red-letter service is temporarily unavailable", gcode.CodeInternalError,
	)
)
