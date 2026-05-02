package ordinaryserviceprovider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

func (c *Client) GenerateJSAPIPayParams(prepayID string) (*contracts.JSAPIPayParams, error) {
	if c == nil || c.privateKey == nil {
		return nil, fmt.Errorf("ordinary service provider private key: not configured")
	}
	nonceStr, err := generateNonceStr()
	if err != nil {
		return nil, err
	}
	timeStamp := fmt.Sprintf("%d", time.Now().Unix())
	packageStr := "prepay_id=" + prepayID
	signStr := fmt.Sprintf("%s\n%s\n%s\n%s\n", c.config.ServiceProviderAppID, timeStamp, nonceStr, packageStr)
	paySign, err := c.signWithRSA(signStr)
	if err != nil {
		return nil, fmt.Errorf("sign ordinary service provider pay params: %w", err)
	}
	return &contracts.JSAPIPayParams{
		TimeStamp: timeStamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  contracts.JSAPIPaySignTypeRSA,
		PaySign:   paySign,
	}, nil
}

func (c *Client) signWithRSA(message string) (string, error) {
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func generateNonceStr() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := readBoundedConfigFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return rsaKey, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := readBoundedConfigFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaKey, nil
}

func readBoundedConfigFile(path string) ([]byte, error) {
	cleanedPath := filepath.Clean(path)
	rootDir := filepath.Dir(cleanedPath)
	fileName := filepath.Base(cleanedPath)
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open config root: %w", err)
	}
	defer root.Close()
	file, err := root.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	return data, nil
}
