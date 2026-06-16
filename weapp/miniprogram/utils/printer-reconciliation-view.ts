import {
  buildPrinterReconciliationJobStatusView,
  type PrinterReconciliationJobResponse,
  type PrinterReconciliationStatusTheme,
  type PrinterType
} from '../api/table-device-management'

const PRINTER_TYPE_LABELS: Record<PrinterType, string> = {
  feieyun: '飞鹅云',
  shangpeng: '商鹏云',
  self_cloud: '乐客来福云打印机',
  yilianyun: '易联云',
  other: '其他'
}

export interface PrinterReconciliationJobView extends PrinterReconciliationJobResponse {
  display_printer_name: string
  printer_type_label: string
  desired_action_label: string
  source_action_label: string
  status_label: string
  status_theme: PrinterReconciliationStatusTheme
  issue_title: string
  action_hint: string
  retry_count_label: string
  created_at_label: string
  updated_at_label: string
}

export function buildPrinterTypeLabel(type?: string) {
  if (!type) return '未设置'
  return PRINTER_TYPE_LABELS[type as PrinterType] || type
}

function desiredActionLabel(action?: string) {
  switch (action) {
    case 'register':
      return '恢复注册'
    case 'remove':
      return '恢复移除'
    default:
      return '恢复同步'
  }
}

function sourceActionLabel(action?: string) {
  switch (action) {
    case 'create':
      return '添加设备'
    case 'delete':
      return '删除设备'
    default:
      return '设备变更'
  }
}

function actionHint(action?: string) {
  switch (action) {
    case 'register':
      return '系统会重新向云打印平台注册这台设备。'
    case 'remove':
      return '系统会重新向云打印平台移除这台设备。'
    default:
      return '系统会重新同步这台设备的云端状态。'
  }
}

export function buildPrinterReconciliationJobView(
  job: PrinterReconciliationJobResponse,
  formatTimeLabel: (value?: string) => string
): PrinterReconciliationJobView {
  const statusView = buildPrinterReconciliationJobStatusView(job.status)
  const displayPrinterName = job.printer_name || '打印设备'
  return {
    ...job,
    display_printer_name: displayPrinterName,
    printer_type_label: buildPrinterTypeLabel(job.printer_type),
    desired_action_label: desiredActionLabel(job.desired_action),
    source_action_label: sourceActionLabel(job.source_action),
    status_label: statusView.label,
    status_theme: statusView.theme,
    issue_title: `${displayPrinterName}同步异常`,
    action_hint: actionHint(job.desired_action),
    retry_count_label: job.retry_count > 0 ? `已重试 ${job.retry_count} 次` : '尚未重试',
    created_at_label: formatTimeLabel(job.created_at),
    updated_at_label: formatTimeLabel(job.updated_at)
  }
}

export function buildReconciliationLoadErrorMessage(message: string, hasTrustedJobs: boolean) {
  return hasTrustedJobs ? `${message}，当前已保留上次同步结果` : message
}
