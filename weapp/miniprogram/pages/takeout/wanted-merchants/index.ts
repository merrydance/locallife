import {
  listWantedMerchants,
  submitWantedMerchant,
  voteWantedMerchant,
  type WantedMerchantItem
} from '../../../api/wanted-merchant'
import { getCurrentRegion } from '../../../api/location'
import Navigation from '../../../utils/navigation'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

interface WantedMerchantViewItem extends WantedMerchantItem {
  rankLabel: string
  rankClass: string
  highlight: boolean
  voting: boolean
  plusAnimating: boolean
}

function buildRankLabel(rank: number): string {
  if (rank === 1) return '冠'
  if (rank === 2) return '亚'
  if (rank === 3) return '季'
  return String(rank)
}

function buildRankClass(rank: number): string {
  if (rank <= 3) return `rank-badge rank-badge--top rank-badge--${rank}`
  if (rank <= 5) return 'rank-badge rank-badge--medal'
  return 'rank-badge'
}

function toViewItem(item: WantedMerchantItem, focusId = 0): WantedMerchantViewItem {
  return {
    ...item,
    rankLabel: buildRankLabel(item.rank),
    rankClass: buildRankClass(item.rank),
    highlight: item.id === focusId,
    voting: false,
    plusAnimating: false
  }
}

