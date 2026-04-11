package util

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz"

// RandomInt generates a random integer between min and max
func RandomInt(min, max int64) int64 {
	if max <= min {
		return min
	}
	delta, err := crand.Int(crand.Reader, big.NewInt(max-min+1))
	if err != nil {
		panic(fmt.Errorf("crypto rand int: %w", err))
	}
	return min + delta.Int64()
}

// RandomString generates a random string of length n
func RandomString(n int) string {
	if n <= 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(n)
	k := len(alphabet)

	for i := 0; i < n; i++ {
		idx, err := crand.Int(crand.Reader, big.NewInt(int64(k)))
		if err != nil {
			panic(fmt.Errorf("crypto rand string: %w", err))
		}
		c := alphabet[idx.Int64()]
		sb.WriteByte(c)
	}

	return sb.String()
}

// RandomOwner generates a random owner name
func RandomOwner() string {
	return RandomString(6)
}

// RandomMoney generates a random amount of money
func RandomMoney() int64 {
	return RandomInt(0, 1000)
}

// RandomEmail generates a random email
func RandomEmail() string {
	return fmt.Sprintf("%s@email.com", RandomString(6))
}

// ValueOrDefault returns the value of the pointer or a default value if the pointer is nil
func ValueOrDefault[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}
	return *ptr
}
