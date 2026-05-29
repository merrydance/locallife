import type { PaymentWorkflowStatus } from '../_main_shared/services/payment-workflow'

export type { PaymentWorkflowStatus }

export type PaymentResultTheme = 'success' | 'warning' | 'error' | 'default'
export type PaymentResultAction = 'detail_page' | 'list_page' | 'refresh_status' | 'retry_payment' | 'home'

export interface PaymentResultView {
  navTitle: string
  title: string
  description: string
  theme: PaymentResultTheme
  primaryButtonText: string
  primaryAction: PaymentResultAction
  secondaryButtonText: string
  secondaryAction: PaymentResultAction
}

export function normalizePaymentWorkflowStatus(status?: string): PaymentWorkflowStatus {
  switch (status) {
    case 'paid':
    case 'failed':
    case 'cancelled':
    case 'pending_confirmation':
    case 'create_failed':
    case 'pay_params_missing':
    case 'closed':
      return status
    default:
      return 'pending_confirmation'
  }
}

export function buildPaymentResultView(status?: string): PaymentResultView {
  const normalizedStatus = normalizePaymentWorkflowStatus(status)

  switch (normalizedStatus) {
    case 'paid':
      return {
        navTitle: '支付已确认',
        title: '支付已确认',
        description: '系统已确认支付成功，订单状态会继续更新。',
        theme: 'success',
        primaryButtonText: '查看详情',
        primaryAction: 'detail_page',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
    case 'cancelled':
      return {
        navTitle: '支付已取消',
        title: '支付已取消',
        description: '本次支付未完成，订单已保留，可在详情页继续支付。',
        theme: 'warning',
        primaryButtonText: '查看详情',
        primaryAction: 'detail_page',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
    case 'pending_confirmation':
      return {
        navTitle: '结果待确认',
        title: '支付结果待确认',
        description: '支付结果还在同步中，请稍后刷新或查看订单详情。',
        theme: 'warning',
        primaryButtonText: '刷新结果',
        primaryAction: 'refresh_status',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
    case 'create_failed':
      return {
        navTitle: '支付未发起',
        title: '支付未发起',
        description: '支付单创建失败，本次没有产生待确认支付，请稍后重试。',
        theme: 'error',
        primaryButtonText: '返回详情',
        primaryAction: 'detail_page',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
    case 'pay_params_missing':
      return {
        navTitle: '暂不能支付',
        title: '暂不能支付',
        description: '支付参数暂未准备好，请回到详情页重新发起。',
        theme: 'error',
        primaryButtonText: '返回详情',
        primaryAction: 'detail_page',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
    case 'closed':
      return {
        navTitle: '支付已关闭',
        title: '支付单已关闭',
        description: '当前支付单已关闭，如仍需支付请重新发起。',
        theme: 'default',
        primaryButtonText: '查看详情',
        primaryAction: 'detail_page',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
    default:
      return {
        navTitle: '支付未完成',
        title: '支付未完成',
        description: '系统未确认支付成功，请返回详情页重新发起或稍后查看。',
        theme: 'error',
        primaryButtonText: '查看详情',
        primaryAction: 'detail_page',
        secondaryButtonText: '返回订单列表',
        secondaryAction: 'list_page'
      }
  }
}
