import { getErrorDebugMessage, getErrorUserMessage } from '../../../utils/user-facing'

const DEFAULT_REPLY_ERROR_MESSAGE = '回复评价失败，请稍后重试'
const MISSING_WECHAT_OPENID_MESSAGE = '当前微信登录信息不完整，请重新登录后再回复'
const TEXT_CONTENT_SAFETY_FAILED_MESSAGE = '回复内容未通过安全检查，请调整后再提交'
const CONTENT_SAFETY_PROVIDER_UNAVAILABLE_MESSAGE = '内容安全检查暂不可用，请稍后重试'

interface ErrorWithStatusCode {
  statusCode?: unknown
  code?: unknown
}

function getNumericStatusCode(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') {
    return undefined
  }

  const knownError = error as ErrorWithStatusCode
  const candidates = [knownError.statusCode, knownError.code]

  for (const candidate of candidates) {
    const numericCode = typeof candidate === 'number' ? candidate : Number(candidate)
    if (Number.isFinite(numericCode)) {
      return numericCode
    }
  }

  return undefined
}

export function getMerchantReviewReplyErrorMessage(
  error: unknown,
  fallback: string = DEFAULT_REPLY_ERROR_MESSAGE
): string {
  const debugMessage = getErrorDebugMessage(error).toLowerCase()

  if (debugMessage.includes('missing wechat openid')) {
    return MISSING_WECHAT_OPENID_MESSAGE
  }

  if (debugMessage.includes('text content safety check failed')) {
    return TEXT_CONTENT_SAFETY_FAILED_MESSAGE
  }

  if (debugMessage.includes('wechat msg sec check')) {
    return CONTENT_SAFETY_PROVIDER_UNAVAILABLE_MESSAGE
  }

  const statusCode = getNumericStatusCode(error)
  if (statusCode === 502) {
    return CONTENT_SAFETY_PROVIDER_UNAVAILABLE_MESSAGE
  }

  return getErrorUserMessage(error, fallback)
}
