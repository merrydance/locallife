// / <reference path="../../../typings/index.d.ts" />

import { globalStore } from '../../utils/global-store'

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
    displayLocation: '' // 内部管理的location显示值
  },

  lifetimes: {
    attached() {
      this.initNavBar()
      this.subscribeToLocationChanges()
    },

    detached() {
      // 取消订阅
      if ((this as any)._unsubscribe) {
        (this as any)._unsubscribe()
      }
    }
  },

  methods: {
    initNavBar() {
      // 获取稳定的高度计算逻辑
      const { getStableBarHeights, isLargeScreen } = require('../../utils/responsive')
      const { statusBarHeight, navBarContentHeight, navBarHeight } = getStableBarHeights()

      const menuButton = wx.getMenuButtonBoundingClientRect()
      const windowInfo = wx.getWindowInfo()
      const capsuleWidth = windowInfo.screenWidth - menuButton.left

      this.setData({
        statusBarHeight,
        navBarHeight,
        navBarContentHeight,
        capsuleWidth
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
      (this as any)._unsubscribe = globalStore.subscribe('location', (newLocation) => {
        console.log('[Navbar] 收到位置更新通知', {
          newLocation,
          hasPropertiesLocation: !!this.properties.location,
          willUpdate: !this.properties.location
        })

        // 只在必要时更新 - 如果properties没传location,才使用全局的
        if (!this.properties.location) {
          this.setData({ displayLocation: newLocation.name || '定位中...' })
          console.log('[Navbar] 已更新 displayLocation:', newLocation.name || '定位中...')
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
        console.log('[Navbar] 使用 properties 传入的位置:', this.properties.location)
      } else {
        // 否则使用全局location
        const location = globalStore.get('location')
        const displayText = location.name || '定位中...'
        this.setData({ displayLocation: displayText })
        console.log('[Navbar] 从 globalStore 读取位置:', location, '显示:', displayText)
      }
    },

    onLocationTap() {
      // 如果当前显示"定位失败"，则调用chooseLocation让用户手动选择
      if (this.data.displayLocation === '定位失败') {
        this.openChooseLocation()
        return
      }

      // 否则重新获取当前位置
      this.setData({ displayLocation: '正在定位...' })

      wx.getLocation({
        type: 'gcj02',
        success: async (res) => {
          try {
            // 使用后端逆地理编码接口
            const { locationService } = require('../../utils/location')
            const locationInfo = await locationService.reverseGeocode(res.latitude, res.longitude)

            const name = locationInfo.street || locationInfo.district || locationInfo.address || '当前位置'

            // 更新全局状态
            const app = getApp<IAppOption>()
            app.globalData.latitude = res.latitude
            app.globalData.longitude = res.longitude
            app.globalData.location = {
              name,
              address: locationInfo.address
            }

            // 使用GlobalStore统一管理,自动同步到所有监听者
            globalStore.updateLocation(
              res.latitude,
              res.longitude,
              name,
              locationInfo.address
            )

            // 局部更新显示
            this.setData({ displayLocation: name })

            // 通知父页面
            this.triggerEvent('locationchange', {
              latitude: res.latitude,
              longitude: res.longitude,
              name,
              address: locationInfo.address
            })

            wx.showToast({ title: '位置已更新', icon: 'success', duration: 1500 })
          } catch (err) {
            // 逆地理编码失败，使用坐标
            const displayLocation = `${res.latitude.toFixed(4)}, ${res.longitude.toFixed(4)}`

            const app = getApp<IAppOption>()
            app.globalData.latitude = res.latitude
            app.globalData.longitude = res.longitude
            app.globalData.location = {
              name: displayLocation,
              address: displayLocation
            }

            globalStore.updateLocation(
              res.latitude,
              res.longitude,
              displayLocation,
              displayLocation
            )
            this.setData({ displayLocation })
            this.triggerEvent('locationchange', res)
          }
        },
        fail: (err) => {
          // getLocation失败，显示"定位失败"让用户点击重试（会触发chooseLocation）
          this.setData({ displayLocation: '定位失败' })

          // 更新全局状态
          const app = getApp<IAppOption>()
          app.globalData.location = { name: '定位失败' }

          if (err.errMsg.includes('auth deny')) {
            wx.showModal({
              title: '需要位置权限',
              content: '请在设置中开启位置权限',
              confirmText: '去设置',
              success: (modalRes) => {
                if (modalRes.confirm) {
                  wx.openSetting()
                }
              }
            })
          }
        }
      })
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
    }
  }
})
