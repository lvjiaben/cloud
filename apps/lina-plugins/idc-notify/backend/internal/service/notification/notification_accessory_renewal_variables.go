// This file localizes accessory renewal period units only at template rendering.
// Frozen event variables retain their original billing units and monetary values.
package notification

import (
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Accessory renewal event identifiers and their trusted business owner.
const (
	accessoryRenewalDueEventCode       = "billing.accessory_renewal_due"
	accessoryRenewalAttentionEventCode = "billing.accessory_renewal_attention"
	accessoryRenewalSourcePlugin       = "idc-core"
)

// accessoryRenewalPeriodUnit is the frozen unit contract accepted for display.
type accessoryRenewalPeriodUnit string

// Supported renewal units mirror the event contract; unknown values stay intact.
const (
	accessoryRenewalPeriodDay   accessoryRenewalPeriodUnit = "day"
	accessoryRenewalPeriodMonth accessoryRenewalPeriodUnit = "month"
	accessoryRenewalPeriodYear  accessoryRenewalPeriodUnit = "year"
)

// renewalPeriodLabels is loaded once during construction and never mutated.
type renewalPeriodLabels map[string]map[accessoryRenewalPeriodUnit]string

func loadRenewalPeriodLabels(resources fs.FS) (renewalPeriodLabels, error) {
	labels := make(renewalPeriodLabels, 2)
	for _, locale := range []string{"en-US", "zh-CN"} {
		path := "manifest/i18n/" + locale + "/renewal-period.json"
		data, err := fs.ReadFile(resources, path)
		if err != nil {
			return nil, gerror.Wrapf(err, "load renewal period resource %s", path)
		}
		var entries map[string]string
		if err = json.Unmarshal(data, &entries); err != nil {
			return nil, gerror.Wrapf(err, "decode renewal period resource %s", path)
		}
		units := make(map[accessoryRenewalPeriodUnit]string, 3)
		for _, unit := range []accessoryRenewalPeriodUnit{accessoryRenewalPeriodDay, accessoryRenewalPeriodMonth, accessoryRenewalPeriodYear} {
			key := "notification.renewal.period." + string(unit)
			value := entries[key]
			if strings.TrimSpace(value) == "" {
				return nil, gerror.Newf("renewal period resource %s is missing %s", path, key)
			}
			units[unit] = value
		}
		labels[locale] = units
	}
	return labels, nil
}

// renewalTemplateVariables clones due-reminder variables before translating the
// display unit; unknown units/locales and all other event contracts stay intact.
func renewalTemplateVariables(eventCode, locale string, variables map[string]string, labels renewalPeriodLabels) (map[string]string, error) {
	if eventCode != accessoryRenewalDueEventCode {
		return variables, nil
	}
	rendered := cloneVariables(variables)
	unit := accessoryRenewalPeriodUnit(rendered["period_unit"])
	if (locale != "zh-CN" && locale != "en-US") || (unit != accessoryRenewalPeriodDay && unit != accessoryRenewalPeriodMonth && unit != accessoryRenewalPeriodYear) {
		return rendered, nil
	}
	label := labels[locale][unit]
	if label == "" {
		return nil, gerror.Newf("renewal period label unavailable for locale %s and unit %s", locale, unit)
	}
	rendered["period_unit"] = label
	return rendered, nil
}
