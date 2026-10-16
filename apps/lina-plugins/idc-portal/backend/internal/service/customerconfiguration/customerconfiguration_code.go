// This file defines stable website validation errors.

package customerconfiguration

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// Configuration failures retain stable portable error identities.
var (
	codeFailure        = bizerr.MustDefine("CustomerConfigurationFailure", "Customer settings are unavailable. Try again.", gcode.CodeInternalError)
	codeUnavailable    = bizerr.MustDefine("CustomerConfigurationUnavailable", "Install and enable a customer template plugin first.", gcode.CodeInvalidParameter)
	codeNameRequired   = bizerr.MustDefine("CustomerConfigurationNameRequired", "Enter the website name.", gcode.CodeInvalidParameter)
	codeTooLong        = bizerr.MustDefine("CustomerConfigurationTooLong", "Customer content is too long.", gcode.CodeInvalidParameter)
	codeImageInvalid   = bizerr.MustDefine("CustomerConfigurationImageInvalid", "Enter a valid image URL.", gcode.CodeInvalidParameter)
	codeSchemaInvalid  = bizerr.MustDefine("CustomerConfigurationSchemaInvalid", "Website plugin settings are incomplete.", gcode.CodeInvalidParameter)
	codeRequired       = bizerr.MustDefine("CustomerConfigurationRequired", "Fill in the required settings.", gcode.CodeInvalidParameter)
	codeTextInvalid    = bizerr.MustDefine("CustomerConfigurationTextInvalid", "Customer content is invalid or too long.", gcode.CodeInvalidParameter)
	codeOptionInvalid  = bizerr.MustDefine("CustomerConfigurationOptionInvalid", "Customer option is invalid.", gcode.CodeInvalidParameter)
	codeIntegerInvalid = bizerr.MustDefine("CustomerConfigurationIntegerInvalid", "Enter a whole number.", gcode.CodeInvalidParameter)
	codeColorInvalid   = bizerr.MustDefine("CustomerConfigurationColorInvalid", "Use a color such as #1677ff.", gcode.CodeInvalidParameter)
	codeFieldsChanged  = bizerr.MustDefine("CustomerConfigurationFieldsChanged", "Customer settings changed. Refresh and try again.", gcode.CodeInvalidParameter)
)
