package core

import (
	"strings"
	"testing"
)

func TestSetI18nOverridesAndLookup(t *testing.T) {
	SetI18nOverrides(map[string]map[string]string{
		"zh-CN": {"test.greet": "你好，{name}"},
		"en-US": {"test.greet": "Hello, {name}"},
	}, "zh-CN")
	t.Cleanup(func() {
		SetI18nOverrides(map[string]map[string]string{}, "zh-CN")
	})

	if got := T("zh-CN", "test.greet", map[string]string{"name": "小明"}); got != "你好，小明" {
		t.Fatalf("T zh-CN = %q", got)
	}
	if got := T("en-US", "test.greet", map[string]string{"name": "Tom"}); got != "Hello, Tom" {
		t.Fatalf("T en-US = %q", got)
	}
}

func TestTUnknownKeyReturnsKey(t *testing.T) {
	got := T("zh-CN", "no.such.key.exists")
	if got != "no.such.key.exists" {
		t.Fatalf("unknown key should fall back to key itself, got %q", got)
	}
}

func TestTFallbackToDefaultLocale(t *testing.T) {
	SetI18nOverrides(map[string]map[string]string{
		"zh-CN": {"test.only.zh": "只有中文"},
	}, "zh-CN")
	t.Cleanup(func() {
		SetI18nOverrides(map[string]map[string]string{}, "zh-CN")
	})
	// 请求 ja-JP（无覆盖）→ 回退默认语言 zh-CN
	got := T("ja-JP", "test.only.zh")
	if got != "只有中文" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestTEmptyLocaleUsesDefault(t *testing.T) {
	SetI18nOverrides(map[string]map[string]string{
		"zh-CN": {"test.empty.locale": "默认语言"},
	}, "zh-CN")
	t.Cleanup(func() {
		SetI18nOverrides(map[string]map[string]string{}, "zh-CN")
	})
	got := T("", "test.empty.locale")
	if got != "默认语言" {
		t.Fatalf("empty locale = %q", got)
	}
}

func TestParseLocaleSupported(t *testing.T) {
	if got := ParseLocale("zh-CN,en;q=0.9"); got != "zh-CN" {
		t.Fatalf("ParseLocale zh = %q", got)
	}
	if got := ParseLocale("en-US,en;q=0.9"); got != "en-US" {
		t.Fatalf("ParseLocale en = %q", got)
	}
	if !strings.HasPrefix(ParseLocale("fr-FR"), "zh") && ParseLocale("fr-FR") != "zh-CN" {
		t.Fatalf("unsupported locale should fall back to default, got %q", ParseLocale("fr-FR"))
	}
}
