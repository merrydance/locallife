package api

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

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
