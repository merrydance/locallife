import { getToken } from './auth'
import { logger } from './logger'
import { EventBus } from './event-bus'

/**
 * WebSocket 消息类型定义
 */
export enum WSMessageType {
  NOTIFICATION = 'notification',
  PING = 'ping',
  PONG = 'pong',
  
  // 配送业务相关 (与后端 websocket/message_types.go 保持同步)
  DELIVERY_POOL_NEW = 'delivery_pool_new',   // 配送池新增订单
  DELIVERY_POOL_GONE = 'delivery_pool_gone', // 配送池订单被抢/移除
}

/**
 * 全局 WebSocket 管理类
 */
class WebSocketManager {
  private socket: WechatMiniprogram.SocketTask | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts = 0
  // 不设上限：只要 forcedClose=false 就持续重连（指数退避最大 30s）
  private readonly maxReconnectAttempts = Infinity
  private isConnected = false
  private forcedClose = false
  private isConnecting = false
  private readonly eventBus = new EventBus()

  /**
   * 建立连接
   * @param url WebSocket 地址，若不传则从配置获取
   */
  connect(url?: string) {
    const token = getToken()
    if (!token) {
      logger.warn('WebSocket: No token found, aborting connection', undefined, 'WS')
      return
    }

    if (this.socket || this.isConnected || this.isConnecting) {
      logger.info('WebSocket: Connection already exists or connecting', { 
        socket: !!this.socket, 
        isConnected: this.isConnected, 
        isConnecting: this.isConnecting 
      }, 'WS')
      return
    }

    this.isConnecting = true
    this.forcedClose = false
    const wsUrl = url || this.getDefaultUrl(token)

    logger.info(`WebSocket: Connecting to ${wsUrl.split('?')[0]}...`, undefined, 'WS')

    this.socket = wx.connectSocket({
      url: wsUrl,
      success: () => {
        logger.debug('WebSocket: Request sent successfully', undefined, 'WS')
      },
      fail: (err) => {
        this.isConnecting = false
        logger.error('WebSocket: Failed to request connection', err, 'WS')
        this.socket = null
        this.scheduleReconnect()
      }
    })

    this.setupListeners()
  }

  /**
   * 关闭连接
   */
  disconnect() {
    this.forcedClose = true
    if (this.socket) {
      this.socket.close({
        reason: 'Client logout'
      })
      this.socket = null
    }
    this.isConnected = false
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
    }
  }

  /**
   * 监听指定类型的消息
   * @returns 取消监听的函数
   */
  on(type: WSMessageType, callback: (data: unknown) => void) {
    this.eventBus.on(type, callback)
    return () => this.eventBus.off(type, callback)
  }

  private setupListeners() {
    if (!this.socket) return

    this.socket.onOpen(() => {
      logger.info('✅ WebSocket: Connected', undefined, 'WS')
      this.isConnected = true
      this.isConnecting = false
      this.reconnectAttempts = 0
    })

    this.socket.onMessage((res) => {
      try {
        const rawJson = typeof res.data === 'string' ? res.data : new TextDecoder().decode(res.data as ArrayBuffer)
        const msg = JSON.parse(rawJson)
        
        logger.debug('WebSocket: Received message', msg, 'WS')

        // 处理心跳
        if (msg.type === WSMessageType.PING) {
          this.send({ type: WSMessageType.PONG, timestamp: new Date().toISOString() })
          return
        }

        // 分发业务消息
        this.eventBus.emit(msg.type, msg.data)
      } catch (err) {
        logger.error('WebSocket: Failed to parse message', err, 'WS')
      }
    })

    this.socket.onClose((res) => {
      logger.warn('WebSocket: Connection closed', res, 'WS')
      this.isConnected = false
      this.socket = null
      if (!this.forcedClose) {
        this.scheduleReconnect()
      }
    })

    this.socket.onError((err) => {
      logger.error('WebSocket: Connection error', err, 'WS')
      this.isConnected = false
      this.isConnecting = false
      this.socket = null
    })
  }

  private scheduleReconnect() {
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
    this.reconnectAttempts++

    logger.info(`WebSocket: Reconnecting in ${delay}ms (Attempt ${this.reconnectAttempts})...`, undefined, 'WS')

    this.reconnectTimer = setTimeout(() => {
      this.connect()
    }, delay)
  }

  private send(data: Record<string, unknown>) {
    if (this.socket && this.isConnected) {
      this.socket.send({
        data: JSON.stringify(data)
      })
    }
  }

  private getDefaultUrl(token: string): string {
    const { API_CONFIG } = require('../config/index')
    const baseUrl = API_CONFIG.BASE_URL
    
    // 将 https:// 转换为 wss://, http:// 转换为 ws://
    const wsBase = baseUrl.replace(/^http/, 'ws')
    
    return `${wsBase}/v1/ws?token=${token}`
  }
}

export const wsManager = new WebSocketManager()
export default wsManager
