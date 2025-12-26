/**
 * 商户预约管理页面
 * 使用真实后端API
 */

import { isLargeScreen } from '../../../utils/responsive'
import {
  getMerchantReservations,
  confirmReservationByMerchant,
  markReservationNoShow,
  completeReservationByMerchant,
  ReservationResponse
} from '../../../api/reservation'
import { logger } from '../../../utils/logger'

Page({
  data: {
    activeTab: 'pending' as 'pending' | 'paid' | 'completed',
    reservations: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadReservations()
  },

  onShow() {
    // 返回时刷新
    if (this.data.reservations.length > 0) {
      this.loadReservations()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadReservations() {
    this.setData({ loading: true })

    try {
      const { activeTab } = this.data

      const result = await getMerchantReservations({
        page_id: 1,
        page_size: 50,
        status: activeTab as any
      })

      const reservations = (result || []).map((res: ReservationResponse) => ({
        id: res.id,
        contact_name: res.contact_name || '顾客',
        contact_phone: res.contact_phone || '',
        table_id: res.table_id,
        table_no: res.table_no,
        guest_count: res.guest_count,
        reservation_date: res.reservation_date,
        reservation_time: res.reservation_time,
        deposit: res.prepaid_amount || res.deposit_amount || 0,
        status: res.status,
        notes: res.notes,
        created_at: res.created_at
      }))

      this.setData({
        reservations,
        loading: false
      })
    } catch (error) {
      console.error('加载预约失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ activeTab: e.detail.value })
    this.loadReservations()
  },

  onViewDetail(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/reservations/detail/index?id=${id}` })
  },

  // 确认预订
  async onConfirmReservation(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    wx.showModal({
      title: '确认预订',
      content: '确定要确认此预订吗？',
      success: async (res) => {
        if (res.confirm) {
          await this.doConfirmReservation(Number(id))
        }
      }
    })
  },

  async doConfirmReservation(reservationId: number) {
    wx.showLoading({ title: '处理中...' })
    try {
      await confirmReservationByMerchant(reservationId)
      wx.hideLoading()
      wx.showToast({ title: '已确认', icon: 'success' })
      this.loadReservations()
    } catch (error) {
      wx.hideLoading()
      logger.error('确认预订失败', error, 'merchant/reservations.doConfirmReservation')
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  },

  // 标记未到店
  async onMarkNoShow(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    wx.showModal({
      title: '标记未到店',
      content: '确定要标记此预订为未到店吗？定金将被没收。',
      success: async (res) => {
        if (res.confirm) {
          await this.doMarkNoShow(Number(id))
        }
      }
    })
  },

  async doMarkNoShow(reservationId: number) {
    wx.showLoading({ title: '处理中...' })
    try {
      await markReservationNoShow(reservationId)
      wx.hideLoading()
      wx.showToast({ title: '已标记', icon: 'success' })
      this.loadReservations()
    } catch (error) {
      wx.hideLoading()
      logger.error('标记未到店失败', error, 'merchant/reservations.doMarkNoShow')
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  },

  // 完成预订
  async onCompleteReservation(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    if (!id) return

    wx.showModal({
      title: '完成预订',
      content: '确定要完成此预订吗？',
      success: async (res) => {
        if (res.confirm) {
          await this.doCompleteReservation(Number(id))
        }
      }
    })
  },

  async doCompleteReservation(reservationId: number) {
    wx.showLoading({ title: '处理中...' })
    try {
      await completeReservationByMerchant(reservationId)
      wx.hideLoading()
      wx.showToast({ title: '已完成', icon: 'success' })
      this.loadReservations()
    } catch (error) {
      wx.hideLoading()
      logger.error('完成预订失败', error, 'merchant/reservations.doCompleteReservation')
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  }
})
