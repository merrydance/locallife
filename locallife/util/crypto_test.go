package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAESEncryptor_EncryptDecrypt(t *testing.T) {
	// 32 字节密钥（AES-256）
	key := "12345678901234567890123456789012"
	encryptor, err := NewAESEncryptor(key)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "身份证号",
			plaintext: "330123199001011234",
		},
		{
			name:      "银行卡号",
			plaintext: "6222021234567890123",
		},
		{
			name:      "手机号",
			plaintext: "13812345678",
		},
		{
			name:      "中文姓名",
			plaintext: "张三",
		},
		{
			name:      "空字符串",
			plaintext: "",
		},
		{
			name:      "长字符串",
			plaintext: "这是一个很长的测试字符串，用于验证 AES 加密对于较长文本的支持情况",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 加密
			ciphertext, err := encryptor.Encrypt(tc.plaintext)
			require.NoError(t, err)

			if tc.plaintext == "" {
				require.Empty(t, ciphertext)
				return
			}

			// 密文应该与明文不同
			require.NotEqual(t, tc.plaintext, ciphertext)

			// 解密
			decrypted, err := encryptor.Decrypt(ciphertext)
			require.NoError(t, err)
			require.Equal(t, tc.plaintext, decrypted)
		})
	}
}

func TestAESEncryptor_DifferentKeyLengths(t *testing.T) {
	testCases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "AES-128 (16 bytes)",
			key:     "1234567890123456",
			wantErr: false,
		},
		{
			name:    "AES-192 (24 bytes)",
			key:     "123456789012345678901234",
			wantErr: false,
		},
		{
			name:    "AES-256 (32 bytes)",
			key:     "12345678901234567890123456789012",
			wantErr: false,
		},
		{
			name:    "Invalid key (15 bytes)",
			key:     "123456789012345",
			wantErr: true,
		},
		{
			name:    "Invalid key (33 bytes)",
			key:     "123456789012345678901234567890123",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encryptor, err := NewAESEncryptor(tc.key)
			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, ErrInvalidKey, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, encryptor)

				// 测试加密解密
				plaintext := "test data"
				ciphertext, err := encryptor.Encrypt(plaintext)
				require.NoError(t, err)

				decrypted, err := encryptor.Decrypt(ciphertext)
				require.NoError(t, err)
				require.Equal(t, plaintext, decrypted)
			}
		})
	}
}

func TestAESEncryptor_UniqueNonce(t *testing.T) {
	key := "12345678901234567890123456789012"
	encryptor, err := NewAESEncryptor(key)
	require.NoError(t, err)

	plaintext := "same plaintext"

	// 加密同一个明文多次，应该得到不同的密文（因为 nonce 不同）
	ciphertext1, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)

	ciphertext2, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)

	require.NotEqual(t, ciphertext1, ciphertext2, "同一明文多次加密应该产生不同的密文")

	// 但两者都应该能正确解密
	decrypted1, err := encryptor.Decrypt(ciphertext1)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted1)

	decrypted2, err := encryptor.Decrypt(ciphertext2)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted2)
}

func TestAESEncryptor_InvalidCiphertext(t *testing.T) {
	key := "12345678901234567890123456789012"
	encryptor, err := NewAESEncryptor(key)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		ciphertext string
		wantErr    error
	}{
		{
			name:       "Invalid base64",
			ciphertext: "not-valid-base64!!!",
			wantErr:    nil, // base64 解码错误
		},
		{
			name:       "Too short",
			ciphertext: "YWJj", // "abc" in base64，太短
			wantErr:    ErrCiphertextTooShort,
		},
		{
			name:       "Tampered ciphertext",
			ciphertext: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3ODkw", // 随机数据
			wantErr:    ErrDecryptionFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tc.ciphertext)
			require.Error(t, err)
			if tc.wantErr != nil {
				require.Equal(t, tc.wantErr, err)
			}
		})
	}
}

func TestEncryptDecryptSensitiveField(t *testing.T) {
	key := "12345678901234567890123456789012"
	encryptor, err := NewAESEncryptor(key)
	require.NoError(t, err)

	plaintext := "330123199001011234"

	// 使用便捷函数加密
	ciphertext, err := EncryptSensitiveField(encryptor, plaintext)
	require.NoError(t, err)
	require.NotEqual(t, plaintext, ciphertext)

	// 使用便捷函数解密
	decrypted, err := DecryptSensitiveField(encryptor, ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestEncryptDecryptSensitiveField_NilEncryptor(t *testing.T) {
	plaintext := "330123199001011234"

	// encryptor 为 nil 时，应该返回原文
	result, err := EncryptSensitiveField(nil, plaintext)
	require.NoError(t, err)
	require.Equal(t, plaintext, result)

	decrypted, err := DecryptSensitiveField(nil, plaintext)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}
