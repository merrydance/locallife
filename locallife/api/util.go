package api

import (
	"fmt"
	"strings"

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
