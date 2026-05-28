package cloudprint

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func BuildFeieyunCallbackCanonicalString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key, list := range values {
		if key == "sign" || len(list) == 0 || strings.TrimSpace(list[0]) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "&")
}

func VerifyFeieyunCallbackSignature(values url.Values, publicKeyPEM string) error {
	signatureValue := strings.TrimSpace(values.Get("sign"))
	if signatureValue == "" {
		return fmt.Errorf("feieyun callback sign is required")
	}

	publicKey, err := parseFeieyunCallbackPublicKey(publicKeyPEM)
	if err != nil {
		return err
	}

	signature, err := base64.StdEncoding.DecodeString(signatureValue)
	if err != nil {
		return fmt.Errorf("decode feieyun callback sign: %w", err)
	}

	digest := sha256.Sum256([]byte(BuildFeieyunCallbackCanonicalString(values)))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("verify feieyun callback sign: %w", err)
	}
	return nil
}

func parseFeieyunCallbackPublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(publicKeyPEM)))
	if block == nil {
		return nil, fmt.Errorf("decode feieyun callback public key")
	}

	var parsed any
	var err error
	switch block.Type {
	case "PUBLIC KEY":
		parsed, err = x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		parsed, err = x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported feieyun callback public key type %q", block.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("parse feieyun callback public key: %w", err)
	}

	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("feieyun callback public key is not RSA")
	}
	return publicKey, nil
}
