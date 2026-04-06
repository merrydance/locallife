import { ReservationService, ReservationStatus } from '../../../api/reservation'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type BusinessType = 'order' | 'reservation'
type PaymentResultKind = 'success' | 'failed' | 'cancelled' | 'unknown'
type ResultTheme = 'success' | 'warning' | 'error'

function normalizeResultKind(result?: string): PaymentResultKind {
  if (result === 'failed' || result === 'cancelled' || result === 'unknown') {
    return result
  }
  return 'success'
}

function getReservationStatusText(status?: ReservationStatus): string {
  if (!status) return ''

  const map: Record<ReservationStatus, string> = {
    pending: '待支付',
    paid: '已支付',
    confirmed: '已确认',
    checked_in: '已入座',
    completed: '已完成',
    cancelled: '已取消',
    expired: '已过期',
    no_show: '未到店'
  }

  return map[status] || '待支付'
}

function deriveReservationResultKind(status: ReservationStatus | undefined, initialResult: PaymentResultKind): PaymentResultKind {
  if (!status) return initialResult === 'success' ? 'unknown' : initialResult

  if (['paid', 'confirmed', 'checked_in', 'completed'].includes(status)) {
    return 'success'
  }

  if (status === 'cancelled') {
    return 'cancelled'
  }

  if (['expired', 'no_show'].includes(status)) {
    return 'failed'
  }

  if (status === 'pending') {
    if (initialResult === 'cancelled' || initialResult === 'failed') {
      return initialResult
    }
    return 'unknown'
  }

  return 'unknown'
}

