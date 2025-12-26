/**
 * 虚拟列表组件
 * 适用于100+项长列表场景,仅渲染可见区域+缓冲区的元素
 * 大幅减少DOM节点数,提升滚动性能
 */

import { logger } from '../../utils/logger'

interface VirtualListItem {
  id: string | number
  height?: number // 可选的固定高度
  [key: string]: unknown
}

interface VirtualListData {
  // 原始数据
  allItems: VirtualListItem[]
  // 当前渲染的数据
  visibleItems: VirtualListItem[]
  // 滚动偏移量
  scrollTop: number
  // 容器高度
  containerHeight: number
  // 每项高度(固定模式)
  itemHeight: number
  // 上方填充高度
  offsetTop: number
  // 下方填充高度
  offsetBottom: number
  // 缓冲区数量(上下各缓冲几个)
  bufferSize: number
  // 开始索引
  startIndex: number
  // 结束索引
  endIndex: number
}

Component({
  options: {
    multipleSlots: true
  },

  properties: {
    // 完整数据列表
    items: {
      type: Array,
      value: []
    },
    // 每项固定高度(rpx)
    itemHeight: {
      type: Number,
      value: 150
    },
    // 缓冲区大小(上下各缓冲几项)
    bufferSize: {
      type: Number,
      value: 5
    },
    // 唯一标识字段名
    uniqueKey: {
      type: String,
      value: 'id'
    }
  },

  data: {
    allItems: [] as VirtualListItem[],
    visibleItems: [] as VirtualListItem[],
    scrollTop: 0,
    containerHeight: 0,
    offsetTop: 0,
    offsetBottom: 0,
    startIndex: 0,
    endIndex: 0
  } as VirtualListData,

  lifetimes: {
    attached() {
      this.initContainer()
    },

    ready() {
      // 初始化数据
      if (this.data.items && this.data.items.length > 0) {
        this.updateVirtualList()
      }
    }
  },

  observers: {
    items(newItems: VirtualListItem[]) {
      logger.debug('虚拟列表数据更新', { count: newItems.length }, 'VirtualList')
      this.setData({ allItems: newItems })
      this.updateVirtualList()
    }
  },

  methods: {
    /**
     * 初始化容器高度
     */
    initContainer() {
      try {
        const query = this.createSelectorQuery()
        query.select('.virtual-list-container').boundingClientRect()
        query.exec((res) => {
          if (res && res[0]) {
            const containerHeight = res[0].height
            this.setData({ containerHeight })
            logger.debug('虚拟列表容器初始化', { containerHeight }, 'VirtualList')
            this.updateVirtualList()
          }
        })

        // 如果容器查询失败,使用屏幕高度作为fallback
        setTimeout(() => {
          if (this.data.containerHeight === 0) {
            const windowInfo = wx.getWindowInfo()
            this.setData({ containerHeight: windowInfo.windowHeight })
            this.updateVirtualList()
          }
        }, 300)
      } catch (e) {
        logger.error('初始化虚拟列表容器失败', e, 'VirtualList')
      }
    },

    /**
     * 滚动事件处理(节流)
     */
    onScroll(e: WechatMiniprogram.CustomEvent) {
      const scrollTop = e.detail.scrollTop

      // 节流: 滚动距离小于半个item高度时不更新
      const threshold = this.data.itemHeight / 2
      if (Math.abs(scrollTop - this.data.scrollTop) < threshold) {
        return
      }

      this.setData({ scrollTop })
      this.updateVirtualList()
    },

    /**
     * 触底事件(向上传递)
     */
    onScrollToLower() {
      this.triggerEvent('scrolltolower')
    },

    /**
     * 更新虚拟列表(核心算法)
     */
    updateVirtualList() {
      const {
        allItems,
        scrollTop,
        containerHeight,
        itemHeight,
        bufferSize
      } = this.data

      if (allItems.length === 0 || containerHeight === 0) {
        return
      }

      // 转换rpx到px (假设设计稿750rpx)
      const windowInfo = wx.getWindowInfo()
      const rpxRatio = windowInfo.windowWidth / 750
      const itemHeightPx = itemHeight * rpxRatio

      // 计算可见区域的开始和结束索引
      const visibleStart = Math.floor(scrollTop / itemHeightPx)
      const visibleEnd = Math.ceil((scrollTop + containerHeight) / itemHeightPx)

      // 加上缓冲区
      const startIndex = Math.max(0, visibleStart - bufferSize)
      const endIndex = Math.min(allItems.length, visibleEnd + bufferSize)

      // 计算上下填充高度
      const offsetTop = startIndex * itemHeightPx
      const offsetBottom = (allItems.length - endIndex) * itemHeightPx

      // 提取可见项
      const visibleItems = allItems.slice(startIndex, endIndex)

      logger.debug('虚拟列表更新', {
        total: allItems.length,
        visible: visibleItems.length,
        startIndex,
        endIndex,
        scrollTop,
        offsetTop,
        offsetBottom
      }, 'VirtualList')

      this.setData({
        visibleItems,
        startIndex,
        endIndex,
        offsetTop,
        offsetBottom
      })
    },

    /**
     * 滚动到指定索引
     */
    scrollToIndex(index: number) {
      const windowInfo = wx.getWindowInfo()
      const rpxRatio = windowInfo.windowWidth / 750
      const itemHeightPx = this.data.itemHeight * rpxRatio

      const scrollTop = index * itemHeightPx

      this.setData({ scrollTop })
      this.updateVirtualList()
    },

    /**
     * 获取当前可见范围
     */
    getVisibleRange() {
      return {
        startIndex: this.data.startIndex,
        endIndex: this.data.endIndex,
        visibleCount: this.data.visibleItems.length,
        totalCount: this.data.allItems.length
      }
    }
  }
})
