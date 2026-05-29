// / <reference path="../../../typings/index.d.ts" />

import { globalStore } from '../../utils/global-store'
import { logger } from '../../utils/logger'

type NavbarInstance = WechatMiniprogram.Component.InstanceMethods<WechatMiniprogram.IAnyObject> & {
  _unsubscribe?: () => void
}

Component({
  properties: {
    title: {
      type: String,
      value: 'LocalLife'
    },
    location: {
      type: String,
      optionalTypes: [String, null],
      value: ''
    },
    showBack: {
      type: Boolean,
      value: false
    },
    showHome: {
      type: Boolean,
      value: false
    },
    showLocation: {
      type: Boolean,
      value: true
    }
  },

  data: {
    statusBarHeight: 0,
    navBarHeight: 0,
    navBarContentHeight: 44,
    capsuleWidth: 87,
    displayLocation: '', // 内部管理的location显示值
    innerShowHome: false // 内部计算的首页按钮显示状态
  },

  lifetimes: {
    attached() {
      this.initNavBar()
      this.subscribeToLocationChanges()
    },

    detached() {
      // 取消订阅
      const navbar = this as unknown as NavbarInstance
      if (navbar._unsubscribe) {
        navbar._unsubscribe()
      }
    }
  },

  methods: {
    initNavBar() {
      // 获取稳定的高度计算逻辑
      const { getStableBarHeights } = require('../../utils/responsive')
      const { statusBarHeight, navBarContentHeight, navBarHeight } = getStableBarHeights()

      const menuButton = wx.getMenuButtonBoundingClientRect()
      const windowInfo = wx.getWindowInfo()
      const capsuleWidth = windowInfo.screenWidth - menuButton.left

      const pages = getCurrentPages()
      const isHomePage = pages.length > 0 && pages[pages.length - 1].route === 'pages/takeout/index'
      
      this.setData({
        statusBarHeight,
        navBarHeight,
        navBarContentHeight,
        capsuleWidth,
        innerShowHome: this.properties.showHome || !isHomePage
      })

      // 同步到全局状态
      globalStore.set('navBarHeight', navBarHeight)

      // 通知父页面
      this.triggerEvent('navheight', { navBarHeight })

      // 初始化location显示
      this.updateLocationDisplay()
    },

    /**
         * 订阅全局location变化
         */
    subscribeToLocationChanges() {
      const navbar = this as unknown as NavbarInstance
      navbar._unsubscribe = globalStore.subscribe('location', (newLocation: unknown) => {
        const locationData =
          typeof newLocation === 'object' && newLocation !== null
            ? (newLocation as { name?: string })
            : {}

        // 只在必要时更新 - 如果properties没传location,才使用全局的
        if (!this.properties.location) {
          this.setData({ displayLocation: locationData.name || '定位中...' })
        }
      })
    },

    /**
         * 更新location显示
         */
    updateLocationDisplay() {
      // 优先使用properties传入的location
      if (this.properties.location) {
        this.setData({ displayLocation: this.properties.location })
      } else {
        // 否则使用全局location
        const location = globalStore.get('location')
        const displayText = location.name || '定位中...'
        this.setData({ displayLocation: displayText })
      }
    },

    onLocationTap() {
      this.openChooseLocation()
    },

    /**
     * 打开位置选择器（兜底方案）
     */
    openChooseLocation() {
      wx.chooseLocation({
        success: async (res) => {
          try {
            // 使用后端逆地理编码接口获取详细地址
            const { locationService } = require('../../utils/location')
            const locationInfo = await locationService.reverseGeocode(res.latitude, res.longitude)

            const name = res.name || locationInfo.street || locationInfo.district || '选择的位置'

            // 更新全局状态
            const app = getApp<IAppOption>()
            app.globalData.latitude = res.latitude
            app.globalData.longitude = res.longitude
            app.globalData.location = {
              name,
              address: res.address || locationInfo.address
            }

            // 使用GlobalStore统一管理
            globalStore.updateLocation(
              res.latitude,
              res.longitude,
              name,
              res.address || locationInfo.address
            )

            // 局部更新显示
            this.setData({ displayLocation: name })

            // 通知父页面
            this.triggerEvent('locationchange', {
              latitude: res.latitude,
              longitude: res.longitude,
              name,
              address: res.address || locationInfo.address
            })

            wx.showToast({ title: '位置已更新', icon: 'success', duration: 1500 })
          } catch (err) {
            logger.warn('选择位置后逆地理编码失败，使用微信返回位置', err, 'CustomNavbar.openChooseLocation')
            // 逆地理编码失败，使用用户选择的信息
            const name = res.name || '选择的位置'

            const app = getApp<IAppOption>()
            app.globalData.latitude = res.latitude
            app.globalData.longitude = res.longitude
            app.globalData.location = {
              name,
              address: res.address
            }

            globalStore.updateLocation(
              res.latitude,
              res.longitude,
              name,
              res.address
            )
            this.setData({ displayLocation: name })
            this.triggerEvent('locationchange', res)
          }
        },
        fail: () => {
          // 用户取消选择，恢复之前的显示
          this.updateLocationDisplay()
        }
      })
    },

    onBackTap() {
      const pages = getCurrentPages()
      if (pages.length > 1) {
        wx.navigateBack({
          delta: 1
        })
      } else {
        wx.switchTab({
          url: '/pages/takeout/index'
        })
      }
    },

    onHomeTap() {
      wx.switchTab({
        url: '/pages/takeout/index'
      })
    }
  },

  /**
     * 监听properties变化
     */
  observers: {
    'location'(newLocation: string) {
      if (newLocation) {
        this.setData({ displayLocation: newLocation })
      }
    },
    'showHome'(showHome: boolean) {
      const pages = getCurrentPages()
      const isHomePage = pages.length > 0 && pages[pages.length - 1].route === 'pages/takeout/index'
      this.setData({
        innerShowHome: showHome || !isHomePage
      })
    }
  }
})
