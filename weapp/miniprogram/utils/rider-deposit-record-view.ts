export interface DepositRecord {
  id: number
  amount: number
  type: string
  created_at: string
  status?: string
  remark?: string
}

export interface DepositRecordView extends DepositRecord {
  display_type_text: string
  display_remark?: string
  display_time: string
  display_amount_text: string
  display_amount_class: 'positive' | 'negative'
  icon_color: string
  icon_name: 'add-circle' | 'remove-circle'
  status_text: string
  status_theme: 'primary' | 'success' | 'warning' | 'default'
}

const SUCCESS_ICON_COLOR = 'var(--td-success-color)'
const DEFAULT_ICON_COLOR = 'var(--td-text-color-primary)'

const transactionTypeTextMap: Record<string, string> = {
  deposit: '押金充值',
  recharge: '押金充值',
  freeze: '接单预扣',
  unfreeze: '押金解冻',
  deduct: '押金扣减',
  refund: '押金退回',
  withdraw: '账单变动',
  withdraw_rollback: '提现回滚',
  split_income: '分账入账（微信零钱）',
  income: '分账入账（微信零钱）',
  earning: '分账入账（微信零钱）'
}

const transactionAmountSignMap: Record<string, 1 | -1> = {
  deposit: 1,
  recharge: 1,
  unfreeze: 1,
  refund: 1,
  withdraw_rollback: 1,
  split_income: 1,
  income: 1,
  earning: 1,
  freeze: -1,
  deduct: -1,
  withdraw: -1
}

function getTransactionSign(type: string): 1 | -1 | 0 {
  return transactionAmountSignMap[type] || 0
}

export function formatFenValue(amount: number): string {
  return (Math.max(amount, 0) / 100).toFixed(2)
}

function formatTransactionTime(timeText?: string): string {
  if (!timeText) {
    return '--'
  }

  const date = new Date(timeText)
  if (Number.isNaN(date.getTime())) {
    return timeText
  }

  return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function formatTransactionAmountValue(amount: number, type: string): string {
  const sign = getTransactionSign(type)
  if (sign === 1) {
    return `+${(Math.abs(amount) / 100).toFixed(2)}`
  }
  if (sign === -1) {
    return `-${(Math.abs(amount) / 100).toFixed(2)}`
  }
  const raw = amount / 100
  const prefix = raw > 0 ? '+' : ''
  return `${prefix}${raw.toFixed(2)}`
}

export function decorateDepositRecord(record: DepositRecord): DepositRecordView {
  const remark = record.remark || ''
  let displayTypeText = transactionTypeTextMap[record.type] || '账单变动'
  let displayRemark = remark
  let statusText = '已完成'
  let statusTheme: DepositRecordView['status_theme'] = 'default'

  if (record.type === 'freeze') {
    if (remark === '接单冻结押金') {
      displayTypeText = '配送冻结'
      displayRemark = '订单配送中，押金暂时冻结。'
      statusText = '冻结中'
      statusTheme = 'warning'
    } else if (remark === '押金提现冻结') {
      displayTypeText = '提现处理中'
      displayRemark = '提现申请处理中，到账前金额暂不可用。'
      statusText = '处理中'
      statusTheme = 'warning'
    }
  }

  if (record.type === 'unfreeze') {
    if (remark === '配送完成解冻押金') {
      displayTypeText = '配送解冻'
      displayRemark = '订单已完成，配送冻结已释放。'
      statusText = '已释放'
      statusTheme = 'success'
    } else if (remark === '订单取消解冻押金') {
      displayTypeText = '取消退回'
      displayRemark = '订单取消后，配送冻结已退回可用押金。'
      statusText = '已退回'
      statusTheme = 'success'
    } else if (remark === '押金退款失败解冻') {
      displayTypeText = '提现退回'
      displayRemark = '提现未成功，金额已退回可用押金。'
      statusText = '已退回'
      statusTheme = 'default'
    }
  }

  if (record.type === 'withdraw' && remark === '押金退款提现成功') {
    displayTypeText = '提现完成'
    displayRemark = '提现已退回微信零钱。'
    statusText = '已到账'
    statusTheme = 'success'
  }

  if ((record.type === 'deposit' || record.type === 'recharge') && remark === '微信支付充值') {
    displayRemark = '已通过微信支付完成押金充值。'
    statusText = '已充值'
    statusTheme = 'primary'
  }

  const sign = getTransactionSign(record.type)
  const iconName = sign === 1 ? 'add-circle' : 'remove-circle'
  const displayAmountClass: DepositRecordView['display_amount_class'] = sign === 1 ? 'positive' : 'negative'

  return {
    ...record,
    display_type_text: displayTypeText,
    display_remark: displayRemark || undefined,
    display_time: formatTransactionTime(record.created_at),
    display_amount_text: formatTransactionAmountValue(record.amount, record.type),
    display_amount_class: displayAmountClass,
    icon_color: displayAmountClass === 'positive' ? SUCCESS_ICON_COLOR : DEFAULT_ICON_COLOR,
    icon_name: iconName,
    status_text: statusText,
    status_theme: statusTheme
  }
}