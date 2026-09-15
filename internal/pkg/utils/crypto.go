package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// 基于 JWT Secret 派生的对称加密（AES-256-GCM），用于加密 AI Provider API Key。
// 密文格式: base64(nonce || ciphertext || tag)，可安全存入数据库。

func deriveAESKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// EncryptSecret 加密明文，返回 base64 密文
func EncryptSecret(secret, plaintext string) (string, error) {
	block, err := aes.NewCipher(deriveAESKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret 解密 base64 密文
func DecryptSecret(secret, ciphertextB64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(deriveAESKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("密文长度不足")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}
	return string(plain), nil
}

// MaskAPIKey 脱敏展示: sk-****last4
func MaskAPIKey(plaintext string) string {
	if len(plaintext) <= 8 {
		return "****"
	}
	runes := []rune(plaintext)
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}