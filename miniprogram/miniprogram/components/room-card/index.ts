// / <reference path="../../../typings/index.d.ts" />

Component({
  properties: {
    room: {
      type: Object,
      value: {}
    }
  },

  methods: {
    onTap() {
      if (this.data.room) {
        wx.navigateTo({
          url: `/pages/reservation/room-detail/index?id=${this.data.room.id}`
        })
      }
    }
  }
})
