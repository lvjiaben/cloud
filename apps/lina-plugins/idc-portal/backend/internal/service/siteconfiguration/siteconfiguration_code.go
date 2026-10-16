// This file defines stable website validation errors.

package siteconfiguration

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

// Configuration failures retain stable portable error identities.
var (
	codeNavigationInvalid = bizerr.MustDefine("WebsiteConfigurationNavigationInvalid", "Check the navigation labels, pages, links and parent items.", gcode.CodeInvalidParameter)
	codeFailure           = bizerr.MustDefine("WebsiteConfigurationFailure", "Website settings are unavailable. Try again.", gcode.CodeInternalError)
	codeUnavailable       = bizerr.MustDefine("WebsiteConfigurationUnavailable", "Install and enable a website plugin first.", gcode.CodeInvalidParameter)
	codeNameRequired      = bizerr.MustDefine("WebsiteConfigurationNameRequired", "Enter the website name.", gcode.CodeInvalidParameter)
	codeTooLong           = bizerr.MustDefine("WebsiteConfigurationTooLong", "Website content is too long.", gcode.CodeInvalidParameter)
	codeImageInvalid      = bizerr.MustDefine("WebsiteConfigurationImageInvalid", "Enter a valid image URL.", gcode.CodeInvalidParameter)
	codeSchemaInvalid     = bizerr.MustDefine("WebsiteConfigurationSchemaInvalid", "Website plugin settings are incomplete.", gcode.CodeInvalidParameter)
	codeRequired          = bizerr.MustDefine("WebsiteConfigurationRequired", "Fill in the required settings.", gcode.CodeInvalidParameter)
	codeTextInvalid       = bizerr.MustDefine("WebsiteConfigurationTextInvalid", "Website content is invalid or too long.", gcode.CodeInvalidParameter)
	codeOptionInvalid     = bizerr.MustDefine("WebsiteConfigurationOptionInvalid", "Website option is invalid.", gcode.CodeInvalidParameter)
	codeIntegerInvalid    = bizerr.MustDefine("WebsiteConfigurationIntegerInvalid", "Enter a whole number.", gcode.CodeInvalidParameter)
	codeColorInvalid      = bizerr.MustDefine("WebsiteConfigurationColorInvalid", "Use a color such as #1677ff.", gcode.CodeInvalidParameter)
	codeFieldsChanged     = bizerr.MustDefine("WebsiteConfigurationFieldsChanged", "Website settings changed. Refresh and try again.", gcode.CodeInvalidParameter)
)
