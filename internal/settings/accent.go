package settings

import (
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Theme color (PRD §16, API.md §11, 数据库设计 4.1.1). Go never computes
// colors: it validates the shape and stores what the frontend derived.

// AccentDefault is the preset a fresh install and every fallback use.
const AccentDefault = "orange"

// accentPresets are the six fixed palettes; "custom" carries its own hex.
var accentPresets = map[string]bool{
	"orange": true, "indigo": true, "blue": true,
	"green": true, "teal": true, "violet": true,
}

func isAccentPreset(p string) bool { return accentPresets[p] }

// isAccentValue also accepts "custom", which carries its own hex.
func isAccentValue(p string) bool { return p == "custom" || accentPresets[p] }

// isHexColor reports whether s is #rrggbb in either case.
func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func validTokens(t contract.AccentTokens) bool {
	for _, v := range []string{
		t.Accent, t.Hover, t.Active, t.Soft, t.Text,
		t.Btn, t.BtnHover, t.BtnActive, t.OnAccent, t.Focus,
	} {
		if !isHexColor(v) {
			return false
		}
	}
	return true
}

func lowerTokens(t contract.AccentTokens) contract.AccentTokens {
	lower := func(v string) string { return strings.ToLower(v) }
	return contract.AccentTokens{
		Accent: lower(t.Accent), Hover: lower(t.Hover), Active: lower(t.Active),
		Soft: lower(t.Soft), Text: lower(t.Text),
		Btn: lower(t.Btn), BtnHover: lower(t.BtnHover), BtnActive: lower(t.BtnActive),
		OnAccent: lower(t.OnAccent), Focus: lower(t.Focus),
	}
}

// NormalizeAccent applies the read-time fallbacks of 数据库设计 4.1.1. It
// never fails: an unreadable value becomes the default and a broken cache is
// dropped so the frontend recomputes it.
func NormalizeAccent(a contract.AccentSetting) contract.AccentSetting {
	if !isAccentValue(a.Preset) {
		return contract.AccentSetting{Preset: AccentDefault}
	}
	if a.Preset != "custom" {
		return contract.AccentSetting{Preset: a.Preset}
	}
	hex := strings.ToLower(a.Hex)
	if !isHexColor(hex) {
		return contract.AccentSetting{Preset: AccentDefault}
	}
	out := contract.AccentSetting{Preset: "custom", Hex: hex}
	if a.Cache != nil && validTokens(a.Cache.Light) && validTokens(a.Cache.Dark) {
		out.Cache = &contract.AccentCache{Light: lowerTokens(a.Cache.Light), Dark: lowerTokens(a.Cache.Dark)}
	}
	return out
}

// validateAccentPatch is the strict counterpart for M61 (API.md §11.2): a
// patch with any bad field is rejected instead of falling back.
func validateAccentPatch(a *contract.AccentSetting) (contract.AccentSetting, bool) {
	if a == nil || !isAccentValue(a.Preset) {
		return contract.AccentSetting{}, false
	}
	if a.Preset != "custom" {
		return contract.AccentSetting{Preset: a.Preset}, true
	}
	hex := strings.ToLower(a.Hex)
	if !isHexColor(hex) {
		return contract.AccentSetting{}, false
	}
	out := contract.AccentSetting{Preset: "custom", Hex: hex}
	if a.Cache != nil {
		if !validTokens(a.Cache.Light) || !validTokens(a.Cache.Dark) {
			return contract.AccentSetting{}, false
		}
		out.Cache = &contract.AccentCache{Light: lowerTokens(a.Cache.Light), Dark: lowerTokens(a.Cache.Dark)}
	}
	return out, true
}
