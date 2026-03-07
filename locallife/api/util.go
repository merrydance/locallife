package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const fenPerYuan int64 = 100

// parseNumericToFloat 将 pgtype.Numeric 转换为 float64
func parseNumericToFloat(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, fmt.Errorf("numeric is not valid")
	}

	f, err := n.Float64Value()
	if err != nil {
		return 0, err
	}
	return f.Float64, nil
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}

func pgTextToPtr(val pgtype.Text) *string {
	if val.Valid {
		return &val.String
	}
	return nil
}

func pgInt8ToPtr(val pgtype.Int8) *int64 {
	if val.Valid {
		return &val.Int64
	}
	return nil
}

func pgTimeToPtr(val pgtype.Timestamptz) *time.Time {
	if val.Valid {
		return &val.Time
	}
	return nil
}

func yuanToFen(amount float64) int64 {
	return int64(amount * float64(fenPerYuan))
}

func fenToYuanString(amount int64, precision int) string {
	return strconv.FormatFloat(float64(amount)/float64(fenPerYuan), 'f', precision, 64)
}
