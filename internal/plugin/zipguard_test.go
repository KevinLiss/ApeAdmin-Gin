package plugin

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

)

func guardCfg() ZipGuardConfig {
	return ZipGuardConfig{
		MaxZipSizeMB:      50,
		MaxEntries:        1000,
		MaxDecompressedMB: 100,
		DenySymlinks:      true,
	}
}

// makeZip 构造测试 zip（entry 名可注入恶意路径）
func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.zip")
	w, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for name, content := range entries {
		fw, _ := zw.Create(name)
		fw.Write([]byte(content))
	}
	zw.Close()
	w.Close()
	return p
}

func validManifest() string {
	return `{"name":"t","version":"1.0.0","type":"l2","display_name":"t","description":"","author":"a"}`
}

func TestValidateZipAcceptsNormal(t *testing.T) {
	p := makeZip(t, map[string]string{
		"plugin.json": validManifest(),
		"menu.json":   "[]",
	})
	manifest, _, err := ValidateZip(p, guardCfg())
	if err != nil {
		t.Fatalf("合法包被拒: %v", err)
	}
	if manifest.Name != "t" {
		t.Errorf("manifest.Name = %s", manifest.Name)
	}
}

func TestValidateZipRejectsPathTraversal(t *testing.T) {
	for _, evil := range []string{
		"../evil.txt",
		"a/../../evil.txt",
		"/abs/evil.txt",
		"../../evil.txt",
	} {
		p := makeZip(t, map[string]string{
			"plugin.json": validManifest(),
			evil:          "x",
		})
		if _, _, err := ValidateZip(p, guardCfg()); err == nil {
			t.Errorf("路径穿越条目未被拒绝: %s", evil)
		}
	}
}

func TestValidateZipRejectsExecutable(t *testing.T) {
	for _, evil := range []string{"run.exe", "lib.so", "hack.dll", "sh.bin"} {
		p := makeZip(t, map[string]string{
			"plugin.json": validManifest(),
			evil:          "x",
		})
		if _, _, err := ValidateZip(p, guardCfg()); err == nil {
			t.Errorf("可执行条目未被拒绝: %s", evil)
		}
	}
}

func TestValidateZipRejectsMissingManifest(t *testing.T) {
	p := makeZip(t, map[string]string{"readme.md": "hi"})
	if _, _, err := ValidateZip(p, guardCfg()); err == nil {
		t.Fatalf("无 plugin.json 的包未被拒绝")
	}
}

func TestValidateZipRejectsWrongType(t *testing.T) {
	p := makeZip(t, map[string]string{
		"plugin.json": `{"name":"t","version":"1.0.0","type":"l1"}`,
	})
	if _, _, err := ValidateZip(p, guardCfg()); err == nil {
		t.Fatalf("非 l2 类型未被拒绝")
	}
}
