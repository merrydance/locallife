package api

import (
	"fmt"
	"strconv"
	"strings"
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

// normalizeAddress 标准化地址字符串（去除空格、转小写、中文数字转换），用于模糊匹配去重
func normalizeAddress(addr string) string {
	// 1. 去除空白字符
	s := strings.ReplaceAll(addr, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")

	// 2. 转小写
	s = strings.ToLower(s)

	// 3. 中文数字标准化 (简单映射)
	replacer := strings.NewReplacer(
		"一", "1",
		"二", "2",
		"三", "3",
		"四", "4",
		"五", "5",
		"六", "6",
		"七", "7",
		"八", "8",
		"九", "9",
		"零", "0",
		"号", "", // 去除'号'，使 '1号' == '1'
		"室", "", // 去除'室'
		"栋", "", // 去除'栋'
	)
	s = replacer.Replace(s)

	return s
}
