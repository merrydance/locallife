import { ReservationService } from '../../../api/reservation'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  buildReservationResultView,
  getReservationStatusText,
  normalizePaymentResultKind,
  PaymentResultKind,
  ResultTheme
} from '../../../utils/reservation-payment-result-view'
import { getErrorUserMessage } from '../../../utils/user-facing'

type BusinessType = 'order' | 'reservation'

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
      const initialResult = normalizePaymentResultKind(options.result)
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
