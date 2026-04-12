import {
  platformFinanceService,
  type PlatformSubMerchantLimitationRecoverySpecification,
  type PlatformSubMerchantLimitationsResponse
} from '@/api/platform-finance'
import { responsiveBehavior } from '@/utils/responsive'
import { getErrorUserMessage } from '@/utils/user-facing'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type InputEvent = WechatMiniprogram.CustomEvent<{ value?: string }>
type CopyTapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      value?: string
      label?: string
    }
  }
}

interface PlatformSubMerchantLimitationRecoverySpecificationView extends PlatformSubMerchantLimitationRecoverySpecification {
  relate_limitations: string[]
}

interface PlatformSubMerchantLimitationsView extends PlatformSubMerchantLimitationsResponse {
  limited_functions: string[]
  recovery_specifications: PlatformSubMerchantLimitationRecoverySpecificationView[]
  limitedFunctionCount: number
  recoveryCount: number
  statusTheme: 'success' | 'warning'
  statusLabel: string
}

function normalizeStringList(items?: string[]): string[] {
  if (!Array.isArray(items)) {
    return []
  }

  return items
    .map((item) => String(item || '').trim())
    .filter((item) => item.length > 0)
}

function toRecoverySpecificationView(
  item: PlatformSubMerchantLimitationRecoverySpecification
): PlatformSubMerchantLimitationRecoverySpecificationView {
  return {
    ...item,
    relate_limitations: normalizeStringList(item.relate_limitations)
  }
}

function toLimitationsView(result: PlatformSubMerchantLimitationsResponse): PlatformSubMerchantLimitationsView {
  const limitedFunctions = normalizeStringList(result.limited_functions)
  const recoverySpecifications = Array.isArray(result.recovery_specifications)
    ? result.recovery_specifications.map(toRecoverySpecificationView)
    : []
  const hasLimitations = limitedFunctions.length > 0 || recoverySpecifications.length > 0 || !!result.other_limited_functions

  return {
    ...result,
    limited_functions: limitedFunctions,
    recovery_specifications: recoverySpecifications,
    limitedFunctionCount: limitedFunctions.length,
    recoveryCount: recoverySpecifications.length,
    statusTheme: hasLimitations ? 'warning' : 'success',
    statusLabel: hasLimitations ? '存在限制' : '未见限制'
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    subMchId: '',
    loading: false,
    queried: false,
    error: null as string | null,
    result: null as PlatformSubMerchantLimitationsView | null
  },

  onLoad(options?: Record<string, string | undefined>) {
    const presetSubMchID = String(options?.sub_mch_id || '').trim()
    if (presetSubMchID) {
      this.setData({ subMchId: presetSubMchID })
    }
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  onSubMchIdChange(e: InputEvent) {
    this.setData({ subMchId: String(e?.detail?.value || '') })
  },

  async onSubmit() {
    const subMchId = String(this.data.subMchId || '').trim()
    if (!subMchId) {
      wx.showToast({ title: '请输入子商户号', icon: 'none' })
      return
    }

    try {
      this.setData({ loading: true, error: null, queried: true, result: null, subMchId })
      const response = await platformFinanceService.getSubMerchantLimitations(subMchId)
      this.setData({
        result: toLimitationsView(response),
        error: null
      })
    } catch (error: unknown) {
      this.setData({
        error: getErrorUserMessage(error, '查询失败，请稍后重试'),
        result: null
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onCopyTap(e: CopyTapEvent) {
    const value = String(e.currentTarget.dataset.value || '').trim()
    const label = String(e.currentTarget.dataset.label || '内容').trim()
    if (!value) {
      wx.showToast({ title: `${label}为空`, icon: 'none' })
      return
    }

    wx.setClipboardData({
      data: value,
      success: () => {
        wx.showToast({ title: `${label}已复制`, icon: 'none' })
      }
    })
  }
})