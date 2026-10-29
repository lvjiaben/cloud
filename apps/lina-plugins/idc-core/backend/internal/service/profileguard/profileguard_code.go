// This file defines structured, translatable username and login-contact errors.

package profileguard

import (
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
)

var codeContactVerificationRequired = bizerr.MustDefine("IDC_PROFILE_CONTACT_VERIFICATION_REQUIRED",
	"Login phone and email cannot be changed through personal profile. Contact support for identity verification.", gcode.CodeNotAuthorized)

// codeUsernameInvalid keeps login identifiers separate from contact aliases.
var codeUsernameInvalid = bizerr.MustDefine("IDC_PROFILE_USERNAME_INVALID", "Usernames cannot be numbers, phone numbers or email addresses.", gcode.CodeInvalidParameter)
