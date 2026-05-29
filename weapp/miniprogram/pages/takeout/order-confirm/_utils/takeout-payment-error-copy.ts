const MERCHANT_QUALIFICATION_INCOMPLETE_MESSAGE = '该商户资质不完整，暂不支持下单'
const PAYMENT_CREATE_FAILED_FALLBACK = '支付创建失败，请在订单详情页重新发起支付。'

function collectErrorMessages(error: unknown, messages: string[] = []): string[] {
  if (!error) {
    return messages
  }

  if (typeof error === 'string') {
    messages.push(error)
    return messages
  }

  if (error instanceof Error) {
    messages.push(error.message)
  }

  if (typeof error !== 'object') {
    return messages
  }

  const knownError = error as {
    userMessage?: unknown
    message?: unknown
    detailMessage?: unknown
    errMsg?: unknown
    data?: { message?: unknown }
    body?: { message?: unknown }
    originalError?: unknown
  }

  const candidates = [
    knownError.userMessage,
    knownError.message,
    knownError.detailMessage,
    knownError.errMsg,
    knownError.data?.message,
    knownError.body?.message
  ]

  candidates.forEach((candidate) => {
    if (typeof candidate === 'string' && candidate.trim()) {
      messages.push(candidate)
    }
  })

  if (knownError.originalError && knownError.originalError !== error) {
    collectErrorMessages(knownError.originalError, messages)
  }

  return messages
}

export function isMerchantQualificationPaymentBlocked(error: unknown): boolean {
  const combinedMessage = collectErrorMessages(error).join(' ').replace(/\s+/g, ' ').trim()

  if (!combinedMessage) {
    return false
  }

  return (
    combinedMessage.includes('商户结算账户未开通') ||
    combinedMessage.includes('商户微信支付通道待开通') ||
    combinedMessage.includes('暂不能创建支付订单') ||
    combinedMessage.includes('暂不能创建微信生态支付订单')
  )
}

export function getTakeoutPaymentCreateFailedContent(error: unknown): string {
  if (isMerchantQualificationPaymentBlocked(error)) {
    return MERCHANT_QUALIFICATION_INCOMPLETE_MESSAGE
  }

  return PAYMENT_CREATE_FAILED_FALLBACK
}
