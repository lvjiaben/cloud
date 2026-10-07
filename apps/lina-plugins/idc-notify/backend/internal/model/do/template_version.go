// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TemplateVersion is the golang structure of table plugin_idc_notify_template_version for DAO operations like Where/Data.
type TemplateVersion struct {
	g.Meta                    `orm:"table:plugin_idc_notify_template_version, do:true"`
	TenantId                  any        //
	TemplateNo                any        //
	Version                   any        //
	EventCode                 any        //
	ChannelCode               any        //
	LocaleCode                any        //
	SubjectTemplate           any        //
	BodyTemplate              any        //
	ProviderTemplateId        any        //
	ProviderSmsVariableOrder  any        //
	ProviderWechatDataMapping any        //
	StatusCode                any        //
	ContentDigest             any        //
	CreatedBy                 any        //
	CreatedAt                 *time.Time //
	ProviderCode              any        //
	ProviderSmsDataMapping    any        //
}
