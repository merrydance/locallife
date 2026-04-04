import {
  getMyMerchantBusinessHours,
  MerchantBusinessHour,
  updateMyMerchantBusinessHours
} from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface BusinessHourSlot {
  key: string
  open_time: string
  close_time: string
}

interface WeeklyBusinessHour {
  day_of_week: number
  day_name: string
  is_closed: boolean
  slots: BusinessHourSlot[]
}

interface SpecialBusinessHour {
  key: string
  day_of_week: number
  day_name: string
  open_time: string
  close_time: string
  is_closed: boolean
  special_date: string
}

const DAY_LABELS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

function createSlot(openTime = '09:00', closeTime = '21:00', key?: string): BusinessHourSlot {
  return {
    key: key || `${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
    open_time: openTime,
    close_time: closeTime
  }
}

function createDefaultWeek(): WeeklyBusinessHour[] {
  return DAY_LABELS.map((dayName, dayOfWeek) => ({
    day_of_week: dayOfWeek,
    day_name: dayName,
    is_closed: true,
    slots: [createSlot()]
  }))
}

function sortSlots(slots: BusinessHourSlot[]) {
  return [...slots].sort((left, right) => left.open_time.localeCompare(right.open_time))
}

function normalizeBusinessHours(hours: MerchantBusinessHour[]) {
  const weekly = createDefaultWeek()
  const special: SpecialBusinessHour[] = []

  hours.forEach((hour, index) => {
    const dayName = hour.day_name || DAY_LABELS[hour.day_of_week] || `周${hour.day_of_week}`

    if (hour.special_date) {
      special.push({
        key: `${hour.special_date}-${hour.day_of_week}-${index}`,
        day_of_week: hour.day_of_week,
        day_name: dayName,
        open_time: hour.open_time,
        close_time: hour.close_time,
        is_closed: hour.is_closed,
        special_date: hour.special_date
      })
      return
    }

    const day = weekly[hour.day_of_week]
    if (!day) return

    if (hour.is_closed) {
      day.is_closed = true
      day.slots = [createSlot(hour.open_time || '09:00', hour.close_time || '21:00', `${hour.day_of_week}-closed`)]
      return
    }

    if (day.is_closed) {
      day.is_closed = false
      day.slots = []
    }

    day.slots.push(createSlot(hour.open_time, hour.close_time, `${hour.day_of_week}-${index}`))
  })

  weekly.forEach((day) => {
    if (!day.slots.length) {
      day.slots = [createSlot()]
    } else if (!day.is_closed) {
      day.slots = sortSlots(day.slots)
    }
  })

  return { weekly, special }
}

function cloneWeeklyHours(hours: WeeklyBusinessHour[]) {
  return hours.map((day) => ({
    ...day,
    slots: day.slots.map((slot) => ({ ...slot }))
  }))
}

function hasWeeklyHoursChanged(current: WeeklyBusinessHour[], initial: WeeklyBusinessHour[]) {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

function buildPayload(weekly: WeeklyBusinessHour[], special: SpecialBusinessHour[]) {
  const hours = weekly.reduce<Array<Pick<MerchantBusinessHour, 'day_of_week' | 'open_time' | 'close_time' | 'is_closed'>>>((accumulator, day) => {
    if (day.is_closed) {
      const firstSlot = day.slots[0] || createSlot('00:00', '00:00')
      accumulator.push({
        day_of_week: day.day_of_week,
        open_time: firstSlot.open_time,
        close_time: firstSlot.close_time,
        is_closed: true
      })
      return accumulator
    }

    sortSlots(day.slots).forEach((slot) => {
      accumulator.push({
        day_of_week: day.day_of_week,
        open_time: slot.open_time,
        close_time: slot.close_time,
        is_closed: false
      })
    })
    return accumulator
  }, [])

  return {
    hours: [
      ...hours,
      ...special.map((item) => ({
        day_of_week: item.day_of_week,
        open_time: item.open_time,
        close_time: item.close_time,
        is_closed: item.is_closed,
        special_date: item.special_date
      }))
    ]
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    actionNoticeMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    weeklyHours: createDefaultWeek() as WeeklyBusinessHour[],
    initialWeeklyHours: createDefaultWeek() as WeeklyBusinessHour[],
    specialHours: [] as SpecialBusinessHour[],
    hasChanges: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadBusinessHours()
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      this.loadBusinessHours(false)
    }
  },

  onPullDownRefresh() {
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      wx.showToast({ title: '当前有未保存修改，请先保存后再刷新', icon: 'none' })
      return
    }
    this.loadBusinessHours(false)
  },

  onRetryRefresh() {
    this.loadBusinessHours(false)
  },

  async loadBusinessHours(showLoading = true) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await getMyMerchantBusinessHours()
      const normalized = normalizeBusinessHours(response.hours || [])
      const weeklyHours = cloneWeeklyHours(normalized.weekly)
      this.setData({
        weeklyHours,
        initialWeeklyHours: cloneWeeklyHours(normalized.weekly),
        specialHours: normalized.special,
        actionNoticeMessage: '',
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load merchant business hours failed', err)
      const message = getErrorMessage(err, '营业时间加载失败，请重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onToggleClosed(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { day } = e.currentTarget.dataset as { day: number }
    const weeklyHours = cloneWeeklyHours(this.data.weeklyHours)
    const target = weeklyHours.find((item) => item.day_of_week === day)
    if (!target) return

    target.is_closed = e.detail.value
    if (!target.slots.length) {
      target.slots = [createSlot()]
    }

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      weeklyHours,
      hasChanges: hasWeeklyHoursChanged(weeklyHours, this.data.initialWeeklyHours)
    })
  },

  onTimeChange(e: WechatMiniprogram.PickerChange) {
    const { day, slotIndex, field } = e.currentTarget.dataset as {
      day: number
      slotIndex: number
      field: 'open_time' | 'close_time'
    }
    const weeklyHours = cloneWeeklyHours(this.data.weeklyHours)
    const target = weeklyHours.find((item) => item.day_of_week === day)
    if (!target || !target.slots[slotIndex]) return
    if (typeof e.detail.value !== 'string') return

    target.slots[slotIndex][field] = e.detail.value
    if (!target.is_closed) {
      target.slots = sortSlots(target.slots)
    }

    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      weeklyHours,
      hasChanges: hasWeeklyHoursChanged(weeklyHours, this.data.initialWeeklyHours)
    })
  },

  onAddSlot(e: WechatMiniprogram.TouchEvent) {
    const { day } = e.currentTarget.dataset as { day: number }
    const weeklyHours = cloneWeeklyHours(this.data.weeklyHours)
    const target = weeklyHours.find((item) => item.day_of_week === day)
    if (!target) return
    if (target.slots.length >= 3) {
      wx.showToast({ title: '单日最多配置 3 个时段', icon: 'none' })
      return
    }

    target.is_closed = false
    target.slots.push(createSlot())
    target.slots = sortSlots(target.slots)
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      weeklyHours,
      hasChanges: hasWeeklyHoursChanged(weeklyHours, this.data.initialWeeklyHours)
    })
  },

  onRemoveSlot(e: WechatMiniprogram.TouchEvent) {
    const { day, slotIndex } = e.currentTarget.dataset as { day: number, slotIndex: number }
    const weeklyHours = cloneWeeklyHours(this.data.weeklyHours)
    const target = weeklyHours.find((item) => item.day_of_week === day)
    if (!target || target.slots.length <= 1) return

    target.slots.splice(slotIndex, 1)
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      weeklyHours,
      hasChanges: hasWeeklyHoursChanged(weeklyHours, this.data.initialWeeklyHours)
    })
  },

  validateForm() {
    for (const day of this.data.weeklyHours) {
      if (day.is_closed) continue
      const slots = sortSlots(day.slots)
      if (!slots.length) {
        wx.showToast({ title: `${day.day_name} 至少保留 1 个营业时段`, icon: 'none' })
        return false
      }

      for (let index = 0; index < slots.length; index += 1) {
        const current = slots[index]
        if (current.open_time >= current.close_time) {
          wx.showToast({ title: `${day.day_name} 的开始时间需早于结束时间`, icon: 'none' })
          return false
        }

        const next = slots[index + 1]
        if (next && next.open_time < current.close_time) {
          wx.showToast({ title: `${day.day_name} 存在重叠营业时段`, icon: 'none' })
          return false
        }
      }
    }

    return true
  },

  async onSave() {
    if (this.data.saving || !this.data.hasChanges) return
    if (!this.validateForm()) return

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

    try {
      const response = await updateMyMerchantBusinessHours(buildPayload(this.data.weeklyHours, this.data.specialHours))
      const normalized = normalizeBusinessHours(response.hours || [])
      const weeklyHours = cloneWeeklyHours(normalized.weekly)
      this.setData({
        weeklyHours,
        initialWeeklyHours: cloneWeeklyHours(normalized.weekly),
        specialHours: normalized.special,
        actionNoticeMessage: '营业时间已保存并同步到门店设置。',
        hasChanges: false
      })
    } catch (err: unknown) {
      logger.error('Save merchant business hours failed', err)
      const message = getErrorMessage(err, '保存失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  onRetry() {
    this.loadBusinessHours()
  }
})