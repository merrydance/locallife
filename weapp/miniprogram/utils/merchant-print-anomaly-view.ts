import { MerchantPrintAnomalyItem } from '../api/order-management'

export type PrintAnomalyTheme = 'warning' | 'danger' | 'default'

interface PrintAnomalyStatusView {
  label: string
  theme: PrintAnomalyTheme
  summary: string
}

function isFailedPrintAnomalyStatus(status?: string) {
  return String(status || '').trim().toLowerCase() === 'failed'
}

function isPendingPrintAnomalyStatus(status?: string) {
  return String(status || '').trim().toLowerCase() === 'pending'
}

export function formatPrintAnomalyRetryHint(retryHint?: string) {
  if (!retryHint) return ''
  if (retryHint === 'printer is inactive') {
    return '该打印机当前已停用，请先启用打印机后再重试。'
  }
  if (retryHint === 'printer type is not supported for retry') {
    return '当前打印机类型暂不支持小程序端重试，请到设备配置中核对品牌类型。'
  }
  return retryHint
}

export function getPrintAnomalyStatusView(item: Pick<MerchantPrintAnomalyItem, 'local_status' | 'retry_hint' | 'error_message'>): PrintAnomalyStatusView {
  if (item.retry_hint) {
    return {
      label: isFailedPrintAnomalyStatus(item.local_status) ? '打印失败' : isPendingPrintAnomalyStatus(item.local_status) ? '待回执' : '状态同步中',
      theme: isFailedPrintAnomalyStatus(item.local_status) ? 'danger' : isPendingPrintAnomalyStatus(item.local_status) ? 'warning' : 'default',
      summary: formatPrintAnomalyRetryHint(item.retry_hint)
    }
  }

  if (item.error_message && isFailedPrintAnomalyStatus(item.local_status)) {
    return {
      label: '打印失败',
      theme: 'danger',
      summary: '最近一次打印已明确失败，请先查看失败原因，再决定是否重试补打。'
    }
  }

  if (isPendingPrintAnomalyStatus(item.local_status)) {
    return {
      label: '待回执',
      theme: 'warning',
      summary: '打印任务仍未收到回执，请确认门店设备和云打印平台状态。'
    }
  }

  return {
    label: '状态同步中',
    theme: 'default',
    summary: '打印任务状态异常，请尽快处理。'
  }
}