import { globalStore } from '../../../../utils/global-store'
import {
  getCurrentRegionId,
  getLocalOperatorContactPhone,
  normalizeOperatorPhoneNumber
} from '../../../../utils/operator-contact'

type RegionSubscription = (() => void) | undefined

interface BaofuIntentHintInstance extends WechatMiniprogram.Component.TrivialInstance {
  _unsubscribeRegion?: RegionSubscription
  _loadedOperatorRegionId: number
  _requestedOperatorRegionId: number
  loadOperatorPhone(regionIdParam?: number): Promise<void>
}

function buildOperatorPhoneText(phone: string, loading = false): string {
  if (phone) {
    return phone
  }
  return loading ? '加载中...' : '暂无运营商电话'
}

Component({
  data: {
    operatorPhone: '',
    operatorPhoneText: buildOperatorPhoneText(''),
    loadingPhone: false
  },

  lifetimes: {
    attached() {
      const instance = this as unknown as BaofuIntentHintInstance
      instance._loadedOperatorRegionId = 0
      instance._requestedOperatorRegionId = 0
      void instance.loadOperatorPhone()
      instance._unsubscribeRegion = globalStore.subscribe('currentRegion', (region) => {
        const regionId = Number(region?.id || 0)
        if (regionId && regionId !== instance._loadedOperatorRegionId) {
          void instance.loadOperatorPhone(regionId)
        }
      })
    },

    detached() {
      const instance = this as unknown as BaofuIntentHintInstance
      if (instance._unsubscribeRegion) {
        instance._unsubscribeRegion()
        instance._unsubscribeRegion = undefined
      }
    }
  },

  methods: {
    async loadOperatorPhone(this: BaofuIntentHintInstance, regionIdParam?: number) {
      const regionId = Number(regionIdParam || getCurrentRegionId())
      if (!Number.isFinite(regionId) || regionId <= 0) {
        this.setData({
          operatorPhone: '',
          operatorPhoneText: buildOperatorPhoneText(''),
          loadingPhone: false
        })
        return
      }

      this._requestedOperatorRegionId = regionId
      if (regionId !== this._loadedOperatorRegionId) {
        this.setData({
          operatorPhone: '',
          operatorPhoneText: buildOperatorPhoneText('', true),
          loadingPhone: true
        })
      }

      try {
        const phone = await getLocalOperatorContactPhone(regionId)
        if (regionId !== this._requestedOperatorRegionId) {
          return
        }
        this._loadedOperatorRegionId = regionId
        this.setData({
          operatorPhone: phone,
          operatorPhoneText: buildOperatorPhoneText(phone),
          loadingPhone: false
        })
      } catch (_error) {
        if (regionId !== this._requestedOperatorRegionId) {
          return
        }
        this.setData({
          operatorPhone: '',
          operatorPhoneText: buildOperatorPhoneText(''),
          loadingPhone: false
        })
      }
    },

    onCallOperator() {
      const phoneNumber = normalizeOperatorPhoneNumber(this.data.operatorPhone)
      if (!phoneNumber) {
        wx.showToast({ title: '暂无运营商电话', icon: 'none' })
        return
      }

      wx.makePhoneCall({ phoneNumber })
    }
  }
})
