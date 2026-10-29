// This file converts source-proven finance component tariffs into bounded
// portable schedules. Native inclusive quantity bands become the owner's
// (minimum, maximum] bands; unrepresentable precision or fee rules fail closed.
package finance

import (
	"encoding/json"
	"math/big"
	"sort"
	"strconv"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func nativePriceSchedule(base resourceinterfaceprojectioncap.CatalogPrice, definition nativeCatalogDefinition) (*channelcommercecap.ImportPriceSchedule, error) {
	cycle, ok := nativeCycle(base)
	if !ok {
		return nil, errInvalidResponse
	}
	amount, err := nativeMinor(base.Amount, false)
	if err != nil {
		return nil, err
	}
	setup, err := nativeMinor(base.SetupFee, false)
	if err != nil {
		return nil, err
	}
	schedule := &channelcommercecap.ImportPriceSchedule{CurrencyCode: base.CurrencyCode, TermReference: cycle.amount, ChargeType: "recurring", PeriodUnit: string(base.Period), PeriodCount: base.PeriodCount, PricePhase: "both", QuantityMode: "fixed", MinimumPurchaseCycles: 1,
		Base: []channelcommercecap.ImportPriceBand{{AmountMinor: amount, SetupAmountMinor: setup, RenewalAmountMinor: &amount}}, Options: []channelcommercecap.ImportOptionPrice{},
		Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1},
	}
	if base.Period == resourceinterfaceprojectioncap.CatalogPeriodOnce {
		schedule.ChargeType = "one_time"
		schedule.PeriodUnit = "once"
		schedule.PeriodCount = 1
		schedule.PricePhase = "purchase"
		schedule.Base[0].RenewalAmountMinor = nil
	}
	trees, treeErr := nativeTrees(definition)
	if treeErr != nil {
		return nil, treeErr
	}
	treePrices, treeErr := nativeTreePrices(base.CurrencyCode, cycle, trees)
	if treeErr != nil {
		return nil, treeErr
	}
	schedule.Options = append(schedule.Options, treePrices...)
	for _, group := range definition.Groups {
		for _, option := range group.Options {
			if scalar(option.Type) == "20" {
				continue
			}
			kind, err := strconv.Atoi(scalar(option.Type))
			if err != nil {
				return nil, nil
			}
			if nativeQuantityKind(kind) {
				price, err := nativeQuantityPrice(base.CurrencyCode, cycle, option, kind >= 15)
				if err != nil {
					return nil, err
				}
				if price == nil {
					return nil, nil
				}
				schedule.Options = append(schedule.Options, *price)
				continue
			}
			for _, choice := range option.Choices {
				band, err := nativeChoiceBand(base.CurrencyCode, cycle, choice, false)
				if err != nil {
					return nil, err
				}
				if band == nil {
					return nil, nil
				}
				schedule.Options = append(schedule.Options, channelcommercecap.ImportOptionPrice{FieldKey: "cfg_" + scalar(option.ID), Value: scalar(choice.ID), PricingMode: channelcommercecap.ImportConfigurationPriceFixed, Bands: []channelcommercecap.ImportPriceBand{*band}})
			}
		}
	}
	return schedule, nil
}

func nativeCycle(price resourceinterfaceprojectioncap.CatalogPrice) (catalogCycle, bool) {
	for _, cycle := range catalogCycles {
		if cycle.period == price.Period && (cycle.count == price.PeriodCount || cycle.period == resourceinterfaceprojectioncap.CatalogPeriodHour || cycle.period == resourceinterfaceprojectioncap.CatalogPeriodDay) {
			return cycle, true
		}
	}
	return catalogCycle{}, false
}

func nativeQuantityKind(kind int) bool {
	switch kind {
	case 4, 7, 9, 11, 14, 15, 16, 17, 18, 19:
		return true
	default:
		return false
	}
}

