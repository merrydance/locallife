interface ErrorWithUserMessage {
  userMessage?: unknown
  message?: unknown
  detailMessage?: unknown
  errMsg?: unknown
  code?: unknown
  statusCode?: unknown
  data?: {
    message?: unknown
  }
  body?: {
    message?: unknown
  }
  originalError?: {
    message?: unknown
  }
}

const DIAGNOSTIC_MARKERS = [
  'bad gateway',
  'gateway timeout',
  'service unavailable',
  'internal server error',
  'sql',
  'exception',
  'stack',
  'traceback',
  'request:fail',
  'timeout',
  'timed out',
  'forbidden',
  'unauthorized',
  'not found',
  'no rows in result set',
  'duplicate key',
  'constraint',
  'jwt',
  'token',
  'nginx',
  'panic',
  'http '
]

const SAFE_COPY_PREFIXES = [
  '请',
  '已',
  '当前',
  '暂无',
  '未',
  '需要',
  '至少',
  '无法',
  '正在',
  '定位',
  '网络',
  '支付',
  '上传',
  '提交',
  '保存',
  '加载',
  '添加',
  '删除',
  '更新',
  '切换',
  '商户',
  '微信支付',
  '账号',
  '订单',
  '申请',
  '图片',
  '内容',
  '服务',
  '登录'
]

function asNonEmptyString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined
  }

  const normalized = value.replace(/\s+/g, ' ').trim()
  return normalized || undefined
}

function containsChinese(text: string): boolean {
  return /[\u4e00-\u9fff]/.test(text)
}

function isEnglishOnlyMessage(text: string): boolean {
  return /[a-zA-Z]/.test(text) && !containsChinese(text)
}

function looksLikeDiagnosticMessage(text: string): boolean {
  const normalized = text.toLowerCase()
  return DIAGNOSTIC_MARKERS.some((marker) => normalized.includes(marker))
}

function isLikelyUserSafeCopy(text: string): boolean {
  if (!text || text.length > 180 || text.includes('\n')) {
    return false
  }

  if (looksLikeDiagnosticMessage(text)) {
    return false
  }

  if (!containsChinese(text)) {
    return false
  }

  if (SAFE_COPY_PREFIXES.some((prefix) => text.startsWith(prefix))) {
    return true
  }

  return /请|联系|重试|前往|稍后|核对|刷新/.test(text)
}

function getStatusCode(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') {
    return undefined
  }

  const knownError = error as ErrorWithUserMessage
  const candidates = [knownError.statusCode, knownError.code]

  for (const candidate of candidates) {
    const numericCode = typeof candidate === 'number' ? candidate : Number(candidate)
    if (Number.isFinite(numericCode)) {
      return numericCode
    }
  }

  return undefined
}

function mapErrorByStatusCode(statusCode: number, fallback: string): string | undefined {
  if (statusCode === 400) {
    return fallback
  }
  if (statusCode === 401) {
    return '登录状态已失效，请重新进入后再试'
  }
  if (statusCode === 403) {
    return '当前无权限执行该操作'
  }
  if (statusCode === 404) {
    return '所需信息暂时不可用，请稍后刷新再试'
  }
  if (statusCode === 409) {
    return '当前操作暂时无法完成，请刷新后再试'
  }
  if (statusCode === 422) {
    return fallback
  }
  if (statusCode === 429) {
    return '请求太频繁，请稍后再试'
  }
  if (statusCode >= 500) {
    return '服务暂时不可用，请稍后再试'
  }

  return undefined
}

