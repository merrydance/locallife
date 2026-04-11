package worker

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
