import { getToken } from '../../../../utils/auth'
import { logger } from '../../../../utils/logger'
import { EventBus } from './event-bus'
import { mapBackendMessageToUserMessage } from '../../../../utils/user-facing'

/**
 * WebSocket 消息类型定义
 */
export enum WSMessageType {
  ALERT = 'alert',
  NOTIFICATION = 'notification',
  PING = 'ping',
  PONG = 'pong',
  CONNECTION_BLOCKED = 'connection_blocked',
  MERCHANT_STATUS_CHANGE = 'merchant_status_change',
  
  // 代取业务相关 (与后端 websocket/message_types.go 保持同步)
  DELIVERY_POOL_NEW = 'delivery_pool_new',   // 代取池新增订单
  DELIVERY_POOL_GONE = 'delivery_pool_gone', // 代取池订单被抢/移除
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
  private lastSequence = 0 // 记录最后收到的消息序号，断线重连时带给服务端以触发消息回放
  private readonly eventBus = new EventBus()

  private isConnectionBlocked(detail: string): boolean {
    const normalized = detail.toLowerCase()
    return (
      normalized.includes('merchant is closed') ||
      normalized.includes('403') ||
      normalized.includes('forbidden') ||
      normalized.includes('401') ||
      normalized.includes('unauthorized') ||
      normalized.includes('token')
    )
  }

  private buildConnectionBlockedMessage(detail: string): string {
    const normalized = detail.toLowerCase()

    if (normalized.includes('merchant is closed')) {
      return '当前门店已打烊，实时连接已暂停'
    }

    if (normalized.includes('403') || normalized.includes('forbidden')) {
      return '当前账号暂不可建立实时连接'
    }

    if (normalized.includes('401') || normalized.includes('unauthorized') || normalized.includes('token')) {
      return '登录状态已失效，请重新进入后再试'
    }

    return mapBackendMessageToUserMessage(detail, '实时连接暂不可用，请稍后再试')
  }