export function mapBackendMessageToUserMessage(rawMessage: string, fallback: string): string {
  const message = rawMessage.replace(/\s+/g, ' ').trim()
  if (!message) {
    return fallback
  }

  const normalized = message.toLowerCase()

  if (
    normalized.includes('too many requests') ||
    normalized.includes('rate limit') ||
    normalized.includes('429') ||
    normalized.includes('请求太频繁')
  ) {
    return '请求太频繁，请稍后再试'
  }

  if (
    normalized.includes('request:fail') ||
    normalized.includes('network error') ||
    normalized.includes('network request failed') ||
    normalized.includes('offline') ||
    normalized.includes('dns') ||
    normalized.includes('网络请求失败')
  ) {
    return '网络开小差了，请检查网络后重试'
  }

  if (
    normalized.includes('bad gateway') ||
    normalized.includes('service unavailable') ||
    normalized.includes('gateway timeout') ||
    normalized.includes('服务器内部错误') ||
    normalized.includes('后端服务不可用') ||
    normalized.includes('nginx')
  ) {
    return '服务暂时不可用，请稍后再试'
  }

  if (
    normalized.includes('timeout') ||
    normalized.includes('timed out') ||
    normalized.includes('超时')
  ) {
    return '处理时间有点久，请稍后再试'
  }

  if (
    normalized.includes('unauthorized') ||
    normalized.includes('token') ||
    normalized.includes('登录已过期') ||
    normalized.includes('认证失败')
  ) {
    return '登录状态已失效，请重新进入后再试'
  }

  if (
    normalized.includes('forbidden') ||
    normalized.includes('permission denied') ||
    normalized.includes('无权限') ||
    normalized.includes('权限不足')
  ) {
    return '当前无权限执行该操作'
  }

  if (
    normalized.includes('this endpoint requires merchant role') ||
    normalized.includes('merchant owner role not found') ||
    normalized.includes('not a merchant')
  ) {
    return '当前角色不支持使用此功能，请切换到商户角色后再试。'
  }

  if (
    normalized.includes('this endpoint requires rider role') ||
    normalized.includes('rider role not found') ||
    normalized.includes('not a rider')
  ) {
    return '当前角色不支持使用此功能，请切换到骑手角色后再试。'
  }

  if (
    normalized.includes('this endpoint requires operator role') ||
    normalized.includes('operator role not found') ||
    normalized.includes('operator not found in context')
  ) {
    return '当前角色不支持使用此功能，请切换到运营角色后再试。'
  }

  if (
    normalized.includes('this endpoint requires admin role') ||
    normalized.includes('platform role not found') ||
    normalized.includes('admin role not found')
  ) {
    return '当前角色不支持使用此功能，请切换到平台管理员角色后再试。'
  }

  if (
    normalized.includes('operator account is not active') ||
    normalized.includes('operator is not active') ||
    normalized.includes('账号未激活') ||
    normalized.includes('尚未完成开户')
  ) {
    return '当前账号状态未生效，暂时无法使用该功能，请联系平台处理。'
  }

  const legacyAccountName = '收' + '付通'
  const applymentName = '进' + '件'
  if (
    normalized.includes(`${legacyAccountName}账户未激活`) ||
    normalized.includes(`尚未开通${legacyAccountName}账户`) ||
    normalized.includes(`完成${applymentName}签约`) ||
    normalized.includes(`完成${applymentName}流程`)
  ) {
    return '请先开通微信支付后再恢复营业'
  }

  if (normalized.includes('wallet account not bound')) {
    return '请前往微信支付商户平台/商家助手处理提现账户和提现申请'
  }

  if (normalized.includes('insufficient balance')) {
    return '可用余额不足'
  }

  if (normalized.includes('merchant is closed')) {
    return '商户休息中，请稍后再来'
  }

  if (normalized.includes('invalid coordinates')) {
    return '位置信息已失效，请重新定位'
  }

  if (
    normalized.includes('image moderation is pending') ||
    normalized.includes('内容审核中') ||
    normalized.includes('多媒体内容安全审查') ||
    normalized.includes('安全审查系统拦截')
  ) {
    return '图片暂时无法使用，请更换图片后重试'
  }

  if (
    normalized.includes('invalid media id') ||
    normalized.includes('upload session not found') ||
    normalized.includes('session not found') ||
    normalized.includes('asset not found')
  ) {
    return '图片已失效，请重新上传'
  }

  if (
    normalized.includes('ocr timeout') ||
    normalized.includes('recognize timeout') ||
    normalized.includes('recognition timeout')
  ) {
    return '识别超时，请稍后重试'
  }

  if (
    normalized.includes('ocr failed') ||
    normalized.includes('recognize failed') ||
    normalized.includes('recognition failed')
  ) {
    return '识别失败，请提供更清晰更规整的图片重试'
  }

  if (
    normalized.includes('already submitted') ||
    normalized.includes('application submitted') ||
    normalized.includes('申请已提交')
  ) {
    return '申请已提交，请等待审核'
  }

  if (
    normalized.includes('application can only be submitted in draft state') ||
    normalized.includes('当前申请状态暂不支持再次提交')
  ) {
    return '当前申请状态暂不支持再次提交，请返回查看审核进度'
  }

  if (
    normalized.includes('application can only be modified in draft state') ||
    normalized.includes('暂时不能修改资料')
  ) {
    return '申请已提交，暂时不能修改资料'
  }

  if (
    normalized.includes('already approved') ||
    normalized.includes('application approved') ||
    normalized.includes('已通过')
  ) {
    return '申请已通过，无需重复提交'
  }

  if (
    normalized.includes('no application') ||
    normalized.includes('application not found') ||
    normalized.includes('您还没有申请记录')
  ) {
    return '暂无申请记录'
  }

  if (
    normalized.includes('already') ||
    normalized.includes('duplicate') ||
    normalized.includes('conflict') ||
    normalized.includes('冲突')
  ) {
    return '请勿重复提交，刷新后再试'
  }

  if (
    normalized.includes('not found') ||
    normalized.includes('服务未找到') ||
    normalized.includes('no rows in result set')
  ) {
    return '所需信息暂时不可用，请稍后刷新再试'
  }

  if (isLikelyUserSafeCopy(message)) {
    return message
  }

  if (isEnglishOnlyMessage(message) || looksLikeDiagnosticMessage(message)) {
    return fallback
  }

  return fallback
}