Page({
  data: {
    navBarHeight: 88,
    regionId: 0,
    regionName: '',
    loading: true,
    refreshing: false,
    error: false,
    errorMessage: '',
    items: [] as WantedMerchantViewItem[],
    total: 0,
    inputName: '',
    submitting: false,
    votingId: 0,
    scrollIntoView: '',
    pageReady: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    void this.initPage()
  },

  async initPage() {
    this.setData({ loading: true, error: false, errorMessage: '' })
    try {
      const region = await this.resolveRegion()
      this.setData({
        regionId: region.id,
        regionName: region.name,
        pageReady: true
      })
      await this.loadLeaderboard()
    } catch (error) {
      logger.warn('想吃榜初始化失败', error, 'WantedMerchants.initPage')
      this.setData({
        loading: false,
        error: true,
        errorMessage: '需要先获取当前位置'
      })
    }
  },

  async resolveRegion(): Promise<{ id: number, name: string }> {
    const app = getApp<IAppOption>()
    const cachedRegionId = Number(app.globalData.currentRegion?.id || 0)
    if (cachedRegionId > 0) {
      return {
        id: cachedRegionId,
        name: app.globalData.currentRegion?.name || '当前区县'
      }
    }

    const latitude = Number(app.globalData.latitude || 0)
    const longitude = Number(app.globalData.longitude || 0)
    if (!latitude || !longitude) {
      throw new Error('missing location')
    }

    const region = await getCurrentRegion({ latitude, longitude })
    const currentRegion = {
      id: Number(region.region_id || 0),
      name: region.region_name || '当前区县'
    }
    app.globalData.currentRegion = currentRegion
    const { globalStore } = require('../../../utils/global-store')
    globalStore.set('currentRegion', currentRegion)
    return currentRegion
  },

  async loadLeaderboard(focusId = 0, focusToast = '') {
    if (!this.data.regionId) return
    this.setData({ loading: this.data.items.length === 0, error: false, errorMessage: '' })
    try {
      const result = await listWantedMerchants({
        region_id: this.data.regionId,
        page_id: 1,
        page_size: 50
      })
      this.setData({
        items: result.items.map((item) => toViewItem(item, focusId)),
        total: result.total,
        loading: false
      })
      if (focusId) {
        this.focusWantedMerchant(focusId, focusToast)
      }
    } catch (error) {
      logger.warn('想吃榜加载失败', error, 'WantedMerchants.loadLeaderboard')
      this.setData({
        loading: false,
        error: true,
        errorMessage: '榜单加载失败'
      })
    }
  },

  onInputChange(e: WechatMiniprogram.CustomEvent) {
    const value = String(e.detail?.value ?? '')
    this.setData({ inputName: value })
  },

  onClearInput() {
    this.setData({ inputName: '' })
  },

  async onSubmitManual() {
    const name = this.data.inputName.trim()
    if (!name) {
      wx.showToast({ title: '先填店名', icon: 'none' })
      return
    }
    await this.submitCandidate({
      source: 'manual',
      name
    })
  },

  onPickFromMap() {
    wx.chooseLocation({
      success: (res) => {
        const name = (res.name || '').trim()
        if (!name) {
          wx.showToast({ title: '没有选到店名', icon: 'none' })
          return
        }
        void this.submitCandidate({
          source: 'map',
          name,
          address: res.address,
          latitude: res.latitude,
          longitude: res.longitude
        })
      },
      fail: (error) => {
        if (String(error?.errMsg || '').includes('cancel')) return
        wx.showToast({ title: '地图打开失败，请稍后再试', icon: 'none' })
      }
    })
  },

  async submitCandidate(params: {
    source: 'manual' | 'map'
    name: string
    address?: string
    latitude?: number
    longitude?: number
  }) {
    if (!this.data.regionId || this.data.submitting) return
    this.setData({ submitting: true })
    try {
      const result = await submitWantedMerchant({
        region_id: this.data.regionId,
        ...params
      })
      if (result.result === 'merchant_available' && result.merchant_id) {
        wx.showToast({ title: '这家已经能点外卖了', icon: 'none' })
        Navigation.toRestaurantDetail(result.merchant_id)
        return
      }
      if (result.result === 'found_in_rank' && result.wanted_merchant_id) {
        await this.loadLeaderboard(result.wanted_merchant_id, '已在榜单里，可以直接 +1')
        return
      }
      await this.loadLeaderboard(result.wanted_merchant_id || 0)
      wx.showToast({ title: '已加入想吃榜', icon: 'success' })
      this.setData({ inputName: '' })
    } catch (error) {
      logger.warn('提交想吃商户失败', error, 'WantedMerchants.submitCandidate')
      const userMessage = (error as { userMessage?: string })?.userMessage
      wx.showToast({ title: userMessage || '提交失败，请稍后再试', icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async onVote(e: WechatMiniprogram.CustomEvent) {
    const id = Number((e.currentTarget.dataset as { id?: number }).id || 0)
    if (!id || !this.data.regionId || this.data.votingId) return

    this.setRowState(id, { voting: true })
    this.setData({ votingId: id })
    try {
      const result = await voteWantedMerchant({
        id,
        region_id: this.data.regionId
      })
      if (result.result === 'already_voted') {
        wx.showToast({ title: '你已经 +1 过啦', icon: 'none' })
      } else {
        this.playPlusAnimation(id)
        wx.showToast({ title: '+1 成功', icon: 'success', duration: 700 })
      }
      await this.loadLeaderboard(id)
    } catch (error) {
      logger.warn('想吃榜 +1 失败', error, 'WantedMerchants.onVote')
      const userMessage = (error as { userMessage?: string })?.userMessage
      wx.showToast({ title: userMessage || '+1 失败，请稍后再试', icon: 'none' })
    } finally {
      this.setRowState(id, { voting: false })
      this.setData({ votingId: 0 })
    }
  },

  setRowState(id: number, patch: Partial<WantedMerchantViewItem>) {
    const index = this.data.items.findIndex((item) => item.id === id)
    if (index === -1) return
    const updates: Record<string, unknown> = {}
    Object.entries(patch).forEach(([key, value]) => {
      updates[`items[${index}].${key}`] = value
    })
    this.setData(updates)
  },

  focusWantedMerchant(id: number, title: string) {
    this.setData({ scrollIntoView: `wanted-${id}` })
    if (title) {
      wx.showToast({ title, icon: 'none' })
    }
    setTimeout(() => this.setRowState(id, { highlight: false }), 1600)
  },

  playPlusAnimation(id: number) {
    this.setRowState(id, { plusAnimating: true })
    setTimeout(() => this.setRowState(id, { plusAnimating: false }), 700)
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadLeaderboard()
    } finally {
      this.setData({ refreshing: false })
    }
  },

  onRetry() {
    void this.initPage()
  },

  onShareAppMessage() {
    return {
      title: '你想吃谁家外卖？来给想吃的店 +1',
      path: '/pages/takeout/wanted-merchants/index'
    }
  }
})
