package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var (
	ErrInvalidKey         = errors.New("encryption key must be 16, 24, or 32 bytes")
	ErrCiphertextTooShort = errors.New("ciphertext too short")
	ErrDecryptionFailed   = errors.New("decryption failed")
)

// DataEncryptor 数据加密器接口
type DataEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// AESEncryptor AES-GCM 加密器
// 用于加密存储在数据库中的敏感信息（如身份证号、银行卡号）
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor 创建 AES 加密器
// key 必须是 16、24 或 32 字节（对应 AES-128、AES-192、AES-256）
func NewAESEncryptor(key string) (*AESEncryptor, error) {
	keyBytes := []byte(key)
	keyLen := len(keyBytes)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidKey
	}
	return &AESEncryptor{key: keyBytes}, nil
}

// Encrypt 使用 AES-GCM 加密明文
// 返回 Base64 编码的密文（包含 nonce）
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密并附加 nonce 到密文前
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 AES-GCM 密文
// 输入为 Base64 编码的密文
func (e *AESEncryptor) Decrypt(ciphertextBase64 string) (string, error) {
	if ciphertextBase64 == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrCiphertextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// EncryptSensitiveField 加密敏感字段的便捷函数
// 如果 encryptor 为 nil，返回原文（用于未配置加密的情况）
func EncryptSensitiveField(encryptor DataEncryptor, plaintext string) (string, error) {
	if encryptor == nil {
		return plaintext, nil
	}
	return encryptor.Encrypt(plaintext)
}

// DecryptSensitiveField 解密敏感字段的便捷函数
// 如果 encryptor 为 nil，返回原文
func DecryptSensitiveField(encryptor DataEncryptor, ciphertext string) (string, error) {
	if encryptor == nil {
		return ciphertext, nil
	}
	return encryptor.Decrypt(ciphertext)
}
