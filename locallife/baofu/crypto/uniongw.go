package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var ErrInvalidEnvelopeSignature = errors.New("baofu union-gw invalid envelope signature")

type UnionGWEnvelope struct {
	MerchantID string `json:"merchantNo"`
	TerminalID string `json:"terminalNo"`
	DataType   string `json:"dataType"`
	Data       string `json:"data"`
	Sign       string `json:"sign"`
}

type UnionGWCodec struct {
	key []byte
}

func NewUnionGWCodec(aesKey string) (*UnionGWCodec, error) {
	key := []byte(strings.TrimSpace(aesKey))
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("baofu union-gw aes key must be 16, 24, or 32 bytes")
	}
	return &UnionGWCodec{key: key}, nil
}

func (c *UnionGWCodec) EncryptJSON(v any) (string, error) {
	plaintext, err := canonicalJSON(v)
	if err != nil {
		return "", err
	}
	return c.encrypt(plaintext)
}

func (c *UnionGWCodec) DecryptJSON(ciphertext string) ([]byte, error) {
	return c.decrypt(ciphertext)
}

func (c *UnionGWCodec) SealEnvelope(merchantID, terminalID string, payload any) (UnionGWEnvelope, error) {
	data, err := c.EncryptJSON(payload)
	if err != nil {
		return UnionGWEnvelope{}, err
	}
	envelope := UnionGWEnvelope{
		MerchantID: strings.TrimSpace(merchantID),
		TerminalID: strings.TrimSpace(terminalID),
		DataType:   "json",
		Data:       data,
	}
	envelope.Sign = c.signEnvelope(envelope)
	return envelope, nil
}

func (c *UnionGWCodec) OpenEnvelope(envelope UnionGWEnvelope) ([]byte, error) {
	if !hmac.Equal([]byte(c.signEnvelope(envelope)), []byte(strings.TrimSpace(envelope.Sign))) {
		return nil, ErrInvalidEnvelopeSignature
	}
	return c.DecryptJSON(envelope.Data)
}

func (c *UnionGWCodec) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *UnionGWCodec) decrypt(ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("baofu union-gw ciphertext is too short")
	}
	nonce := raw[:gcm.NonceSize()]
	body := raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

func (c *UnionGWCodec) signEnvelope(envelope UnionGWEnvelope) string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(strings.TrimSpace(envelope.MerchantID)))
	mac.Write([]byte("\n"))
	mac.Write([]byte(strings.TrimSpace(envelope.TerminalID)))
	mac.Write([]byte("\n"))
	mac.Write([]byte(strings.TrimSpace(envelope.DataType)))
	mac.Write([]byte("\n"))
	mac.Write([]byte(strings.TrimSpace(envelope.Data)))
	return hex.EncodeToString(mac.Sum(nil))
}

func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
