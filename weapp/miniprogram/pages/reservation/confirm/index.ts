/**
 * 预约确认页面
 * 使用真实后端API
 */

import { formatTime } from '@/utils/util'
import { createReservation, CreateReservationRequest } from '../../../api/reservation'

Page({
  data: {
    roomId: '',
    tableId: 0,
    roomName: '',
    deposit: 0,
    form: {
      date: '',
      time: '',
      guestCount: 1,
      name: '',
      phone: '',
      remark: ''
    },
    minDate: new Date().getTime(),
    navBarHeight: 88,
    submitting: false,
    dateVisible: false,
    timeVisible: false
  },

  onLoad(options: any) {
    if (options.roomId) {
      this.setData({
        roomId: options.roomId,
        tableId: parseInt(options.tableId) || 0,
        roomName: options.roomName || '',
        deposit: Number(options.deposit) || 0
      })
    }

    // Set default date to tomorrow
    const tomorrow = new Date()
    tomorrow.setDate(tomorrow.getDate() + 1)
    this.setData({
      'form.date': formatTime(tomorrow).split(' ')[0]
    })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onInputChange(e: WechatMiniprogram.CustomEvent) {
    const { field } = e.currentTarget.dataset
    this.setData({
      [`form.${field}`]: e.detail.value
    })
  },

  onGuestCountChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ 'form.guestCount': e.detail.value })
  },

  showDatePicker() {
    this.setData({ dateVisible: true })
  },

  hideDatePicker() {
    this.setData({ dateVisible: false })
  },

  onDateConfirm(e: WechatMiniprogram.CustomEvent) {
    this.setData({
      'form.date': e.detail.value,
      dateVisible: false
    })
  },

  showTimePicker() {
    this.setData({ timeVisible: true })
  },

  hideTimePicker() {
    this.setData({ timeVisible: false })
  },

  onTimeConfirm(e: WechatMiniprogram.CustomEvent) {
    this.setData({
      'form.time': e.detail.value,
      timeVisible: false
    })
  },

  async onSubmit() {
    const { form, tableId, deposit } = this.data

    if (!form.date || !form.time || !form.name || !form.phone) {
      wx.showToast({ title: '请填写完整信息', icon: 'none' })
      return
    }

    if (!/^1\d{10}$/.test(form.phone)) {
      wx.showToast({ title: '手机号格式不正确', icon: 'none' })
      return
    }

    if (!tableId) {
      wx.showToast({ title: '请选择包间', icon: 'none' })
      return
    }

    this.setData({ submitting: true })

    try {
      const reservationData: CreateReservationRequest = {
        table_id: tableId,
        date: form.date,
        time: form.time,
        guest_count: form.guestCount,
        contact_name: form.name,
        contact_phone: form.phone,
        payment_mode: deposit > 0 ? 'deposit' : 'full',
        notes: form.remark || undefined
      }

      await createReservation(reservationData)

      wx.showToast({ title: '预定提交成功', icon: 'success' })

      setTimeout(() => {
        wx.redirectTo({ url: '/pages/orders/list/index' })
      }, 1500)
    } catch (error) {
      console.error('预定提交失败:', error)
      wx.showToast({ title: '提交失败', icon: 'error' })
      this.setData({ submitting: false })
    }
  }
})
