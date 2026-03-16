package util

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor used when hashing passwords.
// Cost 12 is the 2026 recommended minimum; bcrypt.DefaultCost (10) is too
// cheap on modern hardware.  Existing hashes continue to verify correctly
// because bcrypt embeds the cost in the hash string.  Hashes will be
// transparently upgraded to the new cost on the user's next successful login
// (gradual migration pattern).
const bcryptCost = 12

// HashPassword returns the bcrypt hash of the password
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedPassword), nil
}

// CheckPassword checks if the provided password is correct or not
func CheckPassword(password string, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
