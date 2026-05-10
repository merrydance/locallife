package worker

import (
	"regexp"
	"strings"
)

func normalizeRiderOCRDateText(value string) string {
	value = strings.ReplaceAll(value, " 年", "年")
	value = strings.ReplaceAll(value, "年 ", "年")
	value = strings.ReplaceAll(value, " 月", "月")
	value = strings.ReplaceAll(value, "月 ", "月")
	value = strings.ReplaceAll(value, " 日", "日")
	value = strings.ReplaceAll(value, "日 ", "日")
	separatorSpaceRegex := regexp.MustCompile(`\s*([./-])\s*`)
	value = separatorSpaceRegex.ReplaceAllString(value, "$1")
	return strings.TrimSpace(value)
}

func applyRiderHealthCertValidPeriod(data *riderHealthCertOCRData, raw string) {
	normalized := normalizeRiderOCRDateText(raw)
	if normalized == "" {
		return
	}

	datePattern := `\d{4}\s*(?:年|[./-])\s*\d{1,2}\s*(?:月|[./-])\s*\d{1,2}\s*日?`
	validRangeRegex := regexp.MustCompile(`(` + datePattern + `)\s*(?:至|到|-|—|~|～)\s*(` + datePattern + `|长期)`)
	if match := validRangeRegex.FindStringSubmatch(normalized); len(match) > 2 {
		data.ValidStart = normalizeRiderOCRDateText(match[1])
		data.ValidEnd = normalizeRiderOCRDateText(match[2])
		return
	}

	if strings.Contains(normalized, "长期") {
		data.ValidEnd = "长期"
		return
	}

	data.ValidEnd = normalized
}

func applyRiderHealthCertStructuredValidPeriod(data *riderHealthCertOCRData, raw string) {
	if strings.TrimSpace(data.ValidEnd) != "" {
		return
	}
	var parsed riderHealthCertOCRData
	applyRiderHealthCertValidPeriod(&parsed, raw)
	if strings.TrimSpace(parsed.ValidEnd) == "" {
		return
	}
	data.ValidStart = parsed.ValidStart
	data.ValidEnd = parsed.ValidEnd
}

func parseRiderHealthCertOCRText(data *riderHealthCertOCRData, text string) {
	idRegex := regexp.MustCompile(`\b\d{17}[0-9Xx]\b`)
	if match := idRegex.FindString(text); match != "" {
		data.IDNumber = strings.ToUpper(match)
	}
	trimHealthCertName := func(candidate string) string {
		candidate = strings.TrimSpace(candidate)
		candidate = regexp.MustCompile(`(?:性别\s*[:：]?.*|男|女).*$`).ReplaceAllString(candidate, "")
		candidate = regexp.MustCompile(`[^\p{Han}·]`).ReplaceAllString(candidate, "")
		candidate = strings.TrimSpace(candidate)
		if len([]rune(candidate)) < 2 {
			return ""
		}
		return candidate
	}
	namePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)(?:从业人员姓名|持证人|体检者|姓名)\s*[:：]?\s*([^\n\r]{2,20})`),
		regexp.MustCompile(`(?m)([^\n\r]{2,20})\s*(?:性别\s*[:：]?\s*(?:男|女)|男|女)`),
	}
	for _, nameRegex := range namePatterns {
		if match := nameRegex.FindStringSubmatch(text); len(match) > 1 {
			candidate := trimHealthCertName(match[1])
			if candidate != "" {
				data.Name = candidate
				break
			}
		}
	}
	certRegex := regexp.MustCompile(`(?m)(?:健康证号|证书编号|证书号|证号|编号)\s*[:：]?\s*([A-Za-z0-9\-]{5,})`)
	if match := certRegex.FindStringSubmatch(text); len(match) > 1 {
		data.CertNumber = strings.TrimSpace(match[1])
	}
	datePattern := `\d{4}\s*(?:年|[./-])\s*\d{1,2}\s*(?:月|[./-])\s*\d{1,2}\s*日?`
	validToPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:有效期至|有效期限至|有效截止日期|截止日期|截止到|到期日期|到期日|有效日期至|有效期到)\s*[:：]?\s*(` + datePattern + `|长期)`),
		regexp.MustCompile(`(?:有效日期|有效期|有效期限)\s*[:：]?\s*` + datePattern + `\s*(?:至|到|-|—|~|～)\s*(` + datePattern + `|长期)`),
	}
	for _, validToRegex := range validToPatterns {
		if match := validToRegex.FindStringSubmatch(text); len(match) > 1 {
			applyRiderHealthCertValidPeriod(data, match[1])
			break
		}
	}
	validRangeRegex := regexp.MustCompile(`(` + datePattern + `)\s*(?:至|到|-|—|~|～)\s*(` + datePattern + `|长期)`)
	if match := validRangeRegex.FindStringSubmatch(text); len(match) > 2 {
		data.ValidStart = normalizeRiderOCRDateText(match[1])
		data.ValidEnd = normalizeRiderOCRDateText(match[2])
	}
	if data.ValidEnd == "" {
		dateRegex := regexp.MustCompile(datePattern)
		matches := dateRegex.FindAllString(text, -1)
		if len(matches) > 0 {
			if len(matches) > 1 && data.ValidStart == "" {
				data.ValidStart = normalizeRiderOCRDateText(matches[0])
			}
			data.ValidEnd = normalizeRiderOCRDateText(matches[len(matches)-1])
		}
	}
	if data.ValidEnd == "" && strings.Contains(text, "长期") {
		data.ValidEnd = "长期"
	}
}
