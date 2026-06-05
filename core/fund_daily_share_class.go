package core

import (
	"strings"
	"unicode"
)

const (
	fundShareClassA = "A"
	fundShareClassC = "C"
)

func preferClassCFundDailyActions(actions []FundDailyAction) []FundDailyAction {
	if len(actions) == 0 {
		return actions
	}

	groups := make([]string, len(actions))
	classes := make([]string, len(actions))
	groupHasA := map[string]bool{}
	groupHasC := map[string]bool{}
	for idx, action := range actions {
		base, class := splitFundShareClass(action.Name)
		if base == "" {
			base = action.Code
		}
		groups[idx] = base
		classes[idx] = class
		switch class {
		case fundShareClassA:
			groupHasA[base] = true
		case fundShareClassC:
			groupHasC[base] = true
		}
	}

	filtered := make([]FundDailyAction, 0, len(actions))
	for idx, action := range actions {
		group := groups[idx]
		class := classes[idx]
		if class == fundShareClassA && groupHasC[group] {
			continue
		}
		if class == fundShareClassC && groupHasA[group] {
			action.Reasons = prependUniqueDailyReason(action.Reasons, "同一基金存在 A/C 份额，已优先展示 C 类。")
		}
		filtered = append(filtered, action)
	}
	return filtered
}

func splitFundShareClass(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}

	for _, suffix := range []struct {
		text  string
		class string
	}{
		{"A类", fundShareClassA},
		{"C类", fundShareClassC},
		{"A份额", fundShareClassA},
		{"C份额", fundShareClassC},
	} {
		if strings.HasSuffix(name, suffix.text) {
			return normalizeFundShareClassBase(strings.TrimSuffix(name, suffix.text)), suffix.class
		}
	}

	runes := []rune(name)
	if len(runes) < 2 {
		return normalizeFundShareClassBase(name), ""
	}
	last := runes[len(runes)-1]
	if last != 'A' && last != 'C' {
		return normalizeFundShareClassBase(name), ""
	}
	if isLikelyPlainEnglishSuffix(runes[len(runes)-2]) {
		return normalizeFundShareClassBase(name), ""
	}
	return normalizeFundShareClassBase(string(runes[:len(runes)-1])), string(last)
}

func isLikelyPlainEnglishSuffix(previous rune) bool {
	return previous <= unicode.MaxASCII && unicode.IsLetter(previous)
}

func normalizeFundShareClassBase(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimRight(name, " \t\r\n-_/")
	name = strings.TrimRight(name, "()（）")
	return strings.ToUpper(strings.TrimSpace(name))
}

func prependUniqueDailyReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append([]string{reason}, reasons...)
}
