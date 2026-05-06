package contracts

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

func validateOfficialMaxLength(prefix, field, value string, max int) error {
	if utf8.RuneCountInString(strings.TrimSpace(value)) > max {
		return errors.New(prefix + " " + field + " must be at most " + strconv.Itoa(max) + " characters")
	}
	return nil
}
