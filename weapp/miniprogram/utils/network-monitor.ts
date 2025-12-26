/**
 * 网络状态监控器
 * 监听网络变化、提供离线提示、重试机制
 */

import { logger } from './logger'

type NetworkType = 'wifi' | '2g' | '3g' | '4g' | '5g' | 'unknown' | 'none'

interface NetworkState {
    isConnected: boolean
    networkType: NetworkType
    isOfflineMode: boolean
}

class NetworkMonitor {
  private static instance: NetworkMonitor
  private networkState: NetworkState = {
    isConnected: true,
    networkType: 'unknown',
    isOfflineMode: false
  }
  private listeners: Set<(state: NetworkState) => void> = new Set()
  private offlineToastShown: boolean = false

  private constructor() {
    this.init()
  }

  static getInstance(): NetworkMonitor {
    if (!NetworkMonitor.instance) {
      NetworkMonitor.instance = new NetworkMonitor()
    }
    return NetworkMonitor.instance
  }

  /**
     * 初始化网络监控
     */
  private init(): void {
    // 获取初始网络状态
    this.checkNetworkStatus()

    // 监听网络状态变化
    wx.onNetworkStatusChange((res) => {
      const wasConnected = this.networkState.isConnected
            
      this.networkState = {
        isConnected: res.isConnected,
        networkType: res.networkType as NetworkType,
        isOfflineMode: !res.isConnected
      }

      logger.info('网络状态变化', this.networkState, 'NetworkMonitor')

      // 通知所有监听者
      this.notifyListeners()

      // 从离线恢复到在线
      if (!wasConnected && res.isConnected) {
        this.onNetworkRestore()
      }

      // 从在线变为离线
      if (wasConnected && !res.isConnected) {
        this.onNetworkLost()
      }
    })

    logger.info('网络监控已启动', this.networkState, 'NetworkMonitor')
  }

  /**
     * 检查当前网络状态
     */
  private checkNetworkStatus(): void {
    wx.getNetworkType({
      success: (res) => {
        const networkType = res.networkType as NetworkType
        this.networkState = {
          isConnected: networkType !== 'none',
          networkType,
          isOfflineMode: networkType === 'none'
        }
      },
      fail: () => {
        logger.warn('获取网络状态失败', undefined, 'NetworkMonitor')
      }
    })
  }

  /**
     * 网络恢复处理
     */
  private onNetworkRestore(): void {
    this.offlineToastShown = false
        
    wx.showToast({
      title: '网络已恢复',
      icon: 'success',
      duration: 2000
    })

    logger.info('网络已恢复', undefined, 'NetworkMonitor')

    // 可以在这里触发数据重新加载
    // eventBus.emit('network:restored')
  }

  /**
     * 网络断开处理
     */
  private onNetworkLost(): void {
    if (!this.offlineToastShown) {
      wx.showToast({
        title: '网络已断开',
        icon: 'none',
        duration: 3000
      })
      this.offlineToastShown = true
    }

    logger.warn('网络已断开', undefined, 'NetworkMonitor')

    // eventBus.emit('network:lost')
  }

  /**
     * 订阅网络状态变化
     */
  subscribe(listener: (state: NetworkState) => void): () => void {
    this.listeners.add(listener)
        
    // 立即通知当前状态
    listener(this.networkState)

    // 返回取消订阅函数
    return () => {
      this.listeners.delete(listener)
    }
  }

  /**
     * 通知所有监听者
     */
  private notifyListeners(): void {
    this.listeners.forEach((listener) => {
      try {
        listener(this.networkState)
      } catch (error) {
        logger.error('网络状态监听器执行失败', error, 'NetworkMonitor')
      }
    })
  }

  /**
     * 获取当前网络状态
     */
  getState(): Readonly<NetworkState> {
    return { ...this.networkState }
  }

  /**
     * 是否在线
     */
  isOnline(): boolean {
    return this.networkState.isConnected
  }

  /**
     * 是否是良好的网络(WiFi或4G/5G)
     */
  isGoodNetwork(): boolean {
    const { networkType, isConnected } = this.networkState
    return isConnected && ['wifi', '4g', '5g'].includes(networkType)
  }

  /**
     * 显示离线提示
     */
  showOfflineHint(message: string = '当前网络不可用'): void {
    wx.showModal({
      title: '网络异常',
      content: message,
      showCancel: false,
      confirmText: '我知道了'
    })
  }

  /**
     * 检查网络并执行操作
     */
  async checkAndExecute<T>(
    fn: () => Promise<T>,
    options: {
            offlineMessage?: string
            requireGoodNetwork?: boolean
        } = {}
  ): Promise<T> {
    if (!this.isOnline()) {
      this.showOfflineHint(options.offlineMessage)
      throw new Error('Network offline')
    }

    if (options.requireGoodNetwork && !this.isGoodNetwork()) {
      const proceed = await new Promise<boolean>((resolve) => {
        wx.showModal({
          title: '网络较差',
          content: '当前网络环境不佳,是否继续?',
          success: (res) => resolve(res.confirm),
          fail: () => resolve(false)
        })
      })

      if (!proceed) {
        throw new Error('User cancelled due to poor network')
      }
    }

    return fn()
  }
}

export const networkMonitor = NetworkMonitor.getInstance()
