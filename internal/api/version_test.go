package api

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── parseSemver ───

func TestParseSemverBasic(t *testing.T) {
	got := parseSemver("0.4.1")
	want := []int{0, 4, 1}
	if len(got) != len(want) {
		t.Fatalf("parseSemver(%q) = %v, want %v", "0.4.1", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseSemver(%q) = %v, want %v", "0.4.1", got, want)
		}
	}
}

func TestParseSemverVPrefix(t *testing.T) {
	got := parseSemver("v2.8.0")
	if len(got) != 3 || got[0] != 2 || got[1] != 8 || got[2] != 0 {
		t.Fatalf("parseSemver(v2.8.0) = %v, want [2 8 0]", got)
	}
}

func TestParseSemverNonNumericSuffix(t *testing.T) {
	// "1.2.3-beta.1" → 数字段 [1 2 3 1]，字母段丢弃
	got := parseSemver("1.2.3-beta.1")
	if len(got) != 4 || got[0] != 1 || got[1] != 2 || got[2] != 3 || got[3] != 1 {
		t.Fatalf("parseSemver(1.2.3-beta.1) = %v, want [1 2 3 1]", got)
	}
}

func TestParseSemverEmpty(t *testing.T) {
	got := parseSemver("")
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("parseSemver(\"\") = %v, want [0]", got)
	}
}

// ─── semverGreater ───

func TestSemverGreaterTrue(t *testing.T) {
	if !semverGreater("0.4.1", "0.4.0") {
		t.Fatal("0.4.1 should be greater than 0.4.0")
	}
	if !semverGreater("1.0", "0.9.9") {
		t.Fatal("1.0 should be greater than 0.9.9")
	}
	if !semverGreater("v0.5.0", "0.4.9") {
		t.Fatal("v0.5.0 should be greater than 0.4.9")
	}
}

func TestSemverGreaterFalse(t *testing.T) {
	if semverGreater("0.4.0", "0.4.0") {
		t.Fatal("equal versions must not be greater")
	}
	if semverGreater("0.3.9", "0.4.0") {
		t.Fatal("0.3.9 should not be greater than 0.4.0")
	}
	if semverGreater("2.8", "2.10.0") {
		t.Fatal("2.8 should not be greater than 2.10.0 (numeric compare)")
	}
}

func TestSemverGreaterMissingSegments(t *testing.T) {
	// 缺失段按 0 处理：1.0 == 1.0.0
	if semverGreater("1.0", "1.0.0") {
		t.Fatal("1.0 == 1.0.0, must not be greater")
	}
	if !semverGreater("1.0.1", "1.0") {
		t.Fatal("1.0.1 > 1.0 (missing segment treated as 0)")
	}
}

// ─── patchLocalConfigVersion ───

func TestPatchLocalConfigVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "app:\n  name: \"ApeAdmin\"\n  version: \"0.4.0\"\n  debug: true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchLocalConfigVersion(path, "0.4.1"); err != nil {
		t.Fatalf("patch: %v", err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !contains(s, `version: "0.4.1"`) {
		t.Fatalf("patched config missing new version:\n%s", s)
	}
	if contains(s, `version: "0.4.0"`) {
		t.Fatalf("old version still present:\n%s", s)
	}
	if !contains(s, `name: "ApeAdmin"`) {
		t.Fatalf("other lines must be preserved:\n%s", s)
	}
}

func TestPatchLocalConfigVersionMissingFile(t *testing.T) {
	if err := patchLocalConfigVersion(filepath.Join(t.TempDir(), "nope.yaml"), "9.9.9"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ─── 更新包完整性校验 ───

func TestFileMD5MatchesKnownHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg.bin")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := fileMD5(path)
	if err != nil {
		t.Fatalf("fileMD5: %v", err)
	}
	// md5("hello") = 5d41402abc4b2a76b9719d911017c592
	if got != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("md5 = %s", got)
	}
}

func TestIsMD5Hex(t *testing.T) {
	if !isMD5Hex("916e5cf4c00fd81946a718b5c42d9614") {
		t.Fatal("合法 MD5 被拒")
	}
	for _, s := range []string{"", "abc", "916E5CF4C00FD81946A718B5C42D9614", "zzze5cf4c00fd81946a718b5c42d9614"} {
		if isMD5Hex(s) {
			t.Fatalf("%q 不应被当作合法 MD5", s)
		}
	}
}

func TestFileMD5MissingFile(t *testing.T) {
	if _, err := fileMD5(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing file must error")
	}
}

// ─── version 键精确匹配（防 version_xxx 前缀误伤）───

func TestIsVersionKeyLine(t *testing.T) {
	good := []string{`version: "1.0"`, "  version: 1.0", "\tversion: x"}
	for _, line := range good {
		if !isVersionKeyLine(line) {
			t.Fatalf("%q 应识别为 version 键", line)
		}
	}
	bad := []string{"version_extra: 1", "versions: []", "# version: 1.0", "app_version: 2"}
	for _, line := range bad {
		if isVersionKeyLine(line) {
			t.Fatalf("%q 不应识别为 version 键", line)
		}
	}
}

func TestPatchLocalConfigVersionIgnoresSimilarKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "app:\n  version_check: \"on\"\n  version: \"0.4.0\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchLocalConfigVersion(path, "0.4.2"); err != nil {
		t.Fatalf("patch: %v", err)
	}
	out, _ := os.ReadFile(path)
	s := string(out)
	if !contains(s, `version_check: "on"`) {
		t.Fatalf("相似键被误改:\n%s", s)
	}
	if !contains(s, `version: "0.4.2"`) {
		t.Fatalf("版本未更新:\n%s", s)
	}
}