func nativeQuantityPrice(currency string, cycle catalogCycle, option nativeCatalogOption, graduated bool) (*channelcommercecap.ImportOptionPrice, error) {
	var (
		choices         = append([]nativeCatalogChoice(nil), option.Choices...)
		price           = &channelcommercecap.ImportOptionPrice{FieldKey: "cfg_" + scalar(option.ID), PricingMode: channelcommercecap.ImportConfigurationPricePerUnit, Bands: []channelcommercecap.ImportPriceBand{}}
		previousMaximum int64
		firstSetup      int64
	)
	if graduated {
		price.PricingMode = channelcommercecap.ImportConfigurationPriceGraduated
	}
	sort.Slice(choices, func(i, j int) bool {
		a, errA := nativeNonnegative(choices[i].Minimum)
		b, errB := nativeNonnegative(choices[j].Minimum)
		return errA == nil && (errB != nil || a < b)
	})
	maximum, err := nativeNonnegative(option.Maximum)
	if err != nil {
		return nil, err
	}
	for index, choice := range choices {
		minimum, minErr := nativeNonnegative(choice.Minimum)
		upper, maxErr := nativeNonnegative(choice.Maximum)
		if minErr != nil || maxErr != nil || upper < minimum || index > 0 && minimum != previousMaximum+1 {
			return nil, nil
		}
		band, err := nativeChoiceBand(currency, cycle, choice, true)
		if err != nil {
			return nil, err
		}
		if band == nil {
			return nil, nil
		}
		if index == 0 {
			firstSetup = band.SetupAmountMinor
		} else if graduated {
			// Native stage charging keeps the selected stage's setup fee, whereas the
			// portable graduated tariff has one first-band setup fee. Equal fees are exact.
			if band.SetupAmountMinor != firstSetup {
				return nil, nil
			}
			band.SetupAmountMinor = 0
		}
		if index > 0 {
			band.MinimumQuantity = previousMaximum
		}
		if index < len(choices)-1 {
			value := upper
			band.MaximumQuantity = &value
		} else if upper != maximum {
			return nil, nil
		}
		price.Bands = append(price.Bands, *band)
		previousMaximum = upper
	}
	return price, nil
}

func nativeChoiceBand(currency string, cycle catalogCycle, choice nativeCatalogChoice, quantity bool) (*channelcommercecap.ImportPriceBand, error) {
	var selected map[string]json.RawMessage
	for _, row := range choice.Prices {
		if scalar(row["code"]) == currency {
			if selected != nil {
				return nil, errInvalidResponse
			}
			selected = row
		}
	}
	if selected == nil {
		return nil, nil
	}
	amountRaw, amountExists := selected[cycle.amount]
	setupRaw, setupExists := selected[cycle.setup]
	if !amountExists || !setupExists {
		return nil, nil
	}
	amount, err := nativeMinor(scalar(amountRaw), quantity)
	if err != nil {
		return nil, nil
	}
	setup, err := nativeMinor(scalar(setupRaw), false)
	if err != nil {
		return nil, nil
	}
	return &channelcommercecap.ImportPriceBand{AmountMinor: amount, SetupAmountMinor: setup, RenewalAmountMinor: &amount}, nil
}

// Finance uses bcscale(2) for whole components. Quantity rates must already fit
// two decimals, since truncating a rate before multiplication changes the price.
func nativeMinor(value string, exact bool) (int64, error) {
	if catalogMissingPrice.MatchString(value) {
		return 0, nil
	}
	if !catalogDecimal.MatchString(value) {
		return 0, errInvalidResponse
	}
	number, ok := new(big.Rat).SetString(value)
	if !ok {
		return 0, errInvalidResponse
	}
	number.Mul(number, big.NewRat(100, 1))
	if exact && !number.IsInt() {
		return 0, errUnsupported
	}
	minor := new(big.Int).Quo(number.Num(), number.Denom())
	if !minor.IsInt64() {
		return 0, errInvalidResponse
	}
	return minor.Int64(), nil
}

func nativeQuantityTopology(option nativeCatalogOption, minimum, maximum int64) bool {
	choices := append([]nativeCatalogChoice(nil), option.Choices...)
	sort.Slice(choices, func(i, j int) bool {
		a, _ := nativeNonnegative(choices[i].Minimum)
		b, _ := nativeNonnegative(choices[j].Minimum)
		return a < b
	})
	previous := int64(-1)
	for index, choice := range choices {
		low, err := nativeNonnegative(choice.Minimum)
		high, maxErr := nativeNonnegative(choice.Maximum)
		if err != nil || maxErr != nil || high < low || index == 0 && low > minimum || index > 0 && (previous == int64(1<<63-1) || low != previous+1) {
			return false
		}
		previous = high
	}
	return len(choices) > 0 && previous >= maximum
}
