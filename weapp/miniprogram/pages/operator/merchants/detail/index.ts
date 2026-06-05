import {
  loadOperatorMerchantCapabilitiesView,
  loadOperatorMerchantDetailView,
  loadOperatorMerchantStatsView,
  submitOperatorMerchantCapabilities,
  type OperatorMerchantCapabilitiesView,
  type OperatorMerchantCapabilityFormData,
  type OperatorMerchantDetailView,
  type OperatorMerchantStatsView
} from '../../_services/operator-merchant-management'
import type { MerchantCapabilityStatus } from '../../_api/operator-merchant-management'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type CapabilityFormField = 'open_kitchen_status' | 'dine_in_status'

interface CapabilityFieldDataset {
  field?: CapabilityFormField
}

const OPEN_KITCHEN_OPTIONS: Array<{ value: MerchantCapabilityStatus, label: string }> = [
  { value: 'unknown', label: '未确认' },
  { value: 'yes', label: '有明厨亮灶' },
  { value: 'no', label: '无明厨亮灶' }
]

const DINE_IN_OPTIONS: Array<{ value: MerchantCapabilityStatus, label: string }> = [
  { value: 'unknown', label: '未确认' },
  { value: 'yes', label: '支持堂食' },
  { value: 'no', label: '不支持堂食' }
]

function buildCapabilityForm(capabilities?: OperatorMerchantCapabilitiesView | null): OperatorMerchantCapabilityFormData {
  return {
    open_kitchen_status: capabilities?.open_kitchen_status || 'unknown',
    dine_in_status: capabilities?.dine_in_status || 'unknown',
    note: capabilities?.note || ''
  }
}

Page({
  data: {
    id: 0,
    loading: true,
    statsLoading: false,
    capabilityLoading: false,
    capabilitySubmitting: false,
    error: '',
    capabilityError: '',
    navBarHeight: 88,
    detail: null as OperatorMerchantDetailView | null,
    stats: null as OperatorMerchantStatsView | null,
    capabilities: null as OperatorMerchantCapabilitiesView | null,
    capabilityEditorVisible: false,
    capabilityForm: buildCapabilityForm(),
    openKitchenOptions: OPEN_KITCHEN_OPTIONS,
    dineInOptions: DINE_IN_OPTIONS
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '商户ID无效' })
      return
    }
    this.setData({ id })
    this.loadAll()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadAll() {
    if (!this.data.id) return
    this.setData({ loading: true, error: '', stats: null, capabilities: null, capabilityError: '' })
    try {
      this.setData({ detail: await loadOperatorMerchantDetailView(this.data.id), loading: false })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载商户详情失败，请稍后重试')
      this.setData({ loading: false, error: message })
      return
    }

    this.loadCapabilities()

    // 加载经营统计
    this.setData({ statsLoading: true })
    try {
      this.setData({ stats: await loadOperatorMerchantStatsView(this.data.id, 30) })
    } catch {
      // 统计加载失败不阻断主流程
    } finally {
      this.setData({ statsLoading: false })
    }
  },

  async loadCapabilities() {
    if (!this.data.id) return
    this.setData({ capabilityLoading: true, capabilityError: '' })
    try {
      const capabilities = await loadOperatorMerchantCapabilitiesView(this.data.id)
      this.setData({
        capabilities,
        capabilityForm: this.data.capabilityEditorVisible ? this.data.capabilityForm : buildCapabilityForm(capabilities)
      })
    } catch (error: unknown) {
      this.setData({
        capabilityError: getErrorUserMessage(error, '经营能力加载失败，请稍后重试')
      })
    } finally {
      this.setData({ capabilityLoading: false })
    }
  },

  onRetry() {
    this.loadAll()
  },

  onRetryCapabilities() {
    this.loadCapabilities()
  },

  onOpenCapabilityEditor() {
    if (this.data.capabilityLoading || this.data.capabilityError || !this.data.capabilities) {
      wx.showToast({ title: '请先完成经营能力加载', icon: 'none' })
      return
    }
    this.setData({
      capabilityEditorVisible: true,
      capabilityForm: buildCapabilityForm(this.data.capabilities)
    })
  },

  onCapabilityEditorVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible: boolean }>) {
    if (!e.detail.visible) {
      this.onCloseCapabilityEditor()
    }
  },

  onCloseCapabilityEditor() {
    if (this.data.capabilitySubmitting) return
    this.setData({ capabilityEditorVisible: false })
  },

  onCapabilityStatusChange(e: WechatMiniprogram.CustomEvent<{ value: MerchantCapabilityStatus }>) {
    const { field } = e.currentTarget.dataset as CapabilityFieldDataset
    if (!field) return
    this.setData({
      [`capabilityForm.${field}`]: e.detail.value || 'unknown'
    })
  },

  onCapabilityNoteChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({
      'capabilityForm.note': e.detail.value || ''
    })
  },

  async onSubmitCapabilityEditor() {
    if (this.data.capabilitySubmitting || !this.data.id || !this.data.capabilities) return
    this.setData({ capabilitySubmitting: true })
    try {
      const capabilities = await submitOperatorMerchantCapabilities(this.data.id, this.data.capabilityForm)
      this.setData({
        capabilities,
        capabilityForm: buildCapabilityForm(capabilities),
        capabilityEditorVisible: false,
        capabilityError: ''
      })
      wx.showToast({ title: '经营能力已更新', icon: 'success' })
    } catch (error: unknown) {
      wx.showToast({ title: getErrorUserMessage(error, '经营能力保存失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ capabilitySubmitting: false })
    }
  }
})
