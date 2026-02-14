/**
 * 预约确认页面
 * 支持定金模式和全款模式
 */

import { formatPriceNoSymbol } from '@/utils/util'
import { createReservation, CreateReservationRequest } from '../../../api/reservation'
import { checkRoomAvailability } from '../../../api/room'
import { getMyMemberships, MembershipResponse } from '../../../api/personal'
import { createReservationPayment, invokeWechatPay } from '../../../api/payment'
import { calculateCart, CalculateCartResponse } from '../../../api/cart'

interface TimeSlot {
  time: string
  available: boolean
}

type PromotionItem = NonNullable<CalculateCartResponse['applied_promotions']>[number] & {
  amountDisplay: string
}

type LadderItem = NonNullable<CalculateCartResponse['ladder_promotions']>[number] & {
  thresholdDisplay: string
  discountDisplay: string
  missingNeedDisplay: string
}

type VoucherTrialItem = NonNullable<CalculateCartResponse['voucher_trials']>[number] & {
  amountDisplay: string
  trialPayableDisplay: string
}

type PaymentAssessmentItem = NonNullable<CalculateCartResponse['payment_assessment']>

interface ReservationCalculationView {
  subtotal: number
  discount_amount: number
  total_amount: number
  subtotalDisplay: string
  discountDisplay: string
  totalDisplay: string
  applied_promotions: PromotionItem[]
  ladder_promotions: LadderItem[]
  voucher_trials: VoucherTrialItem[]
  payment_assessment: PaymentAssessmentItem | null
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
    paymentMode: 'full' as 'deposit' | 'full',
    
    // 支付及会员相关
    selectedPaymentMethod: 'wechat', // 'wechat' | 'balance'
    memberBalance: 0,
    memberBalanceDisplay: '',
    membershipId: 0,

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
    loadingSlots: false,

    calculation: {
      subtotal: 0,
      subtotalDisplay: '0.00',
      discount_amount: 0,
      discountDisplay: '0.00',
      total_amount: 0,
      totalDisplay: '0.00',
      applied_promotions: [] as PromotionItem[],
      ladder_promotions: [] as LadderItem[],
      voucher_trials: [] as VoucherTrialItem[],
      payment_assessment: null as PaymentAssessmentItem | null
    } as ReservationCalculationView
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
      const depositNum = Number(options.deposit) || 10000
      this.setData({
        roomId: options.roomId,
        tableId: roomIdNum,
        merchantId: merchantIdNum,
        roomName: decodeURIComponent(options.roomName || ''),
        capacity: capacityNum,
        deposit: depositNum,
        depositDisplay: formatPriceNoSymbol(depositNum),
        'calculation.totalDisplay': formatPriceNoSymbol(depositNum)
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

    // 加载会员信息
    this.loadMemberships()
    // 初始化计算金额
    this.calculateAmount()
  },

  async loadMemberships() {
    const { merchantId } = this.data
    if (!merchantId) return

    try {
      const result = await getMyMemberships()
      const membership = result.memberships?.find(
        (m: MembershipResponse) => m.merchant_id === merchantId
      )
      if (membership) {
        this.setData({
          memberBalance: membership.balance,
          memberBalanceDisplay: formatPriceNoSymbol(membership.balance),
          membershipId: membership.id
        })
      }
    } catch (error) {
      console.error('[预订] 加载会员信息失败:', error)
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
      console.error('获取可用时段失败:', error)
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

  onPaymentModeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ paymentMode: e.detail.value })
    this.calculateAmount()
  },

  onSelectedPaymentMethodChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ selectedPaymentMethod: e.detail.value })
  },

  onRecharged() {
    this.loadMemberships()
  },

  onVoucherClaimed() {
    wx.showToast({ title: '领券完成', icon: 'success' })
    this.calculateAmount() // 领券后刷新金额
  },

  /**
   * 计算应付金额
   */
  async calculateAmount() {
    const { merchantId, paymentMode, deposit } = this.data
    if (!merchantId) return

    try {
      const params = {
        merchant_id: merchantId,
        order_type: 'reservation'
      }
      
      const result = await calculateCart(params)
      
      this.setData({
        calculation: {
          subtotal: paymentMode === 'deposit' ? deposit : result.subtotal,
          subtotalDisplay: formatPriceNoSymbol(paymentMode === 'deposit' ? deposit : result.subtotal),
          discount_amount: result.discount_amount,
          discountDisplay: formatPriceNoSymbol(result.discount_amount),
          total_amount: result.total_amount,
          totalDisplay: formatPriceNoSymbol(result.total_amount),
          applied_promotions: (result.applied_promotions || []).map((p) => ({
            ...p,
            amountDisplay: formatPriceNoSymbol(p.amount)
          })),
          ladder_promotions: (result.ladder_promotions || []).map((rule) => ({
            ...rule,
            thresholdDisplay: formatPriceNoSymbol(rule.threshold || 0),
            discountDisplay: formatPriceNoSymbol(rule.discount || 0),
            missingNeedDisplay: formatPriceNoSymbol(rule.missing_need || 0)
          })),
          voucher_trials: (result.voucher_trials || []).map((trial) => ({
            ...trial,
            amountDisplay: formatPriceNoSymbol(trial.amount || 0),
            trialPayableDisplay: formatPriceNoSymbol(trial.trial_payable || 0)
          })),
          payment_assessment: result.payment_assessment || null
        }
      })
    } catch (err) {
      console.error('计算金额失败:', err)
      this.setData({
        'calculation.totalDisplay': formatPriceNoSymbol(paymentMode === 'deposit' ? deposit : 0)
      })
    }
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
        // 定金模式：发起支付
        try {
          const paymentResult = await createReservationPayment(reservation.id)

          if (paymentResult.pay_params) {
            await invokeWechatPay(paymentResult.pay_params)
            wx.showToast({ title: '支付成功', icon: 'success' })
          } else if (paymentResult.status === 'paid') {
            wx.showToast({ title: '余额支付成功', icon: 'success' })
          }

          setTimeout(() => {
            wx.redirectTo({ url: `/pages/reservation/detail/index?id=${reservation.id}` })
          }, 1500)
          
        } catch (payErr) {
          console.error('[预订支付] 支付失败或取消:', payErr)
          wx.showToast({ title: '支付未完成', icon: 'none' })
          setTimeout(() => {
            wx.redirectTo({ url: `/pages/reservation/detail/index?id=${reservation.id}` })
          }, 1500)
        }
      }
    } catch (error) {
      const errMessage = error instanceof Error ? error.message : String(error)
      console.error('预定提交失败:', error)
      wx.showToast({ title: errMessage || '提交失败', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
