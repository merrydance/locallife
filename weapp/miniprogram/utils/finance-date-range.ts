export interface FinanceDateRangeLike {
  start_date: string
  end_date: string
}

export interface FinanceDateRangeValidation {
  valid: boolean
  message: string
}

const DAY_MS = 24 * 60 * 60 * 1000

export function parseFinanceDateValue(value?: string): Date | null {
  if (!value) {
    return null
  }

  const date = new Date(value.replace(/-/g, '/'))
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return date
}

export function getFinanceDateTime(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
}

export function formatFinanceDateParam(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function getFinanceRangeCalendarValue(range: FinanceDateRangeLike): number[] {
  const start = parseFinanceDateValue(range.start_date)
  const end = parseFinanceDateValue(range.end_date)
  if (!start || !end) {
    return []
  }
  return [getFinanceDateTime(start), getFinanceDateTime(end)]
}

export function getFinanceRangeSpanDays(range: FinanceDateRangeLike): number | null {
  const start = parseFinanceDateValue(range.start_date)
  const end = parseFinanceDateValue(range.end_date)
  if (!start || !end) {
    return null
  }
  return (getFinanceDateTime(end) - getFinanceDateTime(start)) / DAY_MS
}

export function validateFinanceDateRange(
  range: FinanceDateRangeLike,
  maxDays: number,
  label: string
): FinanceDateRangeValidation {
  const spanDays = getFinanceRangeSpanDays(range)
  if (spanDays === null) {
    return { valid: false, message: '请选择完整日期区间' }
  }
  if (spanDays < 0) {
    return { valid: false, message: '开始日期不能晚于结束日期' }
  }
  if (maxDays > 0 && spanDays > maxDays) {
    return { valid: false, message: `${label}最多选择${maxDays}天` }
  }
  return { valid: true, message: '' }
}
