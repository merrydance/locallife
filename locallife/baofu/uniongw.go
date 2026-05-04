package baofu

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
)

const (
	UnionGWVerifyTypeRSA     = "1"
	UnionGWSystemRespSuccess = "S_0000"
)

type UnionGWHeader struct {
	MemberID       string `json:"memberId"`
	TerminalID     string `json:"terminalId"`
	ServiceType    string `json:"serviceTp"`
	VerifyType     string `json:"verifyType,omitempty"`
	SystemRespCode string `json:"sysRespCode,omitempty"`
	SystemRespDesc string `json:"sysRespDesc,omitempty"`
}

type UnionGWPlaintextEnvelope struct {
	Header UnionGWHeader   `json:"header"`
	Body   json.RawMessage `json:"body"`
}

func NewUnionGWRequestEnvelope(memberID, terminalID, serviceType string, body any) (UnionGWPlaintextEnvelope, error) {
	bizContent, err := CanonicalJSON(body)
	if err != nil {
		return UnionGWPlaintextEnvelope{}, err
	}
	envelope := UnionGWPlaintextEnvelope{
		Header: UnionGWHeader{
			MemberID:    strings.TrimSpace(memberID),
			TerminalID:  strings.TrimSpace(terminalID),
			ServiceType: strings.TrimSpace(serviceType),
			VerifyType:  UnionGWVerifyTypeRSA,
		},
		Body: bizContent,
	}
	if err := envelope.ValidateRequest(); err != nil {
		return UnionGWPlaintextEnvelope{}, err
	}
	return envelope, nil
}

func (e UnionGWPlaintextEnvelope) ValidateRequest() error {
	if strings.TrimSpace(e.Header.MemberID) == "" {
		return errors.New("baofu union-gw memberId is required")
	}
	if strings.TrimSpace(e.Header.TerminalID) == "" {
		return errors.New("baofu union-gw terminalId is required")
	}
	if strings.TrimSpace(e.Header.ServiceType) == "" {
		return errors.New("baofu union-gw serviceTp is required")
	}
	if strings.TrimSpace(e.Header.VerifyType) != UnionGWVerifyTypeRSA {
		return errors.New("baofu union-gw verifyType must be 1")
	}
	if len(e.Body) == 0 {
		return errors.New("baofu union-gw body is required")
	}
	if !json.Valid(e.Body) {
		return errors.New("baofu union-gw body must be valid JSON")
	}
	return nil
}

func (e UnionGWPlaintextEnvelope) ValidateResponse(memberID, terminalID, serviceType string) error {
	if strings.TrimSpace(e.Header.MemberID) != strings.TrimSpace(memberID) {
		return errors.New("baofu union-gw response memberId mismatch")
	}
	if strings.TrimSpace(e.Header.TerminalID) != strings.TrimSpace(terminalID) {
		return errors.New("baofu union-gw response terminalId mismatch")
	}
	if strings.TrimSpace(e.Header.ServiceType) != strings.TrimSpace(serviceType) {
		return errors.New("baofu union-gw response serviceTp mismatch")
	}
	if strings.TrimSpace(e.Header.SystemRespCode) == "" {
		return errors.New("baofu union-gw response sysRespCode is required")
	}
	if len(e.Body) == 0 {
		return errors.New("baofu union-gw response body is required")
	}
	if !json.Valid(e.Body) {
		return errors.New("baofu union-gw response body must be valid JSON")
	}
	return nil
}

func EncodeUnionGWVerifyType1Content(privateKeyPEM string, plaintext []byte) (string, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(plaintext))
	return rsaPrivateEncryptPKCS1v15Hex(privateKey.N, privateKey.D, privateKey.Size(), encoded)
}

func DecodeUnionGWVerifyType1Content(publicKeyPEM string, ciphertextHex string) ([]byte, error) {
	publicKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return nil, err
	}
	encoded, err := rsaPublicDecryptPKCS1v15Hex(publicKey.N, big.NewInt(int64(publicKey.E)), publicKey.Size(), ciphertextHex)
	if err != nil {
		return nil, err
	}
	plaintext, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func rsaPrivateEncryptPKCS1v15Hex(modulus *big.Int, exponent *big.Int, keySize int, plaintext []byte) (string, error) {
	if keySize <= 11 {
		return "", errors.New("baofu union-gw rsa key is too short")
	}
	maxChunkSize := keySize - 11
	var encrypted []byte
	for start := 0; start < len(plaintext); start += maxChunkSize {
		end := start + maxChunkSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		block, err := pkcs1v15Type1Block(keySize, plaintext[start:end])
		if err != nil {
			return "", err
		}
		cipherInt := new(big.Int).SetBytes(block)
		cipherInt.Exp(cipherInt, exponent, modulus)
		encrypted = append(encrypted, leftPad(cipherInt.Bytes(), keySize)...)
	}
	return hex.EncodeToString(encrypted), nil
}

func rsaPublicDecryptPKCS1v15Hex(modulus *big.Int, exponent *big.Int, keySize int, ciphertextHex string) ([]byte, error) {
	ciphertext, err := hex.DecodeString(strings.TrimSpace(ciphertextHex))
	if err != nil {
		return nil, err
	}
	if keySize == 0 || len(ciphertext)%keySize != 0 {
		return nil, errors.New("baofu union-gw ciphertext length is invalid")
	}
	var plaintext []byte
	for start := 0; start < len(ciphertext); start += keySize {
		cipherInt := new(big.Int).SetBytes(ciphertext[start : start+keySize])
		plainInt := new(big.Int).Exp(cipherInt, exponent, modulus)
		block := leftPad(plainInt.Bytes(), keySize)
		chunk, err := unpadPKCS1v15Block(block)
		if err != nil {
			return nil, err
		}
		plaintext = append(plaintext, chunk...)
	}
	return plaintext, nil
}

func pkcs1v15Type1Block(keySize int, message []byte) ([]byte, error) {
	if len(message) > keySize-11 {
		return nil, errors.New("baofu union-gw rsa plaintext chunk is too long")
	}
	block := make([]byte, keySize)
	block[1] = 1
	paddingEnd := keySize - len(message) - 1
	for i := 2; i < paddingEnd; i++ {
		block[i] = 0xff
	}
	copy(block[paddingEnd+1:], message)
	return block, nil
}

func unpadPKCS1v15Block(block []byte) ([]byte, error) {
	if len(block) < 11 || block[0] != 0 || (block[1] != 1 && block[1] != 2) {
		return nil, errors.New("baofu union-gw rsa padding is invalid")
	}
	separator := 0
	for i := 2; i < len(block); i++ {
		if block[i] == 0 {
			separator = i
			break
		}
	}
	if separator < 10 {
		return nil, errors.New("baofu union-gw rsa padding is invalid")
	}
	return block[separator+1:], nil
}

func leftPad(value []byte, size int) []byte {
	if len(value) >= size {
		return value
	}
	padded := make([]byte, size)
	copy(padded[size-len(value):], value)
	return padded
}
