package baofu

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
)

var ErrInvalidSignature = errors.New("baofu invalid signature")

func CanonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func SignSHA256WithRSA(privateKeyPEM string, message []byte) (string, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}

func VerifySHA256WithRSA(publicKeyPEM string, message []byte, signatureHex string) error {
	publicKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return err
	}
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return ErrInvalidSignature
	}
	digest := sha256.Sum256(message)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return ErrInvalidSignature
	}
	return nil
}

func parseRSAPrivateKey(rawPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(rawPEM))
	if block == nil {
		return nil, errors.New("baofu private key pem decode failed")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("baofu private key is not rsa")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("baofu private key parse failed")
	}
	return key, nil
}

func parseRSAPublicKey(rawPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(rawPEM))
	if block == nil {
		return nil, errors.New("baofu public key pem decode failed")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("baofu public key is not rsa")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("baofu public key parse failed")
	}
	rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("baofu certificate public key is not rsa")
	}
	return rsaKey, nil
}
