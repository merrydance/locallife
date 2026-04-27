import {
  getOrders,
  cancelOrder,
  OrderStatus,
  getOrderDetail,
  OrderType,
  ListOrdersParams
} from '../../../api/order'
import { logger } from '../../../utils/logger'
import { OrderCardAdapter } from '../../../adapters/order-card'
import type { OrderCardViewModel } from '../../../adapters/order-card'
import CartService from '../../../services/cart'
import { OrderAdapter } from '../../../adapters/order'
import {
  createCombinedPaymentOrder,
  createOrderPayment,
  recoverCombinedPaymentOrder
} from '../../../api/payment'
import {
  completeCombinedPaymentWorkflow,
  completePaymentWorkflow,
  isCombinedPaymentWorkflowCancelled,
  isCombinedPaymentWorkflowPaid,
  isPaymentWorkflowPaid,
  shouldRecreateCombinedPaymentWorkflow
} from '../../../services/payment-workflow'
import Navigation from '../../../utils/navigation'
import { getErrorUserMessage } from '../../../utils/user-facing'
const getErrorMessage = getErrorUserMessage
import {
  CANCEL_REASONS,
  getCombinedPaymentToastMessage,
  getDatasetId,
  getSharedCombinedPaymentID,
  isOrderResponse,
  navigateToCombinedPaymentSuccess,
  normalizeOrderType,
  ORDER_REQUEST_DEDUP_MS,
  type OrderTypeFilter,
  STATUS_TABS
} from '../../../utils/orders-list-view'

