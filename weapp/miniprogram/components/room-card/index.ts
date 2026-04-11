// / <reference path="../../../typings/index.d.ts" />

type RoomCardData = {
  id?: number
}

Component({
  properties: {
    room: {
      type: Object,
      value: {}
    }
  },

  methods: {
    onTap() {
      const room = this.data.room as RoomCardData
      if (room?.id) {
        wx.navigateTo({
          url: `/pages/reservation/room-detail/index?id=${room.id}`
        })
      }
    }
  }
})
