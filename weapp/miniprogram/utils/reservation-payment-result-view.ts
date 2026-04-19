import { formatReservationStatus, ReservationStatus } from '../api/reservation'
import { getPaymentStatusView, isPaymentStatusSuccessful } from '../api/payment'

export type PaymentResultKind = 'success' | 'failed' | 'cancelled' | 'unknown'
export type ResultTheme = 'success' | 'warning' | 'error'

interface ReservationResultView {
  resultKind: PaymentResultKind
  resultTheme: ResultTheme
  navTitle: string
  resultTitle: string
  resultDescription: string
  primaryButtonText: string
  secondaryButtonText: string
  hintText: string
}

const RESERVATION_SUCCESS_STATUSES = new Set<ReservationStatus>(['paid', 'confirmed', 'checked_in', 'completed'])
const RESERVATION_FAILED_STATUSES = new Set<ReservationStatus>(['expired', 'no_show'])

export function normalizePaymentResultKind(result?: string): PaymentResultKind {
  if (result === 'failed' || result === 'cancelled' || result === 'unknown') {
    return result
  }
  return 'success'
}

export function mapReservationPaymentStatusToResultKind(status?: string): PaymentResultKind {
  if (isPaymentStatusSuccessful(status)) {
    return 'success'
  }

  const paymentStatusView = getPaymentStatusView(status)

  if (paymentStatusView.normalizedStatus === 'failed') {
    return 'failed'
  }

  if (paymentStatusView.normalizedStatus === 'cancelled' || paymentStatusView.normalizedStatus === 'closed') {
    return 'cancelled'
  }

  return 'unknown'
}

export function getReservationStatusText(status?: ReservationStatus): string {
  if (!status) return ''
  if (status === 'checked_in') return '已入座'
  return formatReservationStatus(status)
}

export function deriveReservationResultKind(status: ReservationStatus | undefined, initialResult: PaymentResultKind): PaymentResultKind {
  if (!status) {
    return initialResult === 'success' ? 'unknown' : initialResult
  }

  if (RESERVATION_SUCCESS_STATUSES.has(status)) {
    return 'success'
  }

  if (status === 'cancelled') {
    return 'cancelled'
  }

  if (RESERVATION_FAILED_STATUSES.has(status)) {
    return 'failed'
  }

  if (status === 'pending') {
    return initialResult === 'cancelled' || initialResult === 'failed' ? initialResult : 'unknown'
  }

  return 'unknown'
}

export function buildReservationResultView(status: ReservationStatus | undefined, initialResult: PaymentResultKind): ReservationResultView {
  const resultKind = deriveReservationResultKind(status, initialResult)

  if (resultKind === 'success') {
    const statusCopyMap: Partial<Record<ReservationStatus, Pick<ReservationResultView, 'resultTitle' | 'resultDescription'>>> = {
      confirmed: {
        resultTitle: '预订已确认',
        resultDescription: '定金支付已完成，商家已确认您的预订。'
      },
      checked_in: {
        resultTitle: '预订已到店',
        resultDescription: '当前预订已进入到店状态，可直接前往详情页查看后续动作。'
      },
      completed: {
        resultTitle: '预订已完成',
        resultDescription: '本次预订流程已完成，详情页中仍可查看完整记录。'
      }
    }

    const statusCopy = status ? statusCopyMap[status] : undefined

    return {
      resultKind,
      resultTheme: 'success',
      navTitle: '支付结果',
      resultTitle: statusCopy?.resultTitle || '定金支付成功',
      resultDescription: statusCopy?.resultDescription || '预订已提交，后续状态会继续在预订详情页更新。',
      primaryButtonText: '查看预订详情',
      secondaryButtonText: '查看我的预订',
      hintText: '支付结果已同步，后续状态变化会继续显示在预订详情页。'
    }
  }

  if (resultKind === 'cancelled') {
    return {
      resultKind,
      resultTheme: 'warning',
      navTitle: '支付结果',
      resultTitle: '支付已取消',
      resultDescription: '本次支付未完成，预订仍会保留在待支付状态，可稍后继续处理。',
      primaryButtonText: '查看预订详情',
      secondaryButtonText: '查看我的预订',
      hintText: '如仍需保留本次预订，请前往详情页确认支付截止时间并继续支付。'
    }
  }

  if (resultKind === 'failed') {
    return {
      resultKind,
      resultTheme: 'error',
      navTitle: '支付结果',
      resultTitle: '支付未完成',
      resultDescription: '系统暂未确认本次支付成功，请先查看预订详情中的最新状态。',
      primaryButtonText: '查看预订详情',
      secondaryButtonText: '查看我的预订',
      hintText: '若仍需完成支付，请前往预订详情页重新发起支付。'
    }
  }

  return {
    resultKind,
    resultTheme: 'warning',
    navTitle: '支付结果',
    resultTitle: '支付结果确认中',
    resultDescription: '支付已发起，系统正在同步最终结果。请重新查询，或先前往预订详情页确认。',
    primaryButtonText: '重新查询结果',
    secondaryButtonText: '查看预订详情',
    hintText: '如果结果暂未同步，请以后续预订详情页中的当前状态为准。'
  }
}