Page({
  _activeRequestKey: '',
  _lastRequestKey: '',
  _lastRequestAt: 0,

  data: {
    orders: [] as OrderCardViewModel[],
    selectedPayMap: {} as Record<number, boolean>,
    selectedPayCount: 0,
    pendingPayableCount: 0,
    paying: false,
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: '',
    refreshErrorMessage: '',
    page: 1,
    pageSize: 10,
    hasMore: true,
    statusTabs: STATUS_TABS,
    currentStatus: '' as OrderStatus | '',
    orderType: '' as OrderTypeFilter,
    pageTitle: '我的订单'
  },

  onLoad(options: { order_type?: string }) {
    wx.showShareMenu({
      withShareTicket: true,
      menus: ['shareAppMessage', 'shareTimeline']
    })

    const orderType = normalizeOrderType(options?.order_type)
    
    // 根据订单类型设置标题
    const titleMap: Record<string, string> = {
      takeout: '外卖订单',
      reservation: '预订订单',
      dine_in: '堂食订单',
      takeaway: '自取订单'
    }
    
    let statusTabs = [...STATUS_TABS]
    if (orderType === 'dine_in' || orderType === 'takeaway' || orderType === 'reservation') {
        statusTabs = statusTabs.filter((tab) => tab.value !== 'delivering')
    }
    
    this.setData({
      orderType,
      statusTabs,
      pageTitle: titleMap[orderType] || '我的订单'
    })
    
    this.loadOrders(true)
  },

  onShow() {
    // 返回时刷新列表，确保状态最新
    if (this.data.orders.length > 0) {
      this.loadOrders(true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
      this.loadOrders(false)
    }
  },

  // 防止冒泡
  preventBubble() {},

  onCallMerchant(e: WechatMiniprogram.BaseEvent) {
    const phone = e.currentTarget.dataset.phone
    if (phone) {
      wx.makePhoneCall({ phoneNumber: phone })
    } else {
      wx.showToast({ title: '暂无商家电话', icon: 'none' })
    }
  },

  async loadOrders(reset = false) {
    if (this.data.loading && !reset) return

    const targetPage = reset ? 1 : this.data.page
    const requestKey = [
      targetPage,
      this.data.pageSize,
      this.data.currentStatus || 'all',
      this.data.orderType || 'all'
    ].join('|')

    const now = Date.now()
    if (this._activeRequestKey === requestKey) {
      return
    }
    if (this._lastRequestKey === requestKey && (now - this._lastRequestAt) < ORDER_REQUEST_DEDUP_MS) {
      return
    }

    this._activeRequestKey = requestKey
    this._lastRequestKey = requestKey
    this._lastRequestAt = now

    this.setData({ loading: true, isError: false, refreshErrorMessage: '' })

    if (reset) {
      this.setData({ page: 1, orders: [], hasMore: true })
    }

    try {
      const { currentStatus, pageSize, orderType } = this.data
      
      const params: ListOrdersParams = {
        page_id: targetPage,
        page_size: pageSize,
        ...(currentStatus ? { status: currentStatus } : {}),
        ...(orderType ? { order_type: orderType } : {})
      }
      
      const result = await getOrders(params)

      // (unwrap logic remains same)
      const unwrap = (payload: unknown): unknown[] => {
        if (Array.isArray(payload)) return payload
        if (payload && typeof payload === 'object') {
          const record = payload as Record<string, unknown>
          if (Array.isArray(record.orders)) return record.orders
          if (Array.isArray(record.list)) return record.list
          if (Array.isArray(record.items)) return record.items
          if (record.data) return unwrap(record.data)
        }
        return []
      }

      const orderDTOsRaw = unwrap(result)

      const orderDTOs = orderDTOsRaw
        .filter(isOrderResponse)
        .map((item) => {
          try {
            return OrderCardAdapter.toCardViewModel(item)
          } catch (err) {
            logger.error('Order map failed', { err, item }, 'Orders.List')
            return null
          }
        })
        .filter(Boolean) as OrderCardViewModel[]

      const sortedOrders = orderDTOs

      const orders = reset
        ? sortedOrders
        : [...this.data.orders, ...sortedOrders]

      const totalCount = result.total ?? orders.length

      this.setData({
        orders,
        page: targetPage + 1,
        selectedPayMap: this.pruneSelectedPayMap(orders),
        selectedPayCount: this.countSelectedPay(this.pruneSelectedPayMap(orders)),
        pendingPayableCount: this.getPayableOrderIDs(orders).length,
        hasMore: orders.length < totalCount && orderDTOs.length > 0,
        loading: false,
        refreshErrorMessage: ''
      })
      
    } catch (error: unknown) {
      logger.error('Load orders failed:', error, 'List')
      // 仅在首屏（page=1 且无数据）时显示全屏错误
      if (targetPage === 1 && this.data.orders.length === 0) {
        this.setData({ 
          loading: false, 
          isError: true, 
          errorMsg: getErrorMessage(error, '加载订单失败'),
          refreshErrorMessage: ''
        })
      } else {
        this.setData({
          loading: false,
          refreshErrorMessage: `${getErrorMessage(error, '加载失败，请稍后重试')}，当前已保留上次结果`
        })
      }
    } finally {
      this._activeRequestKey = ''
    }
  },

  onRetryRefresh() {
    this.loadOrders(true)
  },

  onStatusChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatus | '' }>) {
    const status = e.detail.value || ''
    if (status === this.data.currentStatus) return
    this.setData({
      currentStatus: status,
      selectedPayMap: {},
      selectedPayCount: 0,
      pendingPayableCount: 0
    })
    this.loadOrders(true)
  },

  getPayableOrderIDs(orders: OrderCardViewModel[]): number[] {
    return orders.filter((order) => order.canPay).map((order) => order.id)
  },

  pruneSelectedPayMap(orders: OrderCardViewModel[]): Record<number, boolean> {
    const payableOrderIDs = this.getPayableOrderIDs(orders)
    const payableSet = new Set(payableOrderIDs)
    const nextSelectedMap: Record<number, boolean> = {}

    Object.entries(this.data.selectedPayMap).forEach(([idStr, selected]) => {
      const id = Number(idStr)
      if (selected && payableSet.has(id)) {
        nextSelectedMap[id] = true
      }
    })

    return nextSelectedMap
  },

  countSelectedPay(selectedMap: Record<number, boolean>): number {
    return Object.values(selectedMap).filter(Boolean).length
  },

  onTogglePaySelect(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e)
    if (!id) return

    const targetOrder = this.data.orders.find((order) => order.id === id)
    if (!targetOrder || !targetOrder.canPay) return

    const nextSelectedMap = { ...this.data.selectedPayMap }
    if (nextSelectedMap[id]) {
      delete nextSelectedMap[id]
    } else {
      nextSelectedMap[id] = true
    }

    this.setData({
      selectedPayMap: nextSelectedMap,
      selectedPayCount: this.countSelectedPay(nextSelectedMap)
    })
  },

  onToggleSelectAllPending() {
    const payableOrderIDs = this.getPayableOrderIDs(this.data.orders)
    if (payableOrderIDs.length === 0) {
      this.setData({ selectedPayMap: {}, selectedPayCount: 0 })
      return
    }

    const allSelected = payableOrderIDs.every((id: number) => this.data.selectedPayMap[id])
    if (allSelected) {
      this.setData({ selectedPayMap: {}, selectedPayCount: 0 })
      return
    }

    const nextSelectedMap: Record<number, boolean> = {}
    payableOrderIDs.forEach((id: number) => {
      nextSelectedMap[id] = true
    })

    this.setData({
      selectedPayMap: nextSelectedMap,
      selectedPayCount: payableOrderIDs.length
    })
  },

  onViewOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e)
    if (!id) return
    wx.navigateTo({ url: `/pages/orders/detail/index?id=${id}` })
  },

  onEnterMerchant(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e)
    if (!id) return
    // 跳转到餐厅详情 (假设是 takeout 类型的详情页，若是 din-in 可能不同，但通常共用详情页)
    wx.navigateTo({ url: `/pages/takeout/restaurant-detail/index?id=${id}` })
  },

  onCancelOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e)
    if (!id) return

    wx.showActionSheet({
      itemList: CANCEL_REASONS,
      success: async (res) => {
        const reason = CANCEL_REASONS[res.tapIndex]
        await this.doCancelOrder(Number(id), reason)
      }
    })
  },

  async doCancelOrder(orderId: number, reason: string) {
    wx.showLoading({ title: '取消中...' })
    try {
      await cancelOrder(orderId, { reason })
      wx.hideLoading()
      await this.loadOrders(true)
    } catch (error) {
      wx.hideLoading()
      logger.error('取消订单失败', error, 'List.doCancelOrder')
      wx.showToast({ title: '取消失败', icon: 'error' })
    }
  },

  async onPayOrder(e: WechatMiniprogram.BaseEvent) {
    const id = getDatasetId(e)
    if (!id) {
      wx.showToast({ title: '订单信息缺失', icon: 'none' })
      return
    }

    await this.paySingleOrder(id)
  },

  async paySingleOrder(orderId: number) {
    if (this.data.paying) return

    this.setData({ paying: true })
    wx.showLoading({ title: '拉起支付...' })
    try {
      const targetOrder = this.data.orders.find((order) => order.id === orderId)
      const combinedPaymentID = targetOrder?.paymentContext?.combined_payment_id
        if (combinedPaymentID) {
          const resumeResult = await this.resumeCombinedPayment(combinedPaymentID, [orderId])
          if (resumeResult === 'completed') {
            this.setData({ selectedPayMap: {}, selectedPayCount: 0 })
            return
          }
          if (resumeResult === 'handled') {
            return
          }
        }

      const paymentResult = await completePaymentWorkflow(await createOrderPayment(orderId))

      if (isPaymentWorkflowPaid(paymentResult.status)) {
        this.setData({ selectedPayMap: {}, selectedPayCount: 0 })
      }
      Navigation.toPaymentResult({
        status: paymentResult.status,
        paymentOrderId: paymentResult.paymentOrderId,
        businessId: orderId,
        businessType: paymentResult.businessType || 'order',
        orderNo: paymentResult.outTradeNo || String(orderId),
        amount: paymentResult.amountFen ? (paymentResult.amountFen / 100).toFixed(2) : undefined
      })
    } catch (error) {
      logger.error('单笔支付失败', error, 'List.paySingleOrder')
      wx.showToast({ title: '支付失败', icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ paying: false })
    }
  },

  async resumeCombinedPayment(
    combinedPaymentId: number,
    orderIds: number[]
  ): Promise<'completed' | 'handled' | 'fallback'> {
    const paymentResult = await completeCombinedPaymentWorkflow(await recoverCombinedPaymentOrder(combinedPaymentId))
    const combinedPayment = paymentResult.combinedPayment

    if (isCombinedPaymentWorkflowPaid(paymentResult.status)) {
      navigateToCombinedPaymentSuccess(combinedPayment, orderIds)
      return 'completed'
    }

    if (isCombinedPaymentWorkflowCancelled(paymentResult.status)) {
      wx.showToast({ title: '已取消支付，可继续完成原合单支付', icon: 'none' })
      return 'handled'
    }

    if (shouldRecreateCombinedPaymentWorkflow(paymentResult.status)) {
      return 'fallback'
    }

    wx.showToast({ title: getCombinedPaymentToastMessage(combinedPayment), icon: 'none' })
    return 'handled'
  },

  async onBatchPay() {
    if (this.data.paying) return

    const selectedOrderIDs = Object.entries(this.data.selectedPayMap)
      .filter(([, selected]) => selected)
      .map(([idStr]) => Number(idStr))

    if (selectedOrderIDs.length === 0) {
      wx.showToast({ title: '请选择待支付订单', icon: 'none' })
      return
    }

    if (selectedOrderIDs.length === 1) {
      await this.paySingleOrder(selectedOrderIDs[0])
      return
    }

    this.setData({ paying: true })
    wx.showLoading({ title: '拉起合并支付...' })
    try {
      const existingCombinedPaymentID = getSharedCombinedPaymentID(this.data.orders, selectedOrderIDs)

      if (existingCombinedPaymentID) {
        const resumeResult = await this.resumeCombinedPayment(existingCombinedPaymentID, selectedOrderIDs)
        if (resumeResult === 'completed') {
          this.setData({ selectedPayMap: {}, selectedPayCount: 0 })
          return
        }
        if (resumeResult === 'handled') {
          return
        }
      }

      const paymentResult = await completeCombinedPaymentWorkflow(await createCombinedPaymentOrder({ order_ids: selectedOrderIDs }))
      const combinedPayment = paymentResult.combinedPayment

      if (!isCombinedPaymentWorkflowPaid(paymentResult.status)) {
        if (isCombinedPaymentWorkflowCancelled(paymentResult.status)) {
          wx.showToast({ title: '已取消支付，可继续完成合单支付', icon: 'none' })
          return
        }

        wx.showToast({ title: getCombinedPaymentToastMessage(combinedPayment), icon: 'none' })
        return
      }

      this.setData({ selectedPayMap: {}, selectedPayCount: 0 })
      navigateToCombinedPaymentSuccess(combinedPayment, selectedOrderIDs)
    } catch (error) {
      logger.error('合并支付失败', error, 'List.onBatchPay')
      wx.showToast({ title: '合并支付未完成', icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ paying: false })
    }
  },

  onReorder(e: WechatMiniprogram.BaseEvent) {
    const orderId = getDatasetId(e)
    if (!orderId) {
      wx.showToast({ title: '订单信息缺失', icon: 'none' })
      return
    }

    wx.showLoading({ title: '再次购买中...' });
    (async () => {
      try {
        const orderDTO = await getOrderDetail(orderId)
        const orderDetail = OrderAdapter.toDetailViewModel(orderDTO)

        const orderType: OrderType = orderDetail.type || 'takeout'
        const cartContext: {
          orderType: OrderType
          tableId?: number
          reservationId?: number
        } = { orderType }

        if (orderType === 'dine_in' && orderDetail.tableId) {
          cartContext.tableId = orderDetail.tableId
        }
        if (orderType === 'reservation' && orderDetail.reservationId) {
          cartContext.reservationId = orderDetail.reservationId
        }

        await CartService.loadCart(orderDetail.merchantId, cartContext)

        const addResults = await Promise.all(
          orderDetail.items.map((item) =>
            CartService.addItem({
              merchantId: orderDetail.merchantId,
              dishId: item.dishId,
              comboId: item.comboId,
              quantity: item.quantity
            })
          )
        )

        if (addResults.some((ok) => !ok)) {
          wx.hideLoading()
          wx.showToast({ title: '部分商品添加失败', icon: 'none' })
          return
        }

        wx.hideLoading()
        wx.navigateTo({ url: '/pages/takeout/cart/index' })
      } catch (error) {
        wx.hideLoading()
        logger.error('再次购买失败', error, 'List.onReorder')
        wx.showToast({ title: '操作失败', icon: 'error' })
      }
    })()
  },

  onShareAppMessage() {
    return {
      title: this.data.pageTitle || '我的订单',
      path: `/pages/orders/list/index${this.data.orderType ? `?order_type=${this.data.orderType}` : ''}`
    }
  },

  onShareTimeline() {
    return {
      title: this.data.pageTitle || '我的订单'
    }
  }
})
