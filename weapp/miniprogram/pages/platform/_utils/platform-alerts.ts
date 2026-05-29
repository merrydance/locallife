export interface PlatformAlertData {
  alert_type: string
  level: string
  title: string
  message: string
  related_id: number
  related_type: string
  extra?: Record<string, unknown>
  timestamp?: string
}

export interface ActionableAbnormalRefundAlert {
  alertType: string
  level: string
  title: string
  message: string
  relatedId: number
  relatedType: string
  timestamp: string
  refundOrderId: number
  paymentOrderId: number
  refundId: string
  method: string
  path: string
  defaultType: string
  supportedTypes: string[]
  userBankCardRequiredFields: string[]
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }

  return value as Record<string, unknown>
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function asNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.map((item) => asString(item)).filter(Boolean)
    : []
}

export function parsePlatformAlertData(value: unknown): PlatformAlertData | null {
  const payload = asRecord(value)
  if (!payload) {
    return null
  }

  const alertType = asString(payload.alert_type)
  const title = asString(payload.title)
  const message = asString(payload.message)
  const relatedType = asString(payload.related_type)

  if (!alertType || !title || !message || !relatedType) {
    return null
  }

  return {
    alert_type: alertType,
    level: asString(payload.level),
    title,
    message,
    related_id: asNumber(payload.related_id),
    related_type: relatedType,
    extra: asRecord(payload.extra) || undefined,
    timestamp: asString(payload.timestamp)
  }
}

export function toActionableAbnormalRefundAlert(value: unknown): ActionableAbnormalRefundAlert | null {
  const alert = parsePlatformAlertData(value)
  if (!alert || !alert.extra) {
    return null
  }

  if (alert.alert_type !== 'REFUND_FAILED' || alert.related_type !== 'refund_order') {
    return null
  }

  if (alert.extra.abnormal_refund_api_available !== true) {
    return null
  }

  const refundId = asString(alert.extra.refund_id)
  const method = asString(alert.extra.abnormal_refund_api_method)
  const path = asString(alert.extra.abnormal_refund_api_path)
  const defaultType = asString(alert.extra.abnormal_refund_default_type)
  const supportedTypes = asStringArray(alert.extra.abnormal_refund_supported_types)
  const userBankCardRequiredFields = asStringArray(alert.extra.abnormal_refund_user_bank_card_required_fields)
  const refundOrderId = asNumber(alert.extra.refund_order_id) || alert.related_id
  const paymentOrderId = asNumber(alert.extra.payment_order_id)

  if (!refundId || !method || !path || !defaultType || refundOrderId <= 0) {
    return null
  }

  return {
    alertType: alert.alert_type,
    level: alert.level,
    title: alert.title,
    message: alert.message,
    relatedId: alert.related_id,
    relatedType: alert.related_type,
    timestamp: alert.timestamp || new Date().toISOString(),
    refundOrderId,
    paymentOrderId,
    refundId,
    method,
    path,
    defaultType,
    supportedTypes,
    userBankCardRequiredFields
  }
}

export function formatPlatformAlertTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return '刚刚'
  }

  const now = new Date()
  const diffMinutes = Math.floor((now.getTime() - date.getTime()) / 60000)
  if (diffMinutes < 1) return '刚刚'
  if (diffMinutes < 60) return `${diffMinutes}分钟前`

  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hours}:${minutes}`
}

export function buildAbnormalRefundClipboardText(alert: ActionableAbnormalRefundAlert): string {
  const lines = [
    '异常退款人工处理参数',
    `告警标题: ${alert.title}`,
    `告警时间: ${formatPlatformAlertTime(alert.timestamp)}`,
    `退款单ID: ${alert.refundOrderId}`,
    `支付单ID: ${alert.paymentOrderId || '-'}`,
    `微信退款ID: ${alert.refundId}`,
    `接口: ${alert.method} ${alert.path}`,
    '权限: 平台管理员',
    `默认退款去向: ${alert.defaultType}`,
    `支持退款去向: ${alert.supportedTypes.join(' / ') || alert.defaultType}`
  ]

  if (alert.userBankCardRequiredFields.length > 0) {
    lines.push(`收款到用户银行卡需补充: ${alert.userBankCardRequiredFields.join('、')}`)
  }

  lines.push(`说明: ${alert.message}`)

  return lines.join('\n')
}
