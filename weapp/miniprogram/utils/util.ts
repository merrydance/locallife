export const formatTime = (date: Date) => {
  const year = date.getFullYear()
  const month = date.getMonth() + 1
  const day = date.getDate()
  const hour = date.getHours()
  const minute = date.getMinutes()
  const second = date.getSeconds()

  return (
    `${[year, month, day].map(formatNumber).join('/')
    } ${[hour, minute, second].map(formatNumber).join(':')}`
  )
}

const formatNumber = (n: number) => {
  const s = n.toString()
  return s[1] ? s : `0${s}`
}

/**
 * 格式化价格（分转元）
 * @param amount 金额（分）
 * @param withSymbol 是否带￥符号，默认 true
 * @returns 格式化后的价格字符串
 */
export const formatPrice = (amount: number, withSymbol: boolean = true): string => {
  const yuan = (amount / 100).toFixed(2)
  return withSymbol ? `¥${yuan}` : yuan
}

/**
 * 格式化价格（不带符号，用于 WXML）
 */
export const formatPriceNoSymbol = (amount: number): string => {
  return (amount / 100).toFixed(2)
}
