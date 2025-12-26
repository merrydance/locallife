import { getRoomDetail, Room } from '../../../api/reservation'

Page({
  data: {
    roomId: '',
    room: null as Room | null,
    navBarHeight: 88,
    loading: false
  },

  onLoad(options: any) {
    if (options.id) {
      this.setData({ roomId: options.id })
      this.loadRoomDetail(options.id)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadRoomDetail(id: string) {
    this.setData({ loading: true })

    try {
      const room = await getRoomDetail(id)
      this.setData({
        room,
        loading: false
      })
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onBook() {
    const { room } = this.data
    if (room) {
      wx.navigateTo({
        url: `/pages/reservation/confirm/index?roomId=${room.id}&roomName=${room.name}&deposit=${room.deposit}`
      })
    }
  }
})
