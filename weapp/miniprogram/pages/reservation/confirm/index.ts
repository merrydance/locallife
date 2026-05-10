/**
 * 预约确认页面
 * 支持定金模式和全款模式
 */

import { formatPriceNoSymbol } from '@/utils/util'
import { createReservation, CreateReservationRequest } from '../../../api/reservation'
import { checkRoomAvailability } from '../../../api/room'
import Navigation from '../../../utils/navigation'
import { startPaymentOrderWorkflow } from '../../../services/payment-workflow'
import { getErrorUserMessage } from '../../../utils/user-facing'
import { logger } from '../../../utils/logger'

interface TimeSlot {
  time: string
  available: boolean
}

Page({
  data: {
    roomId: '',
    tableId: 0,
    merchantId: 0,
    roomName: '',
    capacity: 10,
    deposit: 0,
    depositDisplay: '0.00',
    paymentMode: 'deposit' as 'deposit' | 'full',

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
    // 时段选择
    timeSlots: [] as TimeSlot[],
    availableTimeSlots: [] as Array<{ label: string, value: string }>,  // 可用时段列表（picker格式）
    selectedTimeLabel: '',
    timePickerVisible: false,
    loadingSlots: false
  },

  onLoad(options: {
    roomId?: string
    merchantId?: string
    roomName?: string
    capacity?: string
    deposit?: string
    date?: string
    time?: string
  }) {
    if (options.roomId) {
      const roomIdNum = parseInt(options.roomId || '0', 10) || 0
      const merchantIdNum = parseInt(options.merchantId || '0', 10) || 0
      const capacityNum = parseInt(options.capacity || '10', 10) || 10
      const depositNum = Number(options.deposit || 0) || 0
      this.setData({
        roomId: options.roomId,
        tableId: roomIdNum,
        merchantId: merchantIdNum,
        roomName: decodeURIComponent(options.roomName || ''),
        capacity: capacityNum,
        deposit: depositNum,
        depositDisplay: formatPriceNoSymbol(depositNum),
        paymentMode: depositNum > 0 ? 'deposit' : 'full'
      })
    }

    // 处理传入的日期和时间
    if (options.date) {
      this.setData({ 'form.date': options.date })
      if (options.time) {
        this.setData({ 'form.time': options.time, selectedTimeLabel: this.buildTimeLabel(options.time) })
      }
      this.loadAvailability(options.date, parseInt(options.roomId || '0', 10))
    } else {
      // 默认日期为当天
      const today = new Date()
      const pad = (n: number) => n < 10 ? '0' + n : String(n)
      const dateStr = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`
      this.setData({ 'form.date': dateStr })
      if (options.roomId) {
        this.loadAvailability(dateStr, parseInt(options.roomId || '0', 10))
      }
    }
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
    const date = e.detail.value
    this.setData({
      'form.date': date,
      'form.time': '',  // 重置时间选择
      dateVisible: false
    })
    // 加载新日期的可用时段
    this.loadAvailability(date)
  },

  // 加载可用时段
  async loadAvailability(date: string, tableIdOverride?: number) {
    const tableId = tableIdOverride || this.data.tableId
    if (!tableId) return

    this.setData({ loadingSlots: true })

    try {
      const response = await checkRoomAvailability(tableId, { date })

      const timeSlots = response.time_slots || []
      // 转换为picker需要的格式：{label, value}，并标记午餐/晚餐
      const availableTimeSlots = timeSlots
        .filter((slot) => slot.available)
        .map((slot) => {
          const mealLabel = this.buildTimeLabel(slot.time, slot.period)
          return { label: mealLabel, value: slot.time }
        })

      this.setData({
        timeSlots,
        availableTimeSlots,
        loadingSlots: false
      })

      // 验证当前选中的时间是否有效
      if (this.data.form.time) {
        const isStillAvailable = availableTimeSlots.some((slot) => slot.value === this.data.form.time)
        if (!isStillAvailable) {
          // 如果当前时间不可用，自动切换到第一个可用时段
          if (availableTimeSlots.length > 0) {
            const firstSlot = availableTimeSlots[0]
            this.setData({
              'form.time': firstSlot.value,
              selectedTimeLabel: this.buildTimeLabel(firstSlot.value)
            })
            wx.showToast({ title: '已为您切换到最近可用时段', icon: 'none' })
          } else {
            this.setData({
              'form.time': '',
              selectedTimeLabel: ''
            })
          }
        }
      } else if (availableTimeSlots.length > 0) {
        // 如果没选时间，默认选第一个
        const firstSlot = availableTimeSlots[0]
        this.setData({
          'form.time': firstSlot.value,
          selectedTimeLabel: this.buildTimeLabel(firstSlot.value)
        })
      }

      if (availableTimeSlots.length === 0) {
        wx.showToast({ title: '该日期暂无可用时段', icon: 'none' })
      }
    } catch (error) {
      logger.error('获取可用时段失败', error, 'reservation-confirm')
      this.setData({ loadingSlots: false })
      wx.showToast({ title: '获取时段失败', icon: 'error' })
    }
  },

  showTimePicker() {
    if (this.data.availableTimeSlots.length === 0) {
      wx.showToast({ title: '请先选择日期', icon: 'none' })
      return
    }
    this.setData({ timePickerVisible: true })
  },

  hideTimePicker() {
    this.setData({ timePickerVisible: false })
  },

  onTimeSelect(e: WechatMiniprogram.CustomEvent) {
    const { value } = e.detail
    // value 是选中项的label数组，如 ["17:30"]
    const selectedTime = Array.isArray(value) ? value[0] : value
    if (selectedTime) {
      this.setData({
        'form.time': selectedTime,
        selectedTimeLabel: this.buildTimeLabel(selectedTime),
        timePickerVisible: false
      })
    }
  },

  buildTimeLabel(time: string, period?: string) {
    if (!time) return ''
    let meal = ''
    if (period) {
      meal = period === 'lunch' ? '午餐' : (period === 'dinner' ? '晚餐' : '')
    } else {
      const hour = parseInt(time.split(':')[0])
      meal = hour < 17 ? '午餐' : '晚餐'
    }
    return meal ? `${time} (${meal})` : time
  },

  async onSubmit() {
    const { form, tableId, paymentMode, merchantId } = this.data

    // 表单验证
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
      // 1. 创建预订
      const reservationData: CreateReservationRequest = {
        table_id: tableId,
        date: form.date,
        time: form.time,
        guest_count: form.guestCount,
        contact_name: form.name,
        contact_phone: form.phone,
        payment_mode: paymentMode,
        notes: form.remark || undefined
      }

      const reservation = await createReservation(reservationData)

      if (paymentMode === 'full') {
        wx.redirectTo({
          url: `/pages/dine-in/menu/menu?reservation_id=${reservation.id}&merchant_id=${merchantId}`
        })
      } else {
        const resultAmount = formatPriceNoSymbol(reservation.deposit_amount || this.data.deposit)

        try {
          const paymentResult = await startPaymentOrderWorkflow({
            orderId: reservation.id,
            businessType: 'reservation',
            context: this
          })
          Navigation.toPaymentResult({
            status: paymentResult.status,
            paymentOrderId: paymentResult.paymentOrderId,
            businessId: reservation.id,
            businessType: 'reservation',
            orderNo: paymentResult.outTradeNo,
            amount: resultAmount
          })
        } catch (payErr) {
          logger.error('预订支付失败或取消', payErr, 'reservation-confirm')
          Navigation.toPaymentResult({
            status: 'create_failed',
            businessId: reservation.id,
            businessType: 'reservation',
            amount: resultAmount
          })
        }
      }
    } catch (error) {
      const errMessage = getErrorUserMessage(error, '提交失败，请稍后重试')
      logger.error('预订提交失败', error, 'reservation-confirm')
      wx.showToast({ title: errMessage || '提交失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
