import { deliveryFeeService } from '../../../api/delivery-fee'
import type { PeakHourConfigResponse, CreatePeakHourConfigRequest } from '../../../api/delivery-fee'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface PeakHourViewItem extends PeakHourConfigResponse {
  daysText: string
}

interface TimeslotPageOptions {
  region_id?: string
  region_name?: string
}

interface FieldInputEvent {
  detail: { value: string }
  currentTarget: { dataset: { field?: string } }
}

interface DaysChangeEvent {
  detail: { value: Array<string | number> }
}

function timeToMinutes(value: string): number {
  const [hours, minutes] = value.split(':').map((item) => parseInt(item, 10))
  return (hours * 60) + minutes
}

Page({
  data: {
    selectedRegionId: 0,
    selectedRegionName: '',

    initialLoading: true,
    loading: false,
    saving: false,
    error: '',
    navBarHeight: 0,

    peakConfigs: [] as PeakHourViewItem[],

    showPeakModal: false,
    peakStartTimePickerVisible: false,
    peakEndTimePickerVisible: false,
    peakForm: {
      startTime: '11:00',
      endTime: '13:00',
      coefficient: '1.50',
      days: [1, 2, 3, 4, 5] as number[]
    },

    daysOptions: [
      { value: 0, label: '日' },
      { value: 1, label: '一' },
      { value: 2, label: '二' },
      { value: 3, label: '三' },
      { value: 4, label: '四' },
      { value: 5, label: '五' },
      { value: 6, label: '六' }
    ]
  },

  onLoad(options: TimeslotPageOptions) {
    const selectedRegionId = Number(options?.region_id || 0)
    const selectedRegionName = options?.region_name ? decodeURIComponent(options.region_name) : ''

    if (!selectedRegionId) {
      wx.redirectTo({ url: '/pages/operator/region/index?target=rules' })
      return
    }

    this.setData({ selectedRegionId, selectedRegionName })
    this.loadPeakConfigs(selectedRegionId)
  },

  formatDays(days: number[]) {
    const map = ['日', '一', '二', '三', '四', '五', '六']
    return (days || []).map((d) => map[d] || '').filter(Boolean).join('、')
  },

  hasPeakConflict(startTime: string, endTime: string, days: number[]) {
    const nextStart = timeToMinutes(startTime)
    const nextEnd = timeToMinutes(endTime)

    return this.data.peakConfigs.find((item) => {
      const overlapDay = (item.days_of_week || []).some((day) => days.includes(day))
      if (!overlapDay) {
        return false
      }

      const currentStart = timeToMinutes(item.start_time)
      const currentEnd = timeToMinutes(item.end_time)
      return nextStart < currentEnd && nextEnd > currentStart
    })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadPeakConfigs(regionId: number) {
    this.setData({ loading: true, error: '' })
    try {
      const peakConfigs = await deliveryFeeService.getPeakConfigs(regionId)
      const mapped = peakConfigs.map((item) => ({
        ...item,
        daysText: this.formatDays(item.days_of_week)
      }))
      this.setData({ peakConfigs: mapped, loading: false, initialLoading: false })
    } catch (err: unknown) {
      const message = getErrorUserMessage(err, '加载时段配置失败，请稍后重试')
      this.setData({ loading: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    if (this.data.selectedRegionId > 0) {
      this.loadPeakConfigs(this.data.selectedRegionId)
      return
    }
    wx.redirectTo({ url: '/pages/operator/region/index?target=rules' })
  },

  onAddPeak() {
    this.setData({
      showPeakModal: true,
      peakStartTimePickerVisible: false,
      peakEndTimePickerVisible: false,
      peakForm: {
        startTime: '11:00',
        endTime: '13:00',
        coefficient: '1.50',
        days: [1, 2, 3, 4, 5]
      }
    })
  },

  onClosePeakModal() {
    this.setData({
      showPeakModal: false,
      peakStartTimePickerVisible: false,
      peakEndTimePickerVisible: false
    })
  },

  showPeakStartTimePicker() {
    this.setData({ peakStartTimePickerVisible: true })
  },

  hidePeakStartTimePicker() {
    this.setData({ peakStartTimePickerVisible: false })
  },

  showPeakEndTimePicker() {
    this.setData({ peakEndTimePickerVisible: true })
  },

  hidePeakEndTimePicker() {
    this.setData({ peakEndTimePickerVisible: false })
  },

  onPeakStartTimeConfirm(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({
      'peakForm.startTime': String(e.detail.value || ''),
      peakStartTimePickerVisible: false
    })
  },

  onPeakEndTimeConfirm(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({
      'peakForm.endTime': String(e.detail.value || ''),
      peakEndTimePickerVisible: false
    })
  },

  onPeakFormChange(e: FieldInputEvent) {
    const { field } = e.currentTarget.dataset
    if (!field) return
    this.setData({ [`peakForm.${field}`]: e.detail.value })
  },

  onDaysChange(e: DaysChangeEvent) {
  const values = e.detail?.value || []
  const nextDays = values
    .map((value) => parseInt(String(value), 10))
    .filter((value) => !isNaN(value) && value >= 0 && value <= 6)
    .sort((a, b) => a - b)

  this.setData({ 'peakForm.days': nextDays })
  },

  async onSavePeak() {
    const { selectedRegionId, peakForm, saving } = this.data
    if (saving) return
    if (!selectedRegionId) {
      wx.showToast({ title: '请先选择区县', icon: 'none' })
      return
    }

    const coefficient = parseFloat(peakForm.coefficient)
    if (!Number.isFinite(coefficient) || coefficient < 1) {
      wx.showToast({ title: '时段系数需不小于1', icon: 'none' })
      return
    }
    if (peakForm.startTime >= peakForm.endTime) {
      wx.showToast({ title: '结束时间需晚于开始时间', icon: 'none' })
      return
    }
    if (!peakForm.days.length) {
      wx.showToast({ title: '请至少选择一天', icon: 'none' })
      return
    }

    const conflictItem = this.hasPeakConflict(peakForm.startTime, peakForm.endTime, peakForm.days)
    if (conflictItem) {
      wx.showToast({
        title: `与 ${conflictItem.start_time}-${conflictItem.end_time} 冲突`,
        icon: 'none'
      })
      return
    }

    const data: CreatePeakHourConfigRequest = {
      region_id: selectedRegionId,
      start_time: peakForm.startTime,
      end_time: peakForm.endTime,
      coefficient,
      days_of_week: peakForm.days
    }

    try {
      this.setData({ saving: true })
      await deliveryFeeService.createPeakConfig(selectedRegionId, data)
      this.setData({ showPeakModal: false })
      await this.loadPeakConfigs(selectedRegionId)
    } catch (err) {
      wx.showToast({ title: getErrorUserMessage(err, '添加失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ saving: false })
    }
  },

  async onDeletePeak(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    wx.showModal({
      title: '删除确认',
      content: '确定删除该时段系数配置吗？',
      success: async (res) => {
        if (!res.confirm) return

        try {
          await deliveryFeeService.deletePeakConfig(id)
          await this.loadPeakConfigs(this.data.selectedRegionId)
        } catch (err) {
          wx.showToast({ title: getErrorUserMessage(err, '删除失败，请稍后重试'), icon: 'none' })
        }
      }
    })
  }
})
