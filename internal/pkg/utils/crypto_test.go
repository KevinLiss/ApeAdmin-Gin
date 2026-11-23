package utils

import (
	"strings"
	"testing"
)

const testSecret = "unit-test-secret-32-bytes-long-ok!"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintexts := []string{
		"sk-bb78e9fa11424941a259d3cf59e74a25",
		"中文内容也可以",
		"a",
		"",
	}
	for _, p := range plaintexts {
		enc, err := EncryptSecret(testSecret, p)
		if err != nil {
			t.Fatalf("encrypt %q: %v", p, err)
		}
		// 密文不得包含明文（base64 字符集可能恰好含单字符明文，仅对足够长的明文检查）
		if len(p) >= 8 && strings.Contains(enc, p) {
			t.Fatalf("ciphertext leaks plaintext: %s", enc)
		}
		dec, err := DecryptSecret(testSecret, enc)
		if err != nil {
			t.Fatalf("decrypt %q: %v", p, err)
		}
		if dec != p {
			t.Fatalf("round trip mismatch: want %q got %q", p, dec)
		}
	}
}

func TestDecryptWrongSecretFails(t *testing.T) {
	enc, err := EncryptSecret(testSecret, "sensitive-data")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptSecret("another-secret-32-bytes-long-ok!!!!!!", enc); err == nil {
		t.Fatal("decrypt with wrong secret must fail")
	}
}

func TestDecryptGarbageFails(t *testing.T) {
	if _, err := DecryptSecret(testSecret, "not-base64!!!"); err == nil {
		t.Fatal("garbage ciphertext must fail")
	}
}

func TestEncryptDeterministicCipherTextDiffers(t *testing.T) {
	a, _ := EncryptSecret(testSecret, "same-input")
	b, _ := EncryptSecret(testSecret, "same-input")
	// 每次随机 IV → 密文不同（若相同说明未随机化，安全性差）
	if a == b {
		t.Fatal("ciphertext should differ across encryptions (random IV)")
	}
	// 但都能解回同一明文
	da, _ := DecryptSecret(testSecret, a)
	dbb, _ := DecryptSecret(testSecret, b)
	if da != dbb || da != "same-input" {
		t.Fatal("both ciphertexts must decrypt to the same plaintext")
	}
}