  private notifyConnectionBlocked(detail: string) {
    const message = this.buildConnectionBlockedMessage(detail)
    this.eventBus.emit(WSMessageType.CONNECTION_BLOCKED, {
      message,
      detail
    })
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private sendAck(messageId: string, sequence: number) {
    if (!messageId && sequence <= 0) {
      return
    }

    this.send({
      type: 'ack',
      message_id: messageId,
      sequence,
      ts: new Date().toISOString()
    })
  }

  /**
   * 建立连接
   * @param url WebSocket 地址，若不传则从配置获取
   */
  connect(urlOrPath?: string) {
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
    const wsUrl = this.resolveWebSocketUrl(token, urlOrPath, this.lastSequence)

    logger.info(`WebSocket: Connecting to ${wsUrl.split('?')[0]}...`, undefined, 'WS')

    let socketTask: WechatMiniprogram.SocketTask | null = null
    socketTask = wx.connectSocket({
      url: wsUrl,
      success: () => {
        if (this.socket !== socketTask) {
          return
        }
        logger.debug('WebSocket: Request sent successfully', undefined, 'WS')
      },
      fail: (err) => {
        if (this.socket !== socketTask) {
          return
        }
        this.isConnecting = false
        logger.error('WebSocket: Failed to request connection', err, 'WS')
        const errMsg = typeof err?.errMsg === 'string' ? err.errMsg : ''
        if (errMsg && this.isConnectionBlocked(errMsg)) {
          this.notifyConnectionBlocked(errMsg)
        }
        this.socket = null

        if (errMsg && this.isConnectionBlocked(errMsg)) {
          logger.warn('WebSocket: Connection blocked by server, skip auto reconnect', { errMsg }, 'WS')
          return
        }

        this.scheduleReconnect()
      }
    })

    this.socket = socketTask
    this.setupListeners(socketTask)
  }

  /**
   * 关闭连接
   */
  disconnect() {
    this.forcedClose = true
    this.isConnecting = false
    if (this.socket) {
      this.socket.close({
        reason: 'Client logout'
      })
      this.socket = null
    }
    this.isConnected = false
    this.clearReconnectTimer()
  }

  /**
   * 监听指定类型的消息
   * @returns 取消监听的函数
   */
  on(type: WSMessageType, callback: (data: unknown) => void) {
    this.eventBus.on(type, callback)
    return () => this.eventBus.off(type, callback)
  }

  private setupListeners(socketTask: WechatMiniprogram.SocketTask) {
    if (!socketTask) return

    socketTask.onOpen(() => {
      if (this.socket !== socketTask) {
        return
      }
      logger.info('✅ WebSocket: Connected', undefined, 'WS')
      this.isConnected = true
      this.isConnecting = false
      this.reconnectAttempts = 0
      this.clearReconnectTimer()
    })

    socketTask.onMessage((res) => {
      if (this.socket !== socketTask) {
        return
      }

      try {
        const rawJson = typeof res.data === 'string' ? res.data : new TextDecoder().decode(res.data as ArrayBuffer)
        const msg = JSON.parse(rawJson)
        
        logger.debug('WebSocket: Received message', msg, 'WS')

        // 处理心跳
        if (msg.type === WSMessageType.PING) {
          this.send({ type: WSMessageType.PONG, timestamp: new Date().toISOString() })
          return
        }

        // 追踪序号，断线重连时传给服务端触发消息回放
        if (typeof msg.sequence === 'number' && msg.sequence > this.lastSequence) {
          this.lastSequence = msg.sequence
        }

        const messageId = typeof msg.id === 'string' ? msg.id : ''
        const messageSequence = typeof msg.sequence === 'number' ? msg.sequence : 0
        if (msg.type !== WSMessageType.PONG && messageId) {
          this.sendAck(messageId, messageSequence)
        }

        // 分发业务消息
        this.eventBus.emit(msg.type, msg.data)
      } catch (err) {
        logger.error('WebSocket: Failed to parse message', err, 'WS')
      }
    })

    socketTask.onClose((res) => {
      if (this.socket !== socketTask) {
        return
      }

      logger.warn('WebSocket: Connection closed', res, 'WS')
      this.isConnected = false
      this.isConnecting = false
      this.socket = null
      if (!this.forcedClose) {
        this.scheduleReconnect()
      }
    })

    socketTask.onError((err) => {
      if (this.socket !== socketTask) {
        return
      }

      logger.error('WebSocket: Connection error', err, 'WS')
      const errMsg = typeof err?.errMsg === 'string' ? err.errMsg : ''
      if (errMsg && this.isConnectionBlocked(errMsg)) {
        this.notifyConnectionBlocked(errMsg)
      }
      this.isConnected = false
      this.isConnecting = false
      this.socket = null

      if (errMsg && this.isConnectionBlocked(errMsg)) {
        logger.warn('WebSocket: Connection blocked by server, skip auto reconnect', { errMsg }, 'WS')
        return
      }

      if (!this.forcedClose) {
        this.scheduleReconnect()
      }
    })
  }

  private scheduleReconnect() {
    if (this.forcedClose || this.reconnectTimer || this.isConnected || this.isConnecting) {
      return
    }

    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
    this.reconnectAttempts++

    logger.info(`WebSocket: Reconnecting in ${delay}ms (Attempt ${this.reconnectAttempts})...`, undefined, 'WS')

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
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

  private resolveWebSocketUrl(token: string, urlOrPath?: string, lastSequence = 0): string {
    const { API_CONFIG } = require('../../../../config/index')
    const baseUrl = API_CONFIG.BASE_URL
    const wsBase = baseUrl.replace(/^http/, 'ws')
    const seq = lastSequence > 0 ? `&last_sequence=${lastSequence}` : ''

    if (!urlOrPath) {
      return `${wsBase}/v1/ws?token=${encodeURIComponent(token)}${seq}`
    }

    const separator = urlOrPath.includes('?') ? '&' : '?'
    if (/^wss?:\/\//.test(urlOrPath)) {
      return `${urlOrPath}${separator}token=${encodeURIComponent(token)}${seq}`
    }

    const normalizedPath = urlOrPath.startsWith('/') ? urlOrPath : `/${urlOrPath}`
    return `${wsBase}${normalizedPath}${separator}token=${encodeURIComponent(token)}${seq}`
  }
}

export const wsManager = new WebSocketManager()
export default wsManager
