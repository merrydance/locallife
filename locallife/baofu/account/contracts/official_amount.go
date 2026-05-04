package contracts

import (
	"errors"
	"strconv"
	"strings"
)

func YuanStringToFen(raw string) (int64, error) {
	amount := strings.TrimSpace(raw)
	if amount == "" {
		return 0, errors.New("baofu amount is required")
	}
	if strings.HasPrefix(amount, "-") {
		return 0, errors.New("baofu amount must be non-negative")
	}
	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return 0, errors.New("baofu amount is invalid")
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, errors.New("baofu amount is invalid")
	}
	fen := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) > 2 {
			return 0, errors.New("baofu amount supports at most 2 decimal places")
		}
		decimal := parts[1]
		for len(decimal) < 2 {
			decimal += "0"
		}
		fen, err = strconv.ParseInt(decimal, 10, 64)
		if err != nil {
			return 0, errors.New("baofu amount is invalid")
		}
	}
	return yuan*100 + fen, nil
}

func FenToYuanString(amountFen int64) (string, error) {
	if amountFen < 0 {
		return "", errors.New("baofu amount fen must be non-negative")
	}
	return strconv.FormatInt(amountFen/100, 10) + "." + twoDigitFen(amountFen%100), nil
}

func twoDigitFen(fen int64) string {
	if fen < 10 {
		return "0" + strconv.FormatInt(fen, 10)
	}
	return strconv.FormatInt(fen, 10)
}