export function getErrorDebugMessage(error: unknown): string {
  if (typeof error === 'string') {
    return error.trim()
  }

  if (!error || typeof error !== 'object') {
    return ''
  }

  const knownError = error as ErrorWithUserMessage
  const candidates = [
    knownError.detailMessage,
    knownError.message,
    knownError.data?.message,
    knownError.body?.message,
    knownError.originalError?.message,
    knownError.errMsg
  ]

  for (const candidate of candidates) {
    const normalized = asNonEmptyString(candidate)
    if (normalized) {
      return normalized
    }
  }

  return ''
}

export function getErrorUserMessage(error: unknown, fallback: string): string {
  if (!error) {
    return fallback
  }

  if (typeof error === 'string') {
    if (isLikelyUserSafeCopy(error)) {
      return error.trim()
    }
    return mapBackendMessageToUserMessage(error, fallback)
  }

  if (typeof error !== 'object') {
    return fallback
  }

  const knownError = error as ErrorWithUserMessage
  const directUserMessage = asNonEmptyString(knownError.userMessage)
  if (directUserMessage) {
    return directUserMessage
  }

  const candidates = [
    knownError.message,
    knownError.detailMessage,
    knownError.data?.message,
    knownError.body?.message,
    knownError.originalError?.message,
    knownError.errMsg
  ]

  for (const candidate of candidates) {
    const text = asNonEmptyString(candidate)
    if (!text) {
      continue
    }

    if (isLikelyUserSafeCopy(text)) {
      return text
    }

    const mapped = mapBackendMessageToUserMessage(text, fallback)
    if (mapped !== fallback) {
      return mapped
    }
  }

  const statusCode = getStatusCode(error)
  if (typeof statusCode === 'number') {
    return mapErrorByStatusCode(statusCode, fallback) || fallback
  }

  return fallback
}

export function isRateLimitError(error: unknown): boolean {
  const statusCode = getStatusCode(error)
  if (statusCode === 429) {
    return true
  }

  const debugMessage = getErrorDebugMessage(error).toLowerCase()
  return debugMessage.includes('429') || debugMessage.includes('too many requests') || debugMessage.includes('请求太频繁')
}

export function isRetryableNetworkError(error: unknown): boolean {
  const statusCode = getStatusCode(error)
  if (statusCode === 502 || statusCode === 503 || statusCode === 504) {
    return true
  }

  const debugMessage = getErrorDebugMessage(error).toLowerCase()
  return (
    debugMessage.includes('502') ||
    debugMessage.includes('503') ||
    debugMessage.includes('504') ||
    debugMessage.includes('timeout') ||
    debugMessage.includes('timed out') ||
    debugMessage.includes('request:fail') ||
    debugMessage.includes('network request failed') ||
    debugMessage.includes('网络请求失败')
  )
}