function buildReservationResultView(status: ReservationStatus | undefined, initialResult: PaymentResultKind) {
  const resultKind = deriveReservationResultKind(status, initialResult)

  if (resultKind === 'success') {
    let resultTitle = '定金支付成功'
    let resultDescription = '预订已提交，后续状态会继续在预订详情页更新。'

    if (status === 'confirmed') {
      resultTitle = '预订已确认'
      resultDescription = '定金支付已完成，商家已确认您的预订。'
    } else if (status === 'checked_in') {
      resultTitle = '预订已到店'
      resultDescription = '当前预订已进入到店状态，可直接前往详情页查看后续动作。'
    } else if (status === 'completed') {
      resultTitle = '预订已完成'
      resultDescription = '本次预订流程已完成，详情页中仍可查看完整记录。'
    }

    return {
      resultKind,
      resultTheme: 'success' as ResultTheme,
      navTitle: '支付结果',
      resultTitle,
      resultDescription,
      primaryButtonText: '查看预订详情',
      secondaryButtonText: '查看我的预订',
      hintText: '支付结果已同步，后续状态变化会继续显示在预订详情页。'
    }
  }

  if (resultKind === 'cancelled') {
    return {
      resultKind,
      resultTheme: 'warning' as ResultTheme,
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
      resultTheme: 'error' as ResultTheme,
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
    resultTheme: 'warning' as ResultTheme,
    navTitle: '支付结果',
    resultTitle: '支付结果确认中',
    resultDescription: '支付已发起，系统正在同步最终结果。请重新查询，或先前往预订详情页确认。',
    primaryButtonText: '重新查询结果',
    secondaryButtonText: '查看预订详情',
    hintText: '如果结果暂未同步，请以后续预订详情页中的当前状态为准。'
  }
}

Page({
  data: {
    orderId: '',
    orderNo: '',
    amount: '0.00',
    businessType: 'order' as BusinessType,
    initialResult: 'success' as PaymentResultKind,
    source: '' as '' | 'confirm' | 'detail' | 'list',
    returnStatus: '',
    isCombined: false,
    orderCount: 0,
    navTitle: '支付成功',
    resultKind: 'success' as PaymentResultKind,
    resultTheme: 'success' as ResultTheme,
    successTitle: '支付成功',
    successDescription: '您的订单已支付完成，商家正在处理中',
    primaryButtonText: '查看订单',
    secondaryButtonText: '返回首页',
    hintText: '支付成功后，您可以在“我的评价”中查看历史订单详情。',
    statusText: '',
    statusNote: '',
    stateLoading: false,
    refreshing: false,
    navBarHeight: 88
  },

  onLoad(options: { orderId?: string, orderNo?: string, amount?: string, combined?: string, orderCount?: string, businessType?: string, result?: string, source?: string, returnStatus?: string }) {
    const { navBarHeight } = getStableBarHeights()
    const businessType = options.businessType === 'reservation' ? 'reservation' : 'order'

    if (businessType === 'reservation') {
      const initialResult = normalizeResultKind(options.result)
      const source = options.source === 'confirm' || options.source === 'detail' || options.source === 'list' ? options.source : ''
      const initialView = buildReservationResultView(undefined, initialResult)

      this.setData({
        navBarHeight,
        businessType,
        initialResult,
        source,
        returnStatus: options.returnStatus || '',
        orderId: options.orderId || '',
        orderNo: options.orderNo || options.orderId || '',
        amount: options.amount || '0.00',
        navTitle: initialView.navTitle,
        resultKind: initialView.resultKind,
        resultTheme: initialView.resultTheme,
        successTitle: initialView.resultTitle,
        successDescription: initialView.resultDescription,
        primaryButtonText: initialView.primaryButtonText,
        secondaryButtonText: initialView.secondaryButtonText,
        hintText: initialView.hintText,
        stateLoading: true,
        statusText: '',
        statusNote: ''
      })
      this.loadReservationResult(true)
      return
    }

    const isCombined = options.combined === '1'
    const orderCount = Number(options.orderCount || '0') || 0
    this.setData({
      navBarHeight,
      businessType,
      orderId: options.orderId || '',
      orderNo: options.orderNo || '',
      amount: options.amount || '0.00',
      isCombined,
      orderCount,
      navTitle: isCombined ? '合并支付成功' : '支付成功',
      resultKind: 'success',
      resultTheme: 'success',
      successTitle: isCombined ? '合并支付成功' : '支付成功',
      successDescription: isCombined
        ? `已完成${orderCount > 0 ? `${orderCount}笔` : ''}订单合并支付，商家正在处理中`
        : '您的订单已支付完成，商家正在处理中',
      primaryButtonText: isCombined ? '查看订单列表' : '查看订单',
      secondaryButtonText: '返回首页',
      hintText: '支付成功后，您可以在“我的评价”中查看历史订单详情。'
    })
  },

  onShow() {
    if (this.data.businessType === 'reservation' && this.data.orderId && !this.data.stateLoading && !this.data.refreshing) {
      this.loadReservationResult(false)
    }
  },

  async loadReservationResult(showLoading: boolean) {
    const reservationId = Number(this.data.orderId)
    if (!reservationId || (!showLoading && this.data.refreshing)) return

    this.setData(showLoading ? { stateLoading: true, statusNote: '' } : { refreshing: true, statusNote: '' })

    try {
      const reservation = await ReservationService.getReservationDetail(reservationId)
      const view = buildReservationResultView(reservation.status, this.data.initialResult)
      this.setData({
        navTitle: view.navTitle,
        resultKind: view.resultKind,
        resultTheme: view.resultTheme,
        successTitle: view.resultTitle,
        successDescription: view.resultDescription,
        primaryButtonText: view.primaryButtonText,
        secondaryButtonText: view.secondaryButtonText,
        hintText: view.hintText,
        orderNo: String(reservation.id),
        amount: this.data.amount || ((reservation.deposit_amount || reservation.prepaid_amount || 0) / 100).toFixed(2),
        statusText: getReservationStatusText(reservation.status),
        stateLoading: false,
        refreshing: false
      })
    } catch (error) {
      const fallbackView = buildReservationResultView(undefined, this.data.initialResult === 'success' ? 'unknown' : this.data.initialResult)
      this.setData({
        navTitle: fallbackView.navTitle,
        resultKind: fallbackView.resultKind,
        resultTheme: fallbackView.resultTheme,
        successTitle: fallbackView.resultTitle,
        successDescription: fallbackView.resultDescription,
        primaryButtonText: fallbackView.primaryButtonText,
        secondaryButtonText: fallbackView.secondaryButtonText,
        hintText: fallbackView.hintText,
        statusText: '',
        statusNote: getErrorUserMessage(error, '支付结果暂未同步，请稍后重试'),
        stateLoading: false,
        refreshing: false
      })
    }
  },

  onPrimaryAction() {
    if (this.data.businessType === 'reservation') {
      if (this.data.resultKind === 'unknown') {
        this.loadReservationResult(false)
        return
      }

      wx.redirectTo({
        url: `/pages/reservation/detail/index?id=${this.data.orderId}`
      })
      return
    }

    if (this.data.isCombined) {
      wx.redirectTo({
        url: '/pages/orders/list/index'
      })
      return
    }

    wx.redirectTo({
      url: `/pages/orders/detail/index?id=${this.data.orderId}`
    })
  },

  onSecondaryAction() {
    if (this.data.businessType === 'reservation') {
      if (this.data.resultKind === 'unknown') {
        wx.redirectTo({
          url: `/pages/reservation/detail/index?id=${this.data.orderId}`
        })
        return
      }

      if (this.data.source === 'list') {
        wx.navigateBack({
          delta: 1
        })
        return
      }

      wx.redirectTo({
        url: '/pages/user_center/reservations/index'
      })
      return
    }

    wx.switchTab({
      url: '/pages/takeout/index'
    })
  }
})